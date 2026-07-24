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
	"kdbx": true, "kdbx.py": true, "keepassxc-cli": true, "keepassxc": true,
}

// blockedOps mutate the vault or expose a value — a human role.
var blockedOps = map[string]bool{
	"set": true, "delete": true, "mv": true, "import": true, "rekey": true, "export": true,
}

var (
	secretRe   = regexp.MustCompile(`(?i)\.(kdbx|keyx)\b`)
	segSplitRe = regexp.MustCompile(`\|\||&&|[;|\n]`)
)

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

		norm := strings.ReplaceAll(seg, `\`, "/")
		hit := secretRe.FindString(norm)
		frag := matchedConfigFragment(strings.ToLower(norm))
		if hit == "" && frag == "" {
			continue
		}
		prog := programOf(tokens)
		allowed := allowedInvokers[prog] ||
			((prog == "uv" || prog == "uvx") && strings.Contains(strings.ToLower(seg), "kdbx"))
		if allowed {
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
		base := path.Base(strings.ReplaceAll(tok, `\`, "/"))
		if base != "kdbx" && base != "kdbx.py" {
			continue
		}
		if base == prog || prog == "uv" || prog == "uvx" {
			for _, next := range tokens[i+1:] {
				if !strings.HasPrefix(next, "-") {
					return next, true
				}
			}
			return "", true
		}
		return "", false
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
