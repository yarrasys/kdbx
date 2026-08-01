package runpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// ctxFor builds a resolved context from a pointer body, without a vault: Gate
// runs entirely pre-vault.
func ctxFor(t *testing.T, body string) *envctx.Context {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KEEPASSXC_DIR", filepath.Join(dir, "kpxc"))
	t.Setenv("KDBX_ENV", "")
	if err := os.WriteFile(filepath.Join(dir, ".keepassxc.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := envctx.Resolve("", dir)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

const strictBody = `{"project":"demo","envs":{"dev":{` +
	`"policy":{"mode":"strict"},"run":{"allow":["npm test"]}}}}`

func TestGateStandardNoListAllowsAnything(t *testing.T) {
	ctx := ctxFor(t, `{"project":"demo","envs":{"dev":{}}}`)
	anchor, err := Gate(ctx, []string{"env"}, Flags{})
	if err != nil || anchor != nil {
		t.Fatalf("anchor=%v err=%v, want nil/nil", anchor, err)
	}
}

func TestGateStrictRefusesEscapeHatchesAndMissingList(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		f    Flags
	}{
		{"no-mask", strictBody, Flags{NoMask: true}},
		{"any", strictBody, Flags{Any: true}},
		{"missing list", `{"project":"demo","envs":{"dev":{"policy":{"mode":"strict"}}}}`, Flags{}},
	} {
		ctx := ctxFor(t, tc.body)
		_, err := Gate(ctx, []string{"npm", "test"}, tc.f)
		if kdbxerr.KindOf(err) != "NotAllowed" {
			t.Errorf("%s: err=%v, want NotAllowed", tc.name, err)
		}
	}
}

func TestGateStrictAllowedCommandReturnsAnchor(t *testing.T) {
	ctx := ctxFor(t, strictBody)
	anchor, err := Gate(ctx, []string{"npm", "test"}, Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if anchor == nil || anchor.Key != "kdbx:policy:dev" || anchor.Want == "" {
		t.Fatalf("anchor %+v", anchor)
	}
}

func TestGateStrictAuditsARefusal(t *testing.T) {
	ctx := ctxFor(t, strictBody)
	if _, err := Gate(ctx, []string{"env"}, Flags{}); kdbxerr.KindOf(err) != "NotAllowed" {
		t.Fatalf("err=%v", err)
	}
	b, err := os.ReadFile(AuditPath(ctx))
	if err != nil {
		t.Fatalf("no audit line written on refusal: %v", err)
	}
	if !strings.Contains(string(b), "\trefused\tenv") {
		t.Fatalf("audit line %q", b)
	}
}

func TestGateStandardWritesNoAudit(t *testing.T) {
	ctx := ctxFor(t, `{"project":"demo","envs":{"dev":{"run":{"allow":[]}}}}`)
	if _, err := Gate(ctx, []string{"env"}, Flags{}); kdbxerr.KindOf(err) != "NotAllowed" {
		t.Fatalf("err=%v", err)
	}
	if _, err := os.Stat(AuditPath(ctx)); !os.IsNotExist(err) {
		t.Fatal("standard mode wrote an audit file")
	}
}
