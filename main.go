// Command kdbx stores per-project, per-environment secrets in KeePassXC vaults
// and injects them into child processes without ever printing them.
package main

import (
	"os"

	"github.com/yarrasys/kdbx/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
