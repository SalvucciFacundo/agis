package doctor_test

import (
	"context"
	"net"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/doctor"
)

func TestDoctor_CheckServer_Localhost(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    0, // port 0 is always bindable
			APIKey:  "",
		},
	}

	doc := doctor.New(cfg)
	report := doc.Run(context.Background())

	res := report.Find("server")
	if res == nil {
		t.Fatal("expected 'server' check in doctor report")
	}

	if res.Status != doctor.StatusPass {
		t.Errorf("status = %v, want StatusPass; message = %s", res.Status, res.Message)
	}
}

func TestDoctor_CheckServer_PublicWithoutAuthWarning(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Enabled: true,
			Host:    "0.0.0.0",
			Port:    0,
			APIKey:  "",
		},
	}

	doc := doctor.New(cfg)
	report := doc.Run(context.Background())

	res := report.Find("server")
	if res == nil {
		t.Fatal("expected 'server' check in doctor report")
	}

	if res.Status != doctor.StatusWarn {
		t.Errorf("status = %v, want StatusWarn for unauthenticated public server", res.Status)
	}
}

func TestDoctor_CheckServer_PortInUse(t *testing.T) {
	// Occupy a local port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	cfg := &config.Config{
		Server: config.ServerConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    port,
			APIKey:  "secret-token",
		},
	}

	doc := doctor.New(cfg)
	report := doc.Run(context.Background())

	res := report.Find("server")
	if res == nil {
		t.Fatal("expected 'server' check in doctor report")
	}

	if res.Status != doctor.StatusWarn {
		t.Errorf("status = %v, want StatusWarn when port is occupied; message = %s", res.Status, res.Message)
	}
}
