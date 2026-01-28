// internal/daemon/server/ante/errors_test.go
package ante

import (
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
)

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "spec.network",
		Code:    CodeRequired,
		Message: "network is required",
	}

	got := err.Error()
	want := "spec.network: network is required"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestValidationError_GRPCCode(t *testing.T) {
	tests := []struct {
		code string
		want codes.Code
	}{
		{CodeRequired, codes.InvalidArgument},
		{CodeInvalidRange, codes.InvalidArgument},
		{CodeNotFound, codes.NotFound},
	}

	for _, tt := range tests {
		err := &ValidationError{Code: tt.code}
		if got := err.GRPCCode(); got != tt.want {
			t.Errorf("GRPCCode() for %s = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestMultiValidationError_Error(t *testing.T) {
	err := &MultiValidationError{
		Errors: []*ValidationError{
			{Field: "name", Code: CodeRequired, Message: "name is required"},
			{Field: "spec.mode", Code: CodeInvalidValue, Message: "mode must be 'docker' or 'local'"},
		},
	}

	got := err.Error()
	if got == "" {
		t.Error("Error() returned empty string")
	}
	// Should contain both errors
	if !strings.Contains(got, "name: name is required") {
		t.Error("Error() missing first error")
	}
	if !strings.Contains(got, "spec.mode: mode must be 'docker' or 'local'") {
		t.Error("Error() missing second error")
	}
}

func TestToError_Nil(t *testing.T) {
	if err := toError(nil); err != nil {
		t.Errorf("toError(nil) = %v, want nil", err)
	}
}

func TestToError_Single(t *testing.T) {
	errs := []*ValidationError{{Field: "name", Code: CodeRequired, Message: "required"}}
	err := toError(errs)
	if _, ok := err.(*ValidationError); !ok {
		t.Errorf("toError with single error should return *ValidationError, got %T", err)
	}
}

func TestToError_Multiple(t *testing.T) {
	errs := []*ValidationError{
		{Field: "name", Code: CodeRequired, Message: "required"},
		{Field: "spec", Code: CodeRequired, Message: "required"},
	}
	err := toError(errs)
	if _, ok := err.(*MultiValidationError); !ok {
		t.Errorf("toError with multiple errors should return *MultiValidationError, got %T", err)
	}
}
