// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"strings"
)

// posixShells are the shells whose command text the gate parses. Git Bash's bash.exe qualifies; System32's bash.exe is WSL, whose paths are another filesystem, and is refused by the caller.
var posixShells = map[string]bool{"bash": true, "sh": true, "dash": true, "zsh": true}

func posixShellName(name string) bool {
	lower := strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
	if strings.Contains(lower, "/system32/") || strings.Contains(lower, "/sysnative/") {
		return false
	}
	base := lower[strings.LastIndex(lower, "/")+1:]
	return posixShells[strings.TrimSuffix(base, ".exe")]
}

// psSingleQuote writes s as a PowerShell single-quoted string, in which only a doubled quote is special.
func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// psCallFlags are the shell options a call may carry before its script. None takes an argument, so the first
// non-option word, the script, is the last word of the line.
const psCallFlags = "celxu"

// psSafeBareScript is what a script with no space or tab may hold: Windows PowerShell passes such an argument to a
// native program unquoted, and Git Bash's runtime then expands globs and quotes in it before bash sees it.
const psSafeBareScript = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./:=+-"

// powerShellPosixScript reads a PowerShell line that runs a POSIX shell by path with the call operator,
// `& '<path to bash.exe>' -c '<script>'`, the form the opaque-shell deny suggests when bare bash is not Git Bash.
// matched is false for any other line. A matched line returns either the script, as bash receives it, or a refusal
// for a script that Windows PowerShell would not pass to bash intact: it adds double quotes around an argument
// holding a space but leaves a double quote inside unescaped, and a trailing backslash then escapes its closing
// quote. Nothing may follow the script, since PowerShell would run it as a statement of its own.
func powerShellPosixScript(line string) (script, refusal string, matched bool) {
	if strings.ContainsAny(line, "‘’‚‛") {
		// PowerShell reads these typographic quotes as single quotes, which the reader below does not model.
		return "", "", false
	}
	rest := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(rest, "&") {
		return "", "", false
	}
	rest = rest[1:]
	var words []string
	var quoted []bool
	for {
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" {
			break
		}
		word, wasQuoted, after, ok := psWord(rest)
		if !ok {
			return "", "", false
		}
		words = append(words, word)
		quoted = append(quoted, wasQuoted)
		rest = after
	}
	if len(words) < 3 || !posixShellName(words[0]) || !quoted[len(words)-1] {
		return "", "", false
	}
	hasC := false
	for i, flag := range words[1 : len(words)-1] {
		if quoted[i+1] || len(flag) < 2 || flag[0] != '-' || strings.Trim(flag[1:], psCallFlags) != "" {
			return "", "", false
		}
		hasC = hasC || strings.Contains(flag, "c")
	}
	if !hasC {
		return "", "", false
	}
	script = words[len(words)-1]
	switch {
	case strings.TrimSpace(script) == "":
		return "", "Windows PowerShell drops an empty argument, so the script would not reach bash.", true
	case strings.ContainsAny(script, "\"“”„"):
		return "", "Windows PowerShell does not pass a double quote inside an argument to bash intact. Use single quotes inside the command, each written as ''.", true
	case strings.HasSuffix(script, `\`):
		return "", "Windows PowerShell does not pass an argument ending in a backslash to bash intact.", true
	case !strings.ContainsAny(script, " \t") && strings.Trim(script, psSafeBareScript) != "":
		return "", "Git Bash expands a one-word argument holding glob or quote characters before bash sees it. Use bash -c instead.", true
	}
	return script, "", true
}

// psWord reads one word of a PowerShell command: a single-quoted string, in which a doubled quote is one quote,
// or a bare word of characters that PowerShell takes literally in argument mode. Anything else is not read.
func psWord(s string) (word string, quoted bool, rest string, ok bool) {
	if s[0] == '\'' {
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			if s[i] != '\'' {
				b.WriteByte(s[i])
				continue
			}
			if i+1 < len(s) && s[i+1] == '\'' {
				b.WriteByte('\'')
				i++
				continue
			}
			after := s[i+1:]
			if after != "" && after[0] != ' ' && after[0] != '\t' {
				return "", false, "", false
			}
			return b.String(), true, after, true
		}
		return "", false, "", false
	}
	end := strings.IndexAny(s, " \t")
	if end < 0 {
		end = len(s)
	}
	word = s[:end]
	if strings.Trim(word, psSafeBareScript+`\`) != "" {
		return "", false, "", false
	}
	return word, false, s[end:], true
}
