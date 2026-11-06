// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	shellJobTermWait = 3 * time.Second
	shellJobKillWait = time.Second
	shellJobPoll     = 100 * time.Millisecond
)

type procStat struct {
	pid, ppid, pgid, sid int
}

type procReader interface {
	stat(pid int) (procStat, error)
	environ(pid int) ([]byte, error)
}

type liveProcs struct{}

func (liveProcs) stat(pid int) (procStat, error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return procStat{}, err
	}
	return parseProcStat(pid, string(raw))
}

func (liveProcs) environ(pid int) ([]byte, error) {
	return os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
}

// The command name sits in parentheses and may itself hold spaces or parentheses, so fields are read after the last one.
func parseProcStat(pid int, line string) (procStat, error) {
	end := strings.LastIndexByte(line, ')')
	if end < 0 {
		return procStat{}, fmt.Errorf("hook: malformed stat for pid %d", pid)
	}
	fields := strings.Fields(line[end+1:])
	if len(fields) < 4 {
		return procStat{}, fmt.Errorf("hook: short stat for pid %d", pid)
	}
	var nums [3]int
	for i := range nums {
		n, err := strconv.Atoi(fields[i+1])
		if err != nil {
			return procStat{}, fmt.Errorf("hook: stat field for pid %d: %w", pid, err)
		}
		nums[i] = n
	}
	return procStat{pid: pid, ppid: nums[0], pgid: nums[1], sid: nums[2]}, nil
}

// A Shell tab's shell is the leader of its PTY session, as node-pty starts it, and carries the marker the app sets.
func markedLeader(procs procReader, pid int) (procStat, bool) {
	st, err := procs.stat(pid)
	if err != nil || st.pid != st.sid {
		return procStat{}, false
	}
	env, err := procs.environ(pid)
	if err != nil || !bytes.Contains(append([]byte{0}, env...), []byte("\x00"+shellTabEnv+"=1\x00")) {
		return procStat{}, false
	}
	return st, true
}

// Finds the Shell tab's shell and the job running under it, starting from the agent that ran this hook. The agent
// runs hooks in a session of their own, so the walk starts at the parent and climbs while the session stays the same;
// its top must be a marked leader, and the job is the process group of the shell's child on the chain, never the
// shell's own group.
func shellTabJob(procs procReader, agent int) (shell int, job int, ok bool) {
	cur, err := procs.stat(agent)
	if err != nil {
		return 0, 0, false
	}
	below := procStat{}
	for cur.pid != cur.sid {
		parent, err := procs.stat(cur.ppid)
		if err != nil || parent.sid != cur.sid {
			return 0, 0, false
		}
		below, cur = cur, parent
	}
	if below.pid == 0 || below.pgid == cur.pid || below.pgid <= 1 {
		return 0, 0, false
	}
	if _, ok := markedLeader(procs, cur.pid); !ok {
		return 0, 0, false
	}
	return cur.pid, below.pgid, true
}

// The agent stops its hooks when it exits, so the ending runs in a process of its own session that outlives it.
func startShellTabEnd() {
	shell, job, ok := shellTabJob(liveProcs{}, os.Getppid())
	if !ok {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "hook", shellTabEndArg, strconv.Itoa(shell), strconv.Itoa(job))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Start() == nil {
		cmd.Process.Release()
	}
}

// Members of the job inside the shell's session, so a reused group id elsewhere is never touched.
func jobMembers(job, sid, self int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}
		if st, err := (liveProcs{}).stat(pid); err == nil && st.pgid == job && st.sid == sid {
			pids = append(pids, pid)
		}
	}
	return pids
}

func signalJob(job, sid, self int, sig syscall.Signal, wait time.Duration) bool {
	for _, pid := range jobMembers(job, sid, self) {
		syscall.Kill(pid, sig)
	}
	for deadline := time.Now().Add(wait); time.Now().Before(deadline); time.Sleep(shellJobPoll) {
		if len(jobMembers(job, sid, self)) == 0 {
			return true
		}
	}
	return false
}

// Ends the job the way Ctrl+C would, then prints the reason on the tab once the agent has left the terminal. Every
// event of the session starts an ender, so only the first to claim the job prints.
func runShellTabEnd(args []string) int {
	if len(args) != 2 {
		return 1
	}
	shellPid, err1 := strconv.Atoi(args[0])
	job, err2 := strconv.Atoi(args[1])
	if err1 != nil || err2 != nil || job == shellPid || job <= 1 {
		return 1
	}
	shell, ok := markedLeader(liveProcs{}, shellPid)
	if !ok {
		return 1
	}
	claim := filepath.Join(os.TempDir(), fmt.Sprintf("lgrass-shell-job-%d-%d", shell.pid, job))
	f, err := os.OpenFile(claim, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0
	}
	f.Close()
	defer os.Remove(claim)
	self := os.Getpid()
	if len(jobMembers(job, shell.sid, self)) == 0 {
		return 0
	}
	if !signalJob(job, shell.sid, self, syscall.SIGTERM, shellJobTermWait) &&
		!signalJob(job, shell.sid, self, syscall.SIGKILL, shellJobKillWait) {
		return 1
	}
	if tty, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(shell.pid), "fd", "0")); err == nil {
		if out, err := os.OpenFile(tty, os.O_WRONLY|syscall.O_NOCTTY, 0); err == nil {
			fmt.Fprintf(out, "\r\n%s\r\n", shellTabMessage)
			out.Close()
		}
	}
	return 0
}
