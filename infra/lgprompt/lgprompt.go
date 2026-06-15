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

const CmdReconSearch = `#lg.recon.search <query> -- vector search across annotated nodes`
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
		"Lemongrass is law. PASM (Progressive Annotated Semantic Map) is the ideology -- a live symbol map that grows with every annotation, persisting across all sessions and every model that follows. Leave it better than you found it.",
		"",
		"Commands are Bash tool calls -- type them exactly as written, # included. `#lg.knowledge.labels` not `lg knowledge.labels`. One call per Bash invocation. Blocking (#lg.*) sequential. Fire-and-forget (#lg!.*) can run in parallel.",
		"",
		"Always use English in all #lg.* commands, arguments, and knowledge entries -- regardless of what language the user is writing in.",
		"",
		"FILE READING -- two tools, different roles:",
		"  Native Read tool   source code, config, any file you will Edit afterward. Hook gates it: oversized files without a range are rejected, so it is safe. Edit requires a prior Read; using system.read first and then Read wastes context.",
		"  #lg.system.read    docs, markdown, README only -- when you want inspect behaviour (warns if large) and will not Edit the file.",
		"",
		"OUTSIDE THE PROJECT -- #lg.codebase.* commands are anchored to the registered project root; paths outside it get prefixed and will fail. Use native Bash tools (ls, find, grep) for anything outside the project: skill files, home directory, system paths.",
		"",
		"ON SKILL LOAD -- orient yourself before responding. You may be entering a project mid-flight with no prior context. Learn where you are:",
		"  Need knowledge? -> #lg.knowledge.labels, follow up on anything relevant",
		"  Where things? -> #lg.recon.peek on areas that seem relevant, or browse with #lg.codebase.ls",
		"  Form a picture of the project before you act.",
		"",
		"USE CASES",
		"",
		"Entering an unfamiliar area:",
		"  recon.peek -> recon.search -> recon.peruse -> Read if needed",
		"  recon.peek codebase.search | recon.search -> codebase.interim -> codebase.query. PREFERABLE, MOST TIME and TOKEN EFFECTIVE",
		"",
		"After any modification:",
		" #lg.annotate <path:sym:kind>:\"description\":return_type_or_nil:deps [|| ref2 || ...]",
		"",
		"TOOLS",
		"",
		"  #lg.recon.tree [path]                             coverage map",
		"  #lg.recon.peek <dir|file>                         symbols in a dir or file; methods shown as Receiver.Method",
		"  #lg.recon.search <query>                          vector search across annotated nodes",
		"  #lg.recon.peruse <path:symbol:kind>               symbol body; pipe-separate: a|b|c; takes bare symbol name",
		"  #lg.recon.related <path:symbol:kind>              callees and callers",
		"  #lg.codebase.ls [path]                            directory listing with sizes",
		"  " + CmdCodebaseFiles,
		"  " + CmdCodebaseSearch,
		"  #lg.codebase.interim <inputs>                     build queryable codebase into workbench: S:path:sym:kind | F:path | R:glob",
		"  #lg.codebase.query <question>                     query workbench; load once, query many times",
		"  #lg.system.read <path>                            docs/markdown only; warns if >150 lines or >10k chars",
		"  #lg.system.read.confirm <path> [N-M]              docs/markdown only; deliver unconditionally; N-M line range",
		"",
		"ANNOTATION",
		"",
		"  " + CmdAnnotate,
		"  nil = field absent.",
		"",
		"Annotate every symbol you peruse, read, or that appeared in a codebase.query result. Re-annotate every symbol you modify or created without re-reading it.",
		"",
		"KNOWLEDGE -- never use built-in memory; persist with #lg.knowledge.*",
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
