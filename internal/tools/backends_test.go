package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDocker_RunUsesEphemeralContainer(t *testing.T) {
	var gotName string
	var gotArgs []string
	exec := func(_ context.Context, name string, args ...string) (string, error) {
		gotName = name
		gotArgs = args
		return "container output", nil
	}

	d := newDockerWith("debian:12", exec)
	out, err := d.Run(context.Background(), "echo hi")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != "container output" {
		t.Errorf("out = %q", out)
	}
	if gotName != "docker" {
		t.Errorf("binary = %q, want docker", gotName)
	}
	want := []string{"run", "--rm", "debian:12", "sh", "-c", "echo hi"}
	if strings.Join(gotArgs, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v, want %v (--rm guarantees teardown)", gotArgs, want)
	}
	if d.Backend() != "docker" {
		t.Errorf("Backend() = %q", d.Backend())
	}
	if d.Name() != "shell-docker" {
		t.Errorf("Name() = %q, want shell-docker", d.Name())
	}
	if !strings.Contains(d.Description(), "docker") {
		t.Errorf("Description() = %q, want description containing docker", d.Description())
	}
}

func TestDocker_DefaultImage(t *testing.T) {
	var gotArgs []string
	d := newDockerWith("", func(_ context.Context, _ string, args ...string) (string, error) {
		gotArgs = args
		return "", nil
	})
	_, _ = d.Run(context.Background(), "x")
	if gotArgs[2] != "alpine:3" {
		t.Errorf("image = %q, want alpine:3 default", gotArgs[2])
	}
}

func TestSSH_RunBuildsStrictCommand(t *testing.T) {
	var gotName string
	var gotArgs []string
	exec := func(_ context.Context, name string, args ...string) (string, error) {
		gotName = name
		gotArgs = args
		return "remote output", nil
	}

	s := newSSHWith("kuno", "vps.example", "~/.ssh/id_ed25519", exec)
	out, err := s.Run(context.Background(), "uptime")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != "remote output" {
		t.Errorf("out = %q", out)
	}
	if gotName != "ssh" {
		t.Errorf("binary = %q, want ssh", gotName)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"-o StrictHostKeyChecking=yes", "-i ~/.ssh/id_ed25519", "-- kuno@vps.example uptime"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q: %v", want, gotArgs)
		}
	}
}

func TestSSH_KeylessAndConnectionFailure(t *testing.T) {
	s := newSSHWith("root", "down.example", "", func(context.Context, string, ...string) (string, error) {
		return "ssh: connect to host down.example port 22: Connection refused", errors.New("exit 255")
	})

	out, err := s.Run(context.Background(), "uptime")
	if err == nil || !strings.Contains(err.Error(), "root@down.example") {
		t.Errorf("err = %v, want wrapped connection failure naming the host", err)
	}
	if !strings.Contains(out, "Connection refused") {
		t.Errorf("out = %q, want remote diagnostic preserved", out)
	}
	if s.Backend() != "ssh" {
		t.Errorf("Backend() = %q", s.Backend())
	}
	if s.Name() != "shell-ssh" {
		t.Errorf("Name() = %q, want shell-ssh", s.Name())
	}
	if !strings.Contains(s.Description(), "ssh") {
		t.Errorf("Description() = %q, want description containing ssh", s.Description())
	}

	keyless := newSSHWith("root", "h.example", "", func(_ context.Context, _ string, args ...string) (string, error) {
		if strings.Join(args, " ") != "-o StrictHostKeyChecking=yes -- root@h.example cmd" {
			t.Errorf("keyless args = %v, want no -i pair", args)
		}
		return "", nil
	})
	if _, err := keyless.Run(context.Background(), "cmd"); err != nil {
		t.Errorf("keyless Run() error = %v", err)
	}
}
