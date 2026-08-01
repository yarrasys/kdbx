// Package audit appends one line per `kdbx run` decision under a strict
// policy (spec N6). Prevention against an agent that has decided to read a
// value is out of scope by design; what this buys is detection: "did the
// agent touch anything?" becomes reviewable instead of unknowable.
//
// A line never carries a secret value. The API cannot express one: it takes
// the argv and the variable *names*.
package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Append records one decision. Fields are tab-separated: RFC3339 timestamp,
// decision ("run" / "refused"), the argv joined with spaces, and the injected
// variable names joined with commas. The file is created 0600 and only ever
// appended to.
func Append(path, decision string, argv, varNames []string) error {
	// The log lives next to the vault; a strict refusal can fire before the
	// vault (and so its directory) exists, and must still be recorded rather
	// than turning into a confusing write error.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	line := fmt.Sprintf("%s\t%s\t%s\t%s\n",
		time.Now().UTC().Format(time.RFC3339),
		decision,
		strings.Join(argv, " "),
		strings.Join(varNames, ","))
	_, err = f.WriteString(line)
	return err
}
