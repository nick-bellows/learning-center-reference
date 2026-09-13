package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/nick-bellows/learning-center-reference/api/internal/credentials"
	"github.com/nick-bellows/learning-center-reference/api/internal/learning"
	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
)

// TestOpenAPIContract (openapi_test.go) proves the document is valid. This test proves the
// HANDLERS honour it: every status each route can emit — success, validation, auth,
// not-found, conflict, throttling, internal error, and unavailable dependencies — is
// documented for that route, and the response body matches the documented schema.
// An undocumented status or a body that drifts from the schema fails here.

// stubStore is a configurable fake for every store interface the router consumes. Each
// function defaults to a schema-valid success; a case overrides only what it needs.
type stubStore struct {
	resolve     func(subject string) (learning.Member, error)
	courses     func() ([]learning.CourseSummary, error)
	enroll      func(memberID, courseID string) (learning.EnrollmentProgress, bool, error)
	complete    func(memberID, enrollmentID, lessonID string) (learning.EnrollmentProgress, bool, error)
	dashboard   func(memberID string) (learning.Dashboard, error)
	compliance  func() ([]learning.ComplianceMember, error)
	eligibility func(memberID string) (safeguarding.Inputs, error)
	credentials func(subject string) (credentials.Record, error)
}

const (
	learnerID    = "11111111-1111-1111-1111-111111111111"
	adminID      = "44444444-4444-4444-4444-444444444444"
	courseUUID   = "10000000-0000-0000-0000-000000000001"
	enrollmentID = "50000000-0000-0000-0000-000000000001"
	lessonUUID   = "30000000-0000-0000-0000-000000000001"
)

func validProgress() learning.EnrollmentProgress {
	completedAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	return learning.EnrollmentProgress{
		EnrollmentID: enrollmentID, CourseID: courseUUID, CourseTitle: "Grassroots Match-Day Safety",
		Status: "active", CompletedLessons: 1, TotalLessons: 3, PercentComplete: 33,
		Lessons: []learning.LessonProgress{
			{ID: lessonUUID, Title: "Lesson one", Type: "video", Position: 1, Completed: true, CompletedAt: &completedAt},
			{ID: "30000000-0000-0000-0000-000000000002", Title: "Lesson two", Type: "reading", Position: 2},
			{ID: "30000000-0000-0000-0000-000000000003", Title: "Lesson three", Type: "quiz", Position: 3},
		},
	}
}

func newStubStore() *stubStore {
	return &stubStore{
		resolve: func(subject string) (learning.Member, error) {
			switch subject {
			case "subject|learner":
				return learning.Member{ID: learnerID, DisplayName: "Alex Coach", Roles: []string{"coach", "learner"}}, nil
			case "subject|admin":
				return learning.Member{ID: adminID, DisplayName: "Casey Admin", Roles: []string{"admin"}}, nil
			}
			return learning.Member{}, store.ErrNotFound
		},
		courses: func() ([]learning.CourseSummary, error) {
			return []learning.CourseSummary{{ID: courseUUID, Title: "Grassroots Match-Day Safety", Slug: "grassroots", Ordering: "sequential", LessonCount: 3}}, nil
		},
		enroll: func(string, string) (learning.EnrollmentProgress, bool, error) { return validProgress(), true, nil },
		complete: func(string, string, string) (learning.EnrollmentProgress, bool, error) {
			return validProgress(), true, nil
		},
		dashboard: func(string) (learning.Dashboard, error) {
			return learning.Dashboard{
				Member:      learning.Member{ID: learnerID, DisplayName: "Alex Coach", Roles: []string{"learner"}},
				Enrollments: []learning.EnrollmentProgress{validProgress()},
			}, nil
		},
		compliance: func() ([]learning.ComplianceMember, error) {
			expires := time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)
			return []learning.ComplianceMember{
				{ID: learnerID, DisplayName: "Alex Coach", Roles: []string{"coach", "learner"}, Status: "eligible", Reason: "all safeguarding requirements current", NextExpiration: &expires},
				{ID: adminID, DisplayName: "Casey Admin", Roles: []string{"admin"}, Status: "ineligible_lapsed", Reason: "missing SafeSport training"},
			}, nil
		},
		eligibility: func(string) (safeguarding.Inputs, error) {
			return eligibleLearnerRecord().Inputs, nil
		},
		credentials: func(subject string) (credentials.Record, error) {
			if subject != "demo|learner" {
				return credentials.Record{}, store.ErrNotFound
			}
			return eligibleLearnerRecord(), nil
		},
	}
}

func (s *stubStore) ResolveMemberBySubject(_ context.Context, subject string) (learning.Member, error) {
	return s.resolve(subject)
}
func (s *stubStore) ListPublishedCourses(context.Context) ([]learning.CourseSummary, error) {
	return s.courses()
}
func (s *stubStore) Enroll(_ context.Context, memberID, courseID string) (learning.EnrollmentProgress, bool, error) {
	return s.enroll(memberID, courseID)
}
func (s *stubStore) CompleteLesson(_ context.Context, memberID, enrollmentID, lessonID string) (learning.EnrollmentProgress, bool, error) {
	return s.complete(memberID, enrollmentID, lessonID)
}
func (s *stubStore) LoadDashboard(_ context.Context, memberID string) (learning.Dashboard, error) {
	return s.dashboard(memberID)
}
func (s *stubStore) ListCompliance(context.Context) ([]learning.ComplianceMember, error) {
	return s.compliance()
}
func (s *stubStore) LoadSafeguardingInputs(_ context.Context, memberID string) (safeguarding.Inputs, error) {
	return s.eligibility(memberID)
}
func (s *stubStore) LoadMemberCredentials(_ context.Context, subject string) (credentials.Record, error) {
	return s.credentials(subject)
}

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

// fullDeps wires the stub into every dependency, with working auth for people and services.
func fullDeps(s *stubStore) Deps {
	return Deps{
		Eligibility: s, Credentials: s, Identity: s, Learning: s,
		Auth: fakeVerifier{}, ServiceAuth: fakeServiceVerifier{}, DB: okPinger{},
	}
}

type conformanceCase struct {
	name   string
	method string
	path   string
	token  string
	deps   func() Deps
	want   int
}

func TestHandlersConformToOpenAPI(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile("../../openapi.yaml")
	if err != nil {
		t.Fatalf("load OpenAPI: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI: %v", err)
	}
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatalf("build OpenAPI router: %v", err)
	}

	boom := errors.New("simulated database failure")
	failing := func(mutate func(*stubStore)) func() Deps {
		return func() Deps {
			s := newStubStore()
			mutate(s)
			return fullDeps(s)
		}
	}
	healthy := func() Deps { return fullDeps(newStubStore()) }
	// Nothing configured: auth and stores are absent, so protected routes fail closed.
	unconfigured := func() Deps { return Deps{} }
	tooLongSubject := strings.Repeat("s", maxSubjectLength+1)

	cases := []conformanceCase{
		{"health ok", "GET", "/health", "", healthy, 200},
		{"health without database", "GET", "/health", "", unconfigured, 200},
		{"health database down", "GET", "/health", "", func() Deps { return Deps{DB: failingPinger{}} }, 503},

		{"eligibility ok", "GET", "/v1/members/" + learnerID + "/eligibility", "", healthy, 200},
		{"eligibility malformed id", "GET", "/v1/members/not-a-uuid/eligibility", "", healthy, 400},
		{"eligibility unknown member", "GET", "/v1/members/" + adminID + "/eligibility", "",
			failing(func(s *stubStore) {
				s.eligibility = func(string) (safeguarding.Inputs, error) { return safeguarding.Inputs{}, store.ErrNotFound }
			}), 404},
		{"eligibility store failure", "GET", "/v1/members/" + learnerID + "/eligibility", "",
			failing(func(s *stubStore) {
				s.eligibility = func(string) (safeguarding.Inputs, error) { return safeguarding.Inputs{}, boom }
			}), 500},
		{"eligibility store unavailable", "GET", "/v1/members/" + learnerID + "/eligibility", "", unconfigured, 503},

		{"credentials ok", "GET", "/v1/members/demo%7Clearner/credentials", "service-token", healthy, 200},
		{"credentials over-long subject", "GET", "/v1/members/" + tooLongSubject + "/credentials", "service-token", healthy, 400},
		{"credentials no token", "GET", "/v1/members/demo%7Clearner/credentials", "", healthy, 401},
		{"credentials wrong scope", "GET", "/v1/members/demo%7Clearner/credentials", "unscoped-service-token", healthy, 403},
		{"credentials unknown subject", "GET", "/v1/members/demo%7Cnobody/credentials", "service-token", healthy, 404},
		{"credentials store failure", "GET", "/v1/members/demo%7Clearner/credentials", "service-token",
			failing(func(s *stubStore) {
				s.credentials = func(string) (credentials.Record, error) { return credentials.Record{}, boom }
			}), 500},
		{"credentials auth unavailable", "GET", "/v1/members/demo%7Clearner/credentials", "service-token", unconfigured, 503},

		{"courses ok", "GET", "/v1/courses", "learner-token", healthy, 200},
		{"courses no token", "GET", "/v1/courses", "", healthy, 401},
		{"courses bad token", "GET", "/v1/courses", "forged-token", healthy, 401},
		{"courses admin lacks learner role", "GET", "/v1/courses", "admin-token", healthy, 403},
		{"courses store failure", "GET", "/v1/courses", "learner-token",
			failing(func(s *stubStore) { s.courses = func() ([]learning.CourseSummary, error) { return nil, boom } }), 500},
		{"courses handler panic", "GET", "/v1/courses", "learner-token",
			failing(func(s *stubStore) { s.courses = func() ([]learning.CourseSummary, error) { panic("simulated bug") } }), 500},
		{"courses auth unavailable", "GET", "/v1/courses", "learner-token", unconfigured, 503},
		{"courses identity store failure", "GET", "/v1/courses", "learner-token",
			failing(func(s *stubStore) {
				s.resolve = func(string) (learning.Member, error) { return learning.Member{}, boom }
			}), 500},

		{"enroll created", "POST", "/v1/courses/" + courseUUID + "/enrollments", "learner-token", healthy, 201},
		{"enroll retry", "POST", "/v1/courses/" + courseUUID + "/enrollments", "learner-token",
			failing(func(s *stubStore) {
				s.enroll = func(string, string) (learning.EnrollmentProgress, bool, error) { return validProgress(), false, nil }
			}), 200},
		{"enroll malformed course", "POST", "/v1/courses/not-a-uuid/enrollments", "learner-token", healthy, 400},
		{"enroll unknown course", "POST", "/v1/courses/" + courseUUID + "/enrollments", "learner-token",
			failing(func(s *stubStore) {
				s.enroll = func(string, string) (learning.EnrollmentProgress, bool, error) {
					return learning.EnrollmentProgress{}, false, store.ErrNotFound
				}
			}), 404},
		{"enroll no token", "POST", "/v1/courses/" + courseUUID + "/enrollments", "", healthy, 401},
		{"enroll admin forbidden", "POST", "/v1/courses/" + courseUUID + "/enrollments", "admin-token", healthy, 403},

		{"complete ok", "POST", "/v1/enrollments/" + enrollmentID + "/lessons/" + lessonUUID + "/complete", "learner-token", healthy, 200},
		{"complete malformed ids", "POST", "/v1/enrollments/x/lessons/y/complete", "learner-token", healthy, 400},
		{"complete not owner", "POST", "/v1/enrollments/" + enrollmentID + "/lessons/" + lessonUUID + "/complete", "learner-token",
			failing(func(s *stubStore) {
				s.complete = func(string, string, string) (learning.EnrollmentProgress, bool, error) {
					return learning.EnrollmentProgress{}, false, store.ErrForbidden
				}
			}), 403},
		{"complete unknown enrollment", "POST", "/v1/enrollments/" + enrollmentID + "/lessons/" + lessonUUID + "/complete", "learner-token",
			failing(func(s *stubStore) {
				s.complete = func(string, string, string) (learning.EnrollmentProgress, bool, error) {
					return learning.EnrollmentProgress{}, false, store.ErrNotFound
				}
			}), 404},
		{"complete out of order", "POST", "/v1/enrollments/" + enrollmentID + "/lessons/" + lessonUUID + "/complete", "learner-token",
			failing(func(s *stubStore) {
				s.complete = func(string, string, string) (learning.EnrollmentProgress, bool, error) {
					return learning.EnrollmentProgress{}, false, store.ErrOutOfOrder
				}
			}), 409},
		{"complete withdrawn enrollment", "POST", "/v1/enrollments/" + enrollmentID + "/lessons/" + lessonUUID + "/complete", "learner-token",
			failing(func(s *stubStore) {
				s.complete = func(string, string, string) (learning.EnrollmentProgress, bool, error) {
					return learning.EnrollmentProgress{}, false, store.ErrEnrollmentWithdrawn
				}
			}), 409},
		{"complete store failure", "POST", "/v1/enrollments/" + enrollmentID + "/lessons/" + lessonUUID + "/complete", "learner-token",
			failing(func(s *stubStore) {
				s.complete = func(string, string, string) (learning.EnrollmentProgress, bool, error) {
					return learning.EnrollmentProgress{}, false, boom
				}
			}), 500},

		{"dashboard ok", "GET", "/v1/me/dashboard", "learner-token", healthy, 200},
		{"dashboard no token", "GET", "/v1/me/dashboard", "", healthy, 401},
		{"dashboard store failure", "GET", "/v1/me/dashboard", "learner-token",
			failing(func(s *stubStore) {
				s.dashboard = func(string) (learning.Dashboard, error) { return learning.Dashboard{}, boom }
			}), 500},

		{"compliance ok", "GET", "/v1/admin/compliance", "admin-token", healthy, 200},
		{"compliance learner forbidden", "GET", "/v1/admin/compliance", "learner-token", healthy, 403},
		{"compliance no token", "GET", "/v1/admin/compliance", "", healthy, 401},
		{"compliance store failure", "GET", "/v1/admin/compliance", "admin-token",
			failing(func(s *stubStore) { s.compliance = func() ([]learning.ComplianceMember, error) { return nil, boom } }), 500},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewRouter(tc.deps())
			req := httptest.NewRequest(tc.method, "http://localhost:8080"+tc.path, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d; want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
			assertConforms(t, router, req, rec)
		})
	}
}

// TestThrottledResponsesConformToOpenAPI covers the two 429 sources — the per-client rate
// limit and the server-wide concurrency limit — which no single ordinary request triggers.
func TestThrottledResponsesConformToOpenAPI(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile("../../openapi.yaml")
	if err != nil {
		t.Fatalf("load OpenAPI: %v", err)
	}
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatalf("build OpenAPI router: %v", err)
	}

	t.Run("client rate limit", func(t *testing.T) {
		handler := NewRouter(Deps{RateLimitPerMinute: 1})
		for i, want := range []int{200, 429} {
			req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/health", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != want {
				t.Fatalf("request %d status = %d; want %d", i+1, rec.Code, want)
			}
			assertConforms(t, router, req, rec)
		}
	})

	t.Run("concurrency limit", func(t *testing.T) {
		pinger := newBlockingPinger()
		handler := NewRouter(Deps{DB: pinger, MaxConcurrentRequests: 1})
		done := make(chan struct{})
		go func() {
			defer close(done)
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://localhost:8080/health", nil))
		}()
		<-pinger.entered
		defer func() { close(pinger.release); <-done }()

		req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d; want 429", rec.Code)
		}
		assertConforms(t, router, req, rec)
	})
}

// assertConforms finds the request's operation in the document and validates the
// recorded status, headers, and body against it. Undocumented statuses are failures.
func assertConforms(t *testing.T, router routers.Router, req *http.Request, rec *httptest.ResponseRecorder) {
	t.Helper()
	route, pathParams, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("route %s %s is not in openapi.yaml: %v", req.Method, req.URL.Path, err)
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: req, PathParams: pathParams, Route: route,
		},
		Status:  rec.Code,
		Header:  rec.Header(),
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	input.SetBodyBytes(rec.Body.Bytes())
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		t.Fatalf("%s %s -> %d does not conform to openapi.yaml: %v\nbody: %s",
			req.Method, req.URL.Path, rec.Code, err, rec.Body.String())
	}
}
