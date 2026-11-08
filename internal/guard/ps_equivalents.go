// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import "strings"

// psEquivalents returns the POSIX calls and redirects a bound file-core cmdlet amounts to, so the deletion scope,
// secret path, write path and download checks judge it as they judge bash. fromPipeline marks a cmdlet taking its
// paths from the pipeline when none is bound; its equivalents are Indirect, as ls | xargs rm is. A registry path
// becomes the reg command that changes it, so registry-write applies; other providers yield no file equivalent.
func psEquivalents(b psBound, fromPipeline bool) ([]shellCall, []shellRedirect) {
	if b.Refusal != "" {
		return nil, nil
	}
	e := psEquivalent{bound: b, fromPipeline: fromPipeline}
	switch b.Cmdlet {
	case "remove-item":
		e.remove()
	case "copy-item":
		e.copyOrMove("cp", "copy")
	case "move-item":
		e.copyOrMove("mv", "delete")
	case "rename-item":
		e.rename()
	case "set-content", "add-content", "out-file", "tee-object", "export-csv", "export-clixml":
		e.write()
	case "clear-content":
		e.truncate()
	case "new-item":
		e.newItem()
	case "get-content":
		e.read()
	case "set-location", "push-location":
		e.changeDirectory()
	case "pop-location":
		e.calls = append(e.calls, shellCall{Name: "popd"})
	case "invoke-webrequest", "invoke-restmethod":
		e.download()
	}
	return e.calls, e.redirects
}

type psEquivalent struct {
	bound        psBound
	fromPipeline bool
	calls        []shellCall
	redirects    []shellRedirect
}

// psTarget is one path value with the provider it names.
type psTarget struct {
	text    string
	dynamic bool
	kind    psProvider
}

// targets returns the values of the named path parameters, classified by provider. A dynamic value keeps its text
// and counts as a file path, so the dynamic-target and variable checks meet it.
func (e *psEquivalent) targets(names ...string) []psTarget {
	var out []psTarget
	for _, name := range names {
		for _, value := range e.bound.Params[name] {
			if value.Dynamic {
				out = append(out, psTarget{text: value.Text, dynamic: true, kind: psProviderFileSystem})
				continue
			}
			kind, fsPath := psProviderPath(value.Text)
			if kind == psProviderTemp {
				kind = psProviderFileSystem
			}
			out = append(out, psTarget{text: fsPath, kind: kind})
			if kind != psProviderFileSystem {
				out[len(out)-1].text = value.Text
			}
		}
	}
	return out
}

// fileArgs splits targets into the file-system ones, as arguments with their dynamic marks, and emits reg calls for
// the registry ones. An unknown drive, such as one a profile maps, may land anywhere, so it counts as dynamic.
func (e *psEquivalent) fileArgs(targets []psTarget, regVerb string) ([]string, []bool) {
	var args []string
	var dynamic []bool
	for _, target := range targets {
		switch target.kind {
		case psProviderFileSystem, psProviderUnknown:
			args = append(args, target.text)
			dynamic = append(dynamic, target.dynamic || target.kind == psProviderUnknown)
		case psProviderRegistry:
			if regVerb != "" {
				e.calls = append(e.calls, shellCall{Name: "reg", Args: []string{regVerb, target.text}, ArgDynamic: []bool{false, false}})
			}
		}
	}
	return args, dynamic
}

func (e *psEquivalent) call(name string, flags []string, args []string, dynamic []bool) {
	callArgs := append(append([]string{}, flags...), "--")
	callDynamic := make([]bool, len(callArgs))
	callArgs = append(callArgs, args...)
	callDynamic = append(callDynamic, dynamic...)
	e.calls = append(e.calls, shellCall{Name: name, Args: callArgs, ArgDynamic: callDynamic, Indirect: e.indirect()})
}

// indirect reports a cmdlet whose paths come from the pipeline: no path parameter is bound and it runs in a pipeline.
func (e *psEquivalent) indirect() bool {
	if !e.fromPipeline {
		return false
	}
	for _, name := range []string{"path", "literalpath", "filepath"} {
		if len(e.bound.Params[name]) > 0 {
			return false
		}
	}
	return true
}

// remove is Remove-Item as rm, with -r only for -Recurse and -f for -Force, so a single file meets what bash rm meets.
func (e *psEquivalent) remove() {
	args, dynamic := e.fileArgs(e.targets("path", "literalpath"), "delete")
	if len(args) == 0 && !e.indirect() {
		return
	}
	var flags []string
	if e.bound.Switches["recurse"] {
		flags = append(flags, "-r")
	}
	if e.bound.Switches["force"] {
		flags = append(flags, "-f")
	}
	e.call("rm", flags, args, dynamic)
}

// copyOrMove is Copy-Item as cp and Move-Item as mv, the destination last; without -Destination it is the current directory.
func (e *psEquivalent) copyOrMove(name, regVerb string) {
	args, dynamic := e.fileArgs(e.targets("path", "literalpath"), regVerb)
	destination := e.targets("destination")
	if len(destination) == 0 {
		destination = []psTarget{{text: ".", kind: psProviderFileSystem}}
	}
	destArgs, destDynamic := e.fileArgs(destination, "add")
	if (len(args) == 0 && !e.indirect()) || len(destArgs) == 0 {
		return
	}
	var flags []string
	if name == "cp" && e.bound.Switches["recurse"] {
		flags = append(flags, "-r")
	}
	e.call(name, flags, append(args, destArgs...), append(dynamic, destDynamic...))
}

// rename is Rename-Item as mv into the same directory under the new name.
func (e *psEquivalent) rename() {
	targets := e.targets("path", "literalpath")
	args, dynamic := e.fileArgs(targets, "add")
	names := e.bound.Params["newname"]
	if len(args) != 1 || len(names) != 1 {
		return
	}
	dest := names[0].Text
	if !strings.ContainsAny(dest, `/\`) {
		if cut := strings.LastIndexAny(args[0], `/\`); cut >= 0 {
			dest = args[0][:cut+1] + dest
		}
	}
	e.call("mv", nil, []string{args[0], dest}, []bool{dynamic[0], names[0].Dynamic})
}

// write is a cmdlet that writes its path parameter, as a write redirect.
func (e *psEquivalent) write() {
	args, _ := e.fileArgs(e.targets("path", "literalpath", "filepath"), "")
	for _, arg := range args {
		e.redirects = append(e.redirects, shellRedirect{Path: arg, Write: true})
	}
}

// truncate is Clear-Content as truncate -s 0, which file-truncate asks about.
func (e *psEquivalent) truncate() {
	args, dynamic := e.fileArgs(e.targets("path", "literalpath"), "")
	if len(args) == 0 && !e.indirect() {
		return
	}
	e.call("truncate", []string{"-s", "0"}, args, dynamic)
}

// read is Get-Content as cat.
func (e *psEquivalent) read() {
	args, dynamic := e.fileArgs(e.targets("path", "literalpath"), "")
	if len(args) == 0 {
		return
	}
	e.call("cat", nil, args, dynamic)
}

// newItem is New-Item: a directory as mkdir -p, a link as ln -s (ln for a hard link) to its -Value, anything else as a
// write redirect. -Name joins -Path, or the current directory without one.
func (e *psEquivalent) newItem() {
	paths := e.targets("path")
	if names := e.bound.Params["name"]; len(names) == 1 {
		name := psTarget{text: names[0].Text, dynamic: names[0].Dynamic, kind: psProviderFileSystem}
		if len(paths) == 0 {
			paths = []psTarget{name}
		}
		for i := range paths {
			if paths[i].kind == psProviderFileSystem || paths[i].kind == psProviderUnknown {
				paths[i].text = strings.TrimRight(paths[i].text, `/\`) + "/" + name.text
				paths[i].dynamic = paths[i].dynamic || name.dynamic
			}
		}
	}
	args, dynamic := e.fileArgs(paths, "add")
	if len(args) == 0 {
		return
	}
	itemType := ""
	if types := e.bound.Params["itemtype"]; len(types) == 1 && !types[0].Dynamic {
		itemType = strings.ToLower(types[0].Text)
	}
	switch itemType {
	case "directory":
		e.call("mkdir", []string{"-p"}, args, dynamic)
	case "symboliclink", "junction", "hardlink":
		values := e.bound.Params["value"]
		if len(values) != 1 {
			return
		}
		flags := []string{"-s"}
		if itemType == "hardlink" {
			flags = nil
		}
		for i, arg := range args {
			e.call("ln", flags, []string{values[0].Text, arg}, []bool{values[0].Dynamic, dynamic[i]})
		}
	default:
		for _, arg := range args {
			e.redirects = append(e.redirects, shellRedirect{Path: arg, Write: true})
		}
	}
}

// changeDirectory is Set-Location and Push-Location as cd. A location in another provider, an unknown drive or a
// dynamic one makes later relative paths land where the guard cannot follow, so it becomes a cd the deletion scope
// refuses to follow.
func (e *psEquivalent) changeDirectory() {
	targets := e.targets("path", "literalpath")
	if len(targets) == 0 {
		e.calls = append(e.calls, shellCall{Name: "cd", Indirect: e.indirect()})
		return
	}
	target := targets[0]
	if target.dynamic || target.kind != psProviderFileSystem {
		e.calls = append(e.calls, shellCall{Name: "cd", Args: []string{"-", target.text}, ArgDynamic: []bool{false, target.dynamic}})
		return
	}
	e.calls = append(e.calls, shellCall{Name: "cd", Args: []string{"--", target.text}, ArgDynamic: []bool{false, false}})
}

// download is Invoke-WebRequest and Invoke-RestMethod as curl: -OutFile as -o, -InFile as an upload with -T.
func (e *psEquivalent) download() {
	uris := e.bound.Params["uri"]
	if len(uris) != 1 {
		return
	}
	args := []string{}
	dynamic := []bool{}
	if out := e.bound.Params["outfile"]; len(out) == 1 {
		args = append(args, "-o", out[0].Text)
		dynamic = append(dynamic, false, out[0].Dynamic)
	}
	if in := e.bound.Params["infile"]; len(in) == 1 {
		args = append(args, "-T", in[0].Text)
		dynamic = append(dynamic, false, in[0].Dynamic)
	}
	args = append(args, uris[0].Text)
	dynamic = append(dynamic, uris[0].Dynamic)
	e.calls = append(e.calls, shellCall{Name: "curl", Args: args, ArgDynamic: dynamic})
}
