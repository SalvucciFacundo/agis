package version_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SalvucciFacundo/agis/internal/version"
)

func TestGet(t *testing.T) {
	info := version.Get()
	assert.NotEmpty(t, info.Version)
	assert.NotEmpty(t, info.Commit)
	assert.NotEmpty(t, info.BuildDate)
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name        string
		v1          string
		v2          string
		expected    int
		expectError bool
	}{
		{
			name:     "v1 smaller than v2",
			v1:       "v0.3.0",
			v2:       "v0.4.0",
			expected: -1,
		},
		{
			name:     "v1 greater than v2 without v prefix",
			v1:       "0.4.0",
			v2:       "0.3.0",
			expected: 1,
		},
		{
			name:     "v1 equal to v2 with mixed prefixes",
			v1:       "v1.0.0",
			v2:       "1.0.0",
			expected: 0,
		},
		{
			name:     "uppercase V prefix",
			v1:       "V1.0.0",
			v2:       "v1.0.0",
			expected: 0,
		},
		{
			name:     "build metadata ignored in comparison",
			v1:       "v1.0.0+build.1",
			v2:       "v1.0.0+build.2",
			expected: 0,
		},
		{
			name:     "prerelease vs release",
			v1:       "v1.0.0-rc1",
			v2:       "v1.0.0",
			expected: -1,
		},
		{
			name:     "release vs prerelease",
			v1:       "v1.0.0",
			v2:       "v1.0.0-rc1",
			expected: 1,
		},
		{
			name:     "prerelease ordering",
			v1:       "v1.0.0-alpha",
			v2:       "v1.0.0-beta",
			expected: -1,
		},
		{
			name:     "patch version difference",
			v1:       "v1.2.3",
			v2:       "v1.2.4",
			expected: -1,
		},
		{
			name:     "minor version difference",
			v1:       "v1.3.0",
			v2:       "v1.2.9",
			expected: 1,
		},
		{
			name:     "major version difference",
			v1:       "v2.0.0",
			v2:       "v1.9.9",
			expected: 1,
		},
		{
			name:        "invalid v1 string",
			v1:          "not-a-version",
			v2:          "v1.0.0",
			expectError: true,
		},
		{
			name:        "invalid v2 string",
			v1:          "v1.0.0",
			v2:          "invalid",
			expectError: true,
		},
		{
			name:        "empty v1",
			v1:          "",
			v2:          "v1.0.0",
			expectError: true,
		},
		{
			name:        "empty v2",
			v1:          "v1.0.0",
			v2:          "",
			expectError: true,
		},
		{
			name:        "two parts only",
			v1:          "1.0",
			v2:          "1.0.0",
			expectError: true,
		},
		{
			name:        "four parts",
			v1:          "1.0.0.0",
			v2:          "1.0.0",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := version.Compare(tt.v1, tt.v2)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		current  string
		expected bool
	}{
		{
			name:     "newer target",
			target:   "v0.4.0",
			current:  "v0.3.0",
			expected: true,
		},
		{
			name:     "older target",
			target:   "v0.3.0",
			current:  "v0.4.0",
			expected: false,
		},
		{
			name:     "same version",
			target:   "v0.4.0",
			current:  "v0.4.0",
			expected: false,
		},
		{
			name:     "dev build current with valid target",
			target:   "v0.1.0",
			current:  "dev",
			expected: true,
		},
		{
			name:     "empty current version with valid target",
			target:   "v0.1.0",
			current:  "",
			expected: true,
		},
		{
			name:     "dev target with dev current",
			target:   "dev",
			current:  "dev",
			expected: false,
		},
		{
			name:     "dev target with released current",
			target:   "dev",
			current:  "v1.0.0",
			expected: false,
		},
		{
			name:     "invalid target version",
			target:   "invalid",
			current:  "v1.0.0",
			expected: false,
		},
		{
			name:     "invalid current version treated like dev",
			target:   "v1.0.0",
			current:  "invalid-build",
			expected: true,
		},
		{
			name:     "both invalid versions",
			target:   "invalid-target",
			current:  "invalid-current",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := version.IsNewer(tt.target, tt.current)
			assert.Equal(t, tt.expected, got)
		})
	}
}
