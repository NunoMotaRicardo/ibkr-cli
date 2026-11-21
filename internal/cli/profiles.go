package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/secrets"
)

// profile returns the active profile name (default when none selected).
func (a *App) profile() string {
	if a.opts.profile == "" {
		return secrets.DefaultProfile
	}
	return a.opts.profile
}

func (a *App) isDefaultProfile() bool { return a.profile() == secrets.DefaultProfile }

// profileSuffix is " (profile paper)" for non-default profiles, else "".
func (a *App) profileSuffix() string {
	if a.isDefaultProfile() {
		return ""
	}
	return " (profile " + a.profile() + ")"
}

// connectHint is the command that (re)connects the active profile.
func (a *App) connectHint() string {
	if a.isDefaultProfile() {
		return "ibkr connect"
	}
	return "ibkr --profile " + a.profile() + " connect"
}

// cacheDir is the tool-cache directory for the active profile.
func (a *App) cacheDir() string {
	if a.isDefaultProfile() {
		return a.CacheDir
	}
	return filepath.Join(a.CacheDir, "profiles", a.profile())
}

// clientName is what IBKR shows on the consent page.
func (a *App) clientName() string {
	if a.isDefaultProfile() {
		return "ibkr-cli"
	}
	return "ibkr-cli (" + a.profile() + ")"
}

// exemptFromProfile lists commands that never touch credentials.
func exemptFromProfile(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "version", "help", "completion", "profiles", "__complete":
			return true
		}
	}
	return false
}

// selectProfile resolves --profile / IBKR_PROFILE and applies it to the store.
// Once more than one profile exists a profile must be named explicitly, so a
// command can never silently act on the wrong account.
func (a *App) selectProfile(cmd *cobra.Command) error {
	if a.opts.profile == "" {
		a.opts.profile = os.Getenv("IBKR_PROFILE")
	}
	if a.opts.profile != "" {
		if a.opts.profile != secrets.DefaultProfile {
			if err := secrets.ValidProfileName(a.opts.profile); err != nil {
				return usageErr("%v", err)
			}
		}
	}
	a.Store.Profile = a.profile()
	if a.opts.profile != "" || exemptFromProfile(cmd) || envTokenSet() {
		return nil
	}
	names, err := a.Store.Profiles()
	if err != nil {
		return failErr("cannot read the profile list: %v", err)
	}
	if len(names) > 1 {
		return usageErr("more than one profile exists (%s): pass --profile <name> or set IBKR_PROFILE.", strings.Join(names, ", "))
	}
	return nil
}

func (a *App) profilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "profiles",
		Short: "List credential profiles (one per IBKR login, e.g. live and paper)",
		Long: "Lists profiles and whether each is connected. Create one with `ibkr --profile <name> connect`;\n" +
			"select it per command with --profile or for a shell with IBKR_PROFILE. Once more than one profile\n" +
			"exists, every command needs one of those.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := a.Store.Profiles()
			if err != nil {
				return failErr("cannot read the profile list: %v", err)
			}
			if len(names) == 0 {
				fmt.Fprintln(a.Out, "No profiles. Run `ibkr connect` (default profile) or `ibkr --profile <name> connect`.")
				return nil
			}
			for _, n := range names {
				st := *a.Store
				st.Profile = n
				state, storage := "not connected", ""
				if c, src, err := st.Load(); err == nil && c != nil && c.HasToken() {
					state = "connected"
					storage = " (" + string(src) + ")"
				}
				fmt.Fprintf(a.Out, "%s: %s%s\n", n, state, storage)
			}
			return nil
		},
	}
}
