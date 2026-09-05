package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const activeProfilePointerFile = ".active_profile"

// DefaultProfileManager implements ProfileManager backed by the local filesystem.
type DefaultProfileManager struct {
	baseHome string
}

// NewProfileManager returns a ProfileManager targeting baseHome (or BaseHome() if omitted).
func NewProfileManager(baseHome ...string) *DefaultProfileManager {
	home := ""
	if len(baseHome) > 0 && baseHome[0] != "" {
		home = baseHome[0]
	} else {
		home = BaseHome()
	}
	return &DefaultProfileManager{baseHome: home}
}

// ResolveProfilePaths determines active profile paths according to precedence:
// 1. override argument (e.g. from --profile)
// 2. in-process activeProfileOverride
// 3. AGIS_PROFILE env var
// 4. .active_profile pointer file
// 5. default root
func (m *DefaultProfileManager) ResolveProfilePaths(override string) ProfilePaths {
	profileName := ""

	// 1. Explicit override argument
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		profileName = trimmed
	}

	// 2. In-process global override
	if profileName == "" {
		if inProcess := getActiveProfileOverride(); inProcess != "" {
			profileName = inProcess
		}
	}

	// 3. AGIS_PROFILE environment variable
	if profileName == "" {
		if envProfile := strings.TrimSpace(os.Getenv("AGIS_PROFILE")); envProfile != "" {
			profileName = envProfile
		}
	}

	// 4. .active_profile pointer file
	if profileName == "" {
		pointerPath := filepath.Join(m.baseHome, activeProfilePointerFile)
		if data, err := os.ReadFile(pointerPath); err == nil {
			if ptr := strings.TrimSpace(string(data)); ptr != "" {
				profileName = ptr
			}
		}
	}

	// 5. Default root fallback
	if profileName == "" || profileName == "default" {
		return ProfilePaths{
			HomeDir:           m.baseHome,
			ConfigFile:        filepath.Join(m.baseHome, configFileName),
			DBFile:            filepath.Join(m.baseHome, dbFileName),
			SoulFile:          filepath.Join(m.baseHome, "SOUL.md"),
			SkillsDir:         filepath.Join(m.baseHome, skillsDirName),
			PolicyFile:        filepath.Join(m.baseHome, "policy.yaml"),
			ActiveProfileName: "default",
			IsDefault:         true,
		}
	}

	profileDir := filepath.Join(m.baseHome, "profiles", profileName)
	return ProfilePaths{
		HomeDir:           profileDir,
		ConfigFile:        filepath.Join(profileDir, configFileName),
		DBFile:            filepath.Join(profileDir, dbFileName),
		SoulFile:          filepath.Join(profileDir, "SOUL.md"),
		SkillsDir:         filepath.Join(profileDir, skillsDirName),
		PolicyFile:        filepath.Join(profileDir, "policy.yaml"),
		ActiveProfileName: profileName,
		IsDefault:         false,
	}
}

// List enumerates all available profiles including the default profile.
func (m *DefaultProfileManager) List() ([]ProfileInfo, error) {
	active := m.ResolveProfilePaths("").ActiveProfileName

	profiles := []ProfileInfo{
		{
			Name:     "default",
			Path:     m.baseHome,
			IsActive: active == "default",
		},
	}

	profilesDir := filepath.Join(m.baseHome, "profiles")
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return profiles, nil
		}
		return nil, fmt.Errorf("reading profiles directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if err := ValidateProfileName(name); err != nil {
			continue
		}
		profiles = append(profiles, ProfileInfo{
			Name:     name,
			Path:     filepath.Join(profilesDir, name),
			IsActive: active == name,
		})
	}

	return profiles, nil
}

// Create scaffolds a new profile directory under $AGIS_HOME/profiles/<name>/.
// If cloneSource is non-empty, it copies config, soul, policy, and skills,
// but intentionally omits agis.db to maintain fresh database isolation.
func (m *DefaultProfileManager) Create(name, cloneSource string) error {
	trimmedName := strings.TrimSpace(name)
	if err := ValidateProfileName(trimmedName); err != nil {
		return fmt.Errorf("invalid profile name: %w", err)
	}
	if trimmedName == "default" {
		return fmt.Errorf("profile name 'default' is reserved for the root profile")
	}

	targetDir := filepath.Join(m.baseHome, "profiles", trimmedName)
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("profile %q already exists", trimmedName)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking profile directory: %w", err)
	}

	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return fmt.Errorf("creating profile directory %s: %w", targetDir, err)
	}

	skillsDir := filepath.Join(targetDir, skillsDirName)
	if err := os.MkdirAll(skillsDir, 0o700); err != nil {
		return fmt.Errorf("creating skills directory %s: %w", skillsDir, err)
	}

	trimmedClone := strings.TrimSpace(cloneSource)
	if trimmedClone != "" {
		sourceDir := m.baseHome
		if trimmedClone != "default" {
			sourceDir = filepath.Join(m.baseHome, "profiles", trimmedClone)
		}
		if _, err := os.Stat(sourceDir); err != nil {
			return fmt.Errorf("clone source profile %q not found: %w", trimmedClone, err)
		}

		// Copy config.yaml
		sourceConfig := filepath.Join(sourceDir, configFileName)
		if _, err := os.Stat(sourceConfig); err == nil {
			if err := copyFile(sourceConfig, filepath.Join(targetDir, configFileName), 0o600); err != nil {
				return fmt.Errorf("cloning config: %w", err)
			}
		} else {
			// Save default config if source didn't have one
			if err := Save(filepath.Join(targetDir, configFileName), defaults()); err != nil {
				return fmt.Errorf("saving default config: %w", err)
			}
		}

		// Copy SOUL.md
		sourceSoul := filepath.Join(sourceDir, "SOUL.md")
		if _, err := os.Stat(sourceSoul); err == nil {
			if err := copyFile(sourceSoul, filepath.Join(targetDir, "SOUL.md"), 0o600); err != nil {
				return fmt.Errorf("cloning SOUL.md: %w", err)
			}
		}

		// Copy policy.yaml
		sourcePolicy := filepath.Join(sourceDir, "policy.yaml")
		if _, err := os.Stat(sourcePolicy); err == nil {
			if err := copyFile(sourcePolicy, filepath.Join(targetDir, "policy.yaml"), 0o600); err != nil {
				return fmt.Errorf("cloning policy.yaml: %w", err)
			}
		}

		// Copy skills directory contents
		sourceSkills := filepath.Join(sourceDir, skillsDirName)
		if entries, err := os.ReadDir(sourceSkills); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				srcPath := filepath.Join(sourceSkills, e.Name())
				dstPath := filepath.Join(skillsDir, e.Name())
				_ = copyFile(srcPath, dstPath, 0o600)
			}
		}

		// Notice: agis.db is intentionally skipped to ensure isolated state.
	} else {
		// Blank profile creation: save default config.yaml with 0600 mode
		if err := Save(filepath.Join(targetDir, configFileName), defaults()); err != nil {
			return fmt.Errorf("saving default config: %w", err)
		}
	}

	return nil
}

// Show returns metadata for the named profile (or active profile if empty).
func (m *DefaultProfileManager) Show(name string) (ProfileInfo, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = m.ResolveProfilePaths("").ActiveProfileName
	}

	if trimmed == "default" {
		return ProfileInfo{
			Name:     "default",
			Path:     m.baseHome,
			IsActive: m.ResolveProfilePaths("").ActiveProfileName == "default",
		}, nil
	}

	if err := ValidateProfileName(trimmed); err != nil {
		return ProfileInfo{}, fmt.Errorf("invalid profile name: %w", err)
	}

	targetDir := filepath.Join(m.baseHome, "profiles", trimmed)
	if _, err := os.Stat(targetDir); err != nil {
		if os.IsNotExist(err) {
			return ProfileInfo{}, fmt.Errorf("profile %q does not exist", trimmed)
		}
		return ProfileInfo{}, fmt.Errorf("accessing profile directory: %w", err)
	}

	return ProfileInfo{
		Name:     trimmed,
		Path:     targetDir,
		IsActive: m.ResolveProfilePaths("").ActiveProfileName == trimmed,
	}, nil
}

// Switch updates the .active_profile pointer file in baseHome.
func (m *DefaultProfileManager) Switch(name string) error {
	trimmed := strings.TrimSpace(name)
	pointerPath := filepath.Join(m.baseHome, activeProfilePointerFile)

	if trimmed == "" || trimmed == "default" {
		if err := os.Remove(pointerPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing active profile pointer: %w", err)
		}
		return nil
	}

	if err := ValidateProfileName(trimmed); err != nil {
		return fmt.Errorf("invalid profile name: %w", err)
	}

	targetDir := filepath.Join(m.baseHome, "profiles", trimmed)
	if _, err := os.Stat(targetDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("profile %q does not exist", trimmed)
		}
		return fmt.Errorf("accessing profile directory: %w", err)
	}

	if err := os.MkdirAll(m.baseHome, 0o700); err != nil {
		return fmt.Errorf("creating base directory %s: %w", m.baseHome, err)
	}

	tmpFile, err := os.CreateTemp(m.baseHome, ".active_profile.tmp.*")
	if err != nil {
		return fmt.Errorf("creating temp pointer file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("setting pointer permissions: %w", err)
	}

	if _, err := tmpFile.WriteString(trimmed + "\n"); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing pointer file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("syncing pointer file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp pointer file: %w", err)
	}

	if err := os.Rename(tmpPath, pointerPath); err != nil {
		return fmt.Errorf("atomically updating active profile pointer: %w", err)
	}

	_ = os.Chmod(pointerPath, 0o600)
	return nil
}

// Delete removes a profile directory and resets the active profile pointer if needed.
func (m *DefaultProfileManager) Delete(name string, force bool) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed == "default" {
		return fmt.Errorf("cannot delete default profile")
	}

	if err := ValidateProfileName(trimmed); err != nil {
		return fmt.Errorf("invalid profile name: %w", err)
	}

	targetDir := filepath.Join(m.baseHome, "profiles", trimmed)
	if _, err := os.Stat(targetDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("profile %q does not exist", trimmed)
		}
		return fmt.Errorf("accessing profile directory: %w", err)
	}

	active := m.ResolveProfilePaths("").ActiveProfileName
	if active == trimmed {
		if !force {
			return fmt.Errorf("profile %q is currently active; switch profiles first or use -force", trimmed)
		}
		// Reset active profile pointer
		pointerPath := filepath.Join(m.baseHome, activeProfilePointerFile)
		_ = os.Remove(pointerPath)
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("deleting profile directory %s: %w", targetDir, err)
	}

	return nil
}

// copyFile copies a file from src to dst with atomic writing and specified permissions.
func copyFile(src, dst string, mode os.FileMode) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, ".copy.tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := tmpFile.Chmod(mode); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if _, err := io.Copy(tmpFile, sourceFile); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		return err
	}

	_ = os.Chmod(dst, mode)
	return nil
}
