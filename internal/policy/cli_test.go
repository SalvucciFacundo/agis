package policy

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI executes one CLI invocation against a temp AGIS_HOME.
func runCLI(t *testing.T, home string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("AGIS_HOME", home)
	var out, errW bytes.Buffer
	code := RunCLI(args, &out, &errW)
	return code, out.String(), errW.String()
}

func TestCLI_RoundTrip(t *testing.T) {
	home := t.TempDir()

	// init
	code, out, errOut := runCLI(t, home, "init")
	if code != 0 {
		t.Fatalf("init: code=%d out=%q err=%q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(home, "policy.yaml")); err != nil {
		t.Fatalf("policy.yaml not created: %v", err)
	}

	// init refuses overwrite without --force
	if code, _, errOut := runCLI(t, home, "init"); code == 0 || !strings.Contains(errOut, "already exists") {
		t.Errorf("second init: code=%d err=%q; want refusal", code, errOut)
	}
	if code, _, _ := runCLI(t, home, "init", "--force"); code != 0 {
		t.Error("init --force refused")
	}

	// set for one backend
	if code, _, errOut := runCLI(t, home, "set", "-backend", "local", "commands", "git status", "allow"); code != 0 {
		t.Fatalf("set: code=%d err=%q", code, errOut)
	}
	// set across all backends
	if code, _, errOut := runCLI(t, home, "set", "network", "api.example.com", "allow"); code != 0 {
		t.Fatalf("set all backends: code=%d err=%q", code, errOut)
	}

	// show reflects both rules and sandbox tiers
	code, out, _ = runCLI(t, home, "show")
	if code != 0 {
		t.Fatalf("show: code=%d", code)
	}
	for _, want := range []string{"local", "sandbox", "git status", "allow", "api.example.com"} {
		if !strings.Contains(out, want) {
			t.Errorf("show output missing %q:\n%s", want, out)
		}
	}

	// escalate local to standard so unlisted subjects ask
	if code, _, _ := runCLI(t, home, "tier", "local", "standard"); code != 0 {
		t.Fatal("tier standard failed")
	}

	// test previews decisions without executing
	code, out, _ = runCLI(t, home, "test", "-backend", "local", "git", "status")
	if code != 0 || strings.TrimSpace(out) != "allow" {
		t.Errorf("test allow: code=%d out=%q; want allow", code, out)
	}
	code, out, _ = runCLI(t, home, "test", "-backend", "local", "make", "build")
	if code != 0 || strings.TrimSpace(out) != "ask" {
		t.Errorf("test standard-ask: code=%d out=%q; want ask", code, out)
	}

	// rm removes from every backend
	if code, _, _ := runCLI(t, home, "rm", "network", "api.example.com"); code != 0 {
		t.Fatal("rm failed")
	}
	s, err := Load(filepath.Join(home, "policy.yaml"))
	if err != nil {
		t.Fatalf("reload after rm: %v", err)
	}
	rules, _ := s.Rules(context.Background())
	network := 0
	for _, r := range rules {
		if r.Category == "network" {
			network++
		}
	}
	if network != 0 {
		t.Errorf("%d network rules survived rm", network)
	}
}

func TestCLI_TierRefusesFull(t *testing.T) {
	home := t.TempDir()
	runCLI(t, home, "init")

	code, _, errOut := runCLI(t, home, "tier", "local", "full")
	if code == 0 || !strings.Contains(errOut, "session-only") {
		t.Errorf("tier full: code=%d err=%q; want session-only guidance refusal", code, errOut)
	}

	if code, _, _ := runCLI(t, home, "tier", "local", "standard"); code != 0 {
		t.Error("tier standard refused")
	}
}

func TestCLI_UnknownSubcommandAndUsage(t *testing.T) {
	home := t.TempDir()
	if code, _, errOut := runCLI(t, home, "bogus"); code == 0 || !strings.Contains(errOut, "unknown subcommand") {
		t.Errorf("bogus: code=%d err=%q; want unknown subcommand", code, errOut)
	}
	if code, _, errOut := runCLI(t, home); code == 0 || !strings.Contains(errOut, "usage:") {
		t.Errorf("bare: code=%d err=%q; want usage", code, errOut)
	}
}

func TestCLI_CorruptPolicyFailsClosed(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "policy.yaml"), []byte("rules: [broken\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Fail closed means the decision is deny while the parse error surfaces.
	code, out, errOut := runCLI(t, home, "test", "ls")
	if code != 0 || strings.TrimSpace(out) != "deny" {
		t.Errorf("test on corrupt policy: code=%d out=%q; want 0/deny (fail closed)", code, out)
	}
	if !strings.Contains(errOut, "policy") {
		t.Errorf("err = %q, want the parse error surfaced", errOut)
	}
	// show reports the broken store too.
	if code, _, errOut := runCLI(t, home, "show"); code == 0 {
		t.Errorf("show on corrupt policy: code=0 err=%q; want failure", errOut)
	} else if !strings.Contains(errOut, "policy") {
		t.Errorf("show err = %q, want the parse error surfaced", errOut)
	}
}
