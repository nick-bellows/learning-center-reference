// Package learning contains the small set of course-progress domain types shared by
// the HTTP and persistence layers. It intentionally has no database or HTTP concerns.
package learning

import "time"

// Member is the application identity resolved from a verified OIDC subject.
// Roles come from PostgreSQL; token-supplied roles are never trusted.
type Member struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
}

// HasRole reports whether the member was assigned role in the application database.
func (m Member) HasRole(role string) bool {
	for _, assigned := range m.Roles {
		if assigned == role {
			return true
		}
	}
	return false
}

// CourseSummary is the published course catalog shape.
type CourseSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Ordering    string `json:"ordering"`
	LessonCount int    `json:"lesson_count"`
}

// LessonProgress is a lesson plus its completion projection for one enrollment.
type LessonProgress struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Position    int        `json:"position"`
	Completed   bool       `json:"completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// EnrollmentProgress is the learner-facing read model. The counts are maintained
// transactionally from append-only progress events.
type EnrollmentProgress struct {
	EnrollmentID     string           `json:"enrollment_id"`
	CourseID         string           `json:"course_id"`
	CourseTitle      string           `json:"course_title"`
	Status           string           `json:"status"`
	CompletedLessons int              `json:"completed_lessons"`
	TotalLessons     int              `json:"total_lessons"`
	PercentComplete  int              `json:"percent_complete"`
	Lessons          []LessonProgress `json:"lessons"`
}

// Dashboard is the authenticated learner's complete progress view.
type Dashboard struct {
	Member      Member               `json:"member"`
	Enrollments []EnrollmentProgress `json:"enrollments"`
}

// ComplianceMember is the minimum non-sensitive information needed by the
// synthetic administrator compliance view.
type ComplianceMember struct {
	ID             string     `json:"id"`
	DisplayName    string     `json:"display_name"`
	Roles          []string   `json:"roles"`
	Status         string     `json:"status"`
	Reason         string     `json:"reason"`
	NextExpiration *time.Time `json:"next_expiration,omitempty"`
}
