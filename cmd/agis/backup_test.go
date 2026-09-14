package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestRunBackupCLI_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunBackupCLI([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage: agis backup") {
		t.Errorf("expected usage message, got: %s", stdout.String())
	}
}

func TestRunRestoreCLI_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunRestoreCLI([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage: agis restore") {
		t.Errorf("expected usage message, got: %s", stdout.String())
	}
}

func TestRunRestoreCLI_MissingArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunRestoreCLI([]string{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing archive path argument") {
		t.Errorf("expected missing archive path error, got: %s", stderr.String())
	}
}

func TestRunBackupAndRestoreCLI_Roundtrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	// Create a profile with some content
	if err := config.CreateProfile("dev", ""); err != nil {
		t.Fatalf("CreateProfile error: %v", err)
	}
	devDir := config.ProfileDir("dev")
	if err := os.WriteFile(filepath.Join(devDir, "config.yaml"), []byte("llm:\n  provider: test\n"), 0600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(devDir, "SOUL.md"), []byte("# Dev Soul\n"), 0600); err != nil {
		t.Fatalf("writing SOUL.md: %v", err)
	}

	backupFile := filepath.Join(home, "dev-backup.tar.gz")

	// 1. Run backup
	var bStdout, bStderr bytes.Buffer
	bCode := RunBackupCLI([]string{"dev", "-o", backupFile}, &bStdout, &bStderr)
	if bCode != 0 {
		t.Fatalf("backup failed with code %d, stderr: %s", bCode, bStderr.String())
	}
	if !strings.Contains(bStdout.String(), "Backup complete") {
		t.Errorf("expected 'Backup complete' in output, got: %s", bStdout.String())
	}
	if _, err := os.Stat(backupFile); err != nil {
		t.Fatalf("backup archive not created: %v", err)
	}

	// 2. Restore to a new profile "dev-restored"
	var rStdout, rStderr bytes.Buffer
	rCode := RunRestoreCLI([]string{backupFile, "-profile", "dev-restored"}, &rStdout, &rStderr)
	if rCode != 0 {
		t.Fatalf("restore failed with code %d, stderr: %s", rCode, rStderr.String())
	}
	if !strings.Contains(rStdout.String(), "Profile restored successfully") {
		t.Errorf("expected success message, got: %s", rStdout.String())
	}

	restoredDir := config.ProfileDir("dev-restored")
	cfgContent, err := os.ReadFile(filepath.Join(restoredDir, "config.yaml"))
	if err != nil || !strings.Contains(string(cfgContent), "provider: test") {
		t.Fatalf("restored config.yaml content invalid: %s (err: %v)", string(cfgContent), err)
	}
	soulContent, err := os.ReadFile(filepath.Join(restoredDir, "SOUL.md"))
	if err != nil || !strings.Contains(string(soulContent), "Dev Soul") {
		t.Fatalf("restored SOUL.md content invalid: %s (err: %v)", string(soulContent), err)
	}

	// 3. Attempt restore without -force over existing target (dev-restored)
	var rStdout2, rStderr2 bytes.Buffer
	rCode2 := RunRestoreCLI([]string{backupFile, "-profile", "dev-restored"}, &rStdout2, &rStderr2)
	if rCode2 != 1 {
		t.Fatalf("expected restore over existing to fail with code 1, got %d", rCode2)
	}
	if !strings.Contains(rStderr2.String(), "already exists") {
		t.Errorf("expected 'already exists' in stderr, got: %s", rStderr2.String())
	}

	// 4. Overwrite with -force
	var rStdout3, rStderr3 bytes.Buffer
	rCode3 := RunRestoreCLI([]string{backupFile, "-profile", "dev-restored", "-force"}, &rStdout3, &rStderr3)
	if rCode3 != 0 {
		t.Fatalf("restore with -force failed with code %d, stderr: %s", rCode3, rStderr3.String())
	}
}

func TestRunProfileCLI_BackupAndRestoreAliases(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	if err := config.CreateProfile("alias-test", ""); err != nil {
		t.Fatalf("CreateProfile error: %v", err)
	}
	profileDir := config.ProfileDir("alias-test")
	if err := os.WriteFile(filepath.Join(profileDir, "config.yaml"), []byte("llm:\n  provider: alias\n"), 0600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	backupFile := filepath.Join(home, "alias.tar.gz")

	// profile backup alias
	var bStdout, bStderr bytes.Buffer
	code := RunProfileCLI([]string{"backup", "alias-test", "-o", backupFile}, &bStdout, &bStderr)
	if code != 0 {
		t.Fatalf("profile backup failed with code %d, stderr: %s", code, bStderr.String())
	}
	if !strings.Contains(bStdout.String(), "Backup complete") {
		t.Errorf("expected 'Backup complete' in output, got: %s", bStdout.String())
	}

	// profile restore alias
	var rStdout, rStderr bytes.Buffer
	code = RunProfileCLI([]string{"restore", backupFile, "-profile", "alias-restored"}, &rStdout, &rStderr)
	if code != 0 {
		t.Fatalf("profile restore failed with code %d, stderr: %s", code, rStderr.String())
	}
	if !strings.Contains(rStdout.String(), "Profile restored successfully") {
		t.Errorf("expected success message, got: %s", rStdout.String())
	}
}

func TestRunBackupCLI_DefaultProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	defaultDir := config.ProfileDir("default")
	if err := os.MkdirAll(defaultDir, 0700); err != nil {
		t.Fatalf("creating default dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defaultDir, "config.yaml"), []byte("llm:\n  provider: default\n"), 0600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	outFile := filepath.Join(home, "default-out.tar.gz")
	var stdout, stderr bytes.Buffer
	code := RunBackupCLI([]string{"-o", outFile}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("backup default profile failed: %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Profile:         default") {
		t.Errorf("expected profile default in output, got: %s", stdout.String())
	}
}

func TestRunRestoreCLI_Errors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	// Nonexistent file
	var stdout, stderr bytes.Buffer
	code := RunRestoreCLI([]string{filepath.Join(home, "nonexistent.tar.gz")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 for nonexistent file, got %d", code)
	}

	// Corrupt file
	corruptFile := filepath.Join(home, "corrupt.tar.gz")
	_ = os.WriteFile(corruptFile, []byte("not a gzip"), 0600)
	stdout.Reset()
	stderr.Reset()
	code = RunRestoreCLI([]string{corruptFile}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 for corrupt file, got %d", code)
	}
}

