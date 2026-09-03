// Package httpapi builds the HTTP router for the Learning Center API.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nick-bellows/learning-center-reference/api/internal/authn"
	"github.com/nick-bellows/learning-center-reference/api/internal/credentials"
	"github.com/nick-bellows/learning-center-reference/api/internal/learning"
	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
	"github.com/nick-bellows/learning-center-reference/api/internal/store"
)

// EligibilityLoader is the narrow store view needed by the public reference endpoint.
type EligibilityLoader interface {
	LoadSafeguardingInputs(ctx context.Context, memberID string) (safeguarding.Inputs, error)
}

// CredentialsLoader is the store view behind the service-to-service credentials contract.
type CredentialsLoader interface {
	LoadMemberCredentials(ctx context.Context, subject string) (credentials.Record, error)
}

// IdentityResolver maps a verified external subject to local roles.
type IdentityResolver interface {
	ResolveMemberBySubject(ctx context.Context, subject string) (learning.Member, error)
}

// LearningStore is the application workflow used by authenticated handlers.
type LearningStore interface {
	ListPublishedCourses(ctx context.Context) ([]learning.CourseSummary, error)
	Enroll(ctx context.Context, memberID, courseID string) (learning.EnrollmentProgress, bool, error)
	CompleteLesson(ctx context.Context, memberID, enrollmentID, lessonID string) (learning.EnrollmentProgress, bool, error)
	LoadDashboard(ctx context.Context, memberID string) (learning.Dashboard, error)
	ListCompliance(ctx context.Context) ([]learning.ComplianceMember, error)
}

// Pinger is the health check's view of the database.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps holds dependencies created by main and injected into the router.
type Deps struct {
	Eligibility        EligibilityLoader
	Credentials        CredentialsLoader
	Identity           IdentityResolver
	Learning           LearningStore
	Auth               authn.Verifier
	ServiceAuth        authn.ClaimsVerifier
	DB                 Pinger
	Logger             *slog.Logger
	RateLimitPerMinute int
	TrustProxy         bool
}

type memberContextKey struct{}

var uuidRe = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// NewRouter wires middleware and routes. Authentication is fail-closed when omitted.
func NewRouter(deps Deps) http.Handler {
	if deps.Auth == nil {
		deps.Auth = authn.UnavailableVerifier{}
	}
	if deps.ServiceAuth == nil {
		deps.ServiceAuth = authn.UnavailableVerifier{}
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(deps.requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(middleware.RequestSize(1 << 20))
	r.Use(middleware.Throttle(64))
	if deps.RateLimitPerMinute > 0 {
		r.Use(newClientRateLimiter(deps.RateLimitPerMinute, time.Minute, deps.TrustProxy).middleware)
	}

	r.Get("/health", deps.handleHealth)
	r.Get("/v1/members/{id}/eligibility", deps.handleEligibility)
	// Service-to-service contract: the caller is a scoped client, not a person, so this
	// route bypasses the member-resolving authenticate middleware below.
	r.With(deps.authenticateService(credentials.Scope)).
		Get("/v1/members/{subject}/credentials", deps.handleMemberCredentials)

	r.Group(func(r chi.Router) {
		r.Use(deps.authenticate)
		r.With(requireRole("learner")).Get("/v1/courses", deps.handleCourses)
		r.With(requireRole("learner")).Post("/v1/courses/{courseID}/enrollments", deps.handleEnroll)
		r.With(requireRole("learner")).Post("/v1/enrollments/{enrollmentID}/lessons/{lessonID}/complete", deps.handleCompleteLesson)
		r.With(requireRole("learner")).Get("/v1/me/dashboard", deps.handleDashboard)
		r.With(requireRole("admin")).Get("/v1/admin/compliance", deps.handleCompliance)
	})

	return r
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// requestLogger emits operational metadata without authorization headers or PII.
func (deps Deps) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapped, r)
		deps.Logger.InfoContext(r.Context(), "http request",
			"request_id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

func (deps Deps) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer token required"})
			return
		}
		subject, err := deps.Auth.Verify(r.Context(), raw)
		if err != nil {
			if errors.Is(err, authn.ErrUnavailable) {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "authentication not configured"})
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer token"})
			return
		}
		if deps.Identity == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "identity store unavailable"})
			return
		}
		member, err := deps.Identity.ResolveMemberBySubject(r.Context(), subject)
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "identity is not provisioned"})
			return
		}
		if err != nil {
			deps.Logger.ErrorContext(r.Context(), "resolving authenticated identity",
				"request_id", middleware.GetReqID(r.Context()), "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		ctx := context.WithValue(r.Context(), memberContextKey{}, member)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticateService guards routes called by another system rather than a person. The
// token must verify and carry scope; no member row is resolved because the caller is a
// client. Error bodies follow the credentials contract's errors fixture.
func (deps Deps) authenticateService(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			claims, err := deps.ServiceAuth.VerifyClaims(r.Context(), raw)
			if err != nil {
				if errors.Is(err, authn.ErrUnavailable) {
					writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "authentication not configured"})
					return
				}
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			if !claims.HasScope(scope) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func requireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			member, ok := memberFromContext(r.Context())
			if !ok || !member.HasRole(role) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient role"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func memberFromContext(ctx context.Context) (learning.Member, bool) {
	member, ok := ctx.Value(memberContextKey{}).(learning.Member)
	return member, ok
}

// handleHealth reports readiness: a configured database must answer a ping.
func (deps Deps) handleHealth(w http.ResponseWriter, r *http.Request) {
	if deps.DB == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "not configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := deps.DB.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "database": "unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
}

func (deps Deps) handleEligibility(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !uuidRe.MatchString(id) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid member id"})
		return
	}
	if deps.Eligibility == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "eligibility store unavailable"})
		return
	}
	inputs, err := deps.Eligibility.LoadSafeguardingInputs(r.Context(), id)
	if err != nil {
		deps.writeStoreError(w, r, err)
		return
	}
	decision := safeguarding.Evaluate(inputs)
	writeJSON(w, http.StatusOK, map[string]any{
		"member_id": id,
		"status":    decision.Status,
		"reason":    decision.Reason,
	})
}

// maxSubjectLength follows OpenID Connect Core 1.0 section 2: a subject never exceeds
// 255 ASCII characters, so anything longer is a malformed request, not a lookup.
const maxSubjectLength = 255

func (deps Deps) handleMemberCredentials(w http.ResponseWriter, r *http.Request) {
	// chi returns the raw segment when the request path carried escapes that Go's URL
	// parser does not normalise, so the subject is always unescaped here before use.
	subject, err := url.PathUnescape(chi.URLParam(r, "subject"))
	if err != nil || strings.TrimSpace(subject) == "" || len(subject) > maxSubjectLength {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid subject"})
		return
	}
	if deps.Credentials == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "credentials store unavailable"})
		return
	}
	record, err := deps.Credentials.LoadMemberCredentials(r.Context(), subject)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "member not found"})
		return
	}
	if err != nil {
		deps.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, credentials.Build(record))
}

func (deps Deps) handleCourses(w http.ResponseWriter, r *http.Request) {
	if deps.Learning == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "learning store unavailable"})
		return
	}
	courses, err := deps.Learning.ListPublishedCourses(r.Context())
	if err != nil {
		deps.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"courses": courses})
}

func (deps Deps) handleEnroll(w http.ResponseWriter, r *http.Request) {
	if deps.Learning == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "learning store unavailable"})
		return
	}
	courseID := chi.URLParam(r, "courseID")
	if !uuidRe.MatchString(courseID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid course id"})
		return
	}
	member, _ := memberFromContext(r.Context())
	progress, created, err := deps.Learning.Enroll(r.Context(), member.ID, courseID)
	if err != nil {
		deps.writeStoreError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"created": created, "enrollment": progress})
}

func (deps Deps) handleCompleteLesson(w http.ResponseWriter, r *http.Request) {
	if deps.Learning == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "learning store unavailable"})
		return
	}
	enrollmentID := chi.URLParam(r, "enrollmentID")
	lessonID := chi.URLParam(r, "lessonID")
	if !uuidRe.MatchString(enrollmentID) || !uuidRe.MatchString(lessonID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid enrollment or lesson id"})
		return
	}
	member, _ := memberFromContext(r.Context())
	progress, recorded, err := deps.Learning.CompleteLesson(r.Context(), member.ID, enrollmentID, lessonID)
	if err != nil {
		deps.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recorded": recorded, "enrollment": progress})
}

func (deps Deps) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if deps.Learning == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "learning store unavailable"})
		return
	}
	member, _ := memberFromContext(r.Context())
	dashboard, err := deps.Learning.LoadDashboard(r.Context(), member.ID)
	if err != nil {
		deps.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (deps Deps) handleCompliance(w http.ResponseWriter, r *http.Request) {
	if deps.Learning == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "learning store unavailable"})
		return
	}
	members, err := deps.Learning.ListCompliance(r.Context())
	if err != nil {
		deps.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (deps Deps) writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case errors.Is(err, store.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, store.ErrOutOfOrder):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "complete earlier lessons first"})
	default:
		deps.Logger.ErrorContext(r.Context(), "request failed",
			"request_id", middleware.GetReqID(r.Context()), "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
