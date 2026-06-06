package main

/*
#cgo CFLAGS: -I${SRCDIR} -I${SRCDIR}/tree_sitter_src
#cgo LDFLAGS: -ldl
#include "tree_sitter_src/lib.c"

#include <dlfcn.h>
#include <stdlib.h>
#include "tree_sitter/api.h"

typedef TSLanguage *(*LangFn)(void);

static void* load_language(const char *path, const char *sym, TSLanguage **out_lang) {
    void *handle = dlopen(path, RTLD_LAZY | RTLD_GLOBAL);
    if (!handle) return NULL;
    LangFn fn = (LangFn)dlsym(handle, sym);
    if (!fn) {
        dlclose(handle);
        return NULL;
    }
    *out_lang = fn();
    return handle;
}

static void close_library(void *handle) {
    if (handle) dlclose(handle);
}

static const char* last_dl_error() {
    return dlerror();
}

static TSQueryCapture get_query_capture(const TSQueryCapture *captures, uint32_t i) {
    return captures[i];
}
*/
import "C"
import (
	"embed"
	"fmt"
	"strings"
	"unsafe"
)

//go:embed queries
var queriesFS embed.FS

type capture struct {
	name      string
	nodeType  string
	startByte int
	endByte   int
	startLine int
	endLine   int
}

func (c capture) text(src []byte) string {
	if c.startByte >= c.endByte || c.endByte > len(src) {
		return ""
	}
	return string(src[c.startByte:c.endByte])
}

func findCap(caps []capture, name string) (capture, bool) {
	for _, c := range caps {
		if c.name == name {
			return c, true
		}
	}
	return capture{}, false
}

type grammar struct {
	lang   *C.TSLanguage
	query  *C.TSQuery
	name   string
	handle unsafe.Pointer
}

func (gr *grammar) close() {
	if gr.query != nil {
		C.ts_query_delete(gr.query)
		gr.query = nil
	}
	if gr.handle != nil {
		C.close_library(gr.handle)
		gr.handle = nil
	}
}

func (gr *grammar) extract(src []byte) [][]capture {
	if gr.query == nil {
		return nil
	}

	p := C.ts_parser_new()
	defer C.ts_parser_delete(p)
	C.ts_parser_set_language(p, gr.lang)

	var ptr *C.char
	if len(src) > 0 {
		ptr = (*C.char)(unsafe.Pointer(&src[0]))
	}
	tree := C.ts_parser_parse_string(p, nil, ptr, C.uint32_t(len(src)))
	if tree == nil {
		return nil
	}
	defer C.ts_tree_delete(tree)

	root := C.ts_tree_root_node(tree)
	qcursor := C.ts_query_cursor_new()
	defer C.ts_query_cursor_delete(qcursor)
	C.ts_query_cursor_exec(qcursor, gr.query, root)

	var matches [][]capture
	var m C.TSQueryMatch

	for bool(C.ts_query_cursor_next_match(qcursor, &m)) {
		count := int(m.capture_count)
		if count == 0 {
			continue
		}
		caps := make([]capture, count)
		for i := 0; i < count; i++ {
			qc := C.get_query_capture(m.captures, C.uint32_t(i))
			var nameLen C.uint32_t
			namePtr := C.ts_query_capture_name_for_id(gr.query, qc.index, &nameLen)
			startPt := C.ts_node_start_point(qc.node)
			endPt := C.ts_node_end_point(qc.node)
			caps[i] = capture{
				name:      C.GoStringN(namePtr, C.int(nameLen)),
				nodeType:  C.GoString(C.ts_node_type(qc.node)),
				startByte: int(C.ts_node_start_byte(qc.node)),
				endByte:   int(C.ts_node_end_byte(qc.node)),
				startLine: int(startPt.row) + 1,
				endLine:   int(endPt.row) + 1,
			}
		}
		matches = append(matches, caps)
	}
	return matches
}

type grammarLoader struct {
	grammars map[string]*grammar
}

func newGrammarLoader() *grammarLoader {
	return &grammarLoader{grammars: make(map[string]*grammar)}
}

func (g *grammarLoader) load(lang string) error {
	sym := langSymbol(lang)
	var loaded *grammar
	for _, dir := range []string{"/home/lg/.lemongrass/grammars", "/app/grammars"} {
		path := dir + "/" + lang + ".so"
		cpath := C.CString(path)
		csym := C.CString(sym)
		var clang *C.TSLanguage
		handle := C.load_language(cpath, csym, &clang)
		C.free(unsafe.Pointer(cpath))
		C.free(unsafe.Pointer(csym))
		if handle == nil {
			continue
		}
		if existing := g.grammars[lang]; existing != nil {
			existing.close()
		}
		loaded = &grammar{lang: clang, name: lang, handle: handle}
		break
	}
	if loaded == nil {
		cerr := C.last_dl_error()
		if cerr != nil {
			return fmt.Errorf("could not load grammar for %s: %s", lang, C.GoString(cerr))
		}
		return fmt.Errorf("grammar not found for %s", lang)
	}

	schemeData, err := queriesFS.ReadFile("queries/" + lang + ".scm")
	if err != nil {
		// No query file -- grammar loads but extract returns nothing.
		g.grammars[lang] = loaded
		return nil
	}
	csrc := C.CString(string(schemeData))
	defer C.free(unsafe.Pointer(csrc))
	var errOffset C.uint32_t
	var errType C.TSQueryError
	q := C.ts_query_new(loaded.lang, csrc, C.uint32_t(len(schemeData)), &errOffset, &errType)
	if q == nil {
		loaded.close()
		return fmt.Errorf("query compile error for %s at offset %d (error type %d)", lang, errOffset, errType)
	}
	loaded.query = q
	g.grammars[lang] = loaded
	return nil
}

func (g *grammarLoader) closeAll() {
	for _, gr := range g.grammars {
		gr.close()
	}
}

func (g *grammarLoader) get(lang string) *grammar { return g.grammars[lang] }

func (g *grammarLoader) loaded() []string {
	out := make([]string, 0, len(g.grammars))
	for k := range g.grammars {
		out = append(out, k)
	}
	return out
}

func langSymbol(lang string) string {
	switch lang {
	case "ts":
		return "tree_sitter_typescript"
	case "vue":
		return "tree_sitter_vue"
	default:
		return "tree_sitter_" + strings.ReplaceAll(lang, "-", "_")
	}
}
