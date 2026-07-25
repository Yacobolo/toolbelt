package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/catalog"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/ui"
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
		if !ui.WasReported(err) {
			fmt.Fprintf(os.Stderr, "sourcebook: %v\n", err)
		}
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

type upgradeNotifier interface {
	Notice(context.Context, string) (string, error)
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
			if !ui.IsInteractive(command.InOrStdin(), command.OutOrStdout()) {
				return command.Help()
			}
			sources, err := app.Sources()
			if err != nil {
				return err
			}
			return ui.RunDashboard(
				command.Context(),
				command.InOrStdin(),
				command.OutOrStdout(),
				buildVersion,
				app.SkillDir(),
				sources,
				ui.DashboardActions{
					Catalog: app.CatalogEntries(),
					Reload:  app.Sources,
					Update: func(ctx context.Context, names []string) error {
						return app.UpdateSelectedWithProgress(ctx, names, nil)
					},
					Remove: app.Remove,
					AddPreset: func(ctx context.Context, presetID string) error {
						return app.AddPreset(ctx, presetID, nil)
					},
					AddRepository: app.Add,
				},
			)
		},
	}
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetVersionTemplate("sourcebook {{.Version}}\n")
	root.CompletionOptions.DisableDefaultCmd = true
	var upgrader upgradeRunner
	if len(upgradeRunners) > 0 {
		upgrader = upgradeRunners[0]
	}
	root.PersistentPostRunE = func(command *cobra.Command, _ []string) error {
		if command == root || command.Name() == "upgrade" {
			return nil
		}
		notifier, ok := upgrader.(upgradeNotifier)
		if !ok {
			return nil
		}
		ctx, cancel := context.WithTimeout(command.Context(), 2*time.Second)
		defer cancel()
		notice, err := notifier.Notice(ctx, buildVersion)
		if err != nil || notice == "" {
			return nil
		}
		_, _ = fmt.Fprintln(command.ErrOrStderr(), notice)
		return nil
	}

	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(command *cobra.Command, args []string) {
		if command == root {
			fmt.Fprint(command.OutOrStdout(), ui.RenderHelp(ui.ColorEnabled(command.OutOrStdout())))
			return
		}
		defaultHelp(command, args)
	})

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
			var result upgrade.Result
			err := ui.RunAction(
				command.Context(),
				command.InOrStdin(),
				command.OutOrStdout(),
				ui.IsInteractive(command.InOrStdin(), command.OutOrStdout()),
				ui.Action{
					Working: "Checking for Sourcebook updates",
					Success: "Sourcebook update check complete",
					Run: func(ctx context.Context) error {
						var err error
						result, err = upgrader.Run(ctx, buildVersion, checkOnly)
						return err
					},
				},
			)
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
			interactive := ui.IsInteractive(command.InOrStdin(), command.OutOrStdout())
			if len(args) == 1 && presetID != "" {
				return errors.New("repository URL and --preset cannot be used together")
			}
			if len(args) == 1 {
				return ui.RunAction(command.Context(), command.InOrStdin(), command.OutOrStdout(), interactive, ui.Action{
					Working: "Cloning repository",
					Success: "Git repository added to Sourcebook",
					Run: func(ctx context.Context) error {
						return app.Add(ctx, args[0])
					},
				})
			}
			if presetID == "" {
				if !interactive {
					return errors.New("repository URL or --preset is required when not running interactively")
				}
				entries := app.CatalogEntries()
				sources, err := app.Sources()
				if err != nil {
					return err
				}
				var selected bool
				presetID, selected, err = ui.SelectPreset(
					command.Context(),
					command.InOrStdin(),
					command.OutOrStdout(),
					entries,
					sources,
				)
				if err != nil {
					return err
				}
				if !selected {
					return nil
				}
				if presetID == ui.GitRepositorySelection {
					repositoryURL, submitted, err := ui.InputRepositoryURL(
						command.Context(),
						command.InOrStdin(),
						command.OutOrStdout(),
					)
					if err != nil {
						return err
					}
					if !submitted {
						return nil
					}
					return ui.RunAction(
						command.Context(),
						command.InOrStdin(),
						command.OutOrStdout(),
						interactive,
						ui.Action{
							Working: "Cloning repository",
							Success: "Git repository added to Sourcebook",
							Run: func(ctx context.Context) error {
								return app.Add(ctx, repositoryURL)
							},
						},
					)
				}
			}
			source := sourcebook.Source{Name: presetID}
			for _, entry := range app.CatalogEntries() {
				if entry.ID == presetID {
					source.Name = entry.SourceName
					source.Provider = entry.Provider
					source.URL = entry.SourceURL
					source.Title = entry.DisplayName
					break
				}
			}
			return ui.RunSourceAdd(
				command.Context(),
				command.InOrStdin(),
				command.OutOrStdout(),
				interactive,
				source,
				func(ctx context.Context, report sourcebook.UpdateReporter) error {
					return app.AddPreset(ctx, presetID, report)
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
		Short: "Select and refresh sources",
		Args:  cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, args []string) error {
			interactive := ui.IsInteractive(command.InOrStdin(), command.OutOrStdout())
			if updateAll && len(args) > 0 {
				return errors.New("source names and --all cannot be used together")
			}
			if !updateAll && len(args) == 0 && !interactive {
				return errors.New("source names or --all are required when not running interactively")
			}
			sources, err := app.Sources()
			if err != nil {
				return err
			}
			if len(sources) == 0 {
				_, err := fmt.Fprintln(command.OutOrStdout(), "No sources to update.")
				return err
			}

			names := append([]string(nil), args...)
			if updateAll {
				names = make([]string, len(sources))
				for index, source := range sources {
					names[index] = source.Name
				}
			} else if len(names) == 0 {
				var confirmed bool
				names, confirmed, err = ui.SelectSourcesForUpdate(
					command.Context(),
					command.InOrStdin(),
					command.OutOrStdout(),
					sources,
				)
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}

			selected := make([]sourcebook.Source, 0, len(names))
			selectedNames := make(map[string]struct{}, len(names))
			for _, name := range names {
				selectedNames[name] = struct{}{}
			}
			for _, source := range sources {
				if _, exists := selectedNames[source.Name]; exists {
					selected = append(selected, source)
				}
			}
			return ui.RunUpdate(
				command.Context(),
				command.InOrStdin(),
				command.OutOrStdout(),
				interactive,
				selected,
				func(ctx context.Context, report sourcebook.UpdateReporter) error {
					return app.UpdateSelectedWithProgress(ctx, names, report)
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
	command := &cobra.Command{
		Use:   "remove [name]",
		Short: "Select and remove a source",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			interactive := ui.IsInteractive(command.InOrStdin(), command.OutOrStdout())
			name := ""
			var selectedSource sourcebook.Source
			if len(args) == 1 {
				name = args[0]
			} else {
				if !interactive {
					return fmt.Errorf("source name is required when not running interactively; use sourcebook remove <name>")
				}
				sources, err := app.Sources()
				if err != nil {
					return err
				}
				if len(sources) == 0 {
					_, err := fmt.Fprintln(command.OutOrStdout(), "No sources to remove.")
					return err
				}
				var selected bool
				name, selected, err = ui.SelectSource(command.Context(), command.InOrStdin(), command.OutOrStdout(), sources)
				if err != nil {
					return err
				}
				if !selected {
					return nil
				}
				for _, source := range sources {
					if source.Name == name {
						selectedSource = source
						break
					}
				}
				confirmed, err := ui.ConfirmRemoval(
					command.Context(),
					command.InOrStdin(),
					command.OutOrStdout(),
					selectedSource,
				)
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}
			return ui.RunAction(command.Context(), command.InOrStdin(), command.OutOrStdout(), interactive, ui.Action{
				Working: "Removing " + name,
				Success: name + " removed",
				Run: func(context.Context) error {
					return app.Remove(name)
				},
			})
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
	return command
}

func newListCommand(app *sourcebook.App) *cobra.Command {
	command := &cobra.Command{
		Use:               "list",
		Short:             "List sources",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, _ []string) error {
			output := command.OutOrStdout()
			if !ui.IsTerminal(output) {
				return app.List(output)
			}
			sources, err := app.Sources()
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(output, ui.RenderSources(
				sources,
				ui.ColorEnabled(output),
				ui.TerminalWidth(output),
			))
			return err
		},
	}
	return command
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
