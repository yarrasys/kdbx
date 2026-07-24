package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionFlagPrintsVersion(t *testing.T) {
	Version = "1.2.3"
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "kdbx 1.2.3" {
		t.Fatalf("got %q, want %q", got, "kdbx 1.2.3")
	}
}

func TestUnknownCommandIsAnError(t *testing.T) {
	root := RootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"definitely-not-a-command"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected an error for an unknown subcommand")
	}
}
