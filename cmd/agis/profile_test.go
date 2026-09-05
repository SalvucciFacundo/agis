package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestRunProfileCLI_Help(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "empty args", args: []string{}},
		{name: "help flag", args: []string{"--help"}},
		{name: "short help", args: []string{"-h"}},
		{name: "help command", args: []string{"help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunProfileCLI(tt.args, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Usage: agis profile") {
				t.Errorf("stdout missing usage info: %s", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunProfileCLI_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunProfileCLI([]string{"unknown_cmd"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Errorf("stderr = %q, want unknown subcommand error", stderr.String())
	}
}

func TestRunProfileCLI_List(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	// Create a profile to list
	if err := config.CreateProfile("work", ""); err != nil {
		t.Fatalf("CreateProfile error: %v", err)
	}

	t.Run("table output", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"list"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "default") || !strings.Contains(out, "work") {
			t.Errorf("stdout missing profiles: %s", out)
		}
	})

	t.Run("json output", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"list", "-json"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		var list []config.ProfileInfo
		if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
			t.Fatalf("unmarshal json error: %v (%s)", err, stdout.String())
		}
		if len(list) < 2 {
			t.Errorf("expected at least 2 profiles, got %d", len(list))
		}
	})
}

func TestRunProfileCLI_Create(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	t.Run("missing name argument fails with code 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"create"}, &stdout, &stderr)

		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
		if !strings.Contains(stderr.String(), "requires a profile name") {
			t.Errorf("stderr = %q, want profile name required", stderr.String())
		}
	})

	t.Run("create valid profile succeeds", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"create", "dev"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Profile 'dev' created") {
			t.Errorf("stdout = %q, want created confirmation", stdout.String())
		}

		profileDir := filepath.Join(home, "profiles", "dev")
		if _, err := os.Stat(profileDir); err != nil {
			t.Fatalf("profile directory not found: %v", err)
		}
	})

	t.Run("create duplicate profile fails with code 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"create", "dev"}, &stdout, &stderr)

		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "already exists") {
			t.Errorf("stderr = %q, want already exists error", stderr.String())
		}
	})

	t.Run("create with -clone flag clones files but not db", func(t *testing.T) {
		// Populate custom files in dev
		soulPath := filepath.Join(home, "profiles", "dev", "SOUL.md")
		if err := os.WriteFile(soulPath, []byte("Custom Soul"), 0o600); err != nil {
			t.Fatalf("WriteFile soul error: %v", err)
		}
		dbPath := filepath.Join(home, "profiles", "dev", "agis.db")
		if err := os.WriteFile(dbPath, []byte("sqlite db data"), 0o600); err != nil {
			t.Fatalf("WriteFile db error: %v", err)
		}

		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"create", "dev-clone", "-clone", "dev"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}

		clonedSoul := filepath.Join(home, "profiles", "dev-clone", "SOUL.md")
		data, err := os.ReadFile(clonedSoul)
		if err != nil || string(data) != "Custom Soul" {
			t.Errorf("cloned soul mismatch: %s (err: %v)", string(data), err)
		}

		clonedDB := filepath.Join(home, "profiles", "dev-clone", "agis.db")
		if _, err := os.Stat(clonedDB); err == nil {
			t.Errorf("cloned db should not exist, but was found")
		}
	})
}

func TestRunProfileCLI_Show(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	if err := config.CreateProfile("prod", ""); err != nil {
		t.Fatalf("CreateProfile error: %v", err)
	}

	t.Run("show active profile", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"show"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "default") {
			t.Errorf("stdout = %q, want active profile default", stdout.String())
		}
	})

	t.Run("show named profile", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"show", "prod"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "prod") {
			t.Errorf("stdout = %q, want profile prod info", stdout.String())
		}
	})

	t.Run("show named profile with -json", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"show", "prod", "-json"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		var info config.ProfilePaths
		if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
			t.Fatalf("unmarshal json error: %v (%s)", err, stdout.String())
		}
		if info.ActiveProfileName != "prod" {
			t.Errorf("ActiveProfileName = %q, want prod", info.ActiveProfileName)
		}
	})

	t.Run("show nonexistent profile fails with code 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"show", "ghost"}, &stdout, &stderr)

		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "does not exist") {
			t.Errorf("stderr = %q, want does not exist error", stderr.String())
		}
	})
}

func TestRunProfileCLI_UseAndSwitch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	if err := config.CreateProfile("staging", ""); err != nil {
		t.Fatalf("CreateProfile error: %v", err)
	}

	t.Run("missing name argument fails with code 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"use"}, &stdout, &stderr)

		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
	})

	t.Run("switch to existing profile", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"switch", "staging"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Switched active profile to 'staging'") {
			t.Errorf("stdout = %q, want switch confirmation", stdout.String())
		}

		pointerPath := filepath.Join(home, ".active_profile")
		data, err := os.ReadFile(pointerPath)
		if err != nil || strings.TrimSpace(string(data)) != "staging" {
			t.Errorf("pointer content = %q, want staging", string(data))
		}
	})

	t.Run("switch to default resets pointer", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"use", "default"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "default") {
			t.Errorf("stdout = %q, want default confirmation", stdout.String())
		}
	})

	t.Run("switch to nonexistent fails with code 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"use", "missing"}, &stdout, &stderr)

		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "does not exist") {
			t.Errorf("stderr = %q, want does not exist error", stderr.String())
		}
	})
}

func TestRunProfileCLI_Delete(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	if err := config.CreateProfile("ephemeral", ""); err != nil {
		t.Fatalf("CreateProfile error: %v", err)
	}

	t.Run("missing name argument fails with code 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"delete"}, &stdout, &stderr)

		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
	})

	t.Run("delete default profile fails with code 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"delete", "default"}, &stdout, &stderr)

		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "cannot delete default") {
			t.Errorf("stderr = %q, want cannot delete default", stderr.String())
		}
	})

	t.Run("delete active profile without -force fails with code 1", func(t *testing.T) {
		if err := config.SwitchProfile("ephemeral"); err != nil {
			t.Fatalf("SwitchProfile error: %v", err)
		}

		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"delete", "ephemeral"}, &stdout, &stderr)

		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "currently active") {
			t.Errorf("stderr = %q, want currently active warning", stderr.String())
		}
	})

	t.Run("delete active profile with -force succeeds and resets pointer", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunProfileCLI([]string{"delete", "ephemeral", "-force"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Deleted profile 'ephemeral'") {
			t.Errorf("stdout = %q, want deletion confirmation", stdout.String())
		}

		profileDir := filepath.Join(home, "profiles", "ephemeral")
		if _, err := os.Stat(profileDir); !os.IsNotExist(err) {
			t.Errorf("profile dir should be deleted, stat err: %v", err)
		}
	})
}
