# Sourcebook

`sourcebook` keeps one Codex skill backed by shallow Git clones. The skill is
always named `sourcebook` and is stored at `~/.codex/skills/sourcebook`.

## Install

Sourcebook requires `git` on `PATH`. Go is not required when using the release
installers.

### macOS and Linux

```sh
curl -fsSL https://raw.githubusercontent.com/Yacobolo/toolbelt/main/sourcebook/install.sh | sh
```

The installer detects macOS or Linux and amd64 or arm64, verifies the release
checksum, and installs to `~/.local/bin`. Pin a version or choose another
directory with environment variables:

```sh
curl -fsSL https://raw.githubusercontent.com/Yacobolo/toolbelt/main/sourcebook/install.sh |
  SOURCEBOOK_VERSION=0.1.0 SOURCEBOOK_INSTALL_DIR="$HOME/bin" sh
```

### Windows

Run in PowerShell:

```powershell
irm https://raw.githubusercontent.com/Yacobolo/toolbelt/main/sourcebook/install.ps1 | iex
```

The PowerShell installer detects amd64 or arm64, verifies the release checksum,
installs under `%LOCALAPPDATA%\Programs\Sourcebook`, and adds that directory to
the user `PATH`. Set `$env:SOURCEBOOK_VERSION` or
`$env:SOURCEBOOK_INSTALL_DIR` before running it to override the defaults.

### With Go

With Go 1.25 or newer, install directly from the module:

```sh
go install github.com/Yacobolo/toolbelt/sourcebook/cmd/sourcebook@latest
```

To install the current checkout while developing:

```sh
cd sourcebook
go install ./cmd/sourcebook
```

Run `sourcebook` with no arguments to see the command overview. Use
`sourcebook <command> --help` for command-specific help.

Interactive terminals get an inline Bubble Tea spinner, styled completion
states, and a compact repository table. Redirected output stays plain;
`sourcebook list` remains tab-separated for use in scripts.

## Use

Add a repository:

```sh
sourcebook add https://github.com/example/project.git
```

The repository name is derived from the URL. Sourcebook creates this layout on
the first successful add:

```text
~/.codex/skills/sourcebook/
├── SKILL.md
├── repos.json
└── references/
    └── project/
```

Refresh every repository with a new depth-one clone:

```sh
sourcebook update
```

Updates run up to four clones concurrently. An update is installed only after
every clone succeeds, so a failed update leaves all existing references
unchanged. It also regenerates `SKILL.md` so existing installations adopt the
current metadata and table-of-contents format.

During an interactive update, Sourcebook shows every repository as queued,
cloning, completed, failed, or canceled, together with repository counts and
elapsed times.

`sourcebook list` includes the last successful clone time for each repository.
Interactive tables use `YYYY-MM-DD HH:MM UTC`; redirected tab-separated output
uses RFC 3339 timestamps for scripts.

List repositories or open the interactive removal picker:

```sh
sourcebook list
sourcebook remove
```

Use arrow keys or `j`/`k` to navigate, `/` to filter, Enter to remove the
selected repository, and Escape or `q` to cancel. For scripts and other
non-interactive use, pass the repository name directly:

```sh
sourcebook remove project
```

`SKILL.md` names the current repositories in its trigger description and keeps
its body to a concise table of contents. `repos.json` records the repository
names and clone URLs.

Sourcebook prevents overlapping add, update, and remove operations. Repository
clones are staged before installation, and metadata files are written
atomically. A failed clone or write preserves the previous usable Sourcebook.

For private repositories, use SSH or a Git credential helper. Sourcebook
rejects credentials and tokens embedded in repository URLs so they cannot be
written to `repos.json` or `SKILL.md`.

## Develop

```sh
go test -race ./...
go vet ./...
```

Inject a release version when building an artifact:

```sh
go build -ldflags "-X main.version=v0.1.0" -o sourcebook ./cmd/sourcebook
```
