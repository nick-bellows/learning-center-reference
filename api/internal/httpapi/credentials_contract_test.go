package httpapi

// The fixtures under testdata/contracts/learning-center.credentials.v1 are verbatim copies
// of the consumer's reference responses; the source of truth is the
// federation-member-services-lab repository, api/tests/Fixtures/learning-center/credentials.
// Each fixture is turned back into the store record that would produce it, served through
// the router, and compared key for key at every level. Values are compared only where the
// contract fixes them: the contract name, the eligibility status vocabulary, and the
// provider-evaluated valid flags. Reason text is free text and is not compared.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nick-bellows/learning-center-reference/api/internal/credentials"
	"github.com/nick-bellows/learning-center-reference/api/internal/safeguarding"
)

const contractFixtureDir = "../../testdata/contracts/learning-center.credentials.v1"

func TestCredentialsContractFixtures(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join(contractFixtureDir, "*.json"))
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no fixtures found in %s: %v", contractFixtureDir, err)
	}
	for _, path := range fixtures {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var want credentials.Response
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}

			handler := credentialsRouter(fakeCredentials{want.Member.Subject: recordFromFixture(t, want)})
			rec := getCredentials(t, handler,
				"/v1/members/"+url.PathEscape(want.Member.Subject)+"/credentials", "service-token")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d; want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
			}

			var wantShape, gotShape any
			if err := json.Unmarshal(raw, &wantShape); err != nil {
				t.Fatalf("decode fixture shape: %v", err)
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &gotShape); err != nil {
				t.Fatalf("decode response shape: %v", err)
			}
			assertSameShape(t, "$", wantShape, gotShape)

			var got credentials.Response
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got.Contract != want.Contract {
				t.Errorf("contract = %q; want %q", got.Contract, want.Contract)
			}
			if got.Eligibility.Status != want.Eligibility.Status {
				t.Errorf("eligibility.status = %q (%s); want %q", got.Eligibility.Status, got.Eligibility.Reason, want.Eligibility.Status)
			}
			if got.Safeguarding.SafeSportTraining.Valid != want.Safeguarding.SafeSportTraining.Valid ||
				got.Safeguarding.BackgroundCheck.Valid != want.Safeguarding.BackgroundCheck.Valid {
				t.Errorf("safeguarding valid flags = %#v; want %#v", got.Safeguarding, want.Safeguarding)
			}
			for i := range want.RoleCredentials {
				if i < len(got.RoleCredentials) && got.RoleCredentials[i].Valid != want.RoleCredentials[i].Valid {
					t.Errorf("role_credentials[%d].valid = %v; want %v", i, got.RoleCredentials[i].Valid, want.RoleCredentials[i].Valid)
				}
			}
		})
	}
}

// recordFromFixture rebuilds the store record behind a fixture. The role-credential
// reduction mirrors Store.LoadSafeguardingInputs: each credential-requiring role needs
// its latest credential, and the earliest of those latest expiries is the one that counts.
func recordFromFixture(t *testing.T, fixture credentials.Response) credentials.Record {
	t.Helper()
	now, err := time.Parse(time.RFC3339, fixture.AsOf)
	if err != nil {
		t.Fatalf("parse as_of %q: %v", fixture.AsOf, err)
	}
	in := safeguarding.Inputs{
		Now:                    now,
		SafeSportExpires:       parseDate(t, fixture.Safeguarding.SafeSportTraining.ExpiresAt),
		BackgroundCheckExpires: parseDate(t, fixture.Safeguarding.BackgroundCheck.ExpiresAt),
	}
	for _, hold := range fixture.Holds {
		in.ActiveHoldSources = append(in.ActiveHoldSources, hold.Source)
	}

	latest := map[string]*time.Time{}
	rows := make([]credentials.RoleCredential, 0, len(fixture.RoleCredentials))
	for _, c := range fixture.RoleCredentials {
		expires := *parseDate(t, &c.ExpiresAt)
		rows = append(rows, credentials.RoleCredential{
			Role: c.Role, CredentialType: c.CredentialType,
			IssuedAt: *parseDate(t, &c.IssuedAt), ExpiresAt: expires,
		})
		if current := latest[c.Role]; current == nil || expires.After(*current) {
			latest[c.Role] = &expires
		}
	}
	missing := false
	var weakest *time.Time
	for _, role := range fixture.Member.Roles {
		if role != "coach" && role != "referee" {
			continue
		}
		in.RoleCredentialRequired = true
		expires := latest[role]
		if expires == nil {
			missing = true
			continue
		}
		if weakest == nil || expires.Before(*weakest) {
			weakest = expires
		}
	}
	if in.RoleCredentialRequired && !missing {
		in.RoleCredentialExpires = weakest
	}

	return credentials.Record{
		MemberID:        fixture.Member.ID,
		Subject:         fixture.Member.Subject,
		Roles:           fixture.Member.Roles,
		Inputs:          in,
		RoleCredentials: rows,
	}
}

func parseDate(t *testing.T, value *string) *time.Time {
	t.Helper()
	if value == nil {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", *value)
	if err != nil {
		t.Fatalf("parse date %q: %v", *value, err)
	}
	return &parsed
}

// assertSameShape compares decoded JSON structurally: objects must have identical key
// sets, arrays identical lengths with pairwise-identical element shapes, and scalars the
// same JSON type. Values are not compared.
func assertSameShape(t *testing.T, path string, want, got any) {
	t.Helper()
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			t.Errorf("%s: want object, got %T", path, got)
			return
		}
		for key := range w {
			if _, ok := g[key]; !ok {
				t.Errorf("%s: missing key %q", path, key)
			}
		}
		for key := range g {
			if _, ok := w[key]; !ok {
				t.Errorf("%s: unexpected key %q", path, key)
			}
		}
		for key, wv := range w {
			if gv, ok := g[key]; ok {
				assertSameShape(t, path+"."+key, wv, gv)
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok {
			t.Errorf("%s: want array, got %T", path, got)
			return
		}
		if len(g) != len(w) {
			t.Errorf("%s: want %d items, got %d", path, len(w), len(g))
			return
		}
		for i := range w {
			assertSameShape(t, fmt.Sprintf("%s[%d]", path, i), w[i], g[i])
		}
	default:
		if fmt.Sprintf("%T", want) != fmt.Sprintf("%T", got) {
			t.Errorf("%s: want %T, got %T", path, want, got)
		}
	}
}
