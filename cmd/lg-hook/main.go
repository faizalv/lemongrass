package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const defaultServerURL = "http://lg-server:9966/api/lg"

type hookEvent struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

type bashInput struct {
	Command string `json:"command"`
}

type writeInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type lgRequest struct {
	Cmd       string `json:"cmd"`
	Args      string `json:"args"`
	Blocking  bool   `json:"blocking"`
	SessionID string `json:"session_id"`
}

type lgResponse struct {
	Text string `json:"text"`
}

type hookOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName            string            `json:"hookEventName"`
	PermissionDecision       string            `json:"permissionDecision"`
	PermissionDecisionReason string            `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             map[string]string `json:"updatedInput,omitempty"`
}

var hashRe = regexp.MustCompile(`[0-9a-f]{40}`)
var indexLineRe = regexp.MustCompile(`(?m)^index [0-9a-f]+\.\.[0-9a-f]+(?: \d+)?$\n?`)

var permittedGitSubs = map[string]bool{
	"log":    true,
	"diff":   true,
	"show":   true,
	"status": true,
	"blame":  true,
}

var gitApprovalOps = map[string]bool{
	"commit":      true,
	"push":        true,
	"reset":       true,
	"merge":       true,
	"rebase":      true,
	"cherry-pick": true,
}

var permittedCommands = map[string]bool{
	"cat":  true,
	"ls":   true,
	"find": true,
	"grep": true,
	"pwd":  true,
	"head": true,
	"tail": true,
	"wc":   true,
	"echo": true,
}

var destructiveCommands = map[string]bool{
	"rm":    true,
	"rmdir": true,
}

func main() {
	var event hookEvent
	if err := json.NewDecoder(os.Stdin).Decode(&event); err != nil {
		os.Exit(0)
	}

	switch event.ToolName {
	case "Write":
		handleWrite(event.ToolInput)
	case "Read":
		handleRead(event.ToolInput)
	case "Bash":
		handleBash(event.ToolInput)
	default:
		os.Exit(0)
	}
}

func handleWrite(raw json.RawMessage) {
	var input writeInput
	if err := json.Unmarshal(raw, &input); err != nil {
		os.Exit(0)
	}
	sessionID := os.Getenv("LG_SESSION_ID")
	body, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"file_path":  input.FilePath,
		"byte_count": len(input.Content),
	})
	client := &http.Client{Timeout: 5 * time.Second}
	client.Post(defaultServerURL+"/write-trail", "application/json", bytes.NewReader(body))
	os.Exit(0)
}

var groomingAllowedExts = map[string]bool{
	".pdf":  true,
	".md":   true,
	".txt":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
	".gif":  true,
	".log":  true,
}

func handleRead(raw json.RawMessage) {
	if os.Getenv("LG_SESSION_TYPE") != "grooming" {
		os.Exit(0)
	}
	var input struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		os.Exit(0)
	}
	ext := strings.ToLower(filepath.Ext(input.FilePath))
	if groomingAllowedExts[ext] {
		os.Exit(0)
	}
	reject("direct file reads are not permitted during grooming",
		"Use #lg.recon.read <file:symbol:start-end> to read source code through the semantic map.")
}

func handleBash(raw json.RawMessage) {
	var input bashInput
	if err := json.Unmarshal(raw, &input); err != nil {
		os.Exit(0)
	}
	cmd := input.Command

	switch {
	case strings.HasPrefix(cmd, "#lg!."):
		forwardToServer(strings.TrimPrefix(cmd, "#lg!."), false)
	case strings.HasPrefix(cmd, "#lg."):
		forwardToServer(strings.TrimPrefix(cmd, "#lg."), true)
	default:
		handlePermitted(cmd)
	}
}

func forwardToServer(rest string, blocking bool) {
	lgCmd, args := splitCmd(rest)
	body, _ := json.Marshal(lgRequest{Cmd: lgCmd, Args: args, Blocking: blocking, SessionID: os.Getenv("LG_SESSION_ID")})

	timeout := 5 * time.Second
	if blocking {
		timeout = 10 * time.Minute
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(defaultServerURL, "application/json", bytes.NewReader(body))
	if err != nil {
		if blocking {
			deliver(fmt.Sprintf("error: lg-server unreachable (%v)", err))
		} else {
			deliver("ok")
		}
		return
	}
	defer resp.Body.Close()

	if !blocking {
		deliver("ok")
		return
	}

	var r lgResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		deliver(fmt.Sprintf("error: could not parse server response: %v", err))
	} else if r.Text == "" {
		deliver("error: server returned empty response -- session may not be active")
	} else {
		deliver(r.Text)
	}
}

func handlePermitted(cmd string) {
	if hasWriteRedirect(cmd) {
		reject("write redirect detected", "File writes go through the execution protocol, not shell.")
		return
	}

	leading := leadingToken(cmd)

	if destructiveCommands[leading] {
		reject(leading+" is a destructive operation",
			"Use #lg.echo <message> to notify the user and explain what needs to be removed and why.")
		return
	}

	if leading == "git" {
		sub := gitSubcommand(cmd)
		if permittedGitSubs[sub] {
			runLocal(cmd, leading, sub)
			return
		}
		if gitApprovalOps[sub] {
			reject("git "+sub+" requires user approval",
				"Use #lg.echo <message> to surface the intent to the user.")
			return
		}
		reject("git "+sub+" not in permitted git command set",
			"Available git commands: log, diff, show, status, blame.\nUse #lg.echo to ask the user for anything else.")
		return
	}

	if permittedCommands[leading] {
		runLocal(cmd, leading, "")
		return
	}

	reject(leading+" not in permitted command set",
		"Available: git log/diff/show/status/blame, cat, ls, find, grep, pwd, head, tail, wc, echo.\nFor anything else, use #lg.echo to ask the user.")
}

func runLocal(cmd, leading, sub string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	result := transform(leading, sub, string(out))
	if err != nil && len(out) == 0 {
		result = fmt.Sprintf("error: %v", err)
	}
	deliver(result)
}

func reject(reason, guidance string) {
	deny(fmt.Sprintf("Rejected: %s.\n%s", reason, guidance))
}

func leadingToken(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if i := strings.IndexAny(cmd, " \t"); i >= 0 {
		return cmd[:i]
	}
	return cmd
}

func gitSubcommand(cmd string) string {
	parts := strings.Fields(cmd)
	for i, p := range parts {
		if p == "git" {
			for _, q := range parts[i+1:] {
				if !strings.HasPrefix(q, "-") {
					return q
				}
			}
		}
	}
	return ""
}

func hasWriteRedirect(cmd string) bool {
	inSingle := false
	inDouble := false
	for i := 0; i < len(cmd); i++ {
		switch cmd[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '>':
			if !inSingle && !inDouble {
				return true
			}
		}
	}
	return false
}

func transform(leading, sub, output string) string {
	if leading != "git" {
		return output
	}
	switch sub {
	case "log":
		return hashRe.ReplaceAllStringFunc(output, abbrevHash)
	case "diff", "show":
		result := indexLineRe.ReplaceAllString(output, "")
		return hashRe.ReplaceAllStringFunc(result, abbrevHash)
	case "blame":
		return hashRe.ReplaceAllStringFunc(output, abbrevHash)
	}
	return output
}

func abbrevHash(h string) string {
	if len(h) >= 7 {
		return h[:7]
	}
	return h
}

func splitCmd(s string) (cmd, args string) {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i], strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func deliver(content string) {
	out := hookOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
			UpdatedInput:       map[string]string{"command": "printf '%s' " + shellEscape(content)},
		},
	}
	json.NewEncoder(os.Stdout).Encode(out)
	os.Exit(0)
}

func deny(reason string) {
	out := hookOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "deny",
			PermissionDecisionReason: reason,
		},
	}
	json.NewEncoder(os.Stdout).Encode(out)
	os.Exit(0)
}
