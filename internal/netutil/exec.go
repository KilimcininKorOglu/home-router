package netutil

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const defaultTimeout = 30 * time.Second

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func Run(ctx context.Context, name string, args ...string) (*ExecResult, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &ExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		return result, fmt.Errorf("exec %s: %w (stderr: %s)", name, err, stderr.String())
	}

	return result, nil
}

func RunSimple(ctx context.Context, name string, args ...string) (string, error) {
	result, err := Run(ctx, name, args...)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}
