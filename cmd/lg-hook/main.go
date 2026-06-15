package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultServerURL = "http://lg-server:9966/api/lg"

var isHost = "false" // stamped with -X during build
var activeServerURL = defaultServerURL
var activeProjectID int64
var activeProjectDir string
var activeSessionDir string

type lgHostConfig struct {
	ProjectID  int64  `json:"project_id"`
	ServerURL  string `json:"server_url"`
	ProjectDir string `json:"project_dir"`
}

func findLgConfig() *lgHostConfig {
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, ".lemongrass", "config.json"))
		if err == nil {
			var cfg lgHostConfig
			if json.Unmarshal(data, &cfg) == nil && cfg.ProjectID > 0 {
				return &cfg
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

type hookEvent struct {
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	SessionID     string          `json:"session_id"`
	HookEventName string          `json:"hook_event_name"`
	ToolResponse  json.RawMessage `json:"tool_response"`
}

type bashInput struct {
	Command                   string `json:"command"`
	DangerouslyDisableSandbox bool   `json:"dangerouslyDisableSandbox"`
}

type writeInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type lgRequest struct {
	Cmd       string `json:"cmd"`
	Args      string `json:"args"`
	Blocking  bool   `json:"blocking"`
	SessionID string `json:"session_id,omitempty"`
	ProjectID int64  `json:"project_id,omitempty"`
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
var diffHeaderRe = regexp.MustCompile(`(?m)^[-+]{3} [ab]/.+$\n?`)
var noNewlineRe = regexp.MustCompile(`(?m)^\\ No newline at end of file\n?`)

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

var fileReaderCommands = map[string]bool{
	"cat":  true,
	"head": true,
	"tail": true,
}

var permittedCommands = map[string]bool{
	"pwd":  true,
	"wc":   true,
	"echo": true,
}

var destructiveCommands = map[string]bool{
	"rm":    true,
	"rmdir": true,
}

const skillBashThreshold = 5

func lgDir() string { return filepath.Join(os.Getenv("HOME"), ".lemongrass") }

func skillFlagIsSet() bool {
	_, err := os.Stat(filepath.Join(activeSessionDir, ".skill-reload-needed"))
	return err == nil
}

func setSkillFlag() {
	os.WriteFile(filepath.Join(activeSessionDir, ".skill-reload-needed"), nil, 0644)
}

func skillCompactFlagIsSet() bool {
	_, err := os.Stat(filepath.Join(activeSessionDir, ".skill-compact-pending"))
	return err == nil
}

func setSkillCompactFlag() {
	os.WriteFile(filepath.Join(activeSessionDir, ".skill-compact-pending"), nil, 0644)
}

func clearSkillCompactFlag() {
	os.Remove(filepath.Join(activeSessionDir, ".skill-compact-pending"))
}

func skillCompactMsg() string {
	return "[lg] context was compacted -- load the skill first:\n  Read ~/.claude/skills/lemongrass/SKILL.md\n  or call Skill(lemongrass)"
}

func updateLgTime() {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	os.WriteFile(filepath.Join(activeSessionDir, ".last-lg-time"), []byte(ts), 0644)
}

func elapsedSinceLg() time.Duration {
	data, err := os.ReadFile(filepath.Join(activeSessionDir, ".last-lg-time"))
	if err != nil {
		return 0
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return time.Since(time.Unix(ts, 0))
}

func clearSkillFlag() {
	os.Remove(filepath.Join(activeSessionDir, ".skill-reload-needed"))
	os.Remove(filepath.Join(activeSessionDir, ".grep-reminder-shown"))
	os.WriteFile(filepath.Join(activeSessionDir, ".bash-since-lg"), []byte("0"), 0644)
	updateLgTime()
}

func grepReminderShown() bool {
	_, err := os.Stat(filepath.Join(activeSessionDir, ".grep-reminder-shown"))
	return err == nil
}

func setGrepReminder() {
	os.WriteFile(filepath.Join(activeSessionDir, ".grep-reminder-shown"), nil, 0644)
}

func trackBashCall() {
	counterPath := filepath.Join(activeSessionDir, ".bash-since-lg")
	data, _ := os.ReadFile(counterPath)
	n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	n++
	if n >= skillBashThreshold {
		setSkillFlag()
		os.WriteFile(counterPath, []byte("0"), 0644)
	} else {
		os.WriteFile(counterPath, []byte(strconv.Itoa(n)), 0644)
	}
}

func skillReloadMsg() string {
	msg := "[lg] skill not loaded -- reload via Skill(lemongrass) or Read ~/.claude/skills/lemongrass/SKILL.md"
	if elapsed := elapsedSinceLg(); elapsed > 0 {
		msg += fmt.Sprintf(" (%d min since last lg call)", int(elapsed.Minutes()))
	}
	return msg
}

func main() {
	if isHost == "true" {
		if cfg := findLgConfig(); cfg != nil {
			activeServerURL = cfg.ServerURL + "/api/lg"
			activeProjectID = cfg.ProjectID
			activeProjectDir = cfg.ProjectDir
		}
	}

	var rawInput []byte
	rawInput, _ = io.ReadAll(os.Stdin)
	fmt.Fprintf(os.Stderr, "[lg-hook] raw stdin: %s\n", rawInput)

	var event hookEvent
	if err := json.Unmarshal(rawInput, &event); err != nil {
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "[lg-hook] tool=%q session=%q hookEvent=%q projectID=%d\n",
		event.ToolName, event.SessionID, event.HookEventName, activeProjectID)

	activeSessionDir = lgDir()
	if event.SessionID != "" {
		activeSessionDir = filepath.Join(os.TempDir(), "lg-hook-"+event.SessionID)
		os.MkdirAll(activeSessionDir, 0755)
	}

	if event.HookEventName == "PostCompact" {
		handlePostCompact(event.SessionID)
	}

	switch event.ToolName {
	case "Write":
		handleWrite(event.ToolInput, event.HookEventName)
	case "Read":
		handleRead(event.ToolInput, event.SessionID, event.HookEventName, event.ToolResponse)
	case "Edit":
		handleEdit(event.ToolInput, event.HookEventName)
	case "Bash":
		handleBash(event.ToolInput, event.SessionID)
	case "Skill":
		handleSkillPostUse(event.ToolInput, event.HookEventName)
	default:
		os.Exit(0)
	}
}

func handleWrite(raw json.RawMessage, hookEvent string) {
	if isHost == "true" {
		if hookEvent == "PostToolUse" {
			os.Exit(0)
		}
		if activeProjectID > 0 && skillCompactFlagIsSet() {
			deny(skillCompactMsg())
		}
		if activeProjectID > 0 && skillFlagIsSet() {
			deny(skillReloadMsg())
		}
		allowTool()
		return
	}
	var input writeInput
	if err := json.Unmarshal(raw, &input); err != nil {
		os.Exit(0)
	}
	sessionID := os.Getenv("LG_SESSION_ID")
	if hookEvent == "PostToolUse" {
		logWriteTrail(sessionID, input.FilePath, len(input.Content))
		releaseLock(sessionID, input.FilePath)
		allowTool()
		return
	}
	acquireLockOrDeny(sessionID, input.FilePath)
}

type editInput struct {
	FilePath string `json:"file_path"`
}

func handleEdit(raw json.RawMessage, hookEvent string) {
	if isHost == "true" {
		if hookEvent == "PostToolUse" {
			os.Exit(0)
		}
		if activeProjectID > 0 && skillCompactFlagIsSet() {
			deny(skillCompactMsg())
		}
		if activeProjectID > 0 && skillFlagIsSet() {
			deny(skillReloadMsg())
		}
		allowTool()
		return
	}
	var input editInput
	if err := json.Unmarshal(raw, &input); err != nil {
		allowTool()
		return
	}
	sessionID := os.Getenv("LG_SESSION_ID")
	if hookEvent == "PostToolUse" {
		logWriteTrail(sessionID, input.FilePath, 0)
		releaseLock(sessionID, input.FilePath)
		allowTool()
		return
	}
	acquireLockOrDeny(sessionID, input.FilePath)
}


func logWriteTrail(sessionID, filePath string, byteCount int) {
	body, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"file_path":  filePath,
		"byte_count": byteCount,
	})
	client := buildHTTPClient(5 * time.Second)
	client.Post(activeServerURL+"/write-trail", "application/json", bytes.NewReader(body))
}

func acquireLockOrDeny(sessionID, filePath string) {
	body, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"file_path":  filePath,
	})
	client := buildHTTPClient(5 * time.Second)
	resp, err := client.Post(activeServerURL+"/lock-acquire", "application/json", bytes.NewReader(body))
	if err != nil {
		allowTool()
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		var r lgResponse
		json.NewDecoder(resp.Body).Decode(&r)
		deny(r.Text)
		return
	}
	allowTool()
}

func releaseLock(sessionID, filePath string) {
	body, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"file_path":  filePath,
	})
	client := buildHTTPClient(5 * time.Second)
	client.Post(activeServerURL+"/lock-release", "application/json", bytes.NewReader(body))
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

var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".webp": true, ".svg": true,
}

var documentExts = map[string]bool{
	".pdf": true, ".docx": true, ".xlsx": true, ".pptx": true,
}

func notifyFileRead(filePath, sessionID string) {
	if activeProjectID == 0 {
		return
	}
	body, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"file_path":  filePath,
	})
	client := &http.Client{Timeout: 2 * time.Second}
	client.Post(activeServerURL+"/read-trail", "application/json", bytes.NewReader(body))
}

func handleRead(raw json.RawMessage, sessionID, hookEvent string, toolResponse json.RawMessage) {
	var input struct {
		FilePath string `json:"file_path"`
		Offset   *int   `json:"offset"`
		Limit    *int   `json:"limit"`
	}

	if hookEvent == "PostToolUse" {
		json.Unmarshal(raw, &input)
		if strings.HasSuffix(input.FilePath, "skills/lemongrass/SKILL.md") {
			clearSkillCompactFlag()
		}
		os.Exit(0)
	}

	if isHost == "true" {
		json.Unmarshal(raw, &input)

		skillFile := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "lemongrass", "SKILL.md")
		isSkillFile := input.FilePath == skillFile || strings.HasSuffix(input.FilePath, "skills/lemongrass/SKILL.md")

		if activeProjectID > 0 && skillCompactFlagIsSet() {
			if !isSkillFile {
				deny(skillCompactMsg())
				return
			}
		}
		if activeProjectID > 0 && skillFlagIsSet() {
			if isSkillFile {
				clearSkillFlag()
			} else {
				deny(skillReloadMsg())
				return
			}
		}

		ext := strings.ToLower(filepath.Ext(input.FilePath))

		if imageExts[ext] {
			allowTool()
			return
		}

		if documentExts[ext] {
			deny("[lg] use #lg.system.read " + input.FilePath + ". We use markitdown for docs")
			return
		}

		hasRange := input.Offset != nil || input.Limit != nil

		if !hasRange {
			info, err := os.Stat(input.FilePath)
			if err != nil {
				allowTool()
				return
			}
			if info.Size() > 10000 {
				deny(fmt.Sprintf("%s is large -- specify a range: Read(file_path=%q, offset=N, limit=M)", input.FilePath, input.FilePath))
				return
			}
			notifyFileRead(input.FilePath, sessionID)
			allowTool()
			return
		}

		f, err := os.Open(input.FilePath)
		if err != nil {
			allowTool()
			return
		}
		defer f.Close()

		start := 0
		if input.Offset != nil {
			start = *input.Offset
		}
		end := start + 999999
		if input.Limit != nil {
			end = start + *input.Limit
		}

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		lineNum := 0
		for scanner.Scan() {
			if lineNum >= end {
				break
			}
			if lineNum >= start {
				if len(scanner.Bytes()) > 2000 {
					deny(fmt.Sprintf("line %d has %d chars -- use #lg.system.read %s for this file", lineNum+1, len(scanner.Bytes()), input.FilePath))
					return
				}
			}
			lineNum++
		}

		notifyFileRead(input.FilePath, sessionID)
		allowTool()
		return
	}

	if err := json.Unmarshal(raw, &input); err != nil {
		os.Exit(0)
	}

	switch os.Getenv("LG_SESSION_TYPE") {
	case "grooming":
		ext := strings.ToLower(filepath.Ext(input.FilePath))
		if groomingAllowedExts[ext] {
			allowTool()
		}
		reject("direct file reads are not permitted during grooming",
			"Use #lg.recon.read <path:symbol:kind> to read source code through the semantic map.")

	case "execution":
		if strings.HasPrefix(input.FilePath, "/projects/") {
			allowTool()
		}
		reject("reads outside project directories are not permitted",
			"Use #lg.recon.read <path:symbol:kind> for exploration.\nNative Read is only permitted for /projects/ files as a prerequisite for Edit.")

	default:
		allowTool()
	}
}

func handleBash(raw json.RawMessage, sessionID string) {
	var input bashInput
	if err := json.Unmarshal(raw, &input); err != nil {
		os.Exit(0)
	}
	if input.DangerouslyDisableSandbox {
		deny("[lg] dangerouslyDisableSandbox is banned. Behave!")
	}
	cmd := input.Command

	if isHost == "true" && (strings.HasPrefix(cmd, "#lg.") || strings.HasPrefix(cmd, "#lg!.")) {
		clearSkillFlag()
	}

	switch {
	case strings.HasPrefix(cmd, "#lg.system.read.confirm "):
		handleSystemReadConfirm(strings.TrimPrefix(cmd, "#lg.system.read.confirm "))
	case strings.HasPrefix(cmd, "#lg.system.read "):
		handleSystemRead(strings.TrimPrefix(cmd, "#lg.system.read "))
	case strings.HasPrefix(cmd, "#lg!.") || strings.HasPrefix(cmd, "#lg."):
		if isHost == "true" && activeProjectID == 0 {
			deliver("lemongrass is not initialised for this project -- run `lemongrass init` in the project root first")
			return
		}
		if strings.HasPrefix(cmd, "#lg!.") {
			forwardToServerWithSession(strings.TrimPrefix(cmd, "#lg!."), false, sessionID)
		} else {
			forwardToServerWithSession(strings.TrimPrefix(cmd, "#lg."), true, sessionID)
		}
	default:
		leading := leadingToken(cmd)
		if fileReaderCommands[leading] {
			reject(leading+" is not available",
				"Use #lg.system.read <path> for file access.")
			return
		}
		if isHost == "true" {
			if activeProjectID > 0 && skillCompactFlagIsSet() {
				deny(skillCompactMsg())
				return
			}
			if activeProjectID > 0 && skillFlagIsSet() {
				deny(skillReloadMsg())
				return
			}
			if activeProjectID > 0 {
				if cmdTargetsProject(cmd, activeProjectDir) {
					switch leading {
					case "ls":
						reject("ls is not available inside the project",
							"Use #lg.codebase.ls [path] for directory listing.")
						return
					case "find":
						reject("find is not available inside the project",
							"Use #lg.codebase.fl <pattern> to find files by name or glob.")
						return
					case "grep":
						reject("grep is not available inside the project",
							"Use #lg.codebase.search <pattern> [path/] for code search.")
						return
					}
				}
				trackBashCall()
			}
			allowTool()
			return
		}
		handlePermitted(cmd)
	}
}

func buildHTTPClient(timeout time.Duration) *http.Client {
	socketPath := filepath.Join(os.Getenv("HOME"), ".lemongrass", "lg.sock")
	if _, err := os.Stat(socketPath); err == nil {
		return &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
				},
			},
		}
	}
	return &http.Client{Timeout: timeout}
}

func forwardToServer(rest string, blocking bool) {
	forwardToServerWithSession(rest, blocking, "")
}

func forwardToServerWithSession(rest string, blocking bool, sessionID string) {
	lgCmd, args := splitCmd(rest)
	req := lgRequest{Cmd: lgCmd, Args: args, Blocking: blocking}
	if activeProjectID > 0 {
		req.ProjectID = activeProjectID
		req.SessionID = sessionID
	} else {
		req.SessionID = os.Getenv("LG_SESSION_ID")
	}
	body, _ := json.Marshal(req)

	timeout := 5 * time.Second
	if blocking {
		timeout = 10 * time.Minute
	}

	client := buildHTTPClient(timeout)
	resp, err := client.Post(activeServerURL, "application/json", bytes.NewReader(body))
	if err != nil {
		if blocking {
			deliver(fmt.Sprintf("error: lg-server unreachable (%v)", err))
		} else {
			deliver("ok")
		}
		return
	}
	defer resp.Body.Close()

	var r lgResponse
	json.NewDecoder(resp.Body).Decode(&r)
	fmt.Fprintf(os.Stderr, "[lg-hook] server response: %q\n", r.Text)

	if !blocking {
		if strings.HasPrefix(r.Text, "error:") {
			deliver(r.Text)
		} else {
			deliver("ok")
		}
		return
	}

	if r.Text == "" {
		deliver("error: server returned empty response. Session may not be active")
	} else {
		deliver(r.Text)
	}
}

func handlePostCompact(sessionID string) {
	logPath := filepath.Join(os.Getenv("HOME"), ".lemongrass", "logs", "post-compact.log")
	if lf, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		fmt.Fprintf(lf, "=== POST-COMPACT FIRED === sessionID=%q projectID=%d\n", sessionID, activeProjectID)
		lf.Close()
	}
	if sessionID != "" && activeProjectID > 0 {
		req := lgRequest{Cmd: "session.compact", Args: "", Blocking: true, SessionID: sessionID, ProjectID: activeProjectID}
		body, _ := json.Marshal(req)
		client := &http.Client{Timeout: 5 * time.Second}
		client.Post(activeServerURL, "application/json", bytes.NewReader(body))
	}
	setSkillCompactFlag()
	os.WriteFile(filepath.Join(activeSessionDir, ".bash-since-lg"), []byte("0"), 0644)
	fmt.Printf("[lg] context compacted -- load the skill to continue:\n  Read ~/.claude/skills/lemongrass/SKILL.md\n  or call Skill(lemongrass)\n")
	os.Exit(0)
}

func handleSkillPostUse(raw json.RawMessage, hookEvent string) {
	if hookEvent != "PostToolUse" {
		os.Exit(0)
	}
	var input struct {
		Skill string `json:"skill"`
	}
	json.Unmarshal(raw, &input)
	if strings.Contains(strings.ToLower(input.Skill), "lemongrass") {
		clearSkillCompactFlag()
	}
	os.Exit(0)
}

func projectTmpDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(dir, ".lemongrass", "tmp")
}

func convertDocument(path string) {
	tmpDir := projectTmpDir()
	cachedPath := filepath.Join(tmpDir, filepath.Base(path)+".md")

	if _, err := os.Stat(cachedPath); err == nil {
		handleSystemRead(cachedPath)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		deliver(fmt.Sprintf("error: %v", err))
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	body, _ := json.Marshal(map[string]string{
		"content": base64.StdEncoding.EncodeToString(data),
		"ext":     ext,
	})
	convertURL := strings.Replace(activeServerURL, "/api/lg", "/api/convert", 1)
	client := buildHTTPClient(30 * time.Second)
	resp, err := client.Post(convertURL, "application/json", bytes.NewReader(body))
	if err != nil {
		deliver(fmt.Sprintf("error: convert unavailable (%v)", err))
		return
	}
	defer resp.Body.Close()
	var r struct {
		Markdown string `json:"markdown"`
		Error    string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	if r.Error != "" {
		deliver("error: " + r.Error)
		return
	}

	os.MkdirAll(tmpDir, 0755)
	os.WriteFile(cachedPath, []byte(r.Markdown), 0644)
	handleSystemRead(cachedPath)
}

func handleSystemRead(args string) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	path := parts[0]
	if path == "" {
		deliver("error: path required")
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	if documentExts[ext] {
		convertDocument(path)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		deliver(fmt.Sprintf("error: %v", err))
		return
	}
	lines := strings.Split(string(data), "\n")
	lineCount := len(lines)
	charCount := len(data)
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		rangeStr := strings.TrimSpace(parts[1])
		sep := strings.IndexByte(rangeStr, '-')
		if sep < 0 {
			deliver("error: range must be N-M (e.g. 10-50)")
			return
		}
		var start, end int
		fmt.Sscanf(rangeStr[:sep], "%d", &start)
		fmt.Sscanf(rangeStr[sep+1:], "%d", &end)
		if start < 1 {
			start = 1
		}
		if end > lineCount {
			end = lineCount
		}
		if start > end {
			deliver("error: start must be <= end")
			return
		}
		deliver(fmt.Sprintf("%s L%d-%d\n%s", path, start, end, strings.Join(lines[start-1:end], "\n")))
		return
	}
	const maxLines = 150
	const maxChars = 10000
	if lineCount <= maxLines && charCount <= maxChars {
		deliver(fmt.Sprintf("%s (%d lines, %d chars)\n%s", path, lineCount, charCount, string(data)))
		return
	}
	deliver(fmt.Sprintf("%s is %d lines and %d chars -- too large to deliver in full.\n"+
		"Use: #lg.system.read.confirm %s <N-M> to read specific lines.\n"+
		"Or #lg.recon.peruse <path:symbol:kind> for symbol-level access.\n"+
		"Or #lg.codebase.interim F:%s to load into the workbench.",
		path, lineCount, charCount, path, path))
}

func handleSystemReadConfirm(args string) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	path := parts[0]
	if path == "" {
		deliver("error: path required")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		deliver(fmt.Sprintf("error: %v", err))
		return
	}
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		deliver(string(data))
		return
	}
	rangeStr := strings.TrimSpace(parts[1])
	sep := strings.IndexByte(rangeStr, '-')
	if sep < 0 {
		deliver("error: range must be N-M (e.g. 10-50)")
		return
	}
	var start, end int
	fmt.Sscanf(rangeStr[:sep], "%d", &start)
	fmt.Sscanf(rangeStr[sep+1:], "%d", &end)
	lines := strings.Split(string(data), "\n")
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		deliver("error: start must be <= end")
		return
	}
	deliver(strings.Join(lines[start-1:end], "\n"))
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

	if fileReaderCommands[leading] {
		reject(leading+" is not available",
			"Use #lg.system.read <path> for file access.")
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

	switch leading {
	case "ls":
		reject("ls is not available in a lemongrass project",
			"Use #lg.codebase.ls [path] for directory listing.")
		return
	case "find":
		reject("find is not available in a lemongrass project",
			"Use #lg.codebase.fl <pattern> to find files by name or glob.")
		return
	case "grep":
		reject("grep is not available in a lemongrass project",
			"Use #lg.codebase.search <pattern> [path/] for code search.")
		return
	}

	if permittedCommands[leading] {
		runLocal(cmd, leading, "")
		return
	}

	reject(leading+" not in permitted command set",
		"Available: git log/diff/show/status/blame, pwd, wc, echo.\nFor anything else, use #lg.echo to ask the user.")
}

func runLocal(cmd, leading, sub string) {
	if leading == "git" && sub == "log" {
		if !strings.Contains(cmd, "--format") && !strings.Contains(cmd, "--oneline") && !strings.Contains(cmd, "--pretty") {
			cmd = cmd + " --oneline --no-decorate"
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	result := transform(leading, sub, string(out))
	if err != nil && len(out) == 0 {
		result = fmt.Sprintf("error: %v", err)
	}
	if leading == "grep" {
		result = transformGrep(result)
	}
	deliver(capOutput(result))
}

func transformGrep(output string) string {
	type match struct {
		lineno  string
		content string
	}
	fileMatches := map[string][]match{}
	fileOrder := []string{}

	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		// try file:lineno:content (grep -n)
		first := strings.IndexByte(line, ':')
		if first < 0 {
			return output
		}
		rest := line[first+1:]
		second := strings.IndexByte(rest, ':')
		if second >= 0 {
			file := line[:first]
			lineno := rest[:second]
			content := strings.TrimSpace(rest[second+1:])
			// only treat as file:lineno:content if lineno is numeric
			isNum := lineno != "" && func() bool {
				for _, c := range lineno {
					if c < '0' || c > '9' {
						return false
					}
				}
				return true
			}()
			if isNum {
				if _, seen := fileMatches[file]; !seen {
					fileOrder = append(fileOrder, file)
				}
				fileMatches[file] = append(fileMatches[file], match{lineno, content})
				continue
			}
		}
		// fallback: not a parseable grep line, return raw
		return output
	}

	if len(fileOrder) == 0 {
		return output
	}

	var sb strings.Builder
	for _, file := range fileOrder {
		matches := fileMatches[file]
		// deduplicate consecutive repeated content within the file
		deduped := []match{}
		counts := []int{}
		for _, m := range matches {
			if len(deduped) > 0 && deduped[len(deduped)-1].content == m.content {
				counts[len(counts)-1]++
			} else {
				deduped = append(deduped, m)
				counts = append(counts, 1)
			}
		}
		n := len(matches)
		word := "matches"
		if n == 1 {
			word = "match"
		}
		fmt.Fprintf(&sb, "%s (%d %s)\n", file, n, word)
		for i, m := range deduped {
			if counts[i] > 1 {
				fmt.Fprintf(&sb, "  %s: %s (x%d)\n", m.lineno, m.content, counts[i])
			} else {
				fmt.Fprintf(&sb, "  %s: %s\n", m.lineno, m.content)
			}
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func capOutput(output string) string {
	const maxLines = 100
	const maxLineLen = 2000

	lines := strings.Split(output, "\n")
	capped := false
	truncated := false

	if len(lines) > maxLines {
		lines = lines[:maxLines]
		capped = true
	}
	for i, line := range lines {
		if len(line) > maxLineLen {
			lines[i] = line[:maxLineLen] + " [line truncated]"
			truncated = true
		}
	}
	result := strings.Join(lines, "\n")
	if capped || truncated {
		result += "\n!! output capped. Use codebase.search for text or file searches"
	}
	return result
}

func reject(reason, guidance string) {
	deny(fmt.Sprintf("Rejected: %s.\n%s", reason, guidance))
}

// cmdTargetsProject returns true if the command is operating on a path inside
// projectDir (or has no explicit path, implying CWD = project). Returns true
// when projectDir is empty so old configs without the field stay blocked.
func cmdTargetsProject(cmd, projectDir string) bool {
	if projectDir == "" {
		return true
	}
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return true
	}
	home, _ := os.UserHomeDir()
	var pathArgs []string
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "-") {
			continue
		}
		if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") ||
			strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") ||
			p == "." || p == ".." || strings.Contains(p, "/") {
			pathArgs = append(pathArgs, p)
		}
	}
	if len(pathArgs) == 0 {
		return true // no explicit path = CWD = project
	}
	realProject := projectDir
	if r, err := filepath.EvalSymlinks(projectDir); err == nil {
		realProject = r
	}
	for _, p := range pathArgs {
		if strings.HasPrefix(p, "~") {
			p = home + p[1:]
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if r, err := filepath.EvalSymlinks(abs); err == nil {
			abs = r
		}
		rel, err := filepath.Rel(realProject, abs)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
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
		result = diffHeaderRe.ReplaceAllString(result, "")
		result = noNewlineRe.ReplaceAllString(result, "")
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
	fmt.Fprintf(os.Stderr, "[lg-hook] deliver: %q\n", content)
	tmpFile := fmt.Sprintf("/tmp/lg-deliver-%d", os.Getpid())
	os.WriteFile(tmpFile, []byte(content), 0644)
	out := hookOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
			UpdatedInput:       map[string]string{"command": "cat " + tmpFile + "; rm " + tmpFile},
		},
	}
	json.NewEncoder(os.Stdout).Encode(out)
	os.Exit(0)
}

func allowTool() {
	out := hookOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: "allow",
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
