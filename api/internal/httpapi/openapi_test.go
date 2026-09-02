package httpapi

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// The OpenAPI file is an executable contract: CI parses it, resolves references,
// and applies the library's semantic validation on every change.
func TestOpenAPIContract(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile("../../openapi.yaml")
	if err != nil {
		t.Fatalf("load OpenAPI: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI: %v", err)
	}
}
