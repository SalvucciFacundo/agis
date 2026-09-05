// Package version provides build information and Semantic Versioning utilities
// for version comparison and self-updating.
package version

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	// Version is the application version, injected at build time via -ldflags.
	Version = "dev"
	// Commit is the git commit SHA, injected at build time via -ldflags.
	Commit = "none"
	// BuildDate is the RFC3339 build timestamp, injected at build time via -ldflags.
	BuildDate = "unknown"
)

// ErrInvalidVersion is returned when a version string does not conform to SemVer.
var ErrInvalidVersion = errors.New("invalid semantic version")

// Info contains build and version metadata.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// Get returns the current build information.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
	}
}

// semVer represents a parsed semantic version.
type semVer struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

// parseSemVer parses a SemVer string into its numeric components.
func parseSemVer(s string) (semVer, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if s == "" {
		return semVer{}, ErrInvalidVersion
	}

	// Remove build metadata (+...)
	if idx := strings.IndexByte(s, '+'); idx != -1 {
		s = s[:idx]
	}

	var prerelease string
	if idx := strings.IndexByte(s, '-'); idx != -1 {
		prerelease = s[idx+1:]
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return semVer{}, fmt.Errorf("%w: expected 3 dot-separated integers, got %q", ErrInvalidVersion, s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return semVer{}, fmt.Errorf("%w: invalid major version in %q", ErrInvalidVersion, s)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return semVer{}, fmt.Errorf("%w: invalid minor version in %q", ErrInvalidVersion, s)
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return semVer{}, fmt.Errorf("%w: invalid patch version in %q", ErrInvalidVersion, s)
	}

	return semVer{
		major:      major,
		minor:      minor,
		patch:      patch,
		prerelease: prerelease,
	}, nil
}

// Compare compares two semantic version strings (ignoring leading 'v').
// It returns -1 if v1 < v2, 0 if v1 == v2, and 1 if v1 > v2.
func Compare(v1, v2 string) (int, error) {
	sv1, err := parseSemVer(v1)
	if err != nil {
		return 0, err
	}
	sv2, err := parseSemVer(v2)
	if err != nil {
		return 0, err
	}

	if sv1.major != sv2.major {
		if sv1.major < sv2.major {
			return -1, nil
		}
		return 1, nil
	}

	if sv1.minor != sv2.minor {
		if sv1.minor < sv2.minor {
			return -1, nil
		}
		return 1, nil
	}

	if sv1.patch != sv2.patch {
		if sv1.patch < sv2.patch {
			return -1, nil
		}
		return 1, nil
	}

	// Compare prereleases: a version without prerelease is higher than one with prerelease.
	if sv1.prerelease == "" && sv2.prerelease != "" {
		return 1, nil
	}
	if sv1.prerelease != "" && sv2.prerelease == "" {
		return -1, nil
	}
	if sv1.prerelease < sv2.prerelease {
		return -1, nil
	}
	if sv1.prerelease > sv2.prerelease {
		return 1, nil
	}

	return 0, nil
}

// IsNewer returns true if target is semantically newer than current.
// If current is "dev" or invalid, IsNewer returns true for any valid target release.
func IsNewer(target, current string) bool {
	if current == "dev" || strings.TrimSpace(current) == "" {
		_, err := parseSemVer(target)
		return err == nil
	}

	cmp, err := Compare(target, current)
	if err != nil {
		// If current is invalid semver (e.g. custom build tag), any valid target is newer.
		_, currErr := parseSemVer(current)
		if currErr != nil {
			_, targErr := parseSemVer(target)
			return targErr == nil
		}
		return false
	}

	return cmp > 0
}
