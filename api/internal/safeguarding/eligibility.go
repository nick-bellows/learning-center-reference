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

// Status is the computed participation status.
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
// Missing credentials are nil. Passing Now explicitly keeps evaluation deterministic.
type Inputs struct {
	Now                    time.Time
	SafeSportExpires       *time.Time // nil = no SafeSport on file
	BackgroundCheckExpires *time.Time // nil = no background check on file
	ActiveHoldSources      []string   // e.g. ["safesport"]; empty = no active holds
	GraceDays              int        // days after expiry still allowed (0 = flip exactly on expiry)

	// A coaching license or referee recertification applies only when the member's role
	// requires it.
	RoleCredentialRequired bool
	RoleCredentialExpires  *time.Time // nil = none on file
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
	//    SafeSport and the background check always apply; the role credential only when required.
	type cred struct {
		name    string
		expires *time.Time
	}
	required := []cred{
		{"SafeSport certification", in.SafeSportExpires},
		{"background check", in.BackgroundCheckExpires},
	}
	if in.RoleCredentialRequired {
		required = append(required, cred{"role credential", in.RoleCredentialExpires})
	}

	// Expiry dates are INCLUSIVE: a credential whose expires_at is 2027-06-01
	// is valid through that entire day (UTC) and flips to expired at
	// 2027-06-02T00:00:00Z. Database DATE columns arrive as midnight
	// timestamps, so validity runs until expiry + 1 day (+ any grace window).
	grace := time.Duration(in.GraceDays) * 24 * time.Hour
	for _, c := range required {
		if c.expires == nil {
			return Decision{StatusIneligible, "missing " + c.name}
		}
		deadline := c.expires.AddDate(0, 0, 1).Add(grace)
		if !in.Now.Before(deadline) {
			return Decision{StatusIneligible, "expired " + c.name}
		}
	}

	// 3. No hold, and every required credential current -> eligible.
	return Decision{StatusEligible, "all safeguarding requirements current"}
}
