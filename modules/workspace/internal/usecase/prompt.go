package usecase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/faizalv/lemongrass/infra/lgprompt"
	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

func buildExecutionPrompt(projectAlias, handoverContext string) string {
	var handoverBlock string
	if handoverContext != "" {
		handoverBlock = "--- Handover knowledge ---\n" + handoverContext + "\n---\n\n"
	}
	body := strings.Join([]string{
		"Executor model inside Lemongrass. Implement approved task list exactly -- plan is approved, replanning not in scope.",
		"",
		"Project files at /projects/" + projectAlias + " -- use this prefix for all Edit and Write tool calls.",
		"",
		"--- Commands ---",
		"",
		lgprompt.HookCallInstruction,
		"",
		lgprompt.WorkbenchDecisionTree,
		"",
		"#lg.tasks.read -- approved task list with title, reason, impl. Call this first.",
		lgprompt.CmdReconPeek,
		lgprompt.CmdReconRead,
		lgprompt.CmdReconRelated,
		lgprompt.CmdReconSearch,
		lgprompt.CmdKnowledgeSave,
		lgprompt.CmdKnowledgeRead,
		lgprompt.CmdKnowledgeSearch,
		lgprompt.CmdKnowledgeDelete,
		lgprompt.CmdKnowledgeLabels,
		lgprompt.CmdCodebaseSearch,
		"",
		"--- Workbench ---",
		"",
		lgprompt.CmdCodebaseInterim,
		lgprompt.CmdCodebaseQuery,
		"",
		"Use #lg.recon.read for exploration. Native Read is last resort -- only to obtain current file content before Edit.",
		"After any native Read, annotate the symbols you read: " + lgprompt.CmdAnnotate + " (! required -- no blocking annotate exists)",
		"",
		"--- Impl entry types ---",
		"",
		"  symbol at file -- directive",
		"    Read symbol first. Write change. Annotate immediately -- one separate Bash call per symbol:",
		"    " + lgprompt.CmdAnnotate + "  (! required)",
		"    " + lgprompt.AnnotateHookNote,
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
		lgprompt.EchoRule,
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

	return lgprompt.EnvironmentPreamble + "\n\n" + handoverBlock + body
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
