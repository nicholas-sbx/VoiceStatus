package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigPermissions(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode os.FileMode
		ok   bool
	}{
		{"private", 0o600, true},
		{"owner read only", 0o400, true},
		{"group readable", 0o640, false},
		{"other readable", 0o604, false},
		{"group writable", 0o620, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			path := filepath.Join(dir, "VoiceStatus.json")
			contents := `{"TOKEN":"example"}`
			if !tc.ok {
				contents = "invalid JSON"
			}
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, tc.mode); err != nil {
				t.Fatal(err)
			}
			cfg, err := loadConfig()
			if tc.ok {
				if err != nil || cfg.Token != "example" {
					t.Fatalf("private config did not load: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "chmod 600") {
				t.Fatalf("expected permission error before parsing, got %v", err)
			}
		})
	}
}
