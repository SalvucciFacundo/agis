package doctor

import (
	"fmt"
	"strings"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

// Format returns a human-readable, formatted string representing the report.
// If color is false, all ANSI styling is omitted.
func (r *Report) Format(color bool) string {
	var sb strings.Builder

	bold := func(s string) string {
		if !color {
			return s
		}
		return ansiBold + s + ansiReset
	}
	gray := func(s string) string {
		if !color {
			return s
		}
		return ansiGray + s + ansiReset
	}

	sb.WriteString(bold("🩺 AGIS System Health & Diagnostic Report\n"))
	sb.WriteString(gray("=========================================\n\n"))

	for _, res := range r.Results {
		var icon, statusText string
		switch res.Status {
		case StatusPass:
			if color {
				icon = ansiGreen + "✅" + ansiReset
				statusText = ansiGreen + bold("PASS") + ansiReset
			} else {
				icon = "[✓]"
				statusText = "PASS"
			}
		case StatusWarn:
			if color {
				icon = ansiYellow + "⚠️ " + ansiReset
				statusText = ansiYellow + bold("WARN") + ansiReset
			} else {
				icon = "[!]"
				statusText = "WARN"
			}
		case StatusFail:
			if color {
				icon = ansiRed + "❌" + ansiReset
				statusText = ansiRed + bold("FAIL") + ansiReset
			} else {
				icon = "[x]"
				statusText = "FAIL"
			}
		}

		sb.WriteString(fmt.Sprintf("%s %s [%s]\n", icon, bold(res.Title), statusText))
		sb.WriteString(fmt.Sprintf("   %s\n", res.Message))
		for _, d := range res.Details {
			sb.WriteString(fmt.Sprintf("   %s %s\n", gray("•"), d))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(gray("-----------------------------------------\n"))
	var summaryLine string
	if r.HasFailures() {
		msg := fmt.Sprintf("Result: %d failed, %d warnings, %d passed (out of %d checks)",
			r.Summary.Failed, r.Summary.Warnings, r.Summary.Passed, r.Summary.Total)
		if color {
			summaryLine = ansiRed + bold("❌ "+msg) + ansiReset
		} else {
			summaryLine = "FAIL: " + msg
		}
	} else if r.HasWarnings() {
		msg := fmt.Sprintf("Result: 0 failed, %d warnings, %d passed (out of %d checks)",
			r.Summary.Warnings, r.Summary.Passed, r.Summary.Total)
		if color {
			summaryLine = ansiYellow + bold("⚠️  "+msg) + ansiReset
		} else {
			summaryLine = "WARN: " + msg
		}
	} else {
		msg := fmt.Sprintf("Result: All %d checks passed successfully!", r.Summary.Total)
		if color {
			summaryLine = ansiGreen + bold("✅ "+msg) + ansiReset
		} else {
			summaryLine = "PASS: " + msg
		}
	}
	sb.WriteString(summaryLine + "\n")

	return sb.String()
}
