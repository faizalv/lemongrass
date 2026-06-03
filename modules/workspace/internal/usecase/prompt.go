package usecase

import "strings"

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
		"    Create file. Annotate every symbol added.",
		"",
		"  delete: path/to/file.go -- reason",
		"    Call #lg!.recon.drop <path>, then delete file.",
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
