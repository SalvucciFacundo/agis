package cron_test

import (
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/cron"
)

func TestParseSchedule_Valid(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
	}{
		{name: "standard daily", schedule: "0 9 * * *"},
		{name: "every 15 mins", schedule: "*/15 * * * *"},
		{name: "weekdays at 8", schedule: "0 8 * * 1-5"},
		{name: "complex list and dow", schedule: "30 4 1,15 * 5"},
		{name: "boundary max values", schedule: "59 23 31 12 7"},
		{name: "boundary min values", schedule: "0 0 1 1 0"},
		{name: "macro hourly", schedule: "@hourly"},
		{name: "macro daily", schedule: "@daily"},
		{name: "macro midnight", schedule: "@midnight"},
		{name: "macro weekly", schedule: "@weekly"},
		{name: "macro monthly", schedule: "@monthly"},
		{name: "macro yearly", schedule: "@yearly"},
		{name: "macro annually", schedule: "@annually"},
		{name: "duration 10s", schedule: "@every 10s"},
		{name: "duration 30m", schedule: "@every 30m"},
		{name: "duration 1h", schedule: "@every 1h"},
		{name: "duration compound", schedule: "@every 2h30m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := cron.ParseSchedule(tt.schedule)
			if err != nil {
				t.Fatalf("ParseSchedule(%q) unexpected error: %v", tt.schedule, err)
			}
			if s == nil {
				t.Fatalf("ParseSchedule(%q) returned nil schedule", tt.schedule)
			}
		})
	}
}

func TestParseSchedule_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
	}{
		{name: "empty", schedule: ""},
		{name: "whitespace only", schedule: "   "},
		{name: "too few fields", schedule: "0 0 *"},
		{name: "too many fields", schedule: "0 0 * * * *"},
		{name: "minute out of bounds high", schedule: "60 * * * *"},
		{name: "minute out of bounds low", schedule: "-1 * * * *"},
		{name: "hour out of bounds high", schedule: "* 24 * * *"},
		{name: "dom out of bounds low", schedule: "* * 0 * *"},
		{name: "dom out of bounds high", schedule: "* * 32 * *"},
		{name: "month out of bounds low", schedule: "* * * 0 *"},
		{name: "month out of bounds high", schedule: "* * * 13 *"},
		{name: "dow out of bounds high", schedule: "* * * * 8"},
		{name: "inverted range", schedule: "50-20 * * * *"},
		{name: "step zero", schedule: "*/0 * * * *"},
		{name: "invalid characters", schedule: "abc * * * *"},
		{name: "unknown macro", schedule: "@secondly"},
		{name: "@every missing duration", schedule: "@every"},
		{name: "@every zero duration", schedule: "@every 0s"},
		{name: "@every negative duration", schedule: "@every -10s"},
		{name: "@every invalid duration", schedule: "@every invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := cron.ParseSchedule(tt.schedule)
			if err == nil {
				t.Errorf("ParseSchedule(%q) expected error, got schedule: %+v", tt.schedule, s)
			}
		})
	}
}

func TestSchedule_NextCalculation(t *testing.T) {
	ref := time.Date(2026, time.March, 30, 8, 30, 0, 0, time.UTC) // Monday 2026-03-30 08:30:00 UTC

	tests := []struct {
		name     string
		schedule string
		after    time.Time
		wantNext time.Time
	}{
		{
			name:     "every 15m from 08:30",
			schedule: "*/15 * * * *",
			after:    ref,
			wantNext: time.Date(2026, time.March, 30, 8, 45, 0, 0, time.UTC),
		},
		{
			name:     "every 15m from 08:31",
			schedule: "*/15 * * * *",
			after:    ref.Add(time.Minute),
			wantNext: time.Date(2026, time.March, 30, 8, 45, 0, 0, time.UTC),
		},
		{
			name:     "daily at 9:00 from 08:30",
			schedule: "0 9 * * *",
			after:    ref,
			wantNext: time.Date(2026, time.March, 30, 9, 0, 0, 0, time.UTC),
		},
		{
			name:     "daily at 9:00 from 09:00",
			schedule: "0 9 * * *",
			after:    time.Date(2026, time.March, 30, 9, 0, 0, 0, time.UTC),
			wantNext: time.Date(2026, time.March, 31, 9, 0, 0, 0, time.UTC),
		},
		{
			name:     "weekdays at 8:00 from Monday 08:30",
			schedule: "0 8 * * 1-5",
			after:    ref,
			wantNext: time.Date(2026, time.March, 31, 8, 0, 0, 0, time.UTC), // Tuesday
		},
		{
			name:     "duration @every 30m",
			schedule: "@every 30m",
			after:    ref,
			wantNext: ref.Add(30 * time.Minute),
		},
		{
			name:     "duration @every 1h",
			schedule: "@every 1h",
			after:    ref,
			wantNext: ref.Add(1 * time.Hour),
		},
		{
			name:     "macro @hourly",
			schedule: "@hourly",
			after:    ref,
			wantNext: time.Date(2026, time.March, 30, 9, 0, 0, 0, time.UTC),
		},
		{
			name:     "macro @monthly",
			schedule: "@monthly",
			after:    ref,
			wantNext: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "step in range 1-10/3",
			schedule: "1-10/3 * * * *",
			after:    time.Date(2026, time.March, 30, 8, 2, 0, 0, time.UTC),
			wantNext: time.Date(2026, time.March, 30, 8, 4, 0, 0, time.UTC),
		},
		{
			name:     "leap year feb 29",
			schedule: "0 0 29 2 *",
			after:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			wantNext: time.Date(2028, time.February, 29, 0, 0, 0, 0, time.UTC),
		},

	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := cron.ParseSchedule(tt.schedule)
			if err != nil {
				t.Fatalf("ParseSchedule(%q) error: %v", tt.schedule, err)
			}
			got := s.Next(tt.after)
			if !got.Equal(tt.wantNext) {
				t.Errorf("Next(%v) = %v, want %v", tt.after, got, tt.wantNext)
			}
		})
	}
}

func TestValidateJob(t *testing.T) {
	tests := []struct {
		name    string
		job     cron.Job
		wantErr bool
	}{
		{
			name: "valid job without target",
			job: cron.Job{
				Name:     "job-1",
				Schedule: "@every 1h",
				Prompt:   "Hello AGIS",
			},
			wantErr: false,
		},
		{
			name: "valid job with target",
			job: cron.Job{
				Name:      "job-2",
				Schedule:  "0 9 * * *",
				Prompt:    "Daily summary",
				SessionID: "custom-session",
				Target: &cron.Target{
					Adapter:   "telegram",
					Recipient: "123456",
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			job: cron.Job{
				Schedule: "@every 1h",
				Prompt:   "Hello AGIS",
			},
			wantErr: true,
		},
		{
			name: "missing prompt",
			job: cron.Job{
				Name:     "job-3",
				Schedule: "@every 1h",
			},
			wantErr: true,
		},
		{
			name: "invalid schedule",
			job: cron.Job{
				Name:     "job-4",
				Schedule: "invalid-cron",
				Prompt:   "Hello",
			},
			wantErr: true,
		},
		{
			name: "target missing adapter",
			job: cron.Job{
				Name:     "job-5",
				Schedule: "@every 1h",
				Prompt:   "Hello",
				Target: &cron.Target{
					Recipient: "123",
				},
			},
			wantErr: true,
		},
		{
			name: "target missing recipient",
			job: cron.Job{
				Name:     "job-6",
				Schedule: "@every 1h",
				Prompt:   "Hello",
				Target: &cron.Target{
					Adapter: "telegram",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cron.ValidateJob(tt.job)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJob() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
