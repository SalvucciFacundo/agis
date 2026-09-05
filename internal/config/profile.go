package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	profileNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	activeProfileMu sync.RWMutex
	activeProfileOverride string
)

// ProfilePaths holds resolved paths for the active AGIS context.
type ProfilePaths struct {
	HomeDir           string `json:"home_dir"`
	ConfigFile        string `json:"config_file"`
	DBFile            string `json:"db_file"`
	SoulFile          string `json:"soul_file"`
	SkillsDir         string `json:"skills_dir"`
	PolicyFile        string `json:"policy_file"`
	ActiveProfileName string `json:"active_profile_name"`
	IsDefault         bool   `json:"is_default"`
}

// ProfileInfo holds metadata for a discovered AGIS profile.
type ProfileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsActive bool   `json:"is_active"`
}

// ProfileManager defines the operations supported by the multi-profile management subsystem.
type ProfileManager interface {
	List() ([]ProfileInfo, error)
	Create(name, cloneSource string) error
	Delete(name string, force bool) error
	Show(name string) (ProfileInfo, error)
	Switch(name string) error
	ResolveProfilePaths(override string) ProfilePaths
}

// ValidateProfileName validates that a profile name conforms to security and syntax constraints:
// - Length between 1 and 32 characters
// - Matches ^[a-zA-Z0-9_-]+$
// - No path separators, spaces, dots, traversal markers, or control characters
func ValidateProfileName(name string) error {
	if len(name) < 1 {
		return fmt.Errorf("profile name cannot be empty")
	}
	if len(name) > 32 {
		return fmt.Errorf("profile name %q exceeds maximum length of 32 characters", name)
	}
	if !profileNameRegex.MatchString(name) {
		return fmt.Errorf("profile name %q contains invalid characters (must match %s)", name, profileNameRegex.String())
	}
	return nil
}

// ActiveProfile returns the currently resolved active profile name.
func ActiveProfile() string {
	paths := ResolveProfilePaths("")
	return paths.ActiveProfileName
}

// SetActiveProfile sets the in-process global active profile override.
func SetActiveProfile(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed != "" && trimmed != "default" {
		if err := ValidateProfileName(trimmed); err != nil {
			return err
		}
	}
	activeProfileMu.Lock()
	defer activeProfileMu.Unlock()
	activeProfileOverride = trimmed
	return nil
}

// getActiveProfileOverride returns the in-process active profile override if set.
func getActiveProfileOverride() string {
	activeProfileMu.RLock()
	defer activeProfileMu.RUnlock()
	return activeProfileOverride
}

// BaseHome returns the root AGIS home directory ($AGIS_HOME or ~/.agis).
func BaseHome() string {
	if home := os.Getenv("AGIS_HOME"); home != "" {
		return home
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return dotAgisDir
	}
	return filepath.Join(userHome, dotAgisDir)
}

// ProfileDir returns the directory path for the given profile name under BaseHome().
func ProfileDir(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed == "default" {
		return BaseHome()
	}
	return filepath.Join(BaseHome(), "profiles", trimmed)
}

// CurrentProfileDir returns the directory of the currently active profile.
func CurrentProfileDir() string {
	paths := ResolveProfilePaths("")
	return paths.HomeDir
}

// ResolveProfilePaths resolves AGIS filesystem paths according to precedence rules:
// 1. Explicit override argument (e.g. from --profile CLI flag)
// 2. In-process active profile override (from SetActiveProfile)
// 3. AGIS_PROFILE environment variable
// 4. .active_profile pointer file in BaseHome()
// 5. Default root profile ($AGIS_HOME)
func ResolveProfilePaths(override string) ProfilePaths {
	mgr := NewProfileManager(BaseHome())
	return mgr.ResolveProfilePaths(override)
}

// ListProfiles returns all available profiles.
func ListProfiles() ([]ProfileInfo, error) {
	mgr := NewProfileManager(BaseHome())
	return mgr.List()
}

// CreateProfile scaffolds a new profile directory.
func CreateProfile(name string, cloneSource string) error {
	mgr := NewProfileManager(BaseHome())
	return mgr.Create(name, cloneSource)
}

// DeleteProfile removes a profile directory.
func DeleteProfile(name string, force bool) error {
	mgr := NewProfileManager(BaseHome())
	return mgr.Delete(name, force)
}

// SwitchProfile updates the active profile pointer.
func SwitchProfile(name string) error {
	mgr := NewProfileManager(BaseHome())
	return mgr.Switch(name)
}
