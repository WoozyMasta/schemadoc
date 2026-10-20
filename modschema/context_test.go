// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// init implements an isolated fake Go executable for the preparation-cancellation test.
func init() {
	ready := os.Getenv("SCHEMADOC_TEST_FAKE_GO_READY")
	if ready == "" {
		return
	}

	dir, err := os.Getwd()
	if err != nil {
		os.Exit(2)
	}

	_, _ = fmt.Fprintln(os.Stderr, "test Go setup blocked")

	if err := os.WriteFile(ready, []byte(dir), 0o600); err != nil {
		os.Exit(2)
	}

	for {
		time.Sleep(time.Hour)
	}
}

func TestModulePathFromDirSyntax(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{"spaces", "module github.com/acme/project\n", false},
		{"tabs", "module\tgithub.com/acme/project\n", false},
		{"comments", "// Leading comment\n\nmodule\tgithub.com/acme/project // trailing comment\n\ngo 1.25.0\n", false},
		{"quoted", "module \"github.com/acme/project\"\n", false},
		{"missing directive", "go 1.25.0\n", true},
		{"missing path", "module\n", true},
		{"empty quoted path", "module \"\"\n", true},
		{"unclosed quote", "module \"github.com/acme/project\n", true},
		{"duplicate directive", "module github.com/acme/project\nmodule example.com/other\n", true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeGeneratorTestFile(t, filepath.Join(dir, "go.mod"), test.text)
			path, err := modulePathFromDir(dir)

			if test.wantErr {
				if !errors.Is(err, ErrInvalidModuleFile) {
					t.Fatalf("path=%q, error=%v, want ErrInvalidModuleFile", path, err)
				}

				return
			}

			if err != nil || path != "github.com/acme/project" {
				t.Fatalf("path=%q, error=%v", path, err)
			}
		})
	}
}

func TestGenerateContextAlreadyCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data, _, err := GenerateContext(ctx, Options{Module: "missing-module", Type: "Config"})

	if !errors.Is(err, context.Canceled) || len(data) != 0 {
		t.Fatalf("one-shot data=%q, error=%v", data, err)
	}

	var generator Generator
	data, _, err = generator.GenerateContext(ctx, Options{Module: "missing-module", Type: "Config"})

	if !errors.Is(err, context.Canceled) || len(data) != 0 || len(generator.workspaces) != 0 {
		t.Fatalf("session data=%q, error=%v", data, err)
	}
}

func TestContextCancellationWhileWaiting(t *testing.T) {
	t.Parallel()
	var generator Generator

	if err := generator.mu.lock(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer generator.mu.unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)

	go func() {
		_, _, err := generator.GenerateContext(ctx, Options{})
		result <- err
	}()

	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waiting generator error=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled request waited for the session owner")
	}

	var cache toolchainVersionCache

	if err := cache.mu.lock(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer cache.mu.unlock()

	cacheCtx, cancelCache := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelCache()

	go func() {
		_, err := cache.load(cacheCtx, probeGoToolchainVersion)
		result <- err
	}()

	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waiting version cache error=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled request waited for the version cache owner")
	}
}

func TestPreparationCancellationCleanup(t *testing.T) {
	// Resolve and cache the real toolchain version before replacing PATH in this test.
	if _, err := currentGoToolchainVersion(context.Background()); err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	copyTestExecutable(t, executable, filepath.Join(binDir, name))
	readyPath := filepath.Join(t.TempDir(), "ready")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SCHEMADOC_TEST_FAKE_GO_READY", readyPath)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	options := Options{Module: newGeneratorTestModule(t), Type: "Config"}
	result := make(chan error, 1)

	go func() {
		data, _, err := GenerateContext(ctx, options)
		if len(data) != 0 {
			err = fmt.Errorf("cancelled preparation returned schema bytes: %w", err)
		}

		result <- err
	}()

	// Cancel only after setup emitted stderr and published its workspace path.
	dir := waitForTestFile(t, readyPath, 20*time.Second)
	cancel()

	select {
	case err = <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("preparation error=%v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("preparation cancellation did not return promptly")
	}

	if !strings.Contains(err.Error(), "test Go setup blocked") || !strings.Contains(err.Error(), "mod init") {
		t.Fatalf("lost command or stderr detail: %v", err)
	}

	if _, err := os.Stat(string(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial workspace remains: %s: %v", dir, err)
	}
}

func TestGeneratorCancellationDuringInit(t *testing.T) {
	moduleDir := t.TempDir()
	writeGeneratorTestFile(t, filepath.Join(moduleDir, "go.mod"), "module example.com/blocking\n\ngo 1.25.0\n")
	writeGeneratorTestFile(t, filepath.Join(moduleDir, "config.go"), `package blocking

import (
	"fmt"
	"os"
	"time"
)

type Config struct { Value string }

func init() {
	if ready := os.Getenv("SCHEMADOC_TEST_INIT_READY"); ready != "" {
		fmt.Fprintln(os.Stderr, "test target init blocked")
		if err := os.WriteFile(ready, []byte(fmt.Sprint(os.Getpid())), 0600); err != nil { panic(err) }
		for { time.Sleep(time.Hour) }
	}
}
`)
	generator := NewGenerator()
	t.Cleanup(func() {
		if err := generator.Close(); err != nil {
			t.Error(err)
		}
	})
	options := Options{Module: moduleDir, Type: "Config"}
	baseline, _, err := generator.Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	var cancelledDir string

	for _, workspace := range generator.workspaces {
		cancelledDir = workspace.dir
	}

	other := Options{Module: newGeneratorTestModule(t), Type: "Config"}

	if _, _, err := generator.Generate(other); err != nil {
		t.Fatal(err)
	}

	readyPath := filepath.Join(t.TempDir(), "ready")
	t.Setenv("SCHEMADOC_TEST_INIT_READY", readyPath)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := make(chan error, 1)

	go func() {
		data, _, err := generator.GenerateContext(ctx, options)
		if len(data) != 0 {
			err = fmt.Errorf("cancelled request returned schema bytes: %w", err)
		}

		result <- err
	}()

	pid := waitForTestPID(t, readyPath, 20*time.Second)
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "test target init blocked") {
			t.Fatalf("init cancellation error=%v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("go run cancellation did not return promptly")
	}

	assertTestProcessStopped(t, pid)

	if _, err := os.Stat(cancelledDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled workspace remains: %v", err)
	}

	if len(generator.workspaces) != 1 {
		t.Fatalf("cancellation affected unrelated workspaces: %d", len(generator.workspaces))
	}

	t.Setenv("SCHEMADOC_TEST_INIT_READY", "")
	recovered, _, err := generator.GenerateContext(context.Background(), options)
	if err != nil || !bytes.Equal(recovered, baseline) {
		t.Fatalf("generator did not recover after cancellation: %v", err)
	}

	if _, _, err := generator.Generate(other); err != nil {
		t.Fatalf("unrelated workspace is not reusable: %v", err)
	}
}

func TestCommandCancellationTree(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("process-tree assertions require Windows or a supported Unix platform")
	}

	t.Parallel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	parentReady := filepath.Join(dir, "parent")
	childReady := filepath.Join(dir, "child")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestProcessFixture$")
	command.Env = append(os.Environ(), "SCHEMADOC_PROCESS_FIXTURE=parent", "SCHEMADOC_PROCESS_READY="+parentReady, "SCHEMADOC_PROCESS_CHILD_READY="+childReady)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	result := make(chan error, 1)

	go func() { result <- runCommand(ctx, command) }()
	parentPID := waitForTestPID(t, parentReady, 5*time.Second)
	childPID := waitForTestPID(t, childReady, 5*time.Second)
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("tree cancellation error=%v, stderr=%s", err, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("process-tree cancellation hung")
	}

	assertTestProcessStopped(t, parentPID)
	assertTestProcessStopped(t, childPID)
}

func TestCommandDeadline(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	ready := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestProcessFixture$")
	command.Env = append(os.Environ(), "SCHEMADOC_PROCESS_FIXTURE=child", "SCHEMADOC_PROCESS_READY="+ready)

	if err := runCommand(ctx, command); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error=%v", err)
	}

	// The deadline may expire before the fixture executes on a busy race build.
	// In that case there is no fixture PID, but any created root must still stop.
	if command.Process != nil {
		assertTestProcessStopped(t, command.Process.Pid)
	}

	if _, err := os.Stat(ready); err == nil {
		assertTestProcessStopped(t, waitForTestPID(t, ready, time.Second))
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

// TestProcessFixture runs only inside child test binaries used by cancellation tests.
func TestProcessFixture(t *testing.T) {
	mode := os.Getenv("SCHEMADOC_PROCESS_FIXTURE")
	if mode == "" {
		return
	}

	if mode == "parent" || mode == "parent-exit" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}

		command := exec.Command(executable, "-test.run=^TestProcessFixture$")
		command.Env = append(os.Environ(), "SCHEMADOC_PROCESS_FIXTURE=child", "SCHEMADOC_PROCESS_READY="+os.Getenv("SCHEMADOC_PROCESS_CHILD_READY"))
		command.Stdout, command.Stderr = os.Stdout, os.Stderr

		if err := command.Start(); err != nil {
			t.Fatal(err)
		}

		waitForTestPID(t, os.Getenv("SCHEMADOC_PROCESS_CHILD_READY"), 3*time.Second)
	}

	if err := os.WriteFile(os.Getenv("SCHEMADOC_PROCESS_READY"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}

	if mode == "parent-exit" {
		return
	}

	for {
		time.Sleep(time.Hour)
	}
}

// waitForTestFile waits for fixture readiness without assuming startup speed.
func waitForTestFile(t *testing.T, path string, timeout time.Duration) []byte {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		content, err := os.ReadFile(path)
		if err == nil && len(content) != 0 {
			return content
		}

		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("fixture did not become ready: %s", path)
			return nil
		}
	}
}

// waitForTestPID waits for a started fixture without assuming compile or startup speed.
func waitForTestPID(t *testing.T, path string, timeout time.Duration) int {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		content, err := os.ReadFile(path)
		if err == nil {
			if pid, err := strconv.Atoi(string(content)); err == nil && pid > 0 {
				return pid
			}
		}

		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("fixture did not start: %s", path)
			return 0
		}
	}
}

// assertTestProcessStopped checks cancellation of both root and descendant processes.
func assertTestProcessStopped(t *testing.T, pid int) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if !testProcessRunning(pid) {
			return
		}

		select {
		case <-ticker.C:
		case <-deadline.C:
			if process, err := os.FindProcess(pid); err == nil {
				_ = process.Kill()
			}
			t.Fatalf("process %d survived cancellation", pid)
		}
	}
}

// copyTestExecutable installs the test binary as a local fake Go command.
func copyTestExecutable(t *testing.T, source, target string) {
	t.Helper()
	in, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	_, copyErr := io.Copy(out, in)
	if err := errors.Join(copyErr, out.Close()); err != nil {
		t.Fatal(err)
	}
}
