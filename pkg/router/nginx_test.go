package router

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceRouteConfWithValidationRestoresPreviousConfigOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "route.conf")
	previous := []byte("previous route config")
	if err := os.WriteFile(path, previous, 0644); err != nil {
		t.Fatalf("write previous config: %v", err)
	}

	err := replaceRouteConfWithValidation(path, []byte("broken route config"), func() error {
		return errors.New("nginx validation failed")
	})
	if err == nil {
		t.Fatalf("replaceRouteConfWithValidation should fail when validation fails")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored config: %v", err)
	}
	if string(got) != string(previous) {
		t.Fatalf("route config was not restored: got %q, want %q", got, previous)
	}
}

func TestReplaceRouteConfWithValidationCommitsConfigOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "route.conf")
	next := []byte("next route config")

	err := replaceRouteConfWithValidation(path, next, func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("replaceRouteConfWithValidation failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed config: %v", err)
	}
	if string(got) != string(next) {
		t.Fatalf("route config was not committed: got %q, want %q", got, next)
	}
}
