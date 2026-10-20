// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

//go:build unix

package modschema

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCommandCancellationAfterRootExit(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	parentReady, childReady := filepath.Join(dir, "parent"), filepath.Join(dir, "child")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestProcessFixture$")
	command.Env = append(os.Environ(), "SCHEMADOC_PROCESS_FIXTURE=parent-exit", "SCHEMADOC_PROCESS_READY="+parentReady, "SCHEMADOC_PROCESS_CHILD_READY="+childReady)
	var stderr bytes.Buffer
	command.Stderr = &stderr

	if err := runCommand(ctx, command); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("orphan cancellation error=%v, stderr=%s", err, stderr.String())
	}

	assertTestProcessStopped(t, waitForTestPID(t, parentReady, time.Second))
	assertTestProcessStopped(t, waitForTestPID(t, childReady, time.Second))
}

// testProcessRunning treats an exited Linux zombie as stopped even before PID 1 reaps it.
func testProcessRunning(pid int) bool {
	if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		return false
	}

	if runtime.GOOS == "linux" {
		content, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if errors.Is(err, os.ErrNotExist) {
			return false
		}

		if end := strings.LastIndex(string(content), ") "); end >= 0 && len(content) > end+2 && content[end+2] == 'Z' {
			return false
		}
	}

	return true
}
