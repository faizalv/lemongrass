package lgprompt

import "strings"

const EnvironmentPreamble = `You are running inside Lemongrass (lg-runner). Terminal output goes to a log file -- no user reads it. Text outside #lg.* commands (summaries, narration, step recaps) is invisible and burns context. Use #lg.echo for status only.`

const HookCallInstruction = `Every #lg.* and #lg!.* is a direct Bash tool call -- not prose. # routes to lg-hook; ! means fire-and-forget. One command per Bash call.

After #lg.recon.peruse, annotate: #lg!.annotate path:symbol:kind:"description":return_type_or_nil:dep1,dep2_or_nil
Annotation is gated -- rejected if you have not perused the symbol this session.
After context compaction, peruse state resets -- re-peruse before annotating.`

const WorkbenchDecisionTree = `When to reach for each tool:
  concept or term you cannot place      → recon.search
  known symbol identity                 → recon.peruse
  blast radius before touching          → recon.related
  exact identifier or string in files   → codebase.search
  understand an area across many files  → codebase.interim + codebase.query
  raw file access                       → system.read (inspect first, confirm if large)`

const EchoRule = `Call #lg.echo <message> at each major step. No quotes around message:`

const CmdReconSearch  = `#lg.recon.search <query> -- vector search across annotated nodes`
const CmdReconPeek    = `#lg.recon.peek <dir> -- symbols in a directory (non-recursive); pass file path for that file's symbols`
const CmdReconPeruse  = `#lg.recon.peruse <path:symbol:kind> -- symbol body from semantic map; counts toward annotation gate (pipe-separate for multiple: a|b|c)`
const CmdReconRelated = `#lg.recon.related <path:symbol:kind> -- callees and callers for an annotated symbol`

const CmdAnnotate      = `#lg!.annotate <path:symbol:kind>:"description":return_type_or_nil:dep1,dep2_or_nil`
const AnnotateHookNote = `nil means field absent. Must have called recon.peruse on the same path:symbol:kind first.`

const CmdSystemRead        = `#lg.system.read <path> -- inspect file; delivers content if <=150 lines and <=10k chars, otherwise warns and asks for a range`
const CmdSystemReadConfirm = `#lg.system.read.confirm <path> [N-M] -- deliver file content unconditionally; N-M is optional 1-indexed line range`

const CmdCommitment       = `#lg.commitment <path> -- declare annotation scope; path is dir, file, or . (root requires 70% coverage)`
const CmdCommitmentStatus = `#lg.commitment.status -- shows each commitment, method/func progress, and overall status`

const CmdKnowledgeSave   = `#lg.knowledge.save <key>:<content> [label1,label2,...] -- save or update a project insight`
const CmdKnowledgeRead   = `#lg.knowledge.read <key> -- retrieve a saved insight`
const CmdKnowledgeSearch = `#lg.knowledge.search <query>[:<label>] -- vector search across saved knowledge`
const CmdKnowledgeDelete = `#lg.knowledge.delete <key> -- remove a stale or superseded entry`
const CmdKnowledgeLabels = `#lg.knowledge.labels [query] -- list all labels or vector search for relevant ones`

const CmdCodebaseInterim = `#lg.codebase.interim <inputs> -- load files/symbols into session workbench; pipe-separate: S:path:symbol:kind | F:path | R:glob`
const CmdCodebaseQuery   = `#lg.codebase.query <question> -- semantic search across everything loaded into the workbench`
const CmdCodebaseSearch  = `#lg.codebase.search <pattern> -- grep replacement; searches project files, returns matching lines with 2 lines of context; supports regex`

func BuildSkillContent() string {
	return strings.Join([]string{
		"Lemongrass enriches your work on this project. It maintains a live semantic map of every symbol in the codebase -- use it to navigate and annotate rather than reading raw files. The native Read tool is blocked on project files; use #lg.system.read for raw access or #lg.recon.peruse for symbol-level access that counts toward annotation.",
		"",
		"Before starting any task, run #lg.knowledge.labels to surface existing project knowledge.",
		"",
		HookCallInstruction,
		"",
		"NEVER use Claude's built-in memory system. Use #lg.knowledge.* to persist anything worth keeping.",
		"",
		"FINDING THINGS",
		"",
		WorkbenchDecisionTree,
		"",
		"  #lg.recon.tree [path]    coverage map; no arg = whole project",
		"  " + CmdReconPeek,
		"  " + CmdReconSearch,
		"  " + CmdReconPeruse,
		"  " + CmdReconRelated,
		"  " + CmdCodebaseSearch,
		"",
		"peek displays methods as Receiver.Method. recon.peruse, recon.related, codebase.interim S: all take the bare name.",
		"",
		"FILE ACCESS",
		"",
		"  " + CmdSystemRead,
		"  " + CmdSystemReadConfirm,
		"",
		"Native Read tool is not available. Always use system.read (or recon.peruse for symbol-level access).",
		"",
		"WORKBENCH",
		"",
		"The workbench is a semantic index you build on demand. Load any combination of files, symbols, or globs -- then query across all of them at once with natural language. This is the primary tool for understanding an unfamiliar area, tracing data flow across layers, or answering 'how does X work' without reading files linearly.",
		"",
		"  " + CmdCodebaseInterim,
		"  " + CmdCodebaseQuery,
		"",
		"Example: to understand how a request flows from handler to DB --",
		"  #lg.codebase.interim S:modules/lg/internal/handler/http/lg.go:LgHandler:struct | S:modules/lg/internal/usecase/lg.go:LgUsecase:struct | R:modules/lg/internal/repository/*.go",
		"  #lg.codebase.query how does a hook call reach the database?",
		"",
		"Load once, query many times. Each new interim call replaces the previous workbench.",
		"",
		"ANNOTATION",
		"",
		"  " + CmdAnnotate,
		"  " + AnnotateHookNote,
		"",
		"Annotating unexplored nodes improves the semantic map for every future session. Always peruse then annotate.",
		"",
		"KNOWLEDGE",
		"",
		"  " + CmdKnowledgeSave,
		"  " + CmdKnowledgeRead,
		"  " + CmdKnowledgeSearch,
		"  " + CmdKnowledgeLabels,
		"  " + CmdKnowledgeDelete,
		"",
		"WORKSPACES AND TASKS",
		"",
		"  #lg.workspace.create <name>",
		"  #lg.workspace.use <name>",
		"  #lg.tasks.checkpoint    write down the agreed task list",
		"  #lg.tasks.read          read the current task list",
		"",
		"Commitment not required in this mode. Peruse and annotate freely. Checkpoint and tasks work as records.",
	}, "\n")
}
