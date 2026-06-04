package usecase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

const environmentPreamble = `You are running inside Lemongrass (lg-runner). Terminal output goes to a log file -- no user reads it. Text outside #lg.* commands (summaries, narration, step recaps) is invisible and burns context. Use #lg.echo for status only.`

const hookCallInstruction = `Every #lg.* and #lg!.* command in this prompt is a direct Bash tool call -- not prose, not a comment. # routes to lg-hook; ! means fire-and-forget. Each command is one Bash tool call -- never combine multiple #lg.* calls on one line.`

const cmdReconSearch  = `#lg.recon.search <query> -- vector search across annotated nodes; returns coverage context`
const cmdReconPeek    = `#lg.recon.peek <dir> -- all symbols under a directory: kind, name, lines, status`
const cmdReconRead    = `#lg.recon.read <path:symbol:kind> -- raw source for a symbol; server resolves lines from map`
const cmdReconRelated = `#lg.recon.related <path:symbol:kind> -- callees and callers for an annotated symbol`
const cmdAnnotate     = `#lg!.annotate <path:symbol:kind>:"description":return_type_or_nil:dep1,dep2_or_nil`
const annotateHookNote = `nil means field absent.`
const echoRule        = `Call #lg.echo <message> at each major step. No quotes around message:`

func buildExecutionPrompt(projectAlias string) string {
	body := strings.Join([]string{
		"Executor model inside Lemongrass. Implement approved task list exactly -- plan is approved, replanning not in scope.",
		"",
		"Project files at /projects/" + projectAlias + " -- use this prefix for all Edit and Write tool calls.",
		"",
		"--- Commands ---",
		"",
		hookCallInstruction,
		"",
		"#lg.tasks.read -- approved task list with title, reason, impl. Call this first.",
		cmdReconPeek,
		cmdReconRead,
		cmdReconRelated,
		cmdReconSearch,
		"",
		"Use #lg.recon.read for exploration. Native Read is last resort -- only to obtain current file content before Edit.",
		"After any native Read, annotate the symbols you read: " + cmdAnnotate + " (! required -- no blocking annotate exists)",
		"",
		"--- Impl entry types ---",
		"",
		"  symbol at file -- directive",
		"    Read symbol first. Write change. Annotate immediately -- one separate Bash call per symbol:",
		"    " + cmdAnnotate + "  (! required)",
		"    " + annotateHookNote,
		"",
		"  new: path/to/file.go -- contents",
		"    Create file. Immediately annotate every exported symbol (non-blocking, one call per symbol).",
		"",
		"  delete: path/to/file.go -- reason",
		"    1. For each exported symbol: #lg.recon.read, then check #lg.recon.related for callers.",
		"    2. Edit each caller to remove the import and call sites. Re-annotate.",
		"    3. #lg!.recon.drop <path> -- deletes the file AND purges stale map entries.",
		"    Do not use rm. #lg!.recon.drop handles physical deletion.",
		"",
		"Annotate every symbol written or modified -- no exceptions.",
		"",
		"--- Progress ---",
		"",
		echoRule,
		"  Reading task list",
		"  Implementing task 1 -- adding tenant_id filter",
		"  Creating tenant_middleware.go",
		"  All tasks complete",
		"",
		"--- Rules ---",
		"",
		"Implement exactly what tasks describe -- no scope expansion.",
		"#lg!.done only when all tasks are complete.",
	}, "\n")

	return environmentPreamble + "\n\n" + body
}

func buildAmendmentPrompt(requirements []entity.WorkspaceRequirement, approved, rejected []entity.Task, projectPath string) string {
	var reqSB strings.Builder
	for i, r := range requirements {
		if len(requirements) > 1 {
			fmt.Fprintf(&reqSB, "[Requirement %d]\n", i+1)
		}
		switch r.Type {
		case "text":
			reqSB.WriteString(r.TextContent)
		case "pdf":
			reqSB.WriteString("Requirements file at: /home/lg/.lemongrass/workspaces/" + r.WorkspaceID + "/" + r.FilePath)
		case "image":
			reqSB.WriteString("Requirements image at: /home/lg/.lemongrass/workspaces/" + r.WorkspaceID + "/" + r.FilePath)
		}
		if i < len(requirements)-1 {
			reqSB.WriteString("\n\n")
		}
	}

	var amendSB strings.Builder
	if len(approved) > 0 {
		amendSB.WriteString("These tasks were approved -- carry them forward unchanged:\n\n")
		for i, t := range approved {
			impl := strings.Join(implStrings(t.Impl), "\n    ")
			fmt.Fprintf(&amendSB, "  %d. %s\n     impl: %s\n\n", i+1, t.Title, impl)
		}
	}
	if len(rejected) > 0 {
		amendSB.WriteString("These tasks were rejected -- revise based on the feedback:\n\n")
		for i, t := range rejected {
			impl := strings.Join(implStrings(t.Impl), "\n    ")
			fmt.Fprintf(&amendSB, "  %d. %s\n     feedback: %q\n     current impl: %s\n\n", i+1, t.Title, t.AmendmentFeedback, impl)
		}
	}

	base := buildGroomingPrompt(requirements, projectPath)
	amendment := strings.Join([]string{
		"",
		"--- Amendment context ---",
		"",
		"You are resuming a grooming session to revise rejected tasks.",
		"Use full recon access to look up anything referenced in the feedback before revising.",
		"",
		strings.TrimRight(amendSB.String(), "\n"),
		"",
		"When checkpointing: submit ALL tasks -- approved unchanged + revised rejected -- as one array.",
		"Never drop an approved task. Never resubmit a rejected task without addressing the feedback.",
	}, "\n")

	return base + amendment
}

func implStrings(raw []byte) []string {
	var items []string
	if err := jsonUnmarshal(raw, &items); err != nil || len(items) == 0 {
		return []string{"(no impl)"}
	}
	return items
}
