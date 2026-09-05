package doctor

import (
	"context"
	"fmt"
	"time"
)

func (d *Doctor) checkSubagents(_ context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "subagents",
		Title: "Subagent Delegation Engine",
	}

	subCfg := d.cfg.Subagents
	if !subCfg.Enabled {
		res.Status = StatusPass
		res.Message = "Subagents subsystem disabled"
		res.Duration = time.Since(start)
		return res
	}

	res.Status = StatusPass
	res.Message = "Subagents enabled"

	maxConcurrent := subCfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	} else if maxConcurrent > 10 {
		maxConcurrent = 10
	}

	maxDepth := subCfg.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 1
	} else if maxDepth > 2 {
		maxDepth = 2
	}

	maxTurns := subCfg.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 8
	} else if maxTurns > 15 {
		maxTurns = 15
	}

	timeout := subCfg.DefaultTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	} else if timeout > 300*time.Second {
		timeout = 300 * time.Second
	}

	res.Details = append(res.Details, fmt.Sprintf("Max concurrency: %d", maxConcurrent))
	res.Details = append(res.Details, fmt.Sprintf("Max depth: %d (hard limit: 2)", maxDepth))
	res.Details = append(res.Details, fmt.Sprintf("Default timeout: %v", timeout))
	res.Details = append(res.Details, fmt.Sprintf("Max turns per task: %d", maxTurns))

	res.Duration = time.Since(start)
	return res
}
