package lgprompt

import "strings"

const EnvironmentPreamble = `You are running inside Lemongrass (lg-runner). Terminal output goes to a log file -- no user reads it. Text outside #lg.* commands (summaries, narration, step recaps) is invisible and burns context. Use #lg.echo for status only.`

const HookCallInstruction = `Every #lg.* and #lg!.* command in this prompt is a direct Bash tool call -- not prose, not a comment. # routes to lg-hook; ! means fire-and-forget. Each command is one Bash tool call -- never combine multiple #lg.* calls on one line.`

const WorkbenchDecisionTree = `When to reach for each tool:
  concept or term you cannot place      → recon.search
  known symbol identity                 → recon.read
  blast radius before touching          → recon.related
  exact identifier or string in files   → codebase.search  (no interim needed)
  load files for block-level analysis   → codebase.interim
  concept within loaded files           → codebase.query

Do not use codebase.query when you already know path:symbol:kind -- recon.read is cheaper, gives the exact boundary, and counts toward commitment.`

const EchoRule = `Call #lg.echo <message> at each major step. No quotes around message:`

const CmdReconSearch  = `#lg.recon.search <query> -- vector search across annotated nodes; returns coverage context`
const CmdReconPeek    = `#lg.recon.peek <dir> -- symbols in files directly inside a directory + subdirectory symbol counts. Non-recursive. Pass a file path to see that file's symbols only.`
const CmdReconRead    = `#lg.recon.read <path:symbol:kind> -- raw source for a symbol; server resolves lines from map (pipe-separate for multiple: a|b|c)`
const CmdReconRelated = `#lg.recon.related <path:symbol:kind> -- callees and callers for an annotated symbol`

const CmdAnnotate      = `#lg!.annotate <path:symbol:kind>:"description":return_type_or_nil:dep1,dep2_or_nil`
const AnnotateHookNote = `nil means field absent.`

const CmdCommitment       = `#lg.commitment <path> -- declare annotation scope; path is dir, file, or . (root requires 70% coverage)`
const CmdCommitmentStatus = `#lg.commitment.status -- shows each commitment, method/func progress, and overall status`

const CmdKnowledgeSave   = `#lg.knowledge.save <key>:<content> [label1,label2,...] -- save or update a project insight; labels optional (comma-separated, no spaces); response includes [similar: ...] when overlapping entries exist -- read them and delete if superseded; [similar labels: ...] when a near-duplicate label exists -- prefer the existing label`
const CmdKnowledgeRead   = `#lg.knowledge.read <key> -- retrieve a saved insight`
const CmdKnowledgeSearch = `#lg.knowledge.search <query>[:<label>] -- vector search across saved knowledge; optional :<label> suffix filters to entries tagged with that label`
const CmdKnowledgeDelete = `#lg.knowledge.delete <key> -- remove a stale or superseded entry`
const CmdKnowledgeLabels = `#lg.knowledge.labels [query] -- no arg: list all labels; with query: vector search for relevant label names; use to orient then follow with knowledge.search <query>:<label>`

const CmdCodebaseInterim = `#lg.codebase.interim <inputs> -- load files or symbols into session workbench; pipe-separate inputs; replaces previous workbench. Selectors: S:path:symbol:kind (symbol body), F:path/to/file (full file), R:glob (all matching files)`
const CmdCodebaseQuery   = `#lg.codebase.query <question> -- vector search within workbench; use for concepts when symbol identity is unknown`
const CmdCodebaseSearch  = `#lg.codebase.search <pattern> -- grep replacement; searches project files directly, no interim needed; returns matching lines with 3 lines of context`

func BuildSkillContent() string {
	return strings.Join([]string{
		HookCallInstruction,
		"",
		"NEVER use Claude's built-in memory system. Use #lg.knowledge.* to persist anything worth keeping.",
		"",
		"You are working in a lemongrass project. A live semantic map covers the codebase -- every function, method, type, and symbol is indexed with embeddings.",
		"",
		"FINDING THINGS",
		"",
		WorkbenchDecisionTree,
		"",
		"  #lg.recon.tree [path]    coverage map at all depths; no arg = whole project",
		"  " + CmdReconPeek,
		"  " + CmdReconSearch,
		"  " + CmdReconRead,
		"  " + CmdReconRelated,
		"  " + CmdCodebaseSearch,
		"",
		"peek displays methods as Receiver.Method (LgUsecase.HandleByProject). recon.read, recon.related, and codebase.interim S: all take the bare name (HandleByProject).",
		"",
		"Search results carry a status marker. unexplored = provisional embedding from signature only. explored = human-written description exists. Annotating unexplored nodes improves the map for every future session.",
		"",
		"WORKBENCH",
		"",
		"  " + CmdCodebaseInterim,
		"  " + CmdCodebaseQuery,
		"",
		"codebase.interim works on raw file content regardless of annotation state -- use it when the codebase is sparsely annotated or when you need block-level context around a symbol.",
		"",
		"KNOWLEDGE",
		"",
		"Persist things that would cost another session time to re-derive: architectural decisions, module boundaries, non-obvious constraints, build procedures. Not task notes. Not things already readable from the code.",
		"",
		"  " + CmdKnowledgeSave,
		"  " + CmdKnowledgeRead,
		"  " + CmdKnowledgeSearch,
		"  " + CmdKnowledgeLabels,
		"  " + CmdKnowledgeDelete,
		"",
		"knowledge.save response includes [similar: key-a, key-b] when overlapping entries exist. Read and consolidate -- do not accumulate duplicates.",
		"",
		"WORKSPACES AND TASKS",
		"",
		"  #lg.workspace.create <name>",
		"  #lg.workspace.use <name>",
		"  #lg.tasks.checkpoint    write down the agreed task list",
		"  #lg.tasks.read          read the current task list",
		"",
		"Checkpoint writes down a conclusion already reached in conversation. Do not call it speculatively.",
		"",
		"MODES",
		"",
		"PTY mode -- you are running inside lg-runner, driven by the grooming pipeline. Call #lg.commitment <path> before annotating a directory. This registers your scope and gates the checkpoint. Read before annotating -- blind annotations do not count toward commitment.",
		"",
		"  " + CmdCommitment,
		"  " + CmdCommitmentStatus,
		"",
		"Headless mode -- you are Claude Code running on the host, using lemongrass as infrastructure. Commitment is not required. Annotate freely when you read something worth recording. Checkpoint and tasks work as records, not gates.",
	}, "\n")
}
