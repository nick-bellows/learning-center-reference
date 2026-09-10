// Package projection repairs the enrollment_progress read projection from its source of
// truth. progress_event is append-only and authoritative; enrollment_progress is a cache
// of counts derived from it that the learner dashboard reads. The store keeps the two in
// one transaction, so drift is not expected in normal operation. It can still happen —
// a manual data fix, a restored backup taken mid-transaction, a future bug — and when it
// does, an operator needs a command that shows the drift and rebuilds the projection.
package projection

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Counts is one enrollment's projected progress.
type Counts struct {
	CompletedLessons int    `json:"completed_lessons"`
	TotalLessons     int    `json:"total_lessons"`
	PercentComplete  int    `json:"percent_complete"`
	Status           string `json:"status"`
}

// Drift is one enrollment whose projection disagrees with its events.
type Drift struct {
	EnrollmentID string `json:"enrollment_id"`
	// Projected is what enrollment_progress currently holds. Missing is true when the
	// enrollment has no projection row at all.
	Projected Counts `json:"projected"`
	Missing   bool   `json:"projection_missing"`
	// Expected is recomputed from progress_event and the course's current lessons.
	Expected Counts `json:"expected"`
}

// Report summarizes one reconcile run.
type Report struct {
	// Checked is the number of enrollments compared.
	Checked int `json:"enrollments_checked"`
	// Drifted lists every enrollment whose projection did not match its events.
	Drifted []Drift `json:"drifted"`
	// Applied is true when the drifted projections were rewritten in this run.
	Applied bool `json:"applied"`
}

// expectedSQL derives, for every enrollment, the counts the projection should hold. It
// uses the same rules the store uses when it writes the projection: completed comes from
// the append-only event log, total from the course's current lessons, percent is
// integer-truncated, and status is completed only when every lesson is done. A withdrawn
// enrollment keeps its status: withdrawal is an explicit decision, not a derived count.
const expectedSQL = `
	with lesson_totals as (
		select e.id as enrollment_id, count(l.id)::int as total
		from enrollment e
		left join module m on m.course_id = e.course_id
		left join lesson l on l.module_id = m.id
		group by e.id
	),
	completions as (
		select enrollment_id, count(*)::int as completed
		from progress_event
		where event_type = 'lesson_completed'
		group by enrollment_id
	)
	select e.id::text,
	       ep.enrollment_id is null,
	       coalesce(ep.completed_lessons, 0), coalesce(ep.total_lessons, 0),
	       coalesce(ep.percent_complete, 0), e.status,
	       coalesce(c.completed, 0), lt.total,
	       case when lt.total = 0 then 0 else coalesce(c.completed, 0) * 100 / lt.total end,
	       case
	           when e.status = 'withdrawn' then 'withdrawn'
	           when lt.total > 0 and coalesce(c.completed, 0) = lt.total then 'completed'
	           else 'active'
	       end
	from enrollment e
	join lesson_totals lt on lt.enrollment_id = e.id
	left join completions c on c.enrollment_id = e.id
	left join enrollment_progress ep on ep.enrollment_id = e.id
	order by e.enrolled_at, e.id`

// Reconcile compares every enrollment's projection with its events. With apply=false it
// only reports; with apply=true it rewrites each drifted projection (and the enrollment
// status derived from it) inside one transaction, so a partial repair is never visible.
// Enrollment rows are locked for the repair so a concurrent completion, which locks the
// same row, cannot interleave with the rebuild.
func Reconcile(ctx context.Context, pool *pgxpool.Pool, apply bool) (Report, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return Report{}, fmt.Errorf("beginning reconcile: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if apply {
		if _, err := tx.Exec(ctx, `select id from enrollment for update`); err != nil {
			return Report{}, fmt.Errorf("locking enrollments: %w", err)
		}
	}

	rows, err := tx.Query(ctx, expectedSQL)
	if err != nil {
		return Report{}, fmt.Errorf("computing expected progress: %w", err)
	}
	report := Report{Drifted: make([]Drift, 0), Applied: apply}
	for rows.Next() {
		var d Drift
		if err := rows.Scan(&d.EnrollmentID, &d.Missing,
			&d.Projected.CompletedLessons, &d.Projected.TotalLessons, &d.Projected.PercentComplete, &d.Projected.Status,
			&d.Expected.CompletedLessons, &d.Expected.TotalLessons, &d.Expected.PercentComplete, &d.Expected.Status,
		); err != nil {
			rows.Close()
			return Report{}, fmt.Errorf("scanning expected progress: %w", err)
		}
		report.Checked++
		if d.Missing || d.Projected != d.Expected {
			report.Drifted = append(report.Drifted, d)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Report{}, fmt.Errorf("iterating expected progress: %w", err)
	}
	rows.Close()

	if !apply || len(report.Drifted) == 0 {
		return report, nil
	}

	for _, d := range report.Drifted {
		if _, err := tx.Exec(ctx, `
			insert into enrollment_progress
			    (enrollment_id, completed_lessons, total_lessons, percent_complete, updated_at)
			values ($1::uuid, $2, $3, $4, now())
			on conflict (enrollment_id) do update
			set completed_lessons = excluded.completed_lessons,
			    total_lessons     = excluded.total_lessons,
			    percent_complete  = excluded.percent_complete,
			    updated_at        = now()`,
			d.EnrollmentID, d.Expected.CompletedLessons, d.Expected.TotalLessons, d.Expected.PercentComplete,
		); err != nil {
			return Report{}, fmt.Errorf("rebuilding projection for %s: %w", d.EnrollmentID, err)
		}
		if _, err := tx.Exec(ctx, `update enrollment set status = $2 where id = $1::uuid`,
			d.EnrollmentID, d.Expected.Status); err != nil {
			return Report{}, fmt.Errorf("updating status for %s: %w", d.EnrollmentID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Report{}, fmt.Errorf("committing reconcile: %w", err)
	}
	return report, nil
}
