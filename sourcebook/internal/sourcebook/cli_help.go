package sourcebook

const cliHelp = `Build one Codex skill from Git repositories and documentation sources

Usage:
  sourcebook [flags]
  sourcebook [command]

Available Commands:
  add         Add a Git repository or catalogue source
  help        Help about any command
  list        List sources
  remove      Remove a source
  update      Refresh named sources or all sources
  upgrade     Upgrade Sourcebook to the latest release
  version     Print the version

Flags:
  -h, --help      help for sourcebook
  -v, --version   version for sourcebook

Use "sourcebook [command] --help" for more information about a command.`

// CLIHelp returns the root command help shown by sourcebook --help.
func CLIHelp() string {
	return cliHelp
}
