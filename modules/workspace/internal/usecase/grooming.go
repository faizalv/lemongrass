package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	ptyclient "github.com/faizalv/lemongrass/modules/pty/client"
	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type ptyProvider interface {
	Open(prompt, sessionID, sessionType string) (ptyclient.Session, error)
}

type groomingSession interface {
	RegisterSession(workspaceID, projectAlias, sessionType string, projectID int64, session ptyclient.Session)
}

type GroomingUsecase struct {
	ws     workspaceStore
	req    requirementStore
	pty    ptyProvider
	lgSess groomingSession
}

func NewGrooming(ws workspaceStore, req requirementStore, pty ptyProvider) *GroomingUsecase {
	return &GroomingUsecase{ws: ws, req: req, pty: pty}
}

func (u *GroomingUsecase) SetLgSession(s groomingSession) {
	u.lgSess = s
}

func (u *GroomingUsecase) StartGrooming(ctx context.Context, workspaceID string) error {
	if u.pty == nil || u.lgSess == nil {
		return fmt.Errorf("grooming not configured")
	}
	ws, err := u.ws.Get(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}
	if ws.Status != "idle" {
		return fmt.Errorf("workspace is %s, must be idle to start grooming", ws.Status)
	}
	count, err := u.req.CountRequirements(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("check requirements: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no requirements added; add at least one before grooming")
	}
	projectPath, err := u.ws.GetProjectPath(ctx, ws.ProjectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}
	requirements, err := u.req.ListRequirements(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load requirements: %w", err)
	}
	requirements = convertRequirements(ctx, requirements)
	systemPrompt := buildGroomingPrompt(requirements, projectPath)
	if err := u.ws.UpdateStatus(ctx, workspaceID, "grooming"); err != nil {
		return err
	}
	session, err := u.pty.Open(systemPrompt, workspaceID, "grooming")
	if err != nil {
		u.ws.UpdateStatus(ctx, workspaceID, "idle")
		return fmt.Errorf("start grooming PTY: %w", err)
	}
	alias := filepath.Base(projectPath)
	u.lgSess.RegisterSession(workspaceID, alias, "grooming", ws.ProjectID, session)
	session.Write([]byte("Begin grooming.\r"))
	return nil
}

func convertRequirements(ctx context.Context, reqs []entity.WorkspaceRequirement) []entity.WorkspaceRequirement {
	out := make([]entity.WorkspaceRequirement, len(reqs))
	copy(out, reqs)
	for i, r := range out {
		if r.Type != "pdf" && r.Type != "image" {
			continue
		}
		md, err := callMarkitdown(ctx, "/home/lg/.lemongrass/workspaces/"+r.WorkspaceID+"/"+r.FilePath)
		if err != nil {
			continue
		}
		out[i] = entity.WorkspaceRequirement{
			ID:          r.ID,
			WorkspaceID: r.WorkspaceID,
			Type:        "text",
			TextContent: md,
			FilePath:    r.FilePath,
			FileName:    r.FileName,
			CreatedAt:   r.CreatedAt,
		}
	}
	return out
}

func callMarkitdown(ctx context.Context, path string) (string, error) {
	body, _ := json.Marshal(map[string]string{"path": path})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://lg-embed:8080/convert", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("convert: status %d", resp.StatusCode)
	}
	var result struct {
		Markdown string `json:"markdown"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Markdown, nil
}

func buildGroomingPrompt(requirements []entity.WorkspaceRequirement, projectPath string) string {
	var reqSB strings.Builder
	for i, r := range requirements {
		if len(requirements) > 1 {
			fmt.Fprintf(&reqSB, "[Requirement %d]\n", i+1)
		}
		switch r.Type {
		case "text":
			reqSB.WriteString(r.TextContent)
		case "pdf":
			reqSB.WriteString("Your requirements are in the file at: /home/lg/.lemongrass/workspaces/" + r.WorkspaceID + "/" + r.FilePath)
		case "image":
			reqSB.WriteString("Your requirements are in the image file at: /home/lg/.lemongrass/workspaces/" + r.WorkspaceID + "/" + r.FilePath)
		}
		if i < len(requirements)-1 {
			reqSB.WriteString("\n\n")
		}
	}

	body := strings.Join([]string{
		"Grooming model inside Lemongrass. Understand requirements, reason about codebase using semantic map, produce task list for execution model. No code generation.",
		"",
		"Requirements:",
		reqSB.String(),
		"",
		"--- Environment ---",
		"",
		"You are inside the lg-runner Docker container. Your working directory /home/lg is the container filesystem, not the project. Navigate the project exclusively through #lg.* commands -- do not use filesystem paths.",
		"",
		"--- Navigation ---",
		"",
		hookCallInstruction,
		"",
		"#lg.recon.tree [path] -- full project coverage at all directory depths. No argument = whole project. Pass a path to filter to that subtree. Shows n/m explored; n stale per directory.",
		cmdReconPeek,
		"#lg.recon.search <query> -- vector search across annotated nodes. Returns coverage context so you can reason about sparse results.",
		cmdReconRead,
		cmdReconRelated,
		"",
		"Navigation flow: tree to see the full project coverage map. Peek the target directory -- returns only direct-child symbols plus subdirectory counts so you know what is inside without listing everything. Drill into subdirectories by peeking them directly. Read method bodies to understand behavior.",
		"",
		"After reading a method or func node, annotate it (non-blocking) -- this fulfills your peek obligation:",
		"  " + cmdAnnotate,
		"  " + annotateHookNote,
		`  Example: modules/user/repo/user.go:GetByID:method:"fetches user by primary key; no tenant check":*entity.User:db.QueryRowx,db.Get`,
		"",
		"The server remembers every symbol you read this session. If #lg.quota.status shows 'read -- annotate it', fire the annotation from memory -- no need to re-read.",
		"Struct, const, var, and imports annotation is optional. It does not count toward quota and burns context -- skip unless directly useful for planning.",
		"",
		"Config nodes (Dockerfiles, CI pipelines, Compose files, Makefiles) appear in peek, are readable, and are annotatable. Annotating them makes them searchable.",
		"",
		"--- Stale nodes ---",
		"",
		"Nodes marked [STALE] in recon.read output have descriptions that predate a code change. Treat the stored description as a hint only -- the code has changed since it was written. Re-read and re-annotate before using the node in planning.",
		"",
		"--- Annotation quota ---",
		"",
		"Peeking a directory creates an obligation on its unexplored method and func nodes.",
		"30% of those methods must be read then annotated before checkpoint is accepted.",
		"30% of funcs follow (funcs only checked after methods threshold is met).",
		"",
		"#lg.quota.status -- shows each peeked directory, progress, and exactly which nodes are still pending.",
		"Call this before #lg.tasks.checkpoint to confirm obligations are met.",
		"",
		"Do not annotate structs, consts, or imports to meet quota -- they score 0.",
		"Read method bodies first. Quota is on methods, not on total annotation count.",
		"",
		"--- Tasks ---",
		"",
		"When you have enough understanding, make a single checkpoint call with every task for every requirement combined:",
		"",
		`  #lg.tasks.checkpoint {"tasks":[{"title":"...","reason":"...","impl":["...",...]},...]}`,
		"",
		"One call. All requirements, all tasks, one array. Never split by requirement, never call checkpoint more than once per round.",
		"",
		"All three fields required per task. reason is 1-3 sentences -- motivation and what this achieves, not a restatement of impl directives.",
		`Example reason: "Job queries return rows across all tenants -- any authenticated user reads another tenant's data. Scopes every query to caller's tenant_id at DB level."`,
		"",
		"impl entry formats:",
		"  symbol at file -- directive        (modify existing)",
		"  new: path/to/file.go -- contents   (create new file)",
		"  delete: path/to/file.go -- reason  (remove file)",
		"",
		"Creative decisions are in scope. If the requirement implies new files, new data, new content -- propose it. Do not defer; invent what is missing.",
		"",
		"When checkpoint returns \"approved\": call #lg!.handover immediately. No acknowledgment, no summary, nothing else.",
		"When checkpoint returns rejections:",
		"  rejected:",
		`  2: "Add TenantID migration" -- include index on tenant_id`,
		"Revise only rejected tasks, carry approved unchanged, resubmit full list.",
		"",
		"--- Progress ---",
		"",
		echoRule,
		"  Exploring modules/auth/",
		"  Running search for handler registration",
		"  Task list ready, calling checkpoint",
		"One echo per meaningful transition. Short present-tense phrases only.",
		"",
		"--- Rules ---",
		"",
		"Shell commands unavailable -- use lg protocol only.",
		"After every tool result, your next action is a #lg.* command -- never prose.",
		"Annotate every node you read -- semantic map shared across all sessions.",
	}, "\n")

	return environmentPreamble + "\n\n" + body
}
