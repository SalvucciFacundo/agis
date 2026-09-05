package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestValidateProfileName(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		wantErr bool
	}{
		{name: "valid simple", profile: "work", wantErr: false},
		{name: "valid with underscore and hyphen", profile: "dev_env-1", wantErr: false},
		{name: "valid max length 32", profile: strings.Repeat("a", 32), wantErr: false},
		{name: "valid numbers", profile: "profile123", wantErr: false},
		{name: "invalid empty", profile: "", wantErr: true},
		{name: "invalid too long 33 chars", profile: strings.Repeat("a", 33), wantErr: true},
		{name: "invalid path traversal dot dot", profile: "../etc", wantErr: true},
		{name: "invalid forward slash", profile: "work/project", wantErr: true},
		{name: "invalid backward slash", profile: "work\\project", wantErr: true},
		{name: "invalid space", profile: "work project", wantErr: true},
		{name: "invalid dot", profile: "work.profile", wantErr: true},
		{name: "invalid special characters", profile: "work@home!", wantErr: true},
		{name: "invalid non-ascii", profile: "perfilñ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateProfileName(tt.profile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfileName(%q) error = %v, wantErr = %v", tt.profile, err, tt.wantErr)
			}
		})
	}
}

func TestResolveProfilePaths_Precedence(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGIS_HOME", tmpDir)

	mgr := config.NewProfileManager(tmpDir)

	// Create profile directories for testing
	if err := mgr.Create("env_prof", ""); err != nil {
		t.Fatalf("creating env_prof: %v", err)
	}
	if err := mgr.Create("file_prof", ""); err != nil {
		t.Fatalf("creating file_prof: %v", err)
	}
	if err := mgr.Create("flag_prof", ""); err != nil {
		t.Fatalf("creating flag_prof: %v", err)
	}

	// Case 1: Default root when nothing is set
	t.Setenv("AGIS_PROFILE", "")
	paths := mgr.ResolveProfilePaths("")
	if !paths.IsDefault || paths.ActiveProfileName != "default" {
		t.Errorf("expected default profile, got %+v", paths)
	}
	if paths.HomeDir != tmpDir {
		t.Errorf("expected HomeDir %s, got %s", tmpDir, paths.HomeDir)
	}
	if paths.ConfigFile != filepath.Join(tmpDir, "config.yaml") {
		t.Errorf("expected config file %s, got %s", filepath.Join(tmpDir, "config.yaml"), paths.ConfigFile)
	}
	if paths.DBFile != filepath.Join(tmpDir, "agis.db") {
		t.Errorf("expected DB file %s, got %s", filepath.Join(tmpDir, "agis.db"), paths.DBFile)
	}

	// Case 2: .active_profile pointer file set to "file_prof"
	if err := mgr.Switch("file_prof"); err != nil {
		t.Fatalf("switching to file_prof: %v", err)
	}
	paths = mgr.ResolveProfilePaths("")
	if paths.IsDefault || paths.ActiveProfileName != "file_prof" {
		t.Errorf("expected file_prof, got %+v", paths)
	}
	expectedHome := filepath.Join(tmpDir, "profiles", "file_prof")
	if paths.HomeDir != expectedHome {
		t.Errorf("expected HomeDir %s, got %s", expectedHome, paths.HomeDir)
	}
	if paths.DBFile != filepath.Join(expectedHome, "agis.db") {
		t.Errorf("expected DB file in profile dir, got %s", paths.DBFile)
	}

	// Case 3: AGIS_PROFILE env overrides .active_profile file
	t.Setenv("AGIS_PROFILE", "env_prof")
	paths = mgr.ResolveProfilePaths("")
	if paths.IsDefault || paths.ActiveProfileName != "env_prof" {
		t.Errorf("expected env_prof, got %+v", paths)
	}
	expectedHome = filepath.Join(tmpDir, "profiles", "env_prof")
	if paths.HomeDir != expectedHome {
		t.Errorf("expected HomeDir %s, got %s", expectedHome, paths.HomeDir)
	}

	// Case 4: Explicit flag override takes highest precedence
	paths = mgr.ResolveProfilePaths("flag_prof")
	if paths.IsDefault || paths.ActiveProfileName != "flag_prof" {
		t.Errorf("expected flag_prof, got %+v", paths)
	}
	expectedHome = filepath.Join(tmpDir, "profiles", "flag_prof")
	if paths.HomeDir != expectedHome {
		t.Errorf("expected HomeDir %s, got %s", expectedHome, paths.HomeDir)
	}
}

func TestProfileManager_LifecycleAndCloning(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGIS_HOME", tmpDir)
	mgr := config.NewProfileManager(tmpDir)

	// 1. List initial profiles (should contain default)
	profiles, err := mgr.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "default" || !profiles[0].IsActive {
		t.Errorf("unexpected initial profiles: %+v", profiles)
	}

	// 2. Create blank profile "work"
	if err := mgr.Create("work", ""); err != nil {
		t.Fatalf("Create(work) error: %v", err)
	}

	// Verify directory and config permissions
	workDir := filepath.Join(tmpDir, "profiles", "work")
	dirInfo, err := os.Stat(workDir)
	if err != nil {
		t.Fatalf("stat work dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("work dir perm = %04o, want 0700", perm)
	}

	workConfig := filepath.Join(workDir, "config.yaml")
	cfgInfo, err := os.Stat(workConfig)
	if err != nil {
		t.Fatalf("stat work config: %v", err)
	}
	if perm := cfgInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("work config perm = %04o, want 0600", perm)
	}

	// Duplicate create should fail
	if err := mgr.Create("work", ""); err == nil {
		t.Error("Create(work) duplicate expected error, got nil")
	}

	// Creating "default" should fail
	if err := mgr.Create("default", ""); err == nil {
		t.Error("Create(default) expected error, got nil")
	}

	// Create files in "work" to test cloning
	soulPath := filepath.Join(workDir, "SOUL.md")
	if err := os.WriteFile(soulPath, []byte("# Work Soul"), 0o600); err != nil {
		t.Fatalf("writing work soul: %v", err)
	}
	policyPath := filepath.Join(workDir, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte("rules: []"), 0o600); err != nil {
		t.Fatalf("writing work policy: %v", err)
	}
	skillsDir := filepath.Join(workDir, "skills")
	skillFile := filepath.Join(skillsDir, "custom.md")
	if err := os.WriteFile(skillFile, []byte("skill content"), 0o600); err != nil {
		t.Fatalf("writing work skill: %v", err)
	}
	// Create mock DB in work
	dbPath := filepath.Join(workDir, "agis.db")
	if err := os.WriteFile(dbPath, []byte("sqlite-database-data"), 0o600); err != nil {
		t.Fatalf("writing work db: %v", err)
	}

	// 3. Clone "work" into "work-clone"
	if err := mgr.Create("work-clone", "work"); err != nil {
		t.Fatalf("Create(work-clone, cloneSource=work) error: %v", err)
	}

	cloneDir := filepath.Join(tmpDir, "profiles", "work-clone")
	// Verify cloned files exist
	if _, err := os.Stat(filepath.Join(cloneDir, "config.yaml")); err != nil {
		t.Errorf("cloned config.yaml missing: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(cloneDir, "SOUL.md")); err != nil || string(data) != "# Work Soul" {
		t.Errorf("cloned SOUL.md invalid: %s (err=%v)", string(data), err)
	}
	if data, err := os.ReadFile(filepath.Join(cloneDir, "skills", "custom.md")); err != nil || string(data) != "skill content" {
		t.Errorf("cloned skill file invalid: %s (err=%v)", string(data), err)
	}

	// CRITICAL: DB file must NOT be cloned (must start fresh)
	if _, err := os.Stat(filepath.Join(cloneDir, "agis.db")); !os.IsNotExist(err) {
		t.Errorf("agis.db should NOT be cloned, but exists!")
	}

	// 4. Show profile
	info, err := mgr.Show("work")
	if err != nil {
		t.Fatalf("Show(work) error: %v", err)
	}
	if info.Name != "work" || info.Path != workDir {
		t.Errorf("unexpected Show info: %+v", info)
	}

	// Show nonexistent profile fails
	if _, err := mgr.Show("nonexistent"); err == nil {
		t.Error("Show(nonexistent) expected error, got nil")
	}

	// 5. Switch (use)
	if err := mgr.Switch("work"); err != nil {
		t.Fatalf("Switch(work) error: %v", err)
	}

	// Check pointer file permission is 0600
	pointerPath := filepath.Join(tmpDir, ".active_profile")
	ptrInfo, err := os.Stat(pointerPath)
	if err != nil {
		t.Fatalf("stat .active_profile: %v", err)
	}
	if perm := ptrInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf(".active_profile perm = %04o, want 0600", perm)
	}

	// Verify active profile in List
	profiles, err = mgr.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	for _, p := range profiles {
		if p.Name == "work" && !p.IsActive {
			t.Errorf("expected profile work to be active in list: %+v", p)
		}
		if p.Name == "default" && p.IsActive {
			t.Errorf("expected default profile not to be active: %+v", p)
		}
	}

	// Switch to non-existent profile should fail
	if err := mgr.Switch("does_not_exist"); err == nil {
		t.Error("Switch(does_not_exist) expected error, got nil")
	}

	// 6. Delete guards
	// Cannot delete default
	if err := mgr.Delete("default", false); err == nil {
		t.Error("Delete(default) expected error, got nil")
	}

	// Cannot delete active profile without force
	if err := mgr.Delete("work", false); err == nil {
		t.Error("Delete(work, force=false) when active expected error, got nil")
	}

	// Delete active profile with force succeeds and resets active profile
	if err := mgr.Delete("work", true); err != nil {
		t.Fatalf("Delete(work, force=true) error: %v", err)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Errorf("work dir still exists after delete")
	}
	// Active profile pointer should be reset / default
	if mgr.ResolveProfilePaths("").ActiveProfileName != "default" {
		t.Errorf("active profile after deleting active with force should be default, got %s", mgr.ResolveProfilePaths("").ActiveProfileName)
	}

	// Delete non-existent profile fails
	if err := mgr.Delete("work", false); err == nil {
		t.Error("Delete(work) after already deleted expected error, got nil")
	}
}
