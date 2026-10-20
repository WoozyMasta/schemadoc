// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// jobCleanupTimeout bounds cleanup of already terminated job processes.
const jobCleanupTimeout = 5 * time.Second

// jobAccounting follows the JOBOBJECT_BASIC_ACCOUNTING_INFORMATION ABI.
// Field order must match the Windows structure, including the unused counters.
type jobAccounting struct {
	TotalUserTime             int64  // Total user time in 100 ns units.
	TotalKernelTime           int64  // Total kernel time in 100 ns units.
	ThisPeriodTotalUserTime   int64  // User time since the accounting reset.
	ThisPeriodTotalKernelTime int64  // Kernel time since the accounting reset.
	TotalPageFaultCount       uint32 // Accumulated page faults.
	TotalProcesses            uint32 // Processes ever assigned to this job.
	ActiveProcesses           uint32 // Processes still associated with this job.
	TotalTerminatedProcesses  uint32 // Processes terminated by limit violations.
}

// runProcessTree assigns the suspended child to a job before any target code runs.
// Closing the job terminates descendants; it does not isolate their permissions.
func runProcessTree(command *exec.Cmd) (err error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create subprocess job: %w", err)
	}

	defer func() { err = errors.Join(err, windows.CloseHandle(job)) }()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return fmt.Errorf("configure subprocess job: %w", err)
	}

	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW,
	}
	command.Cancel = func() error {
		jobErr := windows.TerminateJobObject(job, 1)
		// Cancellation can race job assignment; also stop the suspended root directly.
		_ = command.Process.Kill()
		return jobErr
	}

	if err := command.Start(); err != nil {
		return err
	}

	//nolint:gosec // Go exposes the Windows DWORD process ID as an int.
	if err := assignAndResumeProcess(job, uint32(command.Process.Pid)); err != nil {
		_ = command.Process.Kill()
		return errors.Join(err, command.Wait())
	}

	err = command.Wait()
	stopErr := windows.TerminateJobObject(job, 1)
	return errors.Join(err, stopErr, waitForJobExit(job))
}

// assignAndResumeProcess prevents child creation before job ownership is established.
func assignAndResumeProcess(job windows.Handle, pid uint32) (err error) {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return fmt.Errorf("open suspended process: %w", err)
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(process)) }()

	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		return fmt.Errorf("assign subprocess job: %w", err)
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("snapshot suspended process threads: %w", err)
	}

	defer func() { err = errors.Join(err, windows.CloseHandle(snapshot)) }()
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	threadErr := windows.Thread32First(snapshot, &entry)

	for ; threadErr == nil; threadErr = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}

		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return fmt.Errorf("open suspended thread: %w", err)
		}

		count, resumeErr := windows.ResumeThread(thread)
		resumeErr = errors.Join(resumeErr, windows.CloseHandle(thread))

		if resumeErr != nil {
			return fmt.Errorf("resume subprocess thread: %w", resumeErr)
		}

		if count == 1 {
			return nil
		}
	}

	if !errors.Is(threadErr, windows.ERROR_NO_MORE_FILES) {
		return fmt.Errorf("enumerate suspended process threads: %w", threadErr)
	}

	return fmt.Errorf("resume subprocess: %w", errSuspendedThreadNotFound)
}

// waitForJobExit lets Windows release executable files before workspace cleanup.
func waitForJobExit(job windows.Handle) error {
	deadline := time.NewTimer(jobCleanupTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		var info jobAccounting
		if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
			return fmt.Errorf("query subprocess job: %w", err)
		}

		if info.ActiveProcesses == 0 {
			return nil
		}

		select {
		case <-ticker.C:
		case <-deadline.C:
			return errProcessTreeCleanupTimeout
		}
	}
}
