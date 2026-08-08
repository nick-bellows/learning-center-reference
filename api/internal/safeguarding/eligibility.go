// Package safeguarding computes whether an adult participant is currently eligible to
// participate, derived from their safeguarding inputs. Eligibility is NEVER stored — it is
// recomputed from facts every time it is asked (see docs/domain-model.md section 4).
//
// Precondition: call this only for adult participants. Minors/players are exempt from these
// requirements; that gate happens before we get here.
package safeguarding

import (
	"strings"
	"time"
)

// Status is the computed participation status. It's a named type over string (not a bare
// string) so the compiler stops us accidentally mixing it up with ordinary text — the same
// idea as a C# enum, but backed by readable string values.
type Status string

const (
	StatusEligible   Status = "eligible"
	StatusSuspended  Status = "suspended"        // an active disciplinary hold
	StatusIneligible Status = "ineligible_lapsed" // a required credential missing or expired
	// StatusProvisional is planned: added once "pending / in-progress" states are wired from
	// the database (e.g. background_check.status = 'pending').
)

// Inputs are the facts about ONE adult participant, evaluated as of Now.
//
// The *time.Time fields are pointers so they can be nil when the item is not on file —
// Go's equivalent of a C# nullable (DateTime?). Passing Now in, instead of calling
// time.Now() inside, makes the function deterministic and trivial to test.
type Inputs struct {
	Now                    time.Time
	SafeSportExpires       *time.Time // nil = no SafeSport on file
	BackgroundCheckExpires *time.Time // nil = no background check on file
	RoleCredentialExpires  *time.Time // coaching license or referee recert; nil = none
	ActiveHoldSources      []string   // e.g. ["safesport"]; empty = no active holds
	GraceDays              int        // days after expiry still allowed (0 = flip exactly on expiry)
}

// Decision is the result: the status plus a human-readable reason for the UI and audit trail.
type Decision struct {
	Status Status
	Reason string
}

// Evaluate applies the safeguarding rule. Order matters: a hold overrides everything else.
func Evaluate(in Inputs) Decision {
	// 1. An active disciplinary hold makes the member ineligible regardless of anything else.
	if len(in.ActiveHoldSources) > 0 {
		return Decision{
			Status: StatusSuspended,
			Reason: "active disciplinary hold (" + strings.Join(in.ActiveHoldSources, ", ") + ")",
		}
	}

	// 2. Every required credential must be present and still current (within any grace window).
	grace := time.Duration(in.GraceDays) * 24 * time.Hour
	required := []struct {
		name    string
		expires *time.Time
	}{
		{"SafeSport certification", in.SafeSportExpires},
		{"background check", in.BackgroundCheckExpires},
		{"role credential", in.RoleCredentialExpires},
	}
	for _, c := range required {
		if c.expires == nil {
			return Decision{StatusIneligible, "missing " + c.name}
		}
		if in.Now.After(c.expires.Add(grace)) {
			return Decision{StatusIneligible, "expired " + c.name}
		}
	}

	// 3. No hold, and every credential current -> eligible.
	return Decision{StatusEligible, "all safeguarding requirements current"}
}
