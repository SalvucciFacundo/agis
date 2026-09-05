package doctor

import (
	"context"
	"fmt"
	"net"
	"time"
)

func (d *Doctor) checkServer(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "server",
		Title: "REST API Server & Network",
	}

	host := d.cfg.Server.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := d.cfg.Server.Port
	if port <= 0 {
		port = 8080
	}

	res.Details = append(res.Details, fmt.Sprintf("Configured Bind Address: %s:%d", host, port))
	res.Details = append(res.Details, fmt.Sprintf("Server Enabled: %t", d.cfg.Server.Enabled))

	// Security Check: If bound to public interface (0.0.0.0, ::) without API key
	isPublic := host == "0.0.0.0" || host == "::"
	if isPublic && d.cfg.Server.APIKey == "" {
		res.Status = StatusWarn
		res.Message = "Server is configured to listen on public interfaces without authentication (API key is empty)"
		res.Details = append(res.Details, "Security Risk: Anyone on your local network or internet can access the API without credentials")
		res.Duration = time.Since(start)
		return res
	}

	// Port availability probe: Try binding to host:port
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		res.Status = StatusWarn
		res.Message = fmt.Sprintf("Port %d on %s is currently unavailable or in use: %v", port, host, err)
		res.Details = append(res.Details, "Note: If AGIS API server is already running, this is expected")
		res.Duration = time.Since(start)
		return res
	}
	_ = ln.Close()

	res.Status = StatusPass
	if d.cfg.Server.APIKey != "" {
		res.Message = fmt.Sprintf("Server configuration is valid and secured (listening on %s:%d)", host, port)
	} else {
		res.Message = fmt.Sprintf("Server configuration is valid (listening on loopback %s:%d)", host, port)
	}

	res.Duration = time.Since(start)
	return res
}
