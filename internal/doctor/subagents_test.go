package doctor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestDoctor_CheckSubagents_Disabled(t *testing.T) {
	cfg := &config.Config{
		Subagents: config.SubagentsConfig{
			Enabled: false,
		},
	}

	doc := New(cfg)
	res := doc.checkSubagents(context.Background())

	if res.Status != StatusPass {
		t.Errorf("checkSubagents() status = %v, want %v", res.Status, StatusPass)
	}
	if res.Name != "subagents" {
		t.Errorf("checkSubagents() name = %q, want %q", res.Name, "subagents")
	}
	if !strings.Contains(res.Message, "Subagents subsystem disabled") {
		t.Errorf("checkSubagents() message = %q, want containing %q", res.Message, "Subagents subsystem disabled")
	}
}

func TestDoctor_CheckSubagents_Enabled(t *testing.T) {
	cfg := &config.Config{
		Subagents: config.SubagentsConfig{
			Enabled:        true,
			MaxConcurrent:  4,
			MaxDepth:       1,
			DefaultTimeout: 45 * time.Second,
			MaxTurns:       10,
		},
	}

	doc := New(cfg)
	res := doc.checkSubagents(context.Background())

	if res.Status != StatusPass {
		t.Errorf("checkSubagents() status = %v, want %v", res.Status, StatusPass)
	}
	if res.Name != "subagents" {
		t.Errorf("checkSubagents() name = %q, want %q", res.Name, "subagents")
	}
	if !strings.Contains(res.Message, "Subagents enabled") {
		t.Errorf("checkSubagents() message = %q, want containing %q", res.Message, "Subagents enabled")
	}

	detailsJoined := strings.Join(res.Details, "\n")
	if !strings.Contains(detailsJoined, "Max concurrency: 4") {
		t.Errorf("expected max concurrency detail, got: %v", res.Details)
	}
	if !strings.Contains(detailsJoined, "Max depth: 1") {
		t.Errorf("expected max depth detail, got: %v", res.Details)
	}
	if !strings.Contains(detailsJoined, "Default timeout: 45s") {
		t.Errorf("expected default timeout detail, got: %v", res.Details)
	}
	if !strings.Contains(detailsJoined, "Max turns per task: 10") {
		t.Errorf("expected max turns detail, got: %v", res.Details)
	}
}

func TestDoctor_CheckSubagents_IncludedInRun(t *testing.T) {
	cfg := &config.Config{
		Subagents: config.SubagentsConfig{
			Enabled: true,
		},
	}

	doc := New(cfg)
	report := doc.Run(context.Background())

	subagentRes := report.Find("subagents")
	if subagentRes == nil {
		t.Fatalf("check 'subagents' not found in doctor report results")
	}
	if subagentRes.Status != StatusPass {
		t.Errorf("subagents check status = %v, want %v", subagentRes.Status, StatusPass)
	}
}
