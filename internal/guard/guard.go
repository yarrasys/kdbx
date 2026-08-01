// Package guard implements the PreToolUse decision that keeps an agent inside
// the read-only half of the kdbx role contract (spec C10, N3). It fails open:
// anything that is not a clear violation is allowed, because a guard must never
// brick the user's shell.
package guard

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/yarrasys/kdbx/internal/paths"
)

// allowedInvokers may touch vault material: kdbx itself and the KeePassXC CLI,
// neither of which prints a secret by default.
var allowedInvokers = map[string]bool{
	"kdbx": true, "keepassxc-cli": true, "keepassxc": true,
}

// blockedOps mutate the vault or expose a value — a human role.
var blockedOps = map[string]bool{
	"set": true, "delete": true, "mv": true, "import": true, "rekey": true, "export": true,
}

var (
	secretRe   = regexp.MustCompile(`(?i)\.(kdbx|keyx)\b`)
	segSplitRe = regexp.MustCompile(`\|\||&&|[;|\n]`)
	// redirRe finds shell output redirections and captures the target path.
	redirRe = regexp.MustCompile(`>>?\s*([^\s>|;&]+)`)
)

// pointerName is the committed pointer file the guard must keep agents from
// rewriting: it selects which vault and key file kdbx opens, so an agent that
// edits it is editing its own permissions. Reads stay allowed (the file is
// committed and holds no secrets), and kdbx itself writes it legitimately.
const pointerName = ".keepassxc.json"

// pointerWriterProgs always write their file arguments.
var pointerWriterProgs = map[string]bool{
	"tee": true, "vi": true, "vim": true, "nvim": true, "nano": true,
	"emacs": true, "code": true, "subl": true,
}

// Decide returns a deny reason for command, or "" to allow it.
func Decide(command string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	for _, rawSeg := range segSplitRe.Split(command, -1) {
		seg := strings.TrimSpace(rawSeg)
		if seg == "" {
			continue
		}
		tokens := tokenize(seg)

		if op := blockedKdbxOp(tokens); op != "" {
			return "kdbx role-guard: '" + op + "' is a human-only operation (it mutates the vault " +
				"or exposes a secret value). Don't run it as the agent — give the command " +
				"to the human to run in their own terminal (or via `!kdbx ...`)."
		}

		if reason := pointerWriteReason(seg, tokens); reason != "" {
			return reason
		}

		norm := strings.ReplaceAll(seg, `\`, "/")
		hit := secretRe.FindString(norm)
		frag := matchedConfigFragment(strings.ToLower(norm))
		if hit == "" && frag == "" {
			continue
		}
		prog := programOf(tokens)
		if allowedInvokers[prog] {
			continue
		}
		what := hit
		if what == "" {
			what = "a KeePassXC config path"
		}
		name := prog
		if name == "" {
			name = "command"
		}
		return "kdbx leak-guard: '" + name + "' would read a KeePassXC vault/keyfile (" + what + "). " +
			"Use `kdbx run -- ...` to inject secrets without printing them."
	}
	return ""
}

// isPointerPath reports whether tok names the pointer file, at any path.
func isPointerPath(tok string) bool {
	tok = strings.Trim(tok, `"'`)
	return path.Base(strings.ReplaceAll(tok, `\`, "/")) == pointerName
}

const pointerWriteDeny = "kdbx role-guard: writing " + pointerName + " is a human-only " +
	"operation (it selects which vault, key file and variable mappings kdbx uses). " +
	"Don't edit it as the agent: ask the human to make the change in their own editor."

// pointerWriteReason reports why this segment would write the pointer file, or
// "" when it is fine. Detection is deliberately heuristic and fail-open: it
// catches the obvious one-liners (redirection, in-place editors, tee, mv/cp
// onto the file), not every conceivable write.
func pointerWriteReason(seg string, tokens []string) string {
	// Shell redirection writes regardless of which program runs.
	for _, m := range redirRe.FindAllStringSubmatch(seg, -1) {
		if isPointerPath(m[1]) {
			return pointerWriteDeny
		}
	}

	prog := programOf(tokens)
	if allowedInvokers[prog] {
		return ""
	}
	touchesPointer := false
	for _, tok := range tokens {
		if isPointerPath(tok) {
			touchesPointer = true
			break
		}
	}
	if !touchesPointer {
		return ""
	}

	switch {
	case pointerWriterProgs[prog]:
		return pointerWriteDeny
	case prog == "sed" || prog == "perl":
		// Only in-place editing writes; without -i these are reads.
		for _, tok := range tokens {
			if strings.HasPrefix(tok, "-i") {
				return pointerWriteDeny
			}
		}
	case prog == "mv" || prog == "cp":
		// Only the destination is a write; the pointer as source is a read.
		for i := len(tokens) - 1; i > 0; i-- {
			if strings.HasPrefix(tokens[i], "-") {
				continue
			}
			if isPointerPath(tokens[i]) {
				return pointerWriteDeny
			}
			break
		}
	}
	return ""
}

// tokenize splits a shell segment into words, stripping simple quoting.
func tokenize(seg string) []string {
	var (
		out   []string
		cur   strings.Builder
		quote rune
	)
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range seg {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// programOf returns the basename of the invoked program, skipping leading
// VAR=value assignments.
func programOf(tokens []string) string {
	for _, tok := range tokens {
		head, _, isAssign := strings.Cut(tok, "=")
		if isAssign && !strings.HasPrefix(tok, "-") && !strings.Contains(head, "/") {
			continue
		}
		return path.Base(strings.ReplaceAll(tok, `\`, "/"))
	}
	return ""
}

// kdbxOp returns the subcommand when this segment actually invokes the kdbx CLI.
func kdbxOp(tokens []string) (string, bool) {
	if len(tokens) == 0 {
		return "", false
	}
	prog := programOf(tokens)
	for i, tok := range tokens {
		if path.Base(strings.ReplaceAll(tok, `\`, "/")) != "kdbx" {
			continue
		}
		if prog != "kdbx" {
			return "", false
		}
		for _, next := range tokens[i+1:] {
			if !strings.HasPrefix(next, "-") {
				return next, true
			}
		}
		return "", true
	}
	return "", false
}

// blockedKdbxOp names the offending operation, or "" when the segment is fine.
func blockedKdbxOp(tokens []string) string {
	op, isKdbx := kdbxOp(tokens)
	if !isKdbx {
		return ""
	}
	if blockedOps[op] {
		return op
	}
	if op == "get" {
		for _, t := range tokens {
			if t == "--reveal" || t == "--clip" {
				return "get --reveal/--clip"
			}
		}
	}
	if op == "policy" {
		for _, t := range tokens {
			if t == "bless" {
				return "policy bless"
			}
		}
	}
	if op == "run" {
		// Only kdbx's own flags count: everything after -- is the child's argv.
		for _, t := range tokens {
			if t == "--" {
				break
			}
			if t == "--no-mask" {
				return "run --no-mask"
			}
			if t == "--any" {
				return "run --any"
			}
		}
	}
	return ""
}

// matchedConfigFragment reports which KeePassXC config-dir fragment appears in
// the lowercased, forward-slashed segment.
func matchedConfigFragment(lowerSeg string) string {
	frags := []string{".config/keepassxc/", "appdata/local/keepassxc/"}
	if v := paths.KeepassxcDir(); v != "" {
		frags = append(frags, strings.ToLower(strings.ReplaceAll(v, `\`, "/"))+"/")
	}
	for _, env := range []string{"XDG_CONFIG_HOME", "LOCALAPPDATA"} {
		if v := os.Getenv(env); v != "" {
			frags = append(frags,
				strings.ToLower(strings.TrimRight(strings.ReplaceAll(v, `\`, "/"), "/"))+"/keepassxc/")
		}
	}
	for _, f := range frags {
		if strings.Contains(lowerSeg, f) {
			return f
		}
	}
	return ""
}

type hookInput struct {
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// Run reads a PreToolUse payload and emits a deny envelope when warranted. It
// always returns 0.
func Run(stdin io.Reader, stdout io.Writer) int {
	b, err := io.ReadAll(stdin)
	if err != nil {
		return 0
	}
	var in hookInput
	if err := json.Unmarshal(b, &in); err != nil {
		return 0
	}
	reason := Decide(in.ToolInput.Command)
	if reason == "" {
		return 0
	}
	var out hookOutput
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "deny"
	out.HookSpecificOutput.PermissionDecisionReason = reason
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(out)
	return 0
}
