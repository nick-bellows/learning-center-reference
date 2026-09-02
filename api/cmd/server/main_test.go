package main

import "testing"

func TestValidateDeploymentConfig(t *testing.T) {
	tests := []struct {
		name       string
		deployment string
		auth       string
		database   string
		issuer     string
		wantError  bool
	}{
		{"local demo remains available", "local", "demo", "", "", false},
		{"local HTTP OIDC fixture", "local", "oidc", "postgres://db", "http://oidc.localhost", false},
		{"public OIDC", "public", "oidc", "postgres://db", "https://tenant.example.com", false},
		{"unknown deployment", "staging", "oidc", "postgres://db", "https://tenant.example.com", true},
		{"public demo rejected", "public", "demo", "postgres://db", "https://tenant.example.com", true},
		{"public missing database", "public", "oidc", "", "https://tenant.example.com", true},
		{"public HTTP issuer rejected", "public", "oidc", "postgres://db", "http://tenant.example.com", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateDeploymentConfig(test.deployment, test.auth, test.database, test.issuer)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError = %v", err, test.wantError)
			}
		})
	}
}
