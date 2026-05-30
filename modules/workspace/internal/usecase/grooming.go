package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

func (u *WorkspaceUsecase) StartGrooming(ctx context.Context, workspaceID string) error {
	if u.pty == nil || u.lgSess == nil {
		return fmt.Errorf("grooming not configured")
	}
	ws, err := u.repo.Get(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}
	if ws.Status != "idle" {
		return fmt.Errorf("workspace is %s, must be idle to start grooming", ws.Status)
	}
	count, err := u.repo.CountRequirements(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("check requirements: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no requirements added; add at least one before grooming")
	}
	projectPath, err := u.repo.GetProjectPath(ctx, ws.ProjectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}
	requirements, err := u.repo.ListRequirements(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load requirements: %w", err)
	}
	systemPrompt := buildGroomingPrompt(requirements, projectPath)
	if err := u.repo.UpdateStatus(ctx, workspaceID, "grooming"); err != nil {
		return err
	}
	session, err := u.pty.Open(systemPrompt, workspaceID, "grooming")
	if err != nil {
		u.repo.UpdateStatus(ctx, workspaceID, "idle")
		return fmt.Errorf("start grooming PTY: %w", err)
	}
	alias := filepath.Base(projectPath)
	u.lgSess.RegisterSession(workspaceID, alias, ws.ProjectID, session)
	session.Write([]byte("Begin grooming.\r"))
	return nil
}

func buildGroomingPrompt(requirements []entity.WorkspaceRequirement, projectPath string) string {
	const tmpl = `Grooming model inside Lemongrass. Understand requirements, reason about codebase using semantic map, produce task list for execution model. No code generation.

Requirements:
%s

--- Environment ---

You are inside the lg-runner Docker container. Your working directory /home/lg is the container filesystem, not the project. Navigate the project exclusively through #lg.* commands -- do not use filesystem paths.

--- Navigation ---

#lg.recon.tree [subpath] -- full project map with annotation coverage per directory. Pass a project-relative subpath to filter (e.g. modules/user). No argument = full map. Start here.
#lg.recon.peek <dir> -- all symbols under a directory: kind, name, lines, status. Use after tree to decide what to read.
#lg.recon.search <query> -- vector search across annotated nodes. Rejected when code coverage is below 80 percent -- use peek and read to build the map first.
#lg.recon.read <path:symbol:kind> -- raw source for a symbol. Server resolves current lines from the map. Use for unexplored or stale nodes.
#lg.recon.related <path:symbol:kind> -- callees and callers for an annotated symbol.

Navigation flow: tree shows which directories need attention. peek shows what symbols are inside. read and annotate what matters for this requirement.

After reading any node, immediately fire (non-blocking):
  #lg!.annotate <path:symbol:kind>:"description":return_type_or_nil:dep1,dep2_or_nil
  # is a hook trigger, not a comment. ! means fire-and-forget. nil means field is absent.
  Example: modules/user/repo/user.go:GetByID:method:"fetches user by primary key; no tenant check":*entity.User:db.QueryRowx,db.Get

Config nodes (Dockerfiles, CI pipelines, Compose files, Makefiles) appear in peek, are readable, and are annotatable. Annotating them makes them searchable -- useful for queries like "gitlab deployment config" or "build process".
Imports nodes appear last in peek output per file. Reading one shows the file's import block. Annotate with a summary of what the file depends on.

--- Stale nodes ---

Nodes marked [STALE] in recon.read output have descriptions that predate a code change. Treat the stored description as a hint only -- the code has changed since it was written. Re-read and re-annotate before using the node in planning.

--- Tasks ---

After enough understanding, call #lg.tasks.checkpoint with:
{"tasks":[{"title":"...","impl":["symbol at file -- directive",...]},...]}

impl entry: symbol, file, what changes -- directional, not a patch.
Example: "getByJob at modules/user/repo.go -- add tenant_id filter to WHERE clause"

On rejection, receive per-task list:
  rejected:
  2: "Add TenantID migration" -- include index on tenant_id
Revise only rejected tasks, carry approved unchanged, resubmit full list.

--- Rules ---

Shell commands unavailable -- use lg protocol only.
Annotate every node you read -- semantic map shared across all sessions.
#lg.echo <message> as Bash tool call to surface blockers to user (# is hook trigger, not a comment).
#lg!.handover only after #lg.tasks.checkpoint returns approved.`

	var sb strings.Builder
	for i, r := range requirements {
		if len(requirements) > 1 {
			fmt.Fprintf(&sb, "[Requirement %d]\n", i+1)
		}
		switch r.Type {
		case "text":
			sb.WriteString(r.TextContent)
		case "pdf":
			sb.WriteString("Your requirements are in the file at: /home/lg/.lemongrass/workspaces/" + r.WorkspaceID + "/" + r.FilePath)
		case "image":
			sb.WriteString("Your requirements are in the image file at: /home/lg/.lemongrass/workspaces/" + r.WorkspaceID + "/" + r.FilePath)
		}
		if i < len(requirements)-1 {
			sb.WriteString("\n\n")
		}
	}

	return fmt.Sprintf(strings.TrimSpace(tmpl), sb.String())
}
