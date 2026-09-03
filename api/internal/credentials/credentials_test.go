package credentials

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func ptr(t time.Time) *time.Time { return &t }

func TestBuildDerivesValidityAndEligibilityFromTheSameInstant(t *testing.T) {
	now := time.Date(2026, 9, 3, 5, 0, 0, 0, time.UTC)
	record := Record{
		MemberID: "33333333-3333-3333-3333-333333333333",
		Subject:  "demo|referee-riley",
		Roles:    []string{"referee"},
		Inputs: safeguarding.Inputs{
			Now:                    now,
			SafeSportExpires:       ptr(date(2027, 6, 1)),
			BackgroundCheckExpires: ptr(date(2028, 1, 1)),
			RoleCredentialRequired: true,
			RoleCredentialExpires:  ptr(date(2025, 1, 15)),
		},
		RoleCredentials: []RoleCredential{
			{Role: "referee", CredentialType: "referee_recert", IssuedAt: date(2024, 1, 15), ExpiresAt: date(2025, 1, 15)},
		},
	}

	got := Build(record)

	if got.Contract != Contract {
		t.Errorf("contract = %q; want %q", got.Contract, Contract)
	}
	if got.AsOf != "2026-09-03T05:00:00Z" {
		t.Errorf("as_of = %q; want RFC 3339 UTC of the evaluation instant", got.AsOf)
	}
	if got.Eligibility.Status != safeguarding.StatusIneligible {
		t.Errorf("status = %q; want %q", got.Eligibility.Status, safeguarding.StatusIneligible)
	}
	if len(got.Holds) != 0 {
		t.Errorf("holds = %#v; want none", got.Holds)
	}
	if !got.Safeguarding.SafeSportTraining.Valid || *got.Safeguarding.SafeSportTraining.ExpiresAt != "2027-06-01" {
		t.Errorf("safesport = %#v", got.Safeguarding.SafeSportTraining)
	}
	if !got.Safeguarding.BackgroundCheck.Valid || *got.Safeguarding.BackgroundCheck.ExpiresAt != "2028-01-01" {
		t.Errorf("background check = %#v", got.Safeguarding.BackgroundCheck)
	}
	if len(got.RoleCredentials) != 1 || got.RoleCredentials[0].Valid ||
		got.RoleCredentials[0].IssuedAt != "2024-01-15" || got.RoleCredentials[0].ExpiresAt != "2025-01-15" {
		t.Errorf("role credentials = %#v", got.RoleCredentials)
	}
}

func TestBuildReportsActiveHoldsWithoutReasons(t *testing.T) {
	got := Build(Record{
		Roles: []string{"referee"},
		Inputs: safeguarding.Inputs{
			Now:                    time.Date(2026, 9, 3, 5, 0, 0, 0, time.UTC),
			SafeSportExpires:       ptr(date(2027, 6, 1)),
			BackgroundCheckExpires: ptr(date(2028, 1, 1)),
			ActiveHoldSources:      []string{"safesport"},
			RoleCredentialRequired: true,
			RoleCredentialExpires:  ptr(date(2027, 1, 15)),
		},
	})
	if got.Eligibility.Status != safeguarding.StatusSuspended {
		t.Errorf("status = %q; want %q", got.Eligibility.Status, safeguarding.StatusSuspended)
	}
	if len(got.Holds) != 1 || got.Holds[0].Source != "safesport" || !got.Holds[0].Active {
		t.Errorf("holds = %#v", got.Holds)
	}
}

// Empty collections and a missing credential must serialize as [] and null, never be
// omitted: the consumer relies on every key being present.
func TestBuildKeepsEveryKeyWhenFactsAreMissing(t *testing.T) {
	body, err := json.Marshal(Build(Record{Inputs: safeguarding.Inputs{Now: time.Now()}}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"contract", "member", "as_of", "eligibility", "holds", "safeguarding", "role_credentials"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("key %q missing from %s", key, body)
		}
	}
	if holds, ok := decoded["holds"].([]any); !ok || len(holds) != 0 {
		t.Errorf("holds = %v; want []", decoded["holds"])
	}
	if roles, ok := decoded["member"].(map[string]any)["roles"].([]any); !ok || len(roles) != 0 {
		t.Errorf("member.roles = %v; want []", decoded["member"])
	}
	safesport := decoded["safeguarding"].(map[string]any)["safesport_training"].(map[string]any)
	if expires, present := safesport["expires_at"]; !present || expires != nil || safesport["valid"] != false {
		t.Errorf("safesport_training = %v; want expires_at null and valid false", safesport)
	}
}
