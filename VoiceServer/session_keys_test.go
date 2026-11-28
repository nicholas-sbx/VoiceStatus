package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionKeysArePrivateAndRotate(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", dir)
	first, err := publishSessionKeys(5462)
	if err != nil {
		t.Fatal(err)
	}
	second, err := publishSessionKeys(5462)
	if err != nil {
		t.Fatal(err)
	}
	if first.client == second.client || first.server == second.server || second.client == second.server {
		t.Fatal("expected independent, freshly generated keys")
	}
	path := filepath.Join(dir, "voicestatus-5462.ini")
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("credential file has mode %o", fi.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != fmt.Sprintf("[General]\nkeys=%s.%s\n", second.client, second.server) {
		t.Fatal("credential file did not contain the current key pair")
	}
}

func TestSessionKeysRequirePrivateRuntimeDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", dir)
	if _, err := publishSessionKeys(5462); err == nil || !strings.Contains(err.Error(), "0700") {
		t.Fatalf("expected a private-directory error, got %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("unexpected files in public directory", err)
	}
}
