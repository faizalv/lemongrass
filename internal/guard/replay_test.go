// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// replayEnv names directories of Claude Code transcripts (os.PathListSeparator between them) whose shell calls the
// replay runs through the path pass. Transcripts hold private work, so the replay only prints counts and short
// samples to the terminal and nothing from them is committed.
const replayEnv = "LGRASS_GUARD_REPLAY"

type replayProgram struct {
	lang, code, cwd string
}

// replayPrograms reads every Bash and PowerShell tool call in the transcripts and returns the inline interpreter
// programs in them, each distinct program once, since one transcript is often reachable from several config dirs.
func replayPrograms(t *testing.T, dirs []string) (programs []replayProgram, calls int) {
	seen := map[string]bool{}
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 0, 1<<20), 64<<20)
			for scanner.Scan() {
				line := scanner.Bytes()
				if !strings.Contains(string(line), `"tool_use"`) {
					continue
				}
				var entry struct {
					Type    string `json:"type"`
					Cwd     string `json:"cwd"`
					Message struct {
						Content json.RawMessage `json:"content"`
					} `json:"message"`
				}
				if json.Unmarshal(line, &entry) != nil || entry.Type != "assistant" {
					continue
				}
				var blocks []struct {
					Type  string `json:"type"`
					Name  string `json:"name"`
					Input struct {
						Command string `json:"command"`
					} `json:"input"`
				}
				if json.Unmarshal(entry.Message.Content, &blocks) != nil {
					continue
				}
				for _, block := range blocks {
					if block.Type != "tool_use" || block.Name != "Bash" || block.Input.Command == "" {
						continue
					}
					calls++
					script, err := parseShell(block.Input.Command)
					if err != nil {
						continue
					}
					for _, program := range script.Programs {
						if program.Code == "" || program.Lang == "sh" || seen[program.Code] {
							continue
						}
						seen[program.Code] = true
						programs = append(programs, replayProgram{lang: program.Lang, code: program.Code, cwd: entry.Cwd})
					}
				}
			}
			return nil
		})
	}
	return programs, calls
}

func ruleSet(verdicts []Verdict) []string {
	var ids []string
	for _, v := range verdicts {
		if !slices.Contains(ids, v.RuleID) {
			ids = append(ids, v.RuleID)
		}
	}
	sort.Strings(ids)
	return ids
}

// durationSummary gives the total and the percentiles of per-program times.
func durationSummary(times []time.Duration) string {
	if len(times) == 0 {
		return "none"
	}
	sorted := slices.Clone(times)
	slices.Sort(sorted)
	var total time.Duration
	for _, d := range sorted {
		total += d
	}
	at := func(q float64) time.Duration { return sorted[min(len(sorted)-1, int(q*float64(len(sorted))))] }
	return fmt.Sprintf("total %s, median %s, p90 %s, p99 %s, max %s", total, at(0.5), at(0.9), at(0.99), sorted[len(sorted)-1])
}

// snippet shows the line of code around a token, cut to a readable width.
func snippet(code, token string) string {
	i := strings.Index(code, token)
	if i < 0 {
		return token
	}
	start := max(strings.LastIndexByte(code[:i], '\n')+1, i-60)
	end := i + len(token)
	if nl := strings.IndexByte(code[end:], '\n'); nl >= 0 {
		end = min(end+nl, end+60)
	} else {
		end = min(len(code), end+60)
	}
	return strings.TrimSpace(code[start:end])
}

// TestReplayPathPass compares today's mention scan with the path pass the guard runs, over real shell calls.
// Run: LGRASS_GUARD_REPLAY=<transcript dirs> go test -run TestReplayPathPass -v ./internal/guard/
func TestReplayPathPass(t *testing.T) {
	env := os.Getenv(replayEnv)
	if env == "" {
		t.Skip("set " + replayEnv + " to directories of transcripts to replay")
	}
	home, _ := os.UserHomeDir()
	programs, calls := replayPrograms(t, filepath.SplitList(env))
	byLang := map[string]int{}
	oldRules := map[string]int{}
	newRules := map[string]int{}
	samples := map[string][]string{}
	var added, removed, kept []string
	fallbacks := 0
	fallbackWhy := map[string]int{}
	var scanTimes, passTimes []time.Duration
	var slowest time.Duration
	slowestBytes, largestBytes := 0, 0
	roundCounts := map[int]int{}
	for _, p := range programs {
		byLang[p.lang]++
		c := checker{in: Input{Tool: "Bash", Cwd: p.cwd, Home: home, ProjectRoot: p.cwd}, home: ResolveDir(home)}
		if p.lang == "python" {
			if _, ok := c.pythonPathVerdicts(p.code); !ok {
				fallbacks++
				reason := "dynamic or unsettled"
				if _, err := pyParse(p.code); err != nil {
					reason = err.Error()
					if pe, ok := err.(*pyParseError); ok {
						reason = fmt.Sprintf("parse stops at %q (kind %d)", pe.near, pe.kind)
					}
				}
				fallbackWhy[reason]++
				if dump := os.Getenv(replayEnv + "_FALLBACK"); dump != "" {
					os.WriteFile(filepath.Join(dump, fmt.Sprintf("fallback-%03d.py", fallbacks)), []byte(p.code), 0o600)
				}
			}
		}
		start := time.Now()
		oldVerdicts := c.mentionScanVerdicts(p.code)
		scanTimes = append(scanTimes, time.Since(start))
		start = time.Now()
		curVerdicts := c.pathPassVerdicts(p.lang, p.code)
		took := time.Since(start)
		passTimes = append(passTimes, took)
		if took > slowest {
			slowest, slowestBytes = took, len(p.code)
		}
		largestBytes = max(largestBytes, len(p.code))
		if p.lang == "python" {
			if prog, err := pyParse(p.code); err == nil {
				guarded := make([]bool, len(prog.lits))
				for i, lit := range prog.lits {
					for _, token := range pathTokens(lit.value) {
						if len(c.pathChecks(token, true)) > 0 {
							guarded[i] = true
						}
					}
				}
				if _, rounds, ok := pyAccessRounds(prog, guarded); ok {
					roundCounts[rounds]++
				}
			}
		}
		old := ruleSet(oldVerdicts)
		cur := ruleSet(curVerdicts)
		for _, id := range old {
			oldRules[id]++
		}
		for _, id := range cur {
			newRules[id]++
		}
		for _, token := range pathTokens(p.code) {
			for _, id := range ruleSet(c.pathChecks(token, true)) {
				if os.Getenv(replayEnv+"_ALL") != "" || len(samples[id]) < 15 {
					samples[id] = append(samples[id], fmt.Sprintf("[%s] %s", p.lang, snippet(p.code, token)))
				}
			}
		}
		for _, id := range cur {
			if !slices.Contains(old, id) {
				added = append(added, fmt.Sprintf("%s [%s] %.200q", id, p.lang, p.code))
			}
		}
		for _, id := range old {
			if !slices.Contains(cur, id) {
				removed = append(removed, fmt.Sprintf("%s [%s] %.200q", id, p.lang, p.code))
			}
		}
		for _, id := range cur {
			kept = append(kept, fmt.Sprintf("%s [%s] %.300q", id, p.lang, p.code))
		}
		if dump := os.Getenv(replayEnv + "_FALLBACK"); dump != "" && len(cur) > 0 {
			os.WriteFile(filepath.Join(dump, fmt.Sprintf("kept-%03d.py", len(kept))), []byte(p.code), 0o600)
		}
	}
	t.Logf("shell calls %d, distinct inline programs %d, by language %v", calls, len(programs), byLang)
	t.Logf("Python programs the reading could not follow (mention scan kept): %d", fallbacks)
	type reasonCount struct {
		why string
		n   int
	}
	var reasons []reasonCount
	for why, n := range fallbackWhy {
		reasons = append(reasons, reasonCount{why, n})
	}
	sort.Slice(reasons, func(i, j int) bool { return reasons[i].n > reasons[j].n })
	for i, r := range reasons {
		if i == 25 {
			break
		}
		t.Logf("  %4d %s", r.n, r.why)
	}
	if os.Getenv(replayEnv+"_ALL") != "" {
		for _, line := range kept {
			t.Logf("still refused: %s", line)
		}
	}
	t.Logf("mention scan time: %s", durationSummary(scanTimes))
	t.Logf("path pass time:    %s", durationSummary(passTimes))
	t.Logf("slowest path pass %s on %d bytes; largest program %d bytes; flow rounds to settle: %v",
		slowest, slowestBytes, largestBytes, roundCounts)
	t.Logf("programs refused by the mention scan, by rule: %v", oldRules)
	t.Logf("programs refused by the path pass, by rule: %v", newRules)
	for id, list := range samples {
		t.Logf("samples for %s:\n  %s", id, strings.Join(list, "\n  "))
	}
	t.Logf("removed by the path pass: %d", len(removed))
	for i, line := range removed {
		if i == 40 {
			break
		}
		t.Logf("  %s", line)
	}
	if len(added) > 0 {
		t.Errorf("the path pass refuses %d programs the mention scan let through:\n  %s", len(added), strings.Join(added, "\n  "))
	}
}
