// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import "strings"

type pyRet int

const (
	// retNone returns nothing a literal can reach: a bool, a number, a file handle, None.
	retNone pyRet = iota
	// retArgs returns a string built from the arguments, such as os.path.join or json.dumps.
	retArgs
	// retPath returns a pathlib path built from the arguments.
	retPath
)

type pyPathArg struct {
	index int
	name  string
	kind  pyAccess
}

// pyFuncSpec describes a function or a path method. kind is the access to the listed path arguments (to every
// argument when none is listed); an argument not listed is content, such as the text a write puts in a file, and is
// not a path. mode, when set, reads the access from the call, as open's mode does.
type pyFuncSpec struct {
	kind  pyAccess
	paths []pyPathArg
	mode  func(pyCall) pyAccess
	ret   pyRet
}

func pathArgs(kind pyAccess, positions ...any) []pyPathArg {
	var out []pyPathArg
	for i := 0; i+1 < len(positions); i += 2 {
		out = append(out, pyPathArg{index: positions[i].(int), name: positions[i+1].(string), kind: kind})
	}
	return out
}

func reads(index int, name string) pyFuncSpec {
	return pyFuncSpec{kind: accessRead, paths: pathArgs(accessRead, index, name)}
}

func writes(index int, name string) pyFuncSpec {
	return pyFuncSpec{kind: accessWrite, paths: pathArgs(accessWrite, index, name)}
}

// copies reads its first argument and writes its second.
func copies(src, dst string) pyFuncSpec {
	return pyFuncSpec{kind: accessWrite, paths: append(pathArgs(accessRead, 0, src), pathArgs(accessWrite, 1, dst)...)}
}

func bothWritten(a, b string) pyFuncSpec {
	return pyFuncSpec{kind: accessWrite, paths: pathArgs(accessWrite, 0, a, 1, b)}
}

// runs takes every argument as part of a command, as a shell argument is.
var runs = pyFuncSpec{kind: accessWrite}

var (
	carries   = pyFuncSpec{ret: retArgs}
	pure      = pyFuncSpec{}
	pathBuild = pyFuncSpec{ret: retPath}
)

// openMode reads an open-style mode argument: any of w, a, x or + writes, a missing mode reads, and a mode the
// guard cannot read is taken as a write.
func openMode(index int) func(pyCall) pyAccess {
	return func(c pyCall) pyAccess {
		_, node, ok := c.arg(index, "mode")
		if !ok {
			return accessRead
		}
		if node.kind != nStr {
			return accessWrite
		}
		if strings.ContainsAny(node.text, "wax+") {
			return accessWrite
		}
		return accessRead
	}
}

// osOpenFlags reads os.open's flags: only O_RDONLY reads.
func osOpenFlags(c pyCall) pyAccess {
	_, node, ok := c.arg(1, "flags")
	if !ok {
		return accessWrite
	}
	var names []string
	var walk func(*pyNode)
	walk = func(n *pyNode) {
		switch n.kind {
		case nAttr, nName:
			names = append(names, n.text)
		}
		for _, k := range n.kids {
			walk(k)
		}
	}
	walk(node)
	if len(names) == 0 {
		return accessWrite
	}
	for _, name := range names {
		if name != "O_RDONLY" && name != "os" {
			return accessWrite
		}
	}
	return accessRead
}

var openSpec = pyFuncSpec{kind: accessRead, paths: pathArgs(accessNone, 0, "file"), mode: openMode(1)}

// pyFuncs are the functions the path reading knows, by their name after imports are resolved. A function missing
// here is unknown, and whatever it is given counts as written.
var pyFuncs = map[string]pyFuncSpec{
	"open":          openSpec,
	"io.open":       openSpec,
	"codecs.open":   openSpec,
	"builtins.open": openSpec,
	"os.open":       {kind: accessRead, paths: pathArgs(accessNone, 0, "path"), mode: osOpenFlags},

	"os.listdir":                 reads(0, "path"),
	"os.scandir":                 reads(0, "path"),
	"os.walk":                    reads(0, "top"),
	"os.stat":                    reads(0, "path"),
	"os.lstat":                   reads(0, "path"),
	"os.access":                  reads(0, "path"),
	"os.readlink":                reads(0, "path"),
	"os.path.exists":             reads(0, "path"),
	"os.path.lexists":            reads(0, "path"),
	"os.path.isfile":             reads(0, "path"),
	"os.path.isdir":              reads(0, "s"),
	"os.path.islink":             reads(0, "path"),
	"os.path.getsize":            reads(0, "filename"),
	"os.path.getmtime":           reads(0, "filename"),
	"os.path.getctime":           reads(0, "filename"),
	"os.path.getatime":           reads(0, "filename"),
	"os.path.samefile":           {kind: accessRead, paths: pathArgs(accessRead, 0, "f1", 1, "f2")},
	"glob.glob":                  reads(0, "pathname"),
	"glob.iglob":                 reads(0, "pathname"),
	"filecmp.cmp":                {kind: accessRead, paths: pathArgs(accessRead, 0, "f1", 1, "f2")},
	"shutil.disk_usage":          reads(0, "path"),
	"shutil.which":               pure,
	"sqlite3.connect":            writes(0, "database"),
	"os.makedirs":                writes(0, "name"),
	"os.mkdir":                   writes(0, "path"),
	"os.chmod":                   writes(0, "path"),
	"os.chown":                   writes(0, "path"),
	"os.lchown":                  writes(0, "path"),
	"os.utime":                   writes(0, "path"),
	"os.truncate":                writes(0, "path"),
	"os.mkfifo":                  writes(0, "path"),
	"os.remove":                  writes(0, "path"),
	"os.unlink":                  writes(0, "path"),
	"os.rmdir":                   writes(0, "path"),
	"os.removedirs":              writes(0, "name"),
	"shutil.rmtree":              writes(0, "path"),
	"os.rename":                  bothWritten("src", "dst"),
	"os.replace":                 bothWritten("src", "dst"),
	"os.renames":                 bothWritten("old", "new"),
	"os.symlink":                 bothWritten("src", "dst"),
	"os.link":                    bothWritten("src", "dst"),
	"shutil.move":                bothWritten("src", "dst"),
	"shutil.copy":                copies("src", "dst"),
	"shutil.copy2":               copies("src", "dst"),
	"shutil.copyfile":            copies("src", "dst"),
	"shutil.copytree":            copies("src", "dst"),
	"shutil.copymode":            copies("src", "dst"),
	"shutil.copystat":            copies("src", "dst"),
	"tempfile.mkdtemp":           writes(2, "dir"),
	"tempfile.mkstemp":           writes(2, "dir"),
	"tarfile.open":               runs,
	"zipfile.ZipFile":            runs,
	"os.system":                  runs,
	"os.popen":                   runs,
	"os.startfile":               runs,
	"pty.spawn":                  runs,
	"subprocess.run":             runs,
	"subprocess.call":            runs,
	"subprocess.check_call":      runs,
	"subprocess.check_output":    runs,
	"subprocess.Popen":           runs,
	"subprocess.getoutput":       runs,
	"subprocess.getstatusoutput": runs,

	"os.path.join":       carries,
	"os.path.expanduser": carries,
	"os.path.expandvars": carries,
	"os.path.abspath":    carries,
	"os.path.realpath":   carries,
	"os.path.normpath":   carries,
	"os.path.normcase":   carries,
	"os.path.dirname":    carries,
	"os.path.basename":   carries,
	"os.path.relpath":    carries,
	"os.path.split":      carries,
	"os.path.splitext":   carries,
	"os.path.commonpath": carries,
	"os.fspath":          carries,
	"os.fsencode":        carries,
	"os.fsdecode":        carries,
	"os.getenv":          carries,
	"os.environ.get":     carries,
	"os.getcwd":          pure,
	"os.path.isabs":      pure,

	"pathlib.Path":            pathBuild,
	"pathlib.PurePath":        pathBuild,
	"pathlib.PosixPath":       pathBuild,
	"pathlib.WindowsPath":     pathBuild,
	"pathlib.PurePosixPath":   pathBuild,
	"pathlib.PureWindowsPath": pathBuild,
	"pathlib.Path.home":       pathBuild,
	"pathlib.Path.cwd":        pathBuild,

	"print": pure, "len": pure, "isinstance": pure, "issubclass": pure, "hasattr": pure, "bool": pure, "int": pure,
	"float": pure, "abs": pure, "round": pure, "ord": pure, "chr": pure, "hex": pure, "oct": pure, "bin": pure,
	"range": pure, "type": pure, "id": pure, "hash": pure, "callable": pure, "any": pure, "all": pure, "sum": pure,
	"input": pure, "exit": pure, "quit": pure, "divmod": pure, "pow": pure, "dir": pure, "super": pure,
	"object": pure, "sys.exit": pure,

	"str": carries, "repr": carries, "ascii": carries, "list": carries, "tuple": carries, "set": carries,
	"frozenset": carries, "dict": carries, "sorted": carries, "reversed": carries, "enumerate": carries,
	"zip": carries, "map": carries, "filter": carries, "iter": carries, "next": carries, "min": carries,
	"max": carries, "format": carries, "bytes": carries, "bytearray": carries,

	"json.dumps": carries, "json.loads": carries, "json.dump": pure, "json.load": pure,
	"textwrap.dedent": carries, "textwrap.indent": carries, "textwrap.fill": carries, "textwrap.wrap": carries,
	"shlex.quote": carries, "shlex.split": carries, "shlex.join": carries,
	"re.sub": carries, "re.subn": carries, "re.split": carries, "re.findall": carries, "re.finditer": carries,
	"re.search": carries, "re.match": carries, "re.fullmatch": carries, "re.compile": carries, "re.escape": carries,
	"difflib.unified_diff": carries, "difflib.ndiff": carries, "difflib.SequenceMatcher": carries,
	"collections.Counter": carries, "collections.OrderedDict": carries, "collections.defaultdict": carries,
	"collections.deque": carries, "itertools.chain": carries,
	"fnmatch.fnmatch": pure, "fnmatch.filter": carries,
	"base64.b64encode": carries, "base64.b64decode": carries,
	"urllib.parse.quote": carries, "urllib.parse.unquote": carries,
	"logging.debug": pure, "logging.info": pure, "logging.warning": pure, "logging.error": pure,
	"logging.critical": pure, "logging.exception": pure,
	"sys.stdout.write": pure, "sys.stderr.write": pure, "time.time": pure, "time.sleep": pure,
}

// pyPathMethods are the methods of a pathlib path. kind is what the method does to the receiver's path.
var pyPathMethods = map[string]pyFuncSpec{
	"read_text": {kind: accessRead}, "read_bytes": {kind: accessRead}, "exists": {kind: accessRead},
	"is_file": {kind: accessRead}, "is_dir": {kind: accessRead}, "is_symlink": {kind: accessRead},
	"is_mount": {kind: accessRead}, "stat": {kind: accessRead}, "lstat": {kind: accessRead},
	"iterdir": {kind: accessRead, ret: retPath}, "glob": {kind: accessRead, ret: retPath},
	"rglob": {kind: accessRead, ret: retPath}, "walk": {kind: accessRead, ret: retPath},
	"samefile": {kind: accessRead}, "readlink": {kind: accessRead, ret: retPath}, "owner": {kind: accessRead},
	"group": {kind: accessRead}, "open": {kind: accessRead, mode: openMode(0)},
	"write_text": {kind: accessWrite}, "write_bytes": {kind: accessWrite}, "mkdir": {kind: accessWrite},
	"touch": {kind: accessWrite}, "chmod": {kind: accessWrite}, "lchmod": {kind: accessWrite},
	"unlink": {kind: accessWrite}, "rmdir": {kind: accessWrite},
	"symlink_to":  {kind: accessWrite, paths: pathArgs(accessRead, 0, "target")},
	"hardlink_to": {kind: accessWrite, paths: pathArgs(accessRead, 0, "target")},
	"rename":      {kind: accessWrite, paths: pathArgs(accessWrite, 0, "target"), ret: retPath},
	"replace":     {kind: accessWrite, paths: pathArgs(accessWrite, 0, "target"), ret: retPath},
	"copy":        {kind: accessRead, paths: pathArgs(accessWrite, 0, "target"), ret: retPath},
	"move":        {kind: accessWrite, paths: pathArgs(accessWrite, 0, "target"), ret: retPath},
	"expanduser":  {ret: retPath}, "absolute": {ret: retPath}, "resolve": {ret: retPath},
	"joinpath": {ret: retPath}, "with_name": {ret: retPath}, "with_suffix": {ret: retPath},
	"with_stem": {ret: retPath}, "relative_to": {ret: retPath}, "with_segments": {ret: retPath},
	"as_posix": {ret: retArgs}, "as_uri": {ret: retArgs}, "__str__": {ret: retArgs}, "__fspath__": {ret: retArgs},
	"is_absolute": {}, "is_relative_to": {}, "match": {}, "full_match": {}, "is_reserved": {},
}

type pyMethodSpec int

const (
	// methodNone returns nothing a literal reaches and takes its arguments as content: a test such as startswith,
	// or a file handle's write.
	methodNone pyMethodSpec = iota
	// methodCarry returns a value built from the receiver and the arguments, as str.replace does.
	methodCarry
	// methodStore puts its arguments into the receiver, as list.append does.
	methodStore
	// methodRecv returns part of the receiver, as dict.get does.
	methodRecv
)

// pyMethods are methods of strings, containers, file handles and regular expressions, which never open a path of
// their own. A method missing here, called on a value that is not a pathlib path, is unknown.
var pyMethods = map[string]pyMethodSpec{
	"capitalize": methodCarry, "casefold": methodCarry, "center": methodCarry, "encode": methodCarry,
	"decode": methodCarry, "expandtabs": methodCarry, "format": methodCarry, "format_map": methodCarry,
	"join": methodCarry, "ljust": methodCarry, "lower": methodCarry, "lstrip": methodCarry,
	"partition": methodCarry, "removeprefix": methodCarry, "removesuffix": methodCarry, "replace": methodCarry,
	"rjust": methodCarry, "rpartition": methodCarry, "rsplit": methodCarry, "rstrip": methodCarry,
	"split": methodCarry, "splitlines": methodCarry, "strip": methodCarry, "swapcase": methodCarry,
	"title": methodCarry, "translate": methodCarry, "upper": methodCarry, "zfill": methodCarry,
	"sub": methodCarry, "subn": methodCarry, "search": methodCarry, "match": methodCarry, "fullmatch": methodCarry,
	"findall": methodCarry, "finditer": methodCarry, "group": methodCarry, "groups": methodCarry,
	"groupdict": methodCarry, "expand": methodCarry, "strftime": methodCarry, "isoformat": methodCarry,

	"count": methodNone, "endswith": methodNone, "find": methodNone, "index": methodNone, "isalnum": methodNone,
	"isalpha": methodNone, "isascii": methodNone, "isdecimal": methodNone, "isdigit": methodNone,
	"isidentifier": methodNone, "islower": methodNone, "isnumeric": methodNone, "isprintable": methodNone,
	"isspace": methodNone, "istitle": methodNone, "isupper": methodNone, "rfind": methodNone, "rindex": methodNone,
	"startswith": methodNone, "span": methodNone, "start": methodNone, "end": methodNone,
	"write": methodNone, "writelines": methodNone, "read": methodNone, "readline": methodNone,
	"readlines": methodNone, "close": methodNone, "flush": methodNone, "seek": methodNone, "tell": methodNone,
	"fileno": methodNone, "sort": methodNone, "reverse": methodNone, "clear": methodNone, "remove": methodNone,
	"discard": methodNone,

	"append": methodStore, "extend": methodStore, "insert": methodStore, "add": methodStore,
	"update": methodStore, "setdefault": methodStore, "appendleft": methodStore, "extendleft": methodStore,

	"get": methodCarry, "pop": methodCarry, "popleft": methodRecv, "copy": methodRecv, "items": methodRecv,
	"keys": methodRecv, "values": methodRecv, "most_common": methodRecv,
}
