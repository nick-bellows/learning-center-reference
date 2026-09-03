// Package credentials implements the provider side of the learning-center.credentials.v1
// contract: the record the store loads for one identity-provider subject and the response
// derived from it. The response shape is fixed by the consumer's fixtures copied under
// testdata/contracts/learning-center.credentials.v1; changing it is a new contract version.
package credentials

import (
	"time"

	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
)

const (
	// Contract is the version identifier the consumer checks before reading any other field.
	Contract = "learning-center.credentials.v1"
	// Scope is the OAuth2 scope a service token must carry to read credentials.
	Scope = "credentials:read"
)

// RoleCredential is one role_credential row.
type RoleCredential struct {
	Role           string
	CredentialType string
	IssuedAt       time.Time
	ExpiresAt      time.Time
}

// Record is everything the store loads for one subject. Inputs.Now is the evaluation
// instant reported as as_of, so the eligibility decision and every valid flag are judged
// at the same moment.
type Record struct {
	MemberID        string
	Subject         string
	Roles           []string
	Inputs          safeguarding.Inputs
	RoleCredentials []RoleCredential
}

// Response is the 200 body of GET /v1/members/{subject}/credentials.
type Response struct {
	Contract        string                 `json:"contract"`
	Member          Member                 `json:"member"`
	AsOf            string                 `json:"as_of"`
	Eligibility     Eligibility            `json:"eligibility"`
	Holds           []Hold                 `json:"holds"`
	Safeguarding    Safeguarding           `json:"safeguarding"`
	RoleCredentials []RoleCredentialStatus `json:"role_credentials"`
}

// Member identifies the person: the provider's id for reconciliation, the subject the
// caller asked for, and the database-managed roles. No display name or date of birth.
type Member struct {
	ID      string   `json:"id"`
	Subject string   `json:"subject"`
	Roles   []string `json:"roles"`
}

// Eligibility carries the public eligibility vocabulary and a human-readable reason.
type Eligibility struct {
	Status safeguarding.Status `json:"status"`
	Reason string              `json:"reason"`
}

// Hold is an active disciplinary hold. The reason text is deliberately not carried.
type Hold struct {
	Source string `json:"source"`
	Active bool   `json:"active"`
}

// Safeguarding holds the two universal credentials.
type Safeguarding struct {
	SafeSportTraining ExpiringFact `json:"safesport_training"`
	BackgroundCheck   ExpiringFact `json:"background_check"`
}

// ExpiringFact is a credential's expiry date (null when nothing is on file) and whether
// the provider judged it valid at as_of.
type ExpiringFact struct {
	ExpiresAt *string `json:"expires_at"`
	Valid     bool    `json:"valid"`
}

// RoleCredentialStatus is one role credential with its provider-evaluated validity.
type RoleCredentialStatus struct {
	Role           string `json:"role"`
	CredentialType string `json:"credential_type"`
	IssuedAt       string `json:"issued_at"`
	ExpiresAt      string `json:"expires_at"`
	Valid          bool   `json:"valid"`
}

const dateLayout = "2006-01-02"

// Build derives the response from a record. Eligibility comes from safeguarding.Evaluate
// and every valid flag from safeguarding.Current, so the contract cannot disagree with
// the public eligibility route about the same facts.
func Build(record Record) Response {
	in := record.Inputs
	decision := safeguarding.Evaluate(in)

	holds := make([]Hold, 0, len(in.ActiveHoldSources))
	for _, source := range in.ActiveHoldSources {
		holds = append(holds, Hold{Source: source, Active: true})
	}

	roleCredentials := make([]RoleCredentialStatus, 0, len(record.RoleCredentials))
	for _, credential := range record.RoleCredentials {
		expires := credential.ExpiresAt
		roleCredentials = append(roleCredentials, RoleCredentialStatus{
			Role:           credential.Role,
			CredentialType: credential.CredentialType,
			IssuedAt:       credential.IssuedAt.Format(dateLayout),
			ExpiresAt:      expires.Format(dateLayout),
			Valid:          safeguarding.Current(in.Now, &expires, in.GraceDays),
		})
	}

	roles := record.Roles
	if roles == nil {
		roles = []string{}
	}

	return Response{
		Contract:    Contract,
		Member:      Member{ID: record.MemberID, Subject: record.Subject, Roles: roles},
		AsOf:        in.Now.UTC().Format(time.RFC3339),
		Eligibility: Eligibility{Status: decision.Status, Reason: decision.Reason},
		Holds:       holds,
		Safeguarding: Safeguarding{
			SafeSportTraining: expiringFact(in.Now, in.SafeSportExpires, in.GraceDays),
			BackgroundCheck:   expiringFact(in.Now, in.BackgroundCheckExpires, in.GraceDays),
		},
		RoleCredentials: roleCredentials,
	}
}

func expiringFact(now time.Time, expires *time.Time, graceDays int) ExpiringFact {
	fact := ExpiringFact{Valid: safeguarding.Current(now, expires, graceDays)}
	if expires != nil {
		formatted := expires.Format(dateLayout)
		fact.ExpiresAt = &formatted
	}
	return fact
}
