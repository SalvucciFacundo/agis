// Package cron implements the periodic task scheduling subsystem for AGIS.
package cron

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Common validation errors.
var (
	ErrInvalidSchedule = errors.New("invalid schedule format")
	ErrInvalidJob      = errors.New("invalid job configuration")
	ErrJobNotFound     = errors.New("job not found")
	ErrSchedulerClosed = errors.New("scheduler closed")
)

// Job defines a periodic background prompt execution unit.
type Job struct {
	Name      string  `json:"name"`
	Schedule  string  `json:"schedule"`
	Prompt    string  `json:"prompt"`
	SessionID string  `json:"session_id,omitempty"`
	Target    *Target `json:"target,omitempty"`
}

// Target defines where outbound results of a job should be routed.
type Target struct {
	Adapter   string `json:"adapter"`
	Recipient string `json:"recipient"`
}

// Scheduler defines the background cron execution lifecycle interface.
type Scheduler interface {
	Start(ctx context.Context) error
	Stop() error
	AddJob(job Job) error
}

// Schedule parses a schedule string and calculates future trigger times.
type Schedule interface {
	Next(after time.Time) time.Time
}

// ValidateJob validates the required fields and schedule of a Job.
func ValidateJob(job Job) error {
	if strings.TrimSpace(job.Name) == "" {
		return fmt.Errorf("%w: job name is required", ErrInvalidJob)
	}
	if strings.TrimSpace(job.Prompt) == "" {
		return fmt.Errorf("%w: job prompt is required", ErrInvalidJob)
	}
	if _, err := ParseSchedule(job.Schedule); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidJob, err)
	}
	if job.Target != nil {
		if strings.TrimSpace(job.Target.Adapter) == "" {
			return fmt.Errorf("%w: target adapter is required when target is specified", ErrInvalidJob)
		}
		if strings.TrimSpace(job.Target.Recipient) == "" {
			return fmt.Errorf("%w: target recipient is required when target is specified", ErrInvalidJob)
		}
	}
	return nil
}

// IntervalSchedule triggers at fixed time durations.
type IntervalSchedule struct {
	Duration time.Duration
}

// Next returns the next trigger time after 'after'.
func (s IntervalSchedule) Next(after time.Time) time.Time {
	if s.Duration <= 0 {
		return time.Time{}
	}
	return after.Add(s.Duration)
}

// CronSchedule represents a 5-field standard cron schedule.
type CronSchedule struct {
	Minutes     map[int]bool
	Hours       map[int]bool
	DaysOfMonth map[int]bool
	Months      map[int]bool
	DaysOfWeek  map[int]bool
}

// ParseSchedule parses a 5-field cron expression, a named macro, or an @every duration string.
func ParseSchedule(spec string) (Schedule, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("%w: schedule string is empty", ErrInvalidSchedule)
	}

	if strings.HasPrefix(spec, "@every ") {
		durStr := strings.TrimSpace(strings.TrimPrefix(spec, "@every "))
		if durStr == "" {
			return nil, fmt.Errorf("%w: missing duration after @every", ErrInvalidSchedule)
		}
		d, err := time.ParseDuration(durStr)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid duration %q: %v", ErrInvalidSchedule, durStr, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("%w: duration must be positive, got %v", ErrInvalidSchedule, d)
		}
		return IntervalSchedule{Duration: d}, nil
	}

	// Check standard macros
	switch spec {
	case "@yearly", "@annually":
		spec = "0 0 1 1 *"
	case "@monthly":
		spec = "0 0 1 * *"
	case "@weekly":
		spec = "0 0 * * 0"
	case "@daily", "@midnight":
		spec = "0 0 * * *"
	case "@hourly":
		spec = "0 * * * *"
	}

	fields := strings.Fields(spec)
	if len(fields) != 5 {
		return nil, fmt.Errorf("%w: expected 5 fields, got %d in %q", ErrInvalidSchedule, len(fields), spec)
	}

	minutes, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("%w: minute field %q: %v", ErrInvalidSchedule, fields[0], err)
	}

	hours, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("%w: hour field %q: %v", ErrInvalidSchedule, fields[1], err)
	}

	dom, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("%w: day-of-month field %q: %v", ErrInvalidSchedule, fields[2], err)
	}

	months, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("%w: month field %q: %v", ErrInvalidSchedule, fields[3], err)
	}

	dow, err := parseField(fields[4], 0, 7)
	if err != nil {
		return nil, fmt.Errorf("%w: day-of-week field %q: %v", ErrInvalidSchedule, fields[4], err)
	}
	// Normalize 7 (Sunday) to 0 (Sunday)
	if dow[7] {
		dow[0] = true
		delete(dow, 7)
	}

	return &CronSchedule{
		Minutes:     minutes,
		Hours:       hours,
		DaysOfMonth: dom,
		Months:      months,
		DaysOfWeek:  dow,
	}, nil
}

func parseField(field string, min, max int) (map[int]bool, error) {
	result := make(map[int]bool)
	parts := strings.Split(field, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty item in list")
		}

		step := 1
		rangePart := part
		if strings.Contains(part, "/") {
			subParts := strings.Split(part, "/")
			if len(subParts) != 2 {
				return nil, fmt.Errorf("invalid step expression %q", part)
			}
			rangePart = subParts[0]
			s, err := strconv.Atoi(subParts[1])
			if err != nil || s <= 0 {
				return nil, fmt.Errorf("invalid step value %q", subParts[1])
			}
			step = s
		}

		var start, end int
		if rangePart == "*" {
			start = min
			end = max
		} else if strings.Contains(rangePart, "-") {
			rangeBounds := strings.Split(rangePart, "-")
			if len(rangeBounds) != 2 {
				return nil, fmt.Errorf("invalid range expression %q", rangePart)
			}
			var err error
			start, err = strconv.Atoi(rangeBounds[0])
			if err != nil {
				return nil, fmt.Errorf("invalid range start %q", rangeBounds[0])
			}
			end, err = strconv.Atoi(rangeBounds[1])
			if err != nil {
				return nil, fmt.Errorf("invalid range end %q", rangeBounds[1])
			}
		} else {
			val, err := strconv.Atoi(rangePart)
			if err != nil {
				return nil, fmt.Errorf("invalid integer value %q", rangePart)
			}
			start = val
			end = val
		}

		if start < min || start > max {
			return nil, fmt.Errorf("value %d out of bounds [%d, %d]", start, min, max)
		}
		if end < min || end > max {
			return nil, fmt.Errorf("value %d out of bounds [%d, %d]", end, min, max)
		}
		if start > end {
			return nil, fmt.Errorf("range start %d greater than end %d", start, end)
		}

		for i := start; i <= end; i += step {
			result[i] = true
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid values in field %q", field)
	}
	return result, nil
}

// Next finds the next time after 'after' that satisfies the cron specification.
func (s *CronSchedule) Next(after time.Time) time.Time {
	loc := after.Location()
	// Start at next minute boundary
	t := after.Truncate(time.Minute).Add(time.Minute)

	// Search up to 5 years ahead
	maxTime := t.AddDate(5, 0, 0)

	for t.Before(maxTime) {
		// 1. Match Month
		if !s.Months[int(t.Month())] {
			// Advance to first day of next month
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc)
			continue
		}

		// 2. Match Day of Month and Day of Week
		domMatch := s.DaysOfMonth[t.Day()]
		dowMatch := s.DaysOfWeek[int(t.Weekday())]
		if !domMatch || !dowMatch {
			// Advance to next day at 00:00
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
			continue
		}

		// 3. Match Hour
		if !s.Hours[t.Hour()] {
			// Advance to next hour at :00
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, loc)
			continue
		}

		// 4. Match Minute
		if !s.Minutes[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}

		return t
	}

	return time.Time{}
}
