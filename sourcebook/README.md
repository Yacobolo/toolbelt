# Sourcebook

`sourcebook` keeps one Codex skill backed by shallow Git clones and curated
documentation sources. The skill is always named `sourcebook` and is stored
under `$CODEX_HOME/skills/sourcebook`. Sourcebook honors the same `CODEX_HOME`
as Codex on macOS, Linux, and Windows; when it is unset, the default is
`~/.codex`, so the skill is stored at `~/.codex/skills/sourcebook`.

## Install

Sourcebook requires `git` on `PATH`. Go is not required when using the release
installers.

### macOS and Linux

```sh
curl -fsSL https://raw.githubusercontent.com/Yacobolo/toolbelt/main/sourcebook/install.sh | sh
```

The installer selects the latest stable Sourcebook release, detects macOS or
Linux and amd64 or arm64, verifies the release checksum, and installs to
`~/.local/bin`. Pin a version or choose another directory with environment
variables:

```sh
curl -fsSL https://raw.githubusercontent.com/Yacobolo/toolbelt/main/sourcebook/install.sh |
  SOURCEBOOK_VERSION=0.1.0 SOURCEBOOK_INSTALL_DIR="$HOME/bin" sh
```

### Windows

Run in PowerShell:

```powershell
irm https://raw.githubusercontent.com/Yacobolo/toolbelt/main/sourcebook/install.ps1 | iex
```

The PowerShell installer selects the latest stable Sourcebook release, detects
amd64 or arm64, verifies the release checksum, installs under
`%LOCALAPPDATA%\Programs\OpenAI\Codex\bin`, matching the documented Codex
installer location, and adds that directory to the user `PATH`. Set
`$env:SOURCEBOOK_VERSION` or
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

## Upgrade Sourcebook

Update the CLI independently from its documentation references:

```sh
sourcebook upgrade
```

Sourcebook finds the latest compatible `sourcebook/v*` GitHub release, verifies
the downloaded archive against `checksums.txt`, and replaces the current
executable with rollback protection. This works for binaries installed by the
macOS/Linux installer, the Windows installer, or a release archive, provided
the installation directory is writable.

Check whether an update is available without installing it:

```sh
sourcebook upgrade --check
```

`sourcebook upgrade` updates the CLI. `sourcebook update` refreshes the
documentation sources in the generated skill.

Sourcebook also performs a best-effort update check after successful commands,
cached for 24 hours. When a newer release exists, it writes at most one short
notice per day to stderr; it never installs an update automatically. Failed or
slow checks are silently ignored, and redirected stdout remains unchanged.

Interactive terminals get an inline Bubble Tea spinner, styled completion
states, source pickers, and a compact source table. Redirected output stays plain;
`sourcebook list` remains tab-separated for use in scripts.

## Use

Add a repository:

```sh
sourcebook add https://github.com/example/project.git
```

The repository name is derived from the URL. Sourcebook creates this layout on
the first successful add:

```text
$CODEX_HOME/skills/sourcebook/
├── SKILL.md
├── sources.json
└── references/
    └── project/
```

Add documentation from the interactive built-in source list:

```sh
sourcebook add
```

The built-in catalogue currently contains:

- `azure-docs` — Microsoft Azure documentation from `articles/`
- `datastar-docs` — the official `data-star.dev/docs.md`, split into indexed Markdown articles
- `dbt-docs` — dbt documentation from `website/docs/` on the `current` branch
- `duckdb-docs` — DuckDB documentation from `docs/`
- `ducklake-docs` — DuckLake documentation and specification from `docs/`
- `netsuite-docs` — Oracle NetSuite online help, scraped into its documentation hierarchy
- `powerbi-docs` — the official
  [`MicrosoftDocs/powerbi-docs`](https://github.com/MicrosoftDocs/powerbi-docs)
  documentation from `powerbi-docs/`

For scripts and other non-interactive environments, select a preset explicitly:

```sh
sourcebook add --preset datastar-docs
sourcebook add --preset netsuite-docs
sourcebook add --preset powerbi-docs
sourcebook add --preset dbt-docs
sourcebook add --preset duckdb-docs
sourcebook add --preset ducklake-docs
sourcebook add --preset azure-docs
```

Generated documentation is stored under `references/datastar-docs` or
`references/netsuite-docs`. Git documentation presets are shallow,
blob-filtered sparse checkouts whose configured documentation root is flattened
directly into `references/<preset>/`. Text and code files are retained while
media and other binary assets are excluded. The temporary Git metadata and
unrelated repository content are discarded. Sourcebook ships the documentation
scrapers inside its Go binary, so no additional language runtime, nested skill,
or script is required.

Select sources to refresh interactively:

```sh
sourcebook update
```

Use Space to select one or more sources and Enter to update them. The first
choice selects every source. For scripts, pass source names or `--all`
explicitly:

```sh
sourcebook update dbt-docs powerbi-docs
sourcebook update --all
```

Selected updates run up to four source providers concurrently. Ad-hoc Git
sources receive new depth-one clones; rooted Git presets receive new depth-one
sparse checkouts. Datastar is rebuilt from the current official Markdown
source. NetSuite is rebuilt from its current table of contents with bounded,
rate-limited requests and retries. A selected set is installed only after every
source in that set succeeds, so a failed update leaves all existing references
unchanged. Unselected sources and their last-updated timestamps are untouched.
Successful updates also regenerate `SKILL.md` so existing installations adopt
the current metadata and table-of-contents format.

During an interactive update, Sourcebook shows every source as queued, updating,
completed, failed, or canceled. Scraped sources include page counts and the
current phase.

`sourcebook list` includes the provider and last successful update time for each
source.
Interactive tables use `YYYY-MM-DD HH:MM UTC`; redirected tab-separated output
uses RFC 3339 timestamps for scripts.

List sources or open the interactive removal picker:

```sh
sourcebook list
sourcebook remove
```

Use arrow keys or `j`/`k` to navigate, `/` to filter, Enter to remove the
selected source, and Escape or `q` to cancel. For scripts and other
non-interactive use, pass the source name directly:

```sh
sourcebook remove project
```

`SKILL.md` names the current sources in its trigger description and keeps its
body to a concise table of contents. `sources.json` records source names,
providers, URLs, and update times. Existing `repos.json` installations migrate
automatically after the next successful mutation.

Sourcebook prevents overlapping add, update, and remove operations. Every
source is staged before installation, and metadata files are written
atomically. A failed provider or write preserves the previous usable Sourcebook.

For private repositories, use SSH or a Git credential helper. Sourcebook
rejects credentials and tokens embedded in repository URLs so they cannot be
written to `sources.json` or `SKILL.md`.

## Develop

Sourcebook keeps retrieval mechanics and selectable content separate:

- Providers implement how a source is refreshed (`git`, `datastar`, or
  `netsuite`).
- Catalogue presets provide the stable ID, title, provider, destination name,
  source URL, optional Git ref, and optional flattened Git root shown by
  `sourcebook add`.

Most curated GitHub additions only need a catalogue entry routed to the
built-in `git` provider. Set `GitRef` when the documentation lives on a
non-default branch and `GitRoot` when only one repository directory should be
published into the reference root. Add a new scraper provider only when its
retrieval mechanics differ from Git.

```sh
go test -race ./...
go vet ./...
```

Inject a release version when building an artifact:

```sh
go build -ldflags "-X main.version=v0.1.0" -o sourcebook ./cmd/sourcebook
```

Stable releases are automated. After the intended commit is on `main`, push a
semantic Sourcebook tag:

```sh
git tag sourcebook/v0.2.0
git push origin sourcebook/v0.2.0
```

The Sourcebook release workflow runs the test suite, uses GoReleaser to build
macOS, Linux, and Windows archives for amd64 and arm64, generates
`checksums.txt`, and publishes the GitHub release consumed by the installers and
`sourcebook upgrade`.
