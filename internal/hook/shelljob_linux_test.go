// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"errors"
	"testing"
)

type fakeProcs struct {
	stats map[int]procStat
	envs  map[int]string
}

func (f fakeProcs) stat(pid int) (procStat, error) {
	st, ok := f.stats[pid]
	if !ok {
		return procStat{}, errors.New("gone")
	}
	return st, nil
}

func (f fakeProcs) environ(pid int) ([]byte, error) {
	return []byte(f.envs[pid]), nil
}

const markedEnv = "HOME=/h\x00LGRASS_SHELL_TAB=1\x00PATH=/bin\x00"

// electron(50, sid 1) > shell(100, leader) > npm(110, job 110) > claude(120, job 110)
func shellTree() fakeProcs {
	return fakeProcs{
		stats: map[int]procStat{
			50:  {pid: 50, ppid: 1, pgid: 50, sid: 1},
			100: {pid: 100, ppid: 50, pgid: 100, sid: 100},
			110: {pid: 110, ppid: 100, pgid: 110, sid: 100},
			120: {pid: 120, ppid: 110, pgid: 110, sid: 100},
		},
		envs: map[int]string{100: markedEnv},
	}
}

func TestParseProcStat(t *testing.T) {
	got, err := parseProcStat(97835, "97835 (a (b) c) S 97834 97835 97834 34834 97835 4194560")
	if err != nil {
		t.Fatal(err)
	}
	if got != (procStat{pid: 97835, ppid: 97834, pgid: 97835, sid: 97834}) {
		t.Errorf("parseProcStat() = %+v", got)
	}
}

func TestShellTabJobFindsTheShellsForegroundJob(t *testing.T) {
	shell, job, ok := shellTabJob(shellTree(), 120)
	if !ok || shell != 100 || job != 110 {
		t.Errorf("shellTabJob() = %+v, %d, %v", shell, job, ok)
	}
}

func TestShellTabJobNeedsTheMarkerOnTheShell(t *testing.T) {
	procs := shellTree()
	procs.envs[100] = "HOME=/h\x00LGRASS_SHELL_TAB=0\x00"
	if _, _, ok := shellTabJob(procs, 120); ok {
		t.Error("an unmarked session leader was taken for a Shell tab")
	}
}

func TestShellTabJobNeverTargetsTheShell(t *testing.T) {
	procs := shellTree()
	procs.stats[120] = procStat{pid: 120, ppid: 100, pgid: 100, sid: 100}
	if _, _, ok := shellTabJob(procs, 120); ok {
		t.Error("a job in the shell's own group was targeted")
	}
	if _, _, ok := shellTabJob(procs, 100); ok {
		t.Error("the shell itself was targeted")
	}
}

func TestShellTabJobStopsWhenTheSessionBreaks(t *testing.T) {
	procs := shellTree()
	procs.stats[120] = procStat{pid: 120, ppid: 110, pgid: 120, sid: 120}
	procs.stats[130] = procStat{pid: 130, ppid: 120, pgid: 130, sid: 120}
	if _, _, ok := shellTabJob(procs, 130); ok {
		t.Error("a walk across a session boundary found a job")
	}
	delete(procs.stats, 110)
	if _, _, ok := shellTabJob(procs, 120); ok {
		t.Error("a broken chain found a job")
	}
}
