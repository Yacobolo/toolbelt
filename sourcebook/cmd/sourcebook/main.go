package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/catalog"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/upgrade"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sourcebook: find home directory: %v\n", err)
		os.Exit(1)
	}
	app := sourcebook.New(
		sourcebook.DefaultSkillDir(homeDir, os.Getenv("CODEX_HOME")),
		sourcebook.GitCloner{Stdin: os.Stdin},
	)
	if err := registerBuiltinCatalogue(app); err != nil {
		fmt.Fprintf(os.Stderr, "sourcebook: register built-in catalogue: %v\n", err)
		os.Exit(1)
	}
	upgrader, err := upgrade.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sourcebook: configure updater: %v\n", err)
		os.Exit(1)
	}
	moduleVersion := ""
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = buildInfo.Main.Version
	}
	command := newRootCommand(app, os.Stdin, os.Stdout, os.Stderr, resolveVersion(version, moduleVersion), upgrader)
	command.SetArgs(os.Args[1:])
	if err := command.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "sourcebook: %v\n", err)
		os.Exit(1)
	}
}

func registerBuiltinCatalogue(app *sourcebook.App) error {
	return catalog.Register(app)
}

func resolveVersion(injected, moduleVersion string) string {
	if injected != "" && injected != "dev" {
		return injected
	}
	if moduleVersion != "" && moduleVersion != "(devel)" {
		return moduleVersion
	}
	return "dev"
}

type upgradeRunner interface {
	Run(context.Context, string, bool) (upgrade.Result, error)
}

func newRootCommand(app *sourcebook.App, stdin io.Reader, stdout, stderr io.Writer, buildVersion string, upgradeRunners ...upgradeRunner) *cobra.Command {
	root := &cobra.Command{
		Use:           "sourcebook",
		Short:         "Build one Codex skill from Git repositories and documentation sources",
		Version:       buildVersion,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetVersionTemplate("sourcebook {{.Version}}\n")
	root.CompletionOptions.DisableDefaultCmd = true
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(command *cobra.Command, args []string) {
		if command == root {
			_, _ = fmt.Fprintln(command.OutOrStdout(), sourcebook.CLIHelp())
			return
		}
		defaultHelp(command, args)
	})

	var upgrader upgradeRunner
	if len(upgradeRunners) > 0 {
		upgrader = upgradeRunners[0]
	}
	root.AddCommand(
		newAddCommand(app),
		newUpdateCommand(app),
		newRemoveCommand(app),
		newListCommand(app),
		newUpgradeCommand(upgrader, buildVersion),
		newVersionCommand(buildVersion),
	)
	return root
}

func runPlainAction(ctx context.Context, output io.Writer, working, success string, run func(context.Context) error) error {
	if _, err := fmt.Fprintf(output, "%s...\n", working); err != nil {
		return err
	}
	if err := run(ctx); err != nil {
		return err
	}
	_, err := fmt.Fprintf(output, "%s.\n", success)
	return err
}

func newUpgradeCommand(upgrader upgradeRunner, buildVersion string) *cobra.Command {
	var checkOnly bool
	command := &cobra.Command{
		Use:               "upgrade",
		Short:             "Upgrade Sourcebook to the latest release",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, _ []string) error {
			if upgrader == nil {
				return errors.New("Sourcebook updater is not configured")
			}
			if _, err := fmt.Fprintln(command.OutOrStdout(), "Checking for Sourcebook updates..."); err != nil {
				return err
			}
			result, err := upgrader.Run(command.Context(), buildVersion, checkOnly)
			if err != nil {
				return err
			}
			switch {
			case result.Updated:
				_, err = fmt.Fprintf(
					command.OutOrStdout(),
					"Sourcebook upgraded from %s to %s.\n",
					result.CurrentVersion,
					result.LatestVersion,
				)
			case result.UpdateAvailable:
				_, err = fmt.Fprintf(
					command.OutOrStdout(),
					"Sourcebook %s is available; run sourcebook upgrade to install it.\n",
					result.LatestVersion,
				)
			default:
				_, err = fmt.Fprintf(
					command.OutOrStdout(),
					"Sourcebook is already up to date (%s).\n",
					result.CurrentVersion,
				)
			}
			return err
		},
	}
	command.Flags().BoolVar(&checkOnly, "check", false, "check for an update without installing it")
	return command
}

func newAddCommand(app *sourcebook.App) *cobra.Command {
	var presetID string
	command := &cobra.Command{
		Use:               "add [repository-url]",
		Short:             "Add a Git repository or catalogue source",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 1 && presetID != "" {
				return errors.New("repository URL and --preset cannot be used together")
			}
			if len(args) == 0 && presetID == "" {
				return errors.New("repository URL or --preset is required")
			}
			if len(args) == 1 {
				source, err := sourcebook.ResolveGitSource(args[0])
				if err != nil {
					return err
				}
				return runPlainAction(
					command.Context(),
					command.OutOrStdout(),
					"Adding "+source.Name,
					source.Name+" added to Sourcebook",
					func(ctx context.Context) error {
						return app.AddWithProgress(ctx, args[0], nil)
					},
				)
			}
			return runPlainAction(
				command.Context(),
				command.OutOrStdout(),
				"Adding "+presetID,
				presetID+" added to Sourcebook",
				func(ctx context.Context) error {
					return app.AddPreset(ctx, presetID, nil)
				},
			)
		},
	}
	command.Flags().StringVar(&presetID, "preset", "", "catalogue preset ID")
	if err := command.RegisterFlagCompletionFunc("preset", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		entries := app.CatalogEntries()
		ids := make([]string, 0, len(entries))
		for _, entry := range entries {
			ids = append(ids, entry.ID)
		}
		return ids, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}
	return command
}

func newUpdateCommand(app *sourcebook.App) *cobra.Command {
	var updateAll bool
	command := &cobra.Command{
		Use:   "update [source...]",
		Short: "Refresh named sources or all sources",
		Args:  cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if updateAll && len(args) > 0 {
				return errors.New("source names and --all cannot be used together")
			}
			if !updateAll && len(args) == 0 {
				return errors.New("source names or --all are required")
			}

			sources, err := app.Sources()
			if err != nil {
				return err
			}
			if len(sources) == 0 {
				_, err := fmt.Fprintln(command.OutOrStdout(), "No sources to update.")
				return err
			}

			if updateAll {
				return runPlainAction(
					command.Context(),
					command.OutOrStdout(),
					"Updating all sources",
					"All sources updated",
					func(ctx context.Context) error {
						return app.UpdateWithProgress(ctx, nil)
					},
				)
			}

			return runPlainAction(
				command.Context(),
				command.OutOrStdout(),
				"Updating "+strings.Join(args, ", "),
				"Selected sources updated",
				func(ctx context.Context) error {
					return app.UpdateSelectedWithProgress(ctx, args, nil)
				},
			)
		},
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			sources, err := app.Sources()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			names := make([]string, 0, len(sources))
			for _, source := range sources {
				names = append(names, source.Name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
	}
	command.Flags().BoolVar(&updateAll, "all", false, "refresh every source")
	return command
}

func newRemoveCommand(app *sourcebook.App) *cobra.Command {
	return &cobra.Command{
		Use:               "remove <name>",
		Short:             "Remove a source",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: sourceNameCompletion(app),
		RunE: func(command *cobra.Command, args []string) error {
			name := args[0]
			return runPlainAction(
				command.Context(),
				command.OutOrStdout(),
				"Removing "+name,
				name+" removed",
				func(context.Context) error {
					return app.Remove(name)
				},
			)
		},
	}
}

func sourceNameCompletion(app *sourcebook.App) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		sources, err := app.Sources()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		names := make([]string, 0, len(sources))
		for _, source := range sources {
			names = append(names, source.Name)
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

func newListCommand(app *sourcebook.App) *cobra.Command {
	return &cobra.Command{
		Use:               "list",
		Short:             "List sources",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, _ []string) error {
			return app.List(command.OutOrStdout())
		},
	}
}

func newVersionCommand(buildVersion string) *cobra.Command {
	return &cobra.Command{
		Use:               "version",
		Short:             "Print the version",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(command.OutOrStdout(), "sourcebook %s\n", buildVersion)
			return err
		},
	}
}
