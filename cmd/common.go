package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/vault"
)

// mustContext resolves the active environment, optionally announcing it.
func mustContext(c *cobra.Command, banner bool) (*envctx.Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ctx, err := envctx.Resolve(opts.env, cwd)
	if err != nil {
		return nil, err
	}
	if banner && !opts.json {
		ctx.WriteBanner(c.ErrOrStderr())
	}
	return ctx, nil
}

// openVault opens the active environment's vault for reading.
func openVault(ctx *envctx.Context) (*vault.Handle, error) {
	return vault.Open(ctx.Vault, ctx.KeyFile)
}
