// Package store reads and writes application data in PostgreSQL using the pgx driver.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nick-bellows/learning-center-reference/api/internal/credentials"
	"github.com/nick-bellows/learning-center-reference/api/internal/learning"
	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// ErrForbidden means the authenticated member does not own the requested resource.
var ErrForbidden = errors.New("forbidden")

// ErrOutOfOrder means a sequential course has an unfinished earlier lesson.
var ErrOutOfOrder = errors.New("complete earlier lessons first")

// Store owns the application's PostgreSQL connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New opens the PostgreSQL pool.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() { s.pool.Close() }

// Pool exposes the underlying pool for setup tasks (migrations, seeding).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Ping verifies the database is reachable — what /health reports on.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// ResolveMemberBySubject maps a verified identity-provider subject to the local member
// and database-managed roles. Authorization never trusts roles supplied by a token.
func (s *Store) ResolveMemberBySubject(ctx context.Context, subject string) (learning.Member, error) {
	var member learning.Member
	err := s.pool.QueryRow(ctx, `
		select m.id::text, m.display_name,
		       coalesce(array_agg(mr.role order by mr.role)
		           filter (where mr.role is not null), '{}')
		from member m
		left join member_role mr on mr.member_id = m.id
		where m.auth_subject = $1
		group by m.id, m.display_name`, subject).
		Scan(&member.ID, &member.DisplayName, &member.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return member, ErrNotFound
	}
	if err != nil {
		return member, fmt.Errorf("resolving member: %w", err)
	}
	return member, nil
}

// ListPublishedCourses returns only catalog entries learners may enroll in.
func (s *Store) ListPublishedCourses(ctx context.Context) ([]learning.CourseSummary, error) {
	rows, err := s.pool.Query(ctx, `
		select c.id::text, c.title, c.slug, c.ordering, count(l.id)::int
		from course c
		left join module m on m.course_id = c.id
		left join lesson l on l.module_id = m.id
		where c.status = 'published'
		group by c.id, c.title, c.slug, c.ordering
		order by c.title`)
	if err != nil {
		return nil, fmt.Errorf("listing courses: %w", err)
	}
	defer rows.Close()

	courses := make([]learning.CourseSummary, 0)
	for rows.Next() {
		var course learning.CourseSummary
		if err := rows.Scan(&course.ID, &course.Title, &course.Slug, &course.Ordering, &course.LessonCount); err != nil {
			return nil, fmt.Errorf("scanning course: %w", err)
		}
		courses = append(courses, course)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating courses: %w", err)
	}
	return courses, nil
}

// Enroll creates at most one enrollment per member/course and initializes its read
// projection. Retrying the same request is safe and returns the existing enrollment.
func (s *Store) Enroll(ctx context.Context, memberID, courseID string) (learning.EnrollmentProgress, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("beginning enrollment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lessonCount int
	if err := tx.QueryRow(ctx, `
		select count(l.id)::int
		from course c
		left join module m on m.course_id = c.id
		left join lesson l on l.module_id = m.id
		where c.id = $1::uuid and c.status = 'published'
		group by c.id`, courseID).Scan(&lessonCount); errors.Is(err, pgx.ErrNoRows) {
		return learning.EnrollmentProgress{}, false, ErrNotFound
	} else if err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("loading course: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		insert into enrollment (member_id, course_id)
		values ($1::uuid, $2::uuid)
		on conflict (member_id, course_id) do nothing`, memberID, courseID)
	if err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("creating enrollment: %w", err)
	}
	created := tag.RowsAffected() == 1

	var enrollmentID string
	if err := tx.QueryRow(ctx, `
		select id::text from enrollment
		where member_id = $1::uuid and course_id = $2::uuid`, memberID, courseID).
		Scan(&enrollmentID); err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("loading enrollment: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into enrollment_progress (enrollment_id, total_lessons)
		values ($1::uuid, $2)
		on conflict (enrollment_id) do nothing`, enrollmentID, lessonCount); err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("initializing progress: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("committing enrollment: %w", err)
	}

	progress, err := s.loadEnrollmentProgress(ctx, memberID, enrollmentID)
	return progress, created, err
}

// CompleteLesson appends one immutable completion event and refreshes the projection
// in the same transaction. The uniqueness constraint makes retries idempotent.
func (s *Store) CompleteLesson(ctx context.Context, memberID, enrollmentID, lessonID string) (learning.EnrollmentProgress, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("beginning completion: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID, courseID string
	if err := tx.QueryRow(ctx, `
		select member_id::text, course_id::text
		from enrollment where id = $1::uuid for update`, enrollmentID).
		Scan(&ownerID, &courseID); errors.Is(err, pgx.ErrNoRows) {
		return learning.EnrollmentProgress{}, false, ErrNotFound
	} else if err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("loading enrollment: %w", err)
	}
	if ownerID != memberID {
		return learning.EnrollmentProgress{}, false, ErrForbidden
	}

	var ordering string
	var modulePosition, lessonPosition int
	if err := tx.QueryRow(ctx, `
		select c.ordering, m.position, l.position
		from lesson l
		join module m on m.id = l.module_id
		join course c on c.id = m.course_id
		where l.id = $1::uuid and c.id = $2::uuid`, lessonID, courseID).
		Scan(&ordering, &modulePosition, &lessonPosition); errors.Is(err, pgx.ErrNoRows) {
		return learning.EnrollmentProgress{}, false, ErrNotFound
	} else if err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("loading lesson: %w", err)
	}

	if ordering == "sequential" {
		var unfinishedEarlier bool
		if err := tx.QueryRow(ctx, `
			select exists (
				select 1
				from lesson previous
				join module pm on pm.id = previous.module_id
				where pm.course_id = $1::uuid
				  and (pm.position < $2 or (pm.position = $2 and previous.position < $3))
				  and not exists (
					select 1 from progress_event pe
					where pe.enrollment_id = $4::uuid
					  and pe.lesson_id = previous.id
					  and pe.event_type = 'lesson_completed'
				  )
			)`, courseID, modulePosition, lessonPosition, enrollmentID).Scan(&unfinishedEarlier); err != nil {
			return learning.EnrollmentProgress{}, false, fmt.Errorf("checking lesson order: %w", err)
		}
		if unfinishedEarlier {
			return learning.EnrollmentProgress{}, false, ErrOutOfOrder
		}
	}

	tag, err := tx.Exec(ctx, `
		insert into progress_event (enrollment_id, lesson_id, actor_member_id, event_type)
		values ($1::uuid, $2::uuid, $3::uuid, 'lesson_completed')
		on conflict (enrollment_id, lesson_id, event_type) do nothing`, enrollmentID, lessonID, memberID)
	if err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("recording completion: %w", err)
	}
	created := tag.RowsAffected() == 1

	// Recompute BOTH counts from source in one statement: completed from the append-only
	// event log, total from the course's current lessons. Deriving total here (rather than
	// trusting the enrollment-time snapshot) keeps the projection consistent with its check
	// constraints even if a course gains lessons after a learner enrolls.
	if _, err := tx.Exec(ctx, `
		with completed as (
			select count(*)::int as n
			from progress_event
			where enrollment_id = $1::uuid and event_type = 'lesson_completed'
		),
		total as (
			select count(l.id)::int as n
			from enrollment e
			join module m on m.course_id = e.course_id
			join lesson l on l.module_id = m.id
			where e.id = $1::uuid
		)
		update enrollment_progress ep
		set completed_lessons = completed.n,
		    total_lessons = total.n,
		    percent_complete = case
		        when total.n = 0 then 0
		        else (completed.n * 100 / total.n)
		    end,
		    updated_at = now()
		from completed, total
		where ep.enrollment_id = $1::uuid`, enrollmentID); err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("projecting completion: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		update enrollment e
		set status = case
		    when ep.total_lessons > 0 and ep.completed_lessons = ep.total_lessons then 'completed'
		    else 'active'
		end
		from enrollment_progress ep
		where e.id = ep.enrollment_id and e.id = $1::uuid`, enrollmentID); err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("updating enrollment status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return learning.EnrollmentProgress{}, false, fmt.Errorf("committing completion: %w", err)
	}

	progress, err := s.loadEnrollmentProgress(ctx, memberID, enrollmentID)
	return progress, created, err
}

// LoadDashboard builds the learner read model from the projection plus course lessons.
func (s *Store) LoadDashboard(ctx context.Context, memberID string) (learning.Dashboard, error) {
	member, err := s.loadMemberByID(ctx, memberID)
	if err != nil {
		return learning.Dashboard{}, err
	}
	dashboard := learning.Dashboard{Member: member, Enrollments: make([]learning.EnrollmentProgress, 0)}

	rows, err := s.pool.Query(ctx, `
		select e.id::text
		from enrollment e
		where e.member_id = $1::uuid
		order by e.enrolled_at, e.id`, memberID)
	if err != nil {
		return dashboard, fmt.Errorf("listing enrollments: %w", err)
	}
	var enrollmentIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return dashboard, fmt.Errorf("scanning enrollment: %w", err)
		}
		enrollmentIDs = append(enrollmentIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return dashboard, fmt.Errorf("iterating enrollments: %w", err)
	}
	rows.Close()

	for _, id := range enrollmentIDs {
		progress, err := s.loadEnrollmentProgress(ctx, memberID, id)
		if err != nil {
			return dashboard, err
		}
		dashboard.Enrollments = append(dashboard.Enrollments, progress)
	}
	return dashboard, nil
}

func (s *Store) loadMemberByID(ctx context.Context, memberID string) (learning.Member, error) {
	var member learning.Member
	err := s.pool.QueryRow(ctx, `
		select m.id::text, m.display_name,
		       coalesce(array_agg(mr.role order by mr.role)
		           filter (where mr.role is not null), '{}')
		from member m
		left join member_role mr on mr.member_id = m.id
		where m.id = $1::uuid
		group by m.id, m.display_name`, memberID).
		Scan(&member.ID, &member.DisplayName, &member.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return member, ErrNotFound
	}
	if err != nil {
		return member, fmt.Errorf("loading member: %w", err)
	}
	return member, nil
}

func (s *Store) loadEnrollmentProgress(ctx context.Context, memberID, enrollmentID string) (learning.EnrollmentProgress, error) {
	var progress learning.EnrollmentProgress
	err := s.pool.QueryRow(ctx, `
		select e.id::text, c.id::text, c.title, e.status,
		       ep.completed_lessons, ep.total_lessons, ep.percent_complete
		from enrollment e
		join course c on c.id = e.course_id
		join enrollment_progress ep on ep.enrollment_id = e.id
		where e.id = $1::uuid and e.member_id = $2::uuid`, enrollmentID, memberID).
		Scan(&progress.EnrollmentID, &progress.CourseID, &progress.CourseTitle, &progress.Status,
			&progress.CompletedLessons, &progress.TotalLessons, &progress.PercentComplete)
	if errors.Is(err, pgx.ErrNoRows) {
		return progress, ErrNotFound
	}
	if err != nil {
		return progress, fmt.Errorf("loading progress: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		select l.id::text, l.title, l.lesson_type,
		       row_number() over (order by m.position, l.position)::int,
		       pe.occurred_at
		from enrollment e
		join module m on m.course_id = e.course_id
		join lesson l on l.module_id = m.id
		left join progress_event pe
		  on pe.enrollment_id = e.id and pe.lesson_id = l.id
		 and pe.event_type = 'lesson_completed'
		where e.id = $1::uuid
		order by m.position, l.position`, enrollmentID)
	if err != nil {
		return progress, fmt.Errorf("loading lessons: %w", err)
	}
	defer rows.Close()
	progress.Lessons = make([]learning.LessonProgress, 0)
	for rows.Next() {
		var lesson learning.LessonProgress
		var completedAt sql.NullTime
		if err := rows.Scan(&lesson.ID, &lesson.Title, &lesson.Type, &lesson.Position, &completedAt); err != nil {
			return progress, fmt.Errorf("scanning lesson: %w", err)
		}
		lesson.Completed = completedAt.Valid
		if completedAt.Valid {
			t := completedAt.Time
			lesson.CompletedAt = &t
		}
		progress.Lessons = append(progress.Lessons, lesson)
	}
	if err := rows.Err(); err != nil {
		return progress, fmt.Errorf("iterating lessons: %w", err)
	}
	return progress, nil
}

// ListCompliance computes every member's current eligibility for an administrator.
// No derived eligibility boolean is stored, so expirations cannot leave stale status.
func (s *Store) ListCompliance(ctx context.Context) ([]learning.ComplianceMember, error) {
	rows, err := s.pool.Query(ctx, `
		select m.id::text, m.display_name,
		       coalesce(array_agg(mr.role order by mr.role)
		           filter (where mr.role is not null), '{}')
		from member m
		left join member_role mr on mr.member_id = m.id
		group by m.id, m.display_name
		order by m.display_name`)
	if err != nil {
		return nil, fmt.Errorf("listing compliance members: %w", err)
	}
	members := make([]learning.ComplianceMember, 0)
	for rows.Next() {
		var member learning.ComplianceMember
		if err := rows.Scan(&member.ID, &member.DisplayName, &member.Roles); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning compliance member: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterating compliance members: %w", err)
	}
	rows.Close()

	for i := range members {
		inputs, err := s.LoadSafeguardingInputs(ctx, members[i].ID)
		if err != nil {
			return nil, err
		}
		decision := safeguarding.Evaluate(inputs)
		members[i].Status = string(decision.Status)
		members[i].Reason = decision.Reason
		members[i].NextExpiration = earliest(inputs.BackgroundCheckExpires, inputs.SafeSportExpires, inputs.RoleCredentialExpires)
	}
	return members, nil
}

func earliest(values ...*time.Time) *time.Time {
	var result *time.Time
	for _, value := range values {
		if value != nil && (result == nil || value.Before(*result)) {
			t := *value
			result = &t
		}
	}
	return result
}

// LoadSafeguardingInputs gathers a member's safeguarding facts into the shape the
// eligibility engine expects. A missing row becomes a nil pointer ("not on file"), which
// Evaluate treats as not-current.
func (s *Store) LoadSafeguardingInputs(ctx context.Context, memberID string) (safeguarding.Inputs, error) {
	in := safeguarding.Inputs{Now: time.Now().UTC()}

	// Reject unknown members before loading optional safeguarding facts.
	var one int
	err := s.pool.QueryRow(ctx, `select 1 from member where id = $1::uuid`, memberID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return in, ErrNotFound
	}
	if err != nil {
		return in, fmt.Errorf("looking up member: %w", err)
	}

	// Latest approved background-check expiry, and latest SafeSport expiry.
	if in.BackgroundCheckExpires, err = s.maxDate(ctx,
		`select max(expires_at) from background_check where member_id = $1::uuid and status = 'approved'`,
		memberID); err != nil {
		return in, fmt.Errorf("background check: %w", err)
	}
	if in.SafeSportExpires, err = s.maxDate(ctx,
		`select max(expires_at) from safesport_training where member_id = $1::uuid`,
		memberID); err != nil {
		return in, fmt.Errorf("safesport: %w", err)
	}

	// Active disciplinary holds (lifted_at is null). Any row here means suspended.
	rows, err := s.pool.Query(ctx,
		`select source from disciplinary_hold where member_id = $1::uuid and lifted_at is null`,
		memberID)
	if err != nil {
		return in, fmt.Errorf("holds: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var source string
		if err := rows.Scan(&source); err != nil {
			return in, fmt.Errorf("scanning hold: %w", err)
		}
		in.ActiveHoldSources = append(in.ActiveHoldSources, source)
	}
	if err := rows.Err(); err != nil {
		return in, fmt.Errorf("iterating holds: %w", err)
	}

	// Role credentials: coach and referee roles each require a current
	// credential (coaching license / referee recertification). For each such
	// role the member holds, take their LATEST credential expiry for that
	// role; across roles the WEAKEST LINK wins — one role with no credential
	// at all means "missing", otherwise the earliest of the latest expiries
	// is the date that matters.
	credRows, err := s.pool.Query(ctx, `
		select mr.role, max(rc.expires_at)
		from member_role mr
		left join role_credential rc
		  on rc.member_id = mr.member_id and rc.role = mr.role
		where mr.member_id = $1::uuid and mr.role in ('coach','referee')
		group by mr.role`,
		memberID)
	if err != nil {
		return in, fmt.Errorf("role credentials: %w", err)
	}
	defer credRows.Close()
	missing := false
	var weakest *time.Time
	for credRows.Next() {
		var role string
		var latest sql.NullTime
		if err := credRows.Scan(&role, &latest); err != nil {
			return in, fmt.Errorf("scanning role credential: %w", err)
		}
		in.RoleCredentialRequired = true
		if !latest.Valid {
			missing = true
			continue
		}
		t := latest.Time
		if weakest == nil || t.Before(*weakest) {
			weakest = &t
		}
	}
	if err := credRows.Err(); err != nil {
		return in, fmt.Errorf("iterating role credentials: %w", err)
	}
	if in.RoleCredentialRequired && !missing {
		in.RoleCredentialExpires = weakest
	}

	return in, nil
}

// maxDate returns nil when the aggregate has no timestamp.
func (s *Store) maxDate(ctx context.Context, query, memberID string) (*time.Time, error) {
	var nt sql.NullTime
	if err := s.pool.QueryRow(ctx, query, memberID).Scan(&nt); err != nil {
		return nil, err
	}
	if !nt.Valid {
		return nil, nil
	}
	t := nt.Time
	return &t, nil
}

// LoadMemberCredentials assembles the provider side of the learning-center.credentials.v1
// contract for one identity-provider subject. The eligibility inputs come from
// LoadSafeguardingInputs so this route and the public eligibility route read the same
// facts; the individual role_credential rows are added because the contract lists each.
func (s *Store) LoadMemberCredentials(ctx context.Context, subject string) (credentials.Record, error) {
	member, err := s.ResolveMemberBySubject(ctx, subject)
	if err != nil {
		return credentials.Record{}, err
	}
	inputs, err := s.LoadSafeguardingInputs(ctx, member.ID)
	if err != nil {
		return credentials.Record{}, err
	}
	record := credentials.Record{
		MemberID:        member.ID,
		Subject:         subject,
		Roles:           member.Roles,
		Inputs:          inputs,
		RoleCredentials: make([]credentials.RoleCredential, 0),
	}

	rows, err := s.pool.Query(ctx, `
		select role, credential_type, issued_at, expires_at
		from role_credential
		where member_id = $1::uuid
		order by role, expires_at desc, issued_at desc`, member.ID)
	if err != nil {
		return credentials.Record{}, fmt.Errorf("role credentials: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var credential credentials.RoleCredential
		if err := rows.Scan(&credential.Role, &credential.CredentialType, &credential.IssuedAt, &credential.ExpiresAt); err != nil {
			return credentials.Record{}, fmt.Errorf("scanning role credential: %w", err)
		}
		record.RoleCredentials = append(record.RoleCredentials, credential)
	}
	if err := rows.Err(); err != nil {
		return credentials.Record{}, fmt.Errorf("iterating role credentials: %w", err)
	}
	return record, nil
}
