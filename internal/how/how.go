// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package how serves the step-by-step instructions behind `lgrass how <domain> <action>`, printing only the branch that applies.
package how

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed content
var content embed.FS

type action struct {
	summary string
	run     func(root string, args []string) (string, error)
	// runEnv replaces run for an action that reads beyond the project's files.
	runEnv func(env Env, args []string) (string, error)
}

// Env is what an action may read: the project root, and sources outside the project's files. A nil source is
// unavailable, and the facts that depend on it fall back to what the files say.
type Env struct {
	Root string
	// TaskStarted reports when biblio/scratchpad/<slug> first appeared in the biblio history.
	TaskStarted func(slug string) (time.Time, bool)
	// EditsSince lists the file edits agent sessions logged after since, in this project's store and the running tab's
	// launch project's store, each store's oldest first.
	EditsSince func(since time.Time) []Edit
	// ProjectOf names the registered project other than Root that holds an absolute path, with the path relative to
	// it, so the footprint can list edits a task made in another project. Nil or false leaves such an edit out.
	ProjectOf func(path string) (name, rel string, ok bool)
	// LastClose reports when this tab was last printed the close steps with this signature for slug, within its
	// current context: the record clears at every session start, compaction included.
	LastClose func(slug, signature string) (time.Time, bool)
	// RecordClose records that this tab was printed the close steps with this signature for slug.
	RecordClose func(slug, signature string)
}

// Edit is one file edit an agent session made, with the path as the agent's tool was given it.
type Edit struct {
	Session string
	Path    string
}

var domains = map[string]map[string]action{
	"ask": {
		"request": {summary: "Ask the human for a db or rester channel or a command run: `lgrass how ask request`", run: static("ask/request.md")},
	},
	"berries": {
		"fix":   {summary: "Fix a stain and confirm the fix with a replay: `lgrass how berries fix`", run: static("berries/fix.md")},
		"map":   {summary: "Write or extend the project's access map: `lgrass how berries map`", run: static("berries/map.md")},
		"run":   {summary: "Record seeds and run berries on a channel: `lgrass how berries run`", run: static("berries/run.md")},
		"stain": {summary: "Triage a stain berries found: `lgrass how berries stain`", run: static("berries/stain.md")},
	},
	"collaborator": {
		"join": {summary: "What to do after the human adds your tab to a workgroup: `lgrass how collaborator join`", run: static("collaborator/join.md")},
	},
	"leader": {
		"addmember": {summary: "Add thinkers to your live workgroup: `lgrass how leader addmember`", run: static("leader/addmember.md")},
		"create":    {summary: "Declare and lead a workgroup: `lgrass how leader create`", run: static("leader/create.md")},
	},
	"git": {
		"resolve": {summary: "Resolve a conflict the human handed you: `lgrass how git resolve`", run: static("git/resolve.md")},
	},
	"keeper": {
		"sweep": {summary: "Keep biblio/ and the code in agreement: `lgrass how keeper sweep`", run: keeperSweep},
	},
	"task": {
		"close":    {summary: "Close a finished scratchpad task: `lgrass how task close <slug>`", runEnv: taskClose},
		"handover": {summary: "Pause or hand over a task: `lgrass how task handover <slug>`", run: taskHandover},
	},
	// write breaks the <domain> <action> shape on purpose: a model about to write knows the subject, so the subject is
	// the second argument, and `lgrass how write` alone lists every format. Each format has its one home here, and the
	// procedures that need it print the same fragment.
	"write": {
		"activities": {summary: "The dated log beside every task: `lgrass how write activities`", run: titled("Writing activities.md", "write/activities.md")},
		"chapter":    {summary: "Where a book chapter goes and how to write it: `lgrass how write chapter`", run: static("write/chapter.md", "write/chapter-rules.md")},
		"debug":      {summary: "The file of a fault being chased: `lgrass how write debug`", run: titled("Writing debug.md", "write/debug.md")},
		"handover":   {summary: "A handover file and its whiteboard row: `lgrass how write handover`", run: titled("Writing a handover", "write/handover.md")},
		"law":        {summary: "A law the human approved, and its summary line: `lgrass how write law`", run: static("write/law.md")},
		"prd":        {summary: "The plan of something being built: `lgrass how write prd`", run: titled("Writing prd.md", "write/prd-path.md", "write/prd.md")},
		"recon":      {summary: "The synthesis of an investigation: `lgrass how write recon`", run: titled("Writing recon.md", "write/recon.md")},
	},
	"work": {
		"debug":      {summary: "Chase a fault to its root cause: `lgrass how work debug <slug>`", run: workDebug},
		"discussion": {summary: "An idea, a question or a design talk with no task file yet: `lgrass how work discussion`", run: workDiscussion},
		"hotfix":     {summary: "A change needed urgently or limited to a named element: `lgrass how work hotfix <slug>`", run: workHotfix},
		"research":   {summary: "Investigate with no build: `lgrass how work research <slug>`", run: workResearch},
		"task":       {summary: "Build something, new or resumed: `lgrass how work task <slug>`", run: workTask},
		"wrapping":   {summary: "Wrap up a session so a new one continues from biblio/ alone: `lgrass how work wrapping <slug>`", run: workWrapping},
	},
	"thinker": {
		"start": {summary: "Your role as a thinker in a workgroup: `lgrass how thinker start`", run: static("thinker/start.md")},
	},
	"toolbox": {
		"add":    {summary: "Put a reusable script, query or runbook into the toolbox: `lgrass how toolbox add`", run: static("toolbox/add.md", "toolbox/about.md")},
		"list":   {summary: "Find and use what the toolbox holds: `lgrass how toolbox list`", run: static("toolbox/list.md", "toolbox/about.md")},
		"remove": {summary: "Take an item out of the toolbox: `lgrass how toolbox remove`", run: static("toolbox/remove.md", "toolbox/about.md")},
		"update": {summary: "Change an item's files, sentence or tags: `lgrass how toolbox update`", run: static("toolbox/update.md", "toolbox/about.md")},
	},
}

// domainSummaries gives each domain one sentence. lgrassconf writes the same list into the orientation, so a model
// knows every domain without calling `lgrass how`; a domain without a sentence fails the tests.
var domainSummaries = map[string]string{
	"ask":          "ask the human for a channel or a command run.",
	"berries":      "test the project's api for access-control holes against its access map, and fix what it finds.",
	"collaborator": "join a workgroup the human added you to.",
	"git":          "resolve a git conflict the human handed you.",
	"keeper":       "keep biblio/ and the code in agreement.",
	"leader":       "declare and lead a workgroup.",
	"task":         "close or hand over a scratchpad task.",
	"thinker":      "work as a thinker in a workgroup.",
	"toolbox":      "add, update, remove or list the scripts and queries the project reuses.",
	"work":         "start, resume or wrap up any work.",
	"write":        "write a biblio file in its exact format, such as a PRD, a handover, a law or a book chapter.",
}

// Domain is one `lgrass how` domain and its sentence.
type Domain struct {
	Name, Summary string
}

// Domains lists every domain with its sentence, sorted by name.
func Domains() []Domain {
	list := make([]Domain, 0, len(domains))
	for _, name := range sortedKeys(domains) {
		list = append(list, Domain{Name: name, Summary: domainSummaries[name]})
	}
	return list
}

// needsBiblio lists the domains whose steps read or write biblio/; the others, such as ask and git, work without it.
var needsBiblio = map[string]bool{"keeper": true, "task": true, "toolbox": true, "work": true, "write": true}

// retiredDomains points a command from an older orientation at the action that replaced it, since a running session
// keeps the orientation it started with.
var retiredDomains = map[string]string{"book": "write chapter", "laws": "write law"}

func hasBiblio(root string) bool {
	info, err := os.Stat(filepath.Join(root, "biblio"))
	return err == nil && info.IsDir()
}

// static serves a manual: fixed text with one branch and no arguments, joined from its fragments in order.
func static(paths ...string) func(root string, args []string) (string, error) {
	return func(_ string, args []string) (string, error) {
		if len(args) > 0 {
			return "", fmt.Errorf("how: %s takes no arguments", strings.TrimSuffix(paths[0], ".md"))
		}
		parts := make([]string, len(paths))
		for i, p := range paths {
			parts[i] = fragment(p, nil)
		}
		return strings.Join(parts, "\n\n") + "\n", nil
	}
}

// titled serves a manual like static, under a heading, for a fragment that procedures also print without one.
func titled(title string, paths ...string) func(root string, args []string) (string, error) {
	body := static(paths...)
	return func(root string, args []string) (string, error) {
		text, err := body(root, args)
		if err != nil {
			return "", err
		}
		return "# " + title + "\n\n" + text, nil
	}
}

// Render resolves args to a domain listing, an action listing, or an action's instructions for the project at root.
func Render(root string, args []string) (string, error) {
	return RenderEnv(Env{Root: root}, args)
}

// RenderEnv is Render with the sources outside the project's files that some actions use.
func RenderEnv(env Env, args []string) (string, error) {
	if len(args) == 0 {
		return listDomains(), nil
	}
	if to, ok := retiredDomains[args[0]]; ok {
		return "", fmt.Errorf("how: `lgrass how %s` is now `lgrass how %s`", strings.Join(args, " "), to)
	}
	actions, ok := domains[args[0]]
	if !ok {
		return "", fmt.Errorf("how: unknown domain %q\n\n%s", args[0], listDomains())
	}
	if len(args) == 1 {
		return listActions(args[0], actions), nil
	}
	act, ok := actions[args[1]]
	if !ok {
		return "", fmt.Errorf("how: unknown action %q in %s\n\n%s", args[1], args[0], listActions(args[0], actions))
	}
	if needsBiblio[args[0]] && !hasBiblio(env.Root) {
		return "", fmt.Errorf("how: this project has no biblio/ yet; run `lgrass biblio init` first, then run this again")
	}
	if act.runEnv != nil {
		return act.runEnv(env, args[2:])
	}
	return act.run(env.Root, args[2:])
}

// Known reports whether domain and action name an existing action, without running it.
func Known(domain, actionName string) bool {
	_, ok := domains[domain][actionName]
	return ok
}

func listDomains() string {
	var b strings.Builder
	b.WriteString("Domains, each listing its actions with `lgrass how <domain>`:\n")
	width := nameWidth(sortedKeys(domains))
	for _, d := range Domains() {
		fmt.Fprintf(&b, "  %-*s  %s\n", width, d.Name, upperFirst(d.Summary))
	}
	return b.String()
}

func listActions(domain string, actions map[string]action) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Actions in %s:\n", domain)
	names := sortedKeys(actions)
	width := nameWidth(names)
	for _, name := range names {
		fmt.Fprintf(&b, "  %-*s  %s\n", width, name, actions[name].summary)
	}
	return b.String()
}

func nameWidth(names []string) int {
	width := 0
	for _, name := range names {
		width = max(width, len(name))
	}
	return width
}

func upperFirst(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fragment(path string, vars map[string]string) string {
	data, err := content.ReadFile("content/" + path)
	if err != nil {
		panic("how: missing embedded fragment " + path)
	}
	text := strings.TrimSpace(string(data))
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{"+k+"}", v)
	}
	return text
}
