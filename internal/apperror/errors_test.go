package apperror

import (
	"errors"
	"testing"
)

func TestAppError_ConstructorsAndMethods(t *testing.T) {
	root := errors.New("root")

	tests := []struct {
		name    string
		err     *AppError
		wantMsg string
		wantCod int
	}{
		{"not found", NewNotFound("missing"), "missing", 404},
		{"conflict", NewConflict("conflict"), "conflict", 409},
		{"bad request", NewBadRequest("bad"), "bad", 400},
		{"forbidden", NewForbidden("no"), "no", 403},
		{"internal", NewInternal("internal", root), "internal: root", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCod {
				t.Fatalf("Code=%d want=%d", tt.err.Code, tt.wantCod)
			}
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Fatalf("Error()=%q want=%q", got, tt.wantMsg)
			}
		})
	}

	internalErr := NewInternal("internal", root)
	if !errors.Is(internalErr.Unwrap(), root) {
		t.Fatalf("expected unwrap root error")
	}
}
