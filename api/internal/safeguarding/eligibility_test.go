package safeguarding

import (
	"testing"
	"time"
)

// ptr returns a pointer to t. Struct literals can't take the address of a value directly,
// so this small helper keeps the test cases readable.
func ptr(t time.Time) *time.Time { return &t }

// TestEvaluate is a "table-driven test" — the idiomatic Go style. We list cases in a slice,
// then loop over them and run each as a named subtest. Adding a case is one line.
func TestEvaluate(t *testing.T) {
	now := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	future := now.AddDate(0, 1, 0) // one month from now
	past := now.AddDate(0, -1, 0)  // one month ago

	cases := []struct {
		name string
		in   Inputs
		want Status
	}{
		{
			name: "all current -> eligible",
			in:   Inputs{Now: now, SafeSportExpires: ptr(future), BackgroundCheckExpires: ptr(future), RoleCredentialExpires: ptr(future)},
			want: StatusEligible,
		},
		{
			name: "active hold overrides everything -> suspended",
			in:   Inputs{Now: now, SafeSportExpires: ptr(future), BackgroundCheckExpires: ptr(future), RoleCredentialExpires: ptr(future), ActiveHoldSources: []string{"safesport"}},
			want: StatusSuspended,
		},
		{
			name: "hold beats expired credentials -> suspended",
			in:   Inputs{Now: now, SafeSportExpires: ptr(past), BackgroundCheckExpires: ptr(past), RoleCredentialExpires: ptr(past), ActiveHoldSources: []string{"us_soccer"}},
			want: StatusSuspended,
		},
		{
			name: "expired background check -> ineligible",
			in:   Inputs{Now: now, SafeSportExpires: ptr(future), BackgroundCheckExpires: ptr(past), RoleCredentialExpires: ptr(future)},
			want: StatusIneligible,
		},
		{
			name: "missing safesport -> ineligible",
			in:   Inputs{Now: now, SafeSportExpires: nil, BackgroundCheckExpires: ptr(future), RoleCredentialExpires: ptr(future)},
			want: StatusIneligible,
		},
		{
			name: "within grace window -> eligible",
			in:   Inputs{Now: now, SafeSportExpires: ptr(now.AddDate(0, 0, -5)), BackgroundCheckExpires: ptr(future), RoleCredentialExpires: ptr(future), GraceDays: 10},
			want: StatusEligible,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Evaluate(tc.in)
			if got.Status != tc.want {
				t.Errorf("Evaluate() = %q (%s); want %q", got.Status, got.Reason, tc.want)
			}
		})
	}
}
