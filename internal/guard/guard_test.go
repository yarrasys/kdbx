package guard

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecideBlocksAgentWriteOps(t *testing.T) {
	for _, cmd := range []string{
		"kdbx set api/openai",
		"kdbx delete api/openai --purge",
		"kdbx mv a b",
		"kdbx import .env",
		"kdbx rekey",
		"kdbx export --out .env",
		"kdbx get api/openai --reveal",
		"kdbx get api/openai --clip",
	} {
		if got := Decide(cmd); got == "" {
			t.Errorf("Decide(%q) allowed a human-only operation", cmd)
		} else if !strings.Contains(got, "human-only operation") {
			t.Errorf("Decide(%q) = %q, want the role-guard wording", cmd, got)
		}
	}
}

func TestDecideAllowsAgentReadOps(t *testing.T) {
	for _, cmd := range []string{
		"kdbx run -- npm test",
		"kdbx get api/openai",
		"kdbx list",
		"kdbx check",
		"kdbx envs",
		"kdbx init",
		"echo kdbx set api/openai",
		"npm test",
		"",
	} {
		if got := Decide(cmd); got != "" {
			t.Errorf("Decide(%q) denied an allowed command: %s", cmd, got)
		}
	}
}

func TestDecideBlocksNonKdbxToolsTouchingVaultFiles(t *testing.T) {
	for _, cmd := range []string{
		"cat ~/.config/keepassxc/demo/dev.kdbx",
		"xxd dev.keyx",
		"cp dev.KDBX /tmp/",
		"base64 $(cat dev.keyx)",
	} {
		if got := Decide(cmd); got == "" {
			t.Errorf("Decide(%q) allowed a vault read via a non-kdbx tool", cmd)
		} else if !strings.Contains(got, "leak-guard") {
			t.Errorf("Decide(%q) = %q, want the leak-guard wording", cmd, got)
		}
	}
}

func TestDecideAllowsKdbxAndKeepassxcCliTouchingVaultFiles(t *testing.T) {
	for _, cmd := range []string{
		"kdbx run -- printenv",
		"keepassxc-cli ls --no-password -k dev.keyx dev.kdbx",
		"uv run kdbx.py list",
	} {
		if got := Decide(cmd); got != "" {
			t.Errorf("Decide(%q) denied an allowed invoker: %s", cmd, got)
		}
	}
}

func TestDecideInspectsEveryShellSegment(t *testing.T) {
	if Decide("npm test && cat dev.kdbx") == "" {
		t.Fatal("must inspect commands after &&")
	}
	if Decide("echo hi; kdbx set api/x") == "" {
		t.Fatal("must inspect commands after ;")
	}
	if Decide("kdbx list | grep api") != "" {
		t.Fatal("a piped allowed command must stay allowed")
	}
}

func TestRunEmitsDenyEnvelopeAndAlwaysExitsZero(t *testing.T) {
	var out bytes.Buffer
	code := Run(strings.NewReader(`{"tool_input":{"command":"kdbx set api/x"}}`), &out)
	if code != 0 {
		t.Fatalf("exit %d, want 0 (the guard must never break the shell)", code)
	}
	s := out.String()
	for _, want := range []string{`"hookEventName":"PreToolUse"`, `"permissionDecision":"deny"`, "human-only operation"} {
		if !strings.Contains(s, want) {
			t.Fatalf("envelope %q missing %q", s, want)
		}
	}
}

func TestRunAllowsSilently(t *testing.T) {
	var out bytes.Buffer
	if code := Run(strings.NewReader(`{"tool_input":{"command":"kdbx list"}}`), &out); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Fatalf("allow must produce no output, got %q", out.String())
	}
}

func TestRunFailsOpenOnGarbageInput(t *testing.T) {
	var out bytes.Buffer
	if code := Run(strings.NewReader("not json at all"), &out); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Fatalf("garbage input must fail open silently, got %q", out.String())
	}
}
