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
			in:   Inputs{Now: now, SafeSportExpires: ptr(future), BackgroundCheckExpires: ptr(future)},
			want: StatusEligible,
		},
		{
			name: "active hold overrides everything -> suspended",
			in:   Inputs{Now: now, SafeSportExpires: ptr(future), BackgroundCheckExpires: ptr(future), ActiveHoldSources: []string{"safesport"}},
			want: StatusSuspended,
		},
		{
			name: "hold beats expired credentials -> suspended",
			in:   Inputs{Now: now, SafeSportExpires: ptr(past), BackgroundCheckExpires: ptr(past), ActiveHoldSources: []string{"us_soccer"}},
			want: StatusSuspended,
		},
		{
			name: "expired background check -> ineligible",
			in:   Inputs{Now: now, SafeSportExpires: ptr(future), BackgroundCheckExpires: ptr(past)},
			want: StatusIneligible,
		},
		{
			name: "missing safesport -> ineligible",
			in:   Inputs{Now: now, SafeSportExpires: nil, BackgroundCheckExpires: ptr(future)},
			want: StatusIneligible,
		},
		{
			name: "within grace window -> eligible",
			in:   Inputs{Now: now, SafeSportExpires: ptr(now.AddDate(0, 0, -5)), BackgroundCheckExpires: ptr(future), GraceDays: 10},
			want: StatusEligible,
		},
		{
			name: "role credential required + current -> eligible",
			in:   Inputs{Now: now, SafeSportExpires: ptr(future), BackgroundCheckExpires: ptr(future), RoleCredentialRequired: true, RoleCredentialExpires: ptr(future)},
			want: StatusEligible,
		},
		{
			name: "role credential required + expired -> ineligible",
			in:   Inputs{Now: now, SafeSportExpires: ptr(future), BackgroundCheckExpires: ptr(future), RoleCredentialRequired: true, RoleCredentialExpires: ptr(past)},
			want: StatusIneligible,
		},
		{
			// Expiry dates are inclusive: expires_at = today means valid
			// through the whole of today (UTC), even at 23:59:59.
			name: "still valid late on the expiry date itself -> eligible",
			in: Inputs{
				Now:                    time.Date(2026, 8, 8, 23, 59, 59, 0, time.UTC),
				SafeSportExpires:       ptr(time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)),
				BackgroundCheckExpires: ptr(future),
			},
			want: StatusEligible,
		},
		{
			// ...and flips to expired exactly at midnight the day after.
			name: "expired at midnight after the expiry date -> ineligible",
			in: Inputs{
				Now:                    time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC),
				SafeSportExpires:       ptr(time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)),
				BackgroundCheckExpires: ptr(future),
			},
			want: StatusIneligible,
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

// TestCurrent pins the shared expiry rule at its boundaries. The credentials contract
// reports one valid flag per credential through this function, so a change here moves
// both the public eligibility decision and the contract together.
func TestCurrent(t *testing.T) {
	expires := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		now       time.Time
		expires   *time.Time
		graceDays int
		want      bool
	}{
		{"nothing on file", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil, 0, false},
		{"well before expiry", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ptr(expires), 0, true},
		{"last second of the expiry day", time.Date(2026, 8, 8, 23, 59, 59, 0, time.UTC), ptr(expires), 0, true},
		{"midnight after the expiry day", time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC), ptr(expires), 0, false},
		{"inside the grace window", time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC), ptr(expires), 5, true},
		{"grace window exhausted", time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC), ptr(expires), 5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Current(tc.now, tc.expires, tc.graceDays); got != tc.want {
				t.Errorf("Current() = %v; want %v", got, tc.want)
			}
		})
	}
}
