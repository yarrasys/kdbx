// Package pointer discovers, reads, and rewrites the committed .keepassxc.json
// pointer file (spec C1).
package pointer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/ojson"
	"github.com/yarrasys/kdbx/internal/paths"
)

// Name is the pointer file's fixed name.
const Name = ".keepassxc.json"

// Pointer is a loaded pointer file that can be edited and saved without
// disturbing key order or formatting.
type Pointer struct {
	Path string
	root *ojson.Object
}

// EnvPaths are the resolved artifacts for one environment.
type EnvPaths struct {
	Vault    string
	KeyFile  string
	Vars     map[string]string
	VarOrder []string
	// Allow is the env's `run.allow` command list. AllowSet distinguishes an
	// absent list (no restriction) from a present-but-empty one (nothing may
	// run without --any).
	Allow    []string
	AllowSet bool
}

// Find walks up from startDir looking for the pointer file.
//
// startDir is symlink-resolved first, matching Python's find_pointer. This is
// load-bearing, not cosmetic: Project() falls back to the pointer directory's
// basename, so reaching the same repo through a symlink would otherwise yield a
// different project name here than in Python — and therefore a different
// default vault path, making a user's secrets look like they had vanished.
func Find(startDir string) (string, error) {
	dir := paths.Resolve(startDir)
	for {
		cand := filepath.Join(dir, Name)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", kdbxerr.NotFound("no .keepassxc.json found (run from inside a configured repo)")
		}
		dir = parent
	}
}

// Load reads and parses the pointer file at path.
func Load(path string) (*Pointer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, kdbxerr.Wrap(err, "NotFound", 2, "reading %s", filepath.Base(path))
	}
	root, err := ojson.Parse(b)
	if err != nil {
		return nil, kdbxerr.Wrap(err, "Preflight", 7, "%s is not valid JSON", filepath.Base(path))
	}
	return &Pointer{Path: path, root: root}, nil
}

// Dir is the directory holding the pointer file (the project root).
func (p *Pointer) Dir() string { return filepath.Dir(p.Path) }

// Project is the pointer's project name, defaulting to the pointer directory's name.
func (p *Pointer) Project() string {
	if v := p.root.Str("project"); v != "" {
		return v
	}
	return filepath.Base(p.Dir())
}

// SelectEnv resolves the active environment and where the choice came from.
func (p *Pointer) SelectEnv(cliEnv string) (name, source string) {
	if cliEnv != "" {
		return cliEnv, "--env"
	}
	if v := os.Getenv("KDBX_ENV"); v != "" {
		return v, "$KDBX_ENV"
	}
	if v := p.root.Str("defaultEnv"); v != "" {
		return v, "pointer"
	}
	return "dev", "pointer"
}

// EnvNames lists configured environments in file order.
func (p *Pointer) EnvNames() []string {
	envs := p.root.Obj("envs")
	if envs == nil {
		return nil
	}
	return envs.Keys()
}

// ResolveEnv returns the artifact paths and var mappings for env.
func (p *Pointer) ResolveEnv(env string) (EnvPaths, error) {
	envs := p.root.Obj("envs")
	if envs == nil {
		return EnvPaths{}, kdbxerr.NotFound("env '%s' not configured in pointer", env)
	}
	cfg := envs.Obj(env)
	if cfg == nil {
		found := false
		for _, k := range envs.Keys() {
			if k == env {
				found = true
			}
		}
		if !found {
			return EnvPaths{}, kdbxerr.NotFound("env '%s' not configured in pointer", env)
		}
		cfg = ojson.New()
	}

	defaultDir := filepath.Join(paths.KeepassxcDir(), p.Project())
	vault, err := resolveArtifact(cfg.Str("vault"), filepath.Join(defaultDir, env+".kdbx"))
	if err != nil {
		return EnvPaths{}, err
	}
	keyfile, err := resolveArtifact(cfg.Str("keyFile"), filepath.Join(defaultDir, env+".keyx"))
	if err != nil {
		return EnvPaths{}, err
	}

	out := EnvPaths{Vault: vault, KeyFile: keyfile, Vars: map[string]string{}}
	if vars := cfg.Obj("vars"); vars != nil {
		for _, k := range vars.Keys() {
			out.Vars[k] = vars.Str(k)
			out.VarOrder = append(out.VarOrder, k)
		}
	}
	if runCfg := cfg.Obj("run"); runCfg.Has("allow") {
		allow, ok := runCfg.Strs("allow")
		if !ok {
			// A present-but-unreadable allowlist fails closed: silently
			// treating it as "no restriction" would fail open on a security
			// setting.
			return EnvPaths{}, kdbxerr.Preflight(
				"env '%s': run.allow must be an array of strings", env)
		}
		out.Allow, out.AllowSet = allow, true
	}
	return out, nil
}

// resolveArtifact picks the configured path or the per-project default. Both
// branches are symlink-resolved, matching Python, which applies .resolve() to
// the default branch as well as to expand_path().
func resolveArtifact(configured, fallback string) (string, error) {
	if configured == "" {
		return paths.Resolve(fallback), nil
	}
	return paths.Expand(configured)
}

// SetVar records varName -> entryPath under env, creating levels as needed.
func (p *Pointer) SetVar(env, varName, entryPath string) {
	p.root.EnsureObj("envs").EnsureObj(env).EnsureObj("vars").SetString(varName, entryPath)
}

// RepointVars rewrites env's mappings that point at srcEntry so they point at
// dstEntry, preserving each mapping's ":field" suffix. Returns how many changed.
func (p *Pointer) RepointVars(env, srcEntry, dstEntry string) int {
	envs := p.root.Obj("envs")
	if envs == nil {
		return 0
	}
	cfg := envs.Obj(env)
	if cfg == nil {
		return 0
	}
	vars := cfg.Obj("vars")
	if vars == nil {
		return 0
	}
	changed := 0
	for _, k := range vars.Keys() {
		val := vars.Str(k)
		if EntryOf(val) != srcEntry {
			continue
		}
		vars.SetString(k, dstEntry+strings.TrimPrefix(val, srcEntry))
		changed++
	}
	return changed
}

// Save atomically rewrites the pointer file with 2-space indentation.
func (p *Pointer) Save() error {
	data, err := p.root.Indent()
	if err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "rendering %s", Name)
	}
	tmp := p.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "writing %s", Name)
	}
	if err := os.Rename(tmp, p.Path); err != nil {
		_ = os.Remove(tmp)
		return kdbxerr.Wrap(err, "Runtime", 1, "replacing %s", Name)
	}
	return nil
}
