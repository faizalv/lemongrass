package lgprompt

import "strings"

const EnvironmentPreamble = `You are running inside Lemongrass (lg-runner). Terminal output goes to a log file -- no user reads it. Text outside #lg.* commands (summaries, narration, step recaps) is invisible and burns context. Use #lg.echo for status only.`

const HookCallInstruction = `Every #lg.* and #lg!.* is a direct Bash tool call -- not prose. # routes to lg-hook; ! means fire-and-forget. One command per Bash call -- do not pipe two #lg.* into a single shell invocation.

Blocking calls (#lg.*) must be sequential -- each waits for a response. Fire-and-forget calls (#lg!.*) return immediately, so multiple can be issued in parallel as separate Bash tool calls.

After reading symbols, annotate: #lg.annotate path:symbol:kind:"description":return_type_or_nil:dep1,dep2_or_nil
Batch multiple with ||: #lg.annotate ref1||ref2||ref3 -- grouped response: [1][2][3] ok  [4][5] error: symbol not read
After context compaction, peruse state resets -- re-peruse or re-query before annotating.`

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

const CmdReconSearch = `#lg.recon.search <query> -- vector search across the full map; all nodes have signature embeddings, not just explored ones`
const CmdReconPeek = `#lg.recon.peek <dir> -- symbols in a directory (non-recursive); pass file path for that file's symbols`
const CmdReconPeruse = `#lg.recon.peruse <path:symbol:kind> -- symbol body from semantic map; counts toward annotation gate; | within any field expands that field: path:sym1|sym2:kind reads both syms from same path; || separates independent full refs: path1:sym1:kind1||path2:sym2:kind2`
const CmdReconRelated = `#lg.recon.related <path:symbol:kind> -- callees and callers for an annotated symbol`

const CmdAnnotate = `#lg.annotate <path:symbol:kind>:"description":return_type_or_nil:dep1,dep2_or_nil [|| path2:...]`
const AnnotateHookNote = `nil means field absent. Gate passes if the symbol was perused or its lines appeared in a codebase.query result this session. || separates multiple entries; returns indexed results.`

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
const CmdCodebaseFiles = `#lg.codebase.fl <pattern> [path] -- all files under project root matching a glob or substring; last token after a space is always a path scope; use \/ in the last token to escape and treat everything as the pattern; grouped by directory; prefix* lines are filename prefix groups -- each indented entry is a suffix, full filename = prefix + suffix`
const CmdCodebaseInterim = `#lg.codebase.interim <inputs> -- load files/symbols into session workbench; pipe-separate: S:path:symbol:kind | F:path | R:glob`
const CmdCodebaseQuery = `#lg.codebase.query <question> -- semantic search across everything loaded into the workbench`
const CmdCodebaseSearch = `#lg.codebase.search <pattern> [path/prefix] [--force] -- grep replacement; last token with / is a path scope filter; supports regex; use | for alternation (not \|); no quotes around pattern; blacklisted dirs (node_modules, vendor, dist, .git, .next, __pycache__) require --force`

func BuildSkillContent() string {
	return strings.Join([]string{
		"Lemongrass is law. PASM: live symbol map, grows with every annotation, shared across all sessions and models. Leave it better than you found it.",
		"",
		"ANNOTATION OBLIGATION -- automatic, enforced, no exceptions:",
		"  Peruse/Read unexplored or stale symbol → obligation. Write file → all read symbols → obligation.",
		"  5 min to annotate. Past halfway: warnings. Past deadline: all #lg.* block except annotate + obligation.",
		"  #lg.obligation: debt + time. Annotate = removes entry. Empty = clock resets.",
		"",
		"Commands = Bash tool calls. # is literal -- `#lg.foo` not `lg.foo`. One per invocation.",
		"",
		"PARALLELISM -- reads: parallel Bash calls ok. writes: sequential:",
		"  parallel      recon.search, recon.peek, recon.tree, recon.related, codebase.search, codebase.query, codebase.fl, codebase.ls, knowledge.read, knowledge.search, knowledge.labels, project.stat, tasks.read, system.read",
		"  sequential    annotate, knowledge.save, knowledge.delete, codebase.interim, tasks.start, tasks.finish, workspace.*, recon.peruse",
		"  #lg!.*        always parallel",
		"  annotate: || batching not parallel -- #lg.annotate ref1||ref2||ref3",
		"",
		"#lg.* commands and args: English only.",
		"",
		"FILE READING:",
		"  Read tool      source/config to Edit; range required if oversized; Edit needs prior Read",
		"  system.read    docs/markdown only; not for files you will Edit",
		"",
		"OUTSIDE PROJECT -- codebase.* anchored to root; outside paths fail. Use native Bash.",
		"",
		"TOOL SELECTION:",
		"  unknown area       recon.search",
		"  know what to load  codebase.interim + query",
		"  know the symbol    codebase.search",
		"  unfamiliar area    recon.search → peek → peruse → Read",
		"",
		"After modify: #lg.annotate <path:sym:kind>:\"description\":return_type_or_nil:deps [|| ref2 || ...]",
		"",
		"TOOLS",
		"",
		"  #lg.obligation                                    annotation debt; blocks after 5 min",
		"  #lg.project.stat                                  coverage + device tier + advice",
		"  #lg.recon.tree [path]                             coverage map",
		"  #lg.recon.peek <dir|file>                         symbols in dir/file; methods as Receiver.Method",
		"  #lg.recon.search <query>                          vector search across all nodes; signatures indexed from day 0",
		"  #lg.recon.peruse <path:symbol:kind>               symbol body; | expands any field, || is new ref: path1|path2:sym:kind||path3:sym2:kind1|kind2",
		"  #lg.recon.related <path:symbol:kind>              callees and callers",
		"  #lg.codebase.ls [path]                            directory listing with sizes",
		"  " + CmdCodebaseFiles,
		"  " + CmdCodebaseSearch,
		"  #lg.codebase.interim <inputs>                     load into workbench: S:path:sym:kind | F:path | R:glob",
		"  #lg.codebase.query <question>                     query workbench",
		"  #lg.system.read <path>                            docs/markdown; warns if large",
		"  #lg.system.read.confirm <path> [N-M]              deliver unconditionally; N-M range",
		"",
		"ANNOTATION",
		"",
		"  " + CmdAnnotate,
		"  nil = field absent.",
		"",
		"Annotate every symbol perused or appeared in codebase.query result. Re-annotate on modify.",
		"",
		"KNOWLEDGE -- lg.knowledge.*: codebase insights, shared across all models. Built-in memory: user/session prefs, pre-loaded free. Use both.",
		"",
		"  " + CmdKnowledgeSave,
		"  " + CmdKnowledgeRead,
		"  " + CmdKnowledgeSearch,
		"  " + CmdKnowledgeLabels,
		"  " + CmdKnowledgeDelete,
		"",
		"WORKSPACES & TASKS",
		"",
		"  #lg.workspace.create <name>",
		"  #lg.workspace.use <name>",
		"  #lg.workspace.list / .search <query> / .delete <name>",
		"  #lg.workspace.requirement.add <text>",
		"  #lg.tasks.checkpoint <json>    call empty for format",
		"  #lg.tasks.read",
	}, "\n")
}
