package doctor

import (
	"context"
	"fmt"
	"time"
)

func (d *Doctor) checkToolSearch(_ context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "tool_search",
		Title: "Dynamic Tool Search",
	}

	cfg := d.cfg.Tools.ToolSearch
	if !cfg.Enabled {
		res.Status = StatusPass
		res.Message = "Dynamic tool search disabled (all registered tools advertised upfront)"
		res.Duration = time.Since(start)
		return res
	}

	if cfg.Threshold <= 0 {
		res.Status = StatusWarn
		res.Message = "Tool search threshold is <= 0, defaulting to 8"
		res.Details = append(res.Details, "Configured threshold: 0 or negative; will be normalized to 8 at runtime")
		res.Duration = time.Since(start)
		return res
	}

	res.Status = StatusPass
	res.Message = fmt.Sprintf("Dynamic tool search enabled (threshold: %d)", cfg.Threshold)
	res.Details = append(res.Details, fmt.Sprintf("Threshold: %d", cfg.Threshold))
	res.Duration = time.Since(start)
	return res
}
