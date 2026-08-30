package tools

import (
	"context"
	"fmt"
	"os/exec"
)

// cmdExec abstracts binary execution so backend tests can fake the wire
// without requiring docker/ssh binaries on the machine.
type cmdExec func(ctx context.Context, name string, args ...string) (string, error)

// defaultCmdExec runs the binary and returns combined output.
func defaultCmdExec(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// available reports whether a binary is on PATH. Package-level seam so tests
// can simulate missing backends.
var lookPath = exec.LookPath

// Docker executes commands inside an ephemeral container (--rm guarantees
// teardown by the container engine itself, even on failure — spec TLS-003).
type Docker struct {
	image string
	exec  cmdExec
}

// NewDocker returns a Docker runner for image (default alpine:3 when empty)
// using the real binary executor.
func NewDocker(image string) *Docker {
	return &Docker{image: defaultIfEmpty(image, "alpine:3"), exec: defaultCmdExec}
}

// newDockerWith is the test seam.
func newDockerWith(image string, exec cmdExec) *Docker {
	return &Docker{image: defaultIfEmpty(image, "alpine:3"), exec: exec}
}

// Backend implements core.ToolRunner.
func (d *Docker) Backend() string { return "docker" }

// Name implements core.ToolRunner.
func (d *Docker) Name() string { return "shell-docker" }

// Description implements core.ToolRunner.
func (d *Docker) Description() string {
	return `Run a shell command on the docker backend. Arguments: {"command": "<the command string>"}. Prefer read-only commands.`
}

// Run executes command via docker run --rm; the container never outlives the
// call.
func (d *Docker) Run(ctx context.Context, command string) (string, error) {
	return d.exec(ctx, "docker", "run", "--rm", d.image, "sh", "-c", command)
}

// SSH executes commands on a remote host per call with strict host-key
// checking on.
type SSH struct {
	user    string
	host    string
	keyPath string
	exec    cmdExec
}

// NewSSH returns an SSH runner for user@host authenticating with keyPath.
func NewSSH(user, host, keyPath string) *SSH {
	return &SSH{user: user, host: host, keyPath: keyPath, exec: defaultCmdExec}
}

// newSSHWith is the test seam.
func newSSHWith(user, host, keyPath string, exec cmdExec) *SSH {
	return &SSH{user: user, host: host, keyPath: keyPath, exec: exec}
}

// Backend implements core.ToolRunner.
func (s *SSH) Backend() string { return "ssh" }

// Name implements core.ToolRunner.
func (s *SSH) Name() string { return "shell-ssh" }

// Description implements core.ToolRunner.
func (s *SSH) Description() string {
	return `Run a shell command on the ssh backend. Arguments: {"command": "<the command string>"}. Prefer read-only commands.`
}

// Run executes command over ssh; connection failures surface as tool errors
// with the remote's output preserved (spec TLS-004).
func (s *SSH) Run(ctx context.Context, command string) (string, error) {
	args := []string{"-o", "StrictHostKeyChecking=yes"}
	if s.keyPath != "" {
		args = append(args, "-i", s.keyPath)
	}
	args = append(args, "--", s.user+"@"+s.host, command)

	out, err := s.exec(ctx, "ssh", args...)
	if err != nil {
		return out, fmt.Errorf("ssh backend %s@%s: %w", s.user, s.host, err)
	}
	return out, nil
}

func defaultIfEmpty(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
