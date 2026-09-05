package doctor

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (d *Doctor) checkWebTools(_ context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "web_tools",
		Title: "Web Search & Content Extraction Tools",
	}

	webCfg := d.cfg.Tools.Web
	if !webCfg.Enabled {
		res.Status = StatusPass
		res.Message = "Web tools disabled"
		res.Duration = time.Since(start)
		return res
	}

	prov := strings.ToLower(strings.TrimSpace(webCfg.DefaultProvider))
	if prov == "" {
		prov = "duckduckgo"
	}

	res.Details = append(res.Details, fmt.Sprintf("Default provider: %s", prov))
	if webCfg.FetchTimeout > 0 {
		res.Details = append(res.Details, fmt.Sprintf("Fetch timeout: %v", webCfg.FetchTimeout))
	}
	if webCfg.MaxFetchBytes > 0 {
		res.Details = append(res.Details, fmt.Sprintf("Max fetch size: %d bytes (%.2f MB)", webCfg.MaxFetchBytes, float64(webCfg.MaxFetchBytes)/(1024*1024)))
	}

	switch prov {
	case "brave":
		apiKey := webCfg.Providers.GetBraveAPIKey()
		if apiKey == "" {
			res.Status = StatusWarn
			res.Message = "Default web search provider 'brave' requires an API key that is not configured"
			res.Details = append(res.Details, "Hint: configure tools.web.providers.brave.api_key or export BRAVE_API_KEY")
			res.Duration = time.Since(start)
			return res
		}
		res.Status = StatusPass
		res.Message = "Web tools enabled (default provider: brave, API key configured)"

	case "tavily":
		apiKey := webCfg.Providers.GetTavilyAPIKey()
		if apiKey == "" {
			res.Status = StatusWarn
			res.Message = "Default web search provider 'tavily' requires an API key that is not configured"
			res.Details = append(res.Details, "Hint: configure tools.web.providers.tavily.api_key or export TAVILY_API_KEY")
			res.Duration = time.Since(start)
			return res
		}
		res.Status = StatusPass
		res.Message = "Web tools enabled (default provider: tavily, API key configured)"

	case "searxng":
		baseURL := webCfg.Providers.GetSearxngURL()
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		res.Status = StatusPass
		res.Message = fmt.Sprintf("Web tools enabled (default provider: searxng, base URL: %s)", baseURL)

	case "duckduckgo", "ddg":
		res.Status = StatusPass
		res.Message = "Web tools enabled (default provider: duckduckgo, no API key required)"

	default:
		res.Status = StatusPass
		res.Message = fmt.Sprintf("Web tools enabled (custom provider: %s)", prov)
	}

	res.Duration = time.Since(start)
	return res
}
