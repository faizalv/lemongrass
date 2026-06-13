package lgprompt

import "strings"

const EnvironmentPreamble = `You are running inside Lemongrass (lg-runner). Terminal output goes to a log file -- no user reads it. Text outside #lg.* commands (summaries, narration, step recaps) is invisible and burns context. Use #lg.echo for status only.`

const HookCallInstruction = `Every #lg.* and #lg!.* is a direct Bash tool call -- not prose. # routes to lg-hook; ! means fire-and-forget. One command per Bash call -- do not pipe two #lg.* into a single shell invocation.

Blocking calls (#lg.*) must be sequential -- each waits for a response. Fire-and-forget calls (#lg!.*) return immediately, so multiple can be issued in parallel as separate Bash tool calls.

After #lg.recon.peruse, annotate: #lg!.annotate path:symbol:kind:"description":return_type_or_nil:dep1,dep2_or_nil
Annotation is gated -- rejected if you have not perused the symbol this session.
After context compaction, peruse state resets -- re-peruse before annotating.`

const WorkbenchDecisionTree = `When to reach for each tool:
  concept or term you cannot place      → recon.search
  known symbol identity                 → recon.peruse
  blast radius before touching          → recon.related
  exact identifier or string in files   → codebase.search
  find files by name or glob            → codebase.fl
  browse project tree with sizes        → codebase.ls
  understand an area across many files  → codebase.interim + codebase.query
  raw file access                       → system.read (inspect first, confirm if large)`

const EchoRule = `Call #lg.echo <message> at each major step. No quotes around message:`

const CmdReconSearch = `#lg.recon.search <query> -- vector search across annotated nodes`
const CmdReconPeek = `#lg.recon.peek <dir> -- symbols in a directory (non-recursive); pass file path for that file's symbols`
const CmdReconPeruse = `#lg.recon.peruse <path:symbol:kind> -- symbol body from semantic map; counts toward annotation gate (pipe-separate for multiple: a|b|c)`
const CmdReconRelated = `#lg.recon.related <path:symbol:kind> -- callees and callers for an annotated symbol`

const CmdAnnotate = `#lg!.annotate <path:symbol:kind>:"description":return_type_or_nil:dep1,dep2_or_nil`
const AnnotateHookNote = `nil means field absent. Must have called recon.peruse on the same path:symbol:kind first.`

const CmdSystemRead = `#lg.system.read <path> -- inspect file; delivers content if <=150 lines and <=10k chars, otherwise warns and asks for a range`
const CmdSystemReadConfirm = `#lg.system.read.confirm <path> [N-M] -- deliver file content unconditionally; N-M is optional 1-indexed line range`

const CmdCommitment = `#lg.commitment <path> -- declare annotation scope; path is dir, file, or . (root requires 70% coverage)`
const CmdCommitmentStatus = `#lg.commitment.status -- shows each commitment, method/func progress, and overall status`

const CmdKnowledgeSave = `#lg.knowledge.save <key>:<content> [label1,label2,...] -- save or update a project insight`
const CmdKnowledgeRead = `#lg.knowledge.read <key> -- retrieve a saved insight`
const CmdKnowledgeSearch = `#lg.knowledge.search <query>[:<label>] -- vector search across saved knowledge`
const CmdKnowledgeDelete = `#lg.knowledge.delete <key> -- remove a stale or superseded entry`
const CmdKnowledgeLabels = `#lg.knowledge.labels [query] -- list all labels or vector search for relevant ones`

const CmdTasksStart = `#lg.tasks.start <n> -- mark task n in_progress (n is the integer task_id from tasks.read); check response for pending rejection notes`
const CmdTasksFinish = `#lg.tasks.finish <n>:<notes> -- mark task n done and record per-task diff; notes explain any divergence from planned impl (empty if none); check response for pending rejection notes`

const CmdWorkspaceRequirementAdd = `#lg.workspace.requirement.add <text> -- add a text requirement to the active workspace`
const CmdWorkspaceList = `#lg.workspace.list -- list all workspaces with name, created date, and status`
const CmdWorkspaceSearch = `#lg.workspace.search <query> -- filter workspaces by name`
const CmdWorkspaceDelete = `#lg.workspace.delete <name> -- delete a workspace (must be idle)`

const CmdCodebaseLs = `#lg.codebase.ls [path] -- directory listing from project root; shows child counts for dirs and sizes for files`
const CmdCodebaseFiles = `#lg.codebase.fl <pattern> -- all files under project root matching a glob or substring; grouped by directory`
const CmdCodebaseInterim = `#lg.codebase.interim <inputs> -- load files/symbols into session workbench; pipe-separate: S:path:symbol:kind | F:path | R:glob`
const CmdCodebaseQuery = `#lg.codebase.query <question> -- semantic search across everything loaded into the workbench`
const CmdCodebaseSearch = `#lg.codebase.search <pattern> [path/prefix] [--force] -- grep replacement; last token with / is a path scope filter; supports regex; use | for alternation (not \|); no quotes around pattern; blacklisted dirs (node_modules, vendor, dist, .git, .next, __pycache__) require --force`

func BuildSkillContent() string {
	return strings.Join([]string{
		"Lemongrass maintains a live semantic map of every symbol in this codebase. Every time you peruse/Read a symbol and annotate it, the map GETS MORE USEFUL, for you later in this session and for every model that works on this project after you. This is PASM: Progressive Annotated Semantic Map.",
		"",
		"The hook is watching. Call #lg.skill.loaded to acknowledge skill reload and clear the block (only after 3 real #lg.* calls. Gaming it prolongs the block.)",
		"",
		"Before starting any task, run #lg.knowledge.labels to surface existing project knowledge. The user has agreed for Lemongrass to enforced the rule here, FOLLOW IT!",
		"",
		HookCallInstruction,
		"",
		"NEVER use built-in memory system. Use #lg.knowledge.* to persist anything worth keeping.",
		"",
		"ANNOTATION -- leave the map better than you found it",
		"",
		"  " + CmdAnnotate,
		"  " + AnnotateHookNote,
		"",
		"Annotate every symbol you peruse or Read, without being asked. Re-annotate every symbol you modify. If you hit a stale node, re-read it and update the annotation before moving on. An unexplored codebase forces the next model to rediscover everything from scratch. An annotated one lets it navigate by meaning alone.",
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
		"  " + CmdCodebaseLs,
		"  " + CmdCodebaseFiles,
		"  " + CmdCodebaseSearch,
		"",
		"peek displays methods as Receiver.Method. recon.peruse, recon.related, codebase.interim S: all take the bare name.",
		"",
		"FILE ACCESS",
		"",
		"  " + CmdSystemRead,
		"  " + CmdSystemReadConfirm,
		"",
		"Use system.read for reading documents, recon.peruse for symbol-level access, native Read for images and files in general.",
		"",
		"WORKBENCH",
		"",
		"The workbench is useful when you want to quickly understand the codebase. Load any combination of files, symbols, or globs -- then query across all of them at once with natural language. This is the primary tool for understanding an unfamiliar area, tracing data flow across layers, or answering 'how does X work' without reading files linearly.",
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
		"  #lg.workspace.create <name>         create a workspace (one per PRD)",
		"  #lg.workspace.use <name>            switch to an existing workspace",
		"  " + CmdWorkspaceList,
		"  " + CmdWorkspaceSearch,
		"  " + CmdWorkspaceDelete,
		"  " + CmdWorkspaceRequirementAdd,
		"  #lg.tasks.checkpoint <json>         save tasks. see format on empty call",
		"  #lg.tasks.read                      read the current task list",
		"  #lg.skill.loaded                    acknowledge skill reload after compaction",
	}, "\n")
}
