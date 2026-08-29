package gateway_test

import (
	"testing"

	"github.com/SalvucciFacundo/agis/internal/gateway"
)

func TestIsAllowed(t *testing.T) {
	tests := []struct {
		name      string
		allowlist []string
		userID    string
		want      bool
	}{
		{
			name:      "empty allowlist denies all",
			allowlist: []string{},
			userID:    "12345",
			want:      false,
		},
		{
			name:      "nil allowlist denies all",
			allowlist: nil,
			userID:    "12345",
			want:      false,
		},
		{
			name:      "matching user ID allowed",
			allowlist: []string{"12345", "67890"},
			userID:    "12345",
			want:      true,
		},
		{
			name:      "second matching user ID allowed",
			allowlist: []string{"12345", "67890"},
			userID:    "67890",
			want:      true,
		},
		{
			name:      "non-matching user ID denied",
			allowlist: []string{"12345", "67890"},
			userID:    "99999",
			want:      false,
		},
		{
			name:      "whitespace or partial match denied",
			allowlist: []string{"12345"},
			userID:    "1234",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gateway.IsAllowed(tt.allowlist, tt.userID)
			if got != tt.want {
				t.Errorf("IsAllowed(%v, %q) = %v, want %v", tt.allowlist, tt.userID, got, tt.want)
			}
		})
	}
}
