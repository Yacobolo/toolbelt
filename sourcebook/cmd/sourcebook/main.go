package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/ui"
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
	app := sourcebook.New(sourcebook.DefaultSkillDir(homeDir), sourcebook.GitCloner{Stdin: os.Stdin})
	moduleVersion := ""
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = buildInfo.Main.Version
	}
	command := newRootCommand(app, os.Stdin, os.Stdout, os.Stderr, resolveVersion(version, moduleVersion))
	command.SetArgs(os.Args[1:])
	if err := command.ExecuteContext(ctx); err != nil {
		if !ui.WasReported(err) {
			fmt.Fprintf(os.Stderr, "sourcebook: %v\n", err)
		}
		os.Exit(1)
	}
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

func newRootCommand(app *sourcebook.App, stdin io.Reader, stdout, stderr io.Writer, buildVersion string) *cobra.Command {
	root := &cobra.Command{
		Use:           "sourcebook",
		Short:         "Build one Codex skill from shallow-cloned repositories",
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
			fmt.Fprint(command.OutOrStdout(), ui.RenderHelp(ui.IsTerminal(command.OutOrStdout())))
			return
		}
		defaultHelp(command, args)
	})

	root.AddCommand(
		newAddCommand(app),
		newUpdateCommand(app),
		newRemoveCommand(app),
		newListCommand(app),
		newVersionCommand(buildVersion),
	)
	return root
}

func newAddCommand(app *sourcebook.App) *cobra.Command {
	command := &cobra.Command{
		Use:               "add <repository-url>",
		Short:             "Add a repository",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, args []string) error {
			return ui.RunAction(command.Context(), command.InOrStdin(), command.OutOrStdout(), ui.IsTerminal(command.OutOrStdout()), ui.Action{
				Working: "Cloning repository",
				Success: "Repository added to Sourcebook",
				Run: func(ctx context.Context) error {
					return app.Add(ctx, args[0])
				},
			})
		},
	}
	return command
}

func newUpdateCommand(app *sourcebook.App) *cobra.Command {
	command := &cobra.Command{
		Use:               "update",
		Short:             "Refresh all repositories",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, _ []string) error {
			repositories, err := app.Repositories()
			if err != nil {
				return err
			}
			return ui.RunUpdate(command.Context(), command.InOrStdin(), command.OutOrStdout(), ui.IsTerminal(command.OutOrStdout()), repositories, app.UpdateWithProgress)
		},
	}
	return command
}

func newRemoveCommand(app *sourcebook.App) *cobra.Command {
	command := &cobra.Command{
		Use:   "remove [name]",
		Short: "Select and remove a repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			interactive := ui.IsTerminal(command.OutOrStdout())
			name := ""
			if len(args) == 1 {
				name = args[0]
			} else {
				if !interactive {
					return fmt.Errorf("repository name is required when not running interactively; use sourcebook remove <name>")
				}
				repositories, err := app.Repositories()
				if err != nil {
					return err
				}
				if len(repositories) == 0 {
					_, err := fmt.Fprintln(command.OutOrStdout(), "No repositories to remove.")
					return err
				}
				var selected bool
				name, selected, err = ui.SelectRepository(command.Context(), command.InOrStdin(), command.OutOrStdout(), repositories)
				if err != nil {
					return err
				}
				if !selected {
					return nil
				}
			}
			return ui.RunAction(command.Context(), command.InOrStdin(), command.OutOrStdout(), ui.IsTerminal(command.OutOrStdout()), ui.Action{
				Working: "Removing " + name,
				Success: name + " removed",
				Run: func(context.Context) error {
					return app.Remove(name)
				},
			})
		},
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			repositories, err := app.Repositories()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			names := make([]string, 0, len(repositories))
			for _, repository := range repositories {
				names = append(names, repository.Name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
	}
	return command
}

func newListCommand(app *sourcebook.App) *cobra.Command {
	command := &cobra.Command{
		Use:               "list",
		Short:             "List repositories",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, _ []string) error {
			output := command.OutOrStdout()
			if !ui.IsTerminal(output) {
				return app.List(output)
			}
			repositories, err := app.Repositories()
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(output, ui.RenderRepositories(repositories, true))
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
