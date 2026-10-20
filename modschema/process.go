// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// commandWaitDelay bounds inherited-pipe cleanup after cancellation or process exit.
const commandWaitDelay = 2 * time.Second

// runCommand preserves cancellation identity while retaining the underlying process error.
func runCommand(ctx context.Context, command *exec.Cmd) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	command.WaitDelay = commandWaitDelay
	if ctx.Done() == nil {
		// Preserve ordinary process/signal behavior for the existing background API.
		return command.Run()
	}

	return errors.Join(runProcessTree(command), ctx.Err())
}

// goCommandOutput executes Go with optional environment overrides without logging them.
func goCommandOutput(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = dir
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := runCommand(ctx, command); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}

		if detail != "" {
			return nil, fmt.Errorf("go %s: %s: %w", strings.Join(args, " "), detail, err)
		}

		return nil, fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}

	return stdout.Bytes(), nil
}
