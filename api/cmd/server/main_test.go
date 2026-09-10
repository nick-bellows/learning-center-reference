package main

import "testing"

func TestValidateDeploymentConfig(t *testing.T) {
	const tenant = "https://tenant.example.com"
	tests := []struct {
		name      string
		config    deploymentConfig
		wantError bool
	}{
		{"local demo remains available", deploymentConfig{env: "local", authMode: "demo"}, false},
		{"local HTTP OIDC fixture", deploymentConfig{env: "local", authMode: "oidc", databaseURL: "postgres://db", issuer: "http://oidc.localhost"}, false},
		{"public OIDC with runtime role", deploymentConfig{env: "public", authMode: "oidc", databaseURL: "postgres://db", issuer: tenant, runtimeRole: "lcr_runtime"}, false},
		{"public OIDC with runtime connection", deploymentConfig{env: "public", authMode: "oidc", databaseURL: "postgres://db", issuer: tenant, runtimeURL: "postgres://app@db"}, false},
		{"unknown deployment", deploymentConfig{env: "staging", authMode: "oidc", databaseURL: "postgres://db", issuer: tenant, runtimeRole: "lcr_runtime"}, true},
		{"public demo rejected", deploymentConfig{env: "public", authMode: "demo", databaseURL: "postgres://db", issuer: tenant, runtimeRole: "lcr_runtime"}, true},
		{"public missing database", deploymentConfig{env: "public", authMode: "oidc", issuer: tenant, runtimeRole: "lcr_runtime"}, true},
		{"public HTTP issuer rejected", deploymentConfig{env: "public", authMode: "oidc", databaseURL: "postgres://db", issuer: "http://tenant.example.com", runtimeRole: "lcr_runtime"}, true},
		{"public owner-only database rejected", deploymentConfig{env: "public", authMode: "oidc", databaseURL: "postgres://db", issuer: tenant}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateDeploymentConfig(test.config)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError = %v", err, test.wantError)
			}
		})
	}
}
