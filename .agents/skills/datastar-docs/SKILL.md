---
name: datastar-docs
description: "Use this skill whenever exact current Datastar documentation matters: data-* attributes, @actions, modifiers, signals, backend requests, SSE events, Rocket, SDKs, security, or implementation guidance from data-star.dev. Prefer this skill over memory when writing, reviewing, or debugging Datastar syntax or behavior."
---

# Datastar Docs

Use this skill as a compact entry point to the official Datastar documentation. Fetch only the current article or search result needed for the task.

The docs change over time, so this skill does not vendor a generated markdown snapshot. Use `scripts/datastar_docs.py` to fetch `https://data-star.dev/docs.md` on demand and slice it by top-level `# H1` sections.

## Commands

Resolve the skill directory first, then run the helper:

```bash
python3 <skill-dir>/scripts/datastar_docs.py index
python3 <skill-dir>/scripts/datastar_docs.py article "Attributes"
python3 <skill-dir>/scripts/datastar_docs.py search "data-indicator"
python3 <skill-dir>/scripts/datastar_docs.py split --out-dir /tmp/datastar-docs
python3 <skill-dir>/scripts/datastar_docs.py download
python3 <skill-dir>/scripts/datastar_docs.py raw
```

Command behavior:

- `index`: list the current top-level articles.
- `article <query>`: print one article by title, slug, filename, or unique partial match.
- `search <query>`: search current docs without loading the full source into context.
- `split`: write current articles to a temp directory or explicit output directory when file-based inspection is useful.
- `download`: write `docs.md`, `index.md`, and split H1 article files to `./.datastar-docs` in the current working directory for `rg`, list, and read workflows. Use `--out-dir` to choose another location.
- `raw`: print the full current upstream markdown only when the task genuinely needs all docs.

Options:

- `--cache-seconds <n>`: reuse a local temp-cache copy for repeated lookups during one task. Default is `0`, which fetches live docs every run.
- `--source-url <url>`: override the markdown source for testing or pinned review.

## Working Guidance

- Prefer the live helper over memory when exact Datastar syntax matters.
- For Go UI work, combine the relevant Datastar docs article with the local gomponents and UI patterns in the target repo.
- When implementing backend-driven UI updates, fetch the backend request, SSE, and attribute reference articles before changing server responses.
- When reviewing an unfamiliar `data-*` attribute or `@action()`, fetch that specific current article or search result rather than loading the full docs.
- When broad local inspection is useful, run `download` from the target project and then use normal file tools such as `rg`, `sed`, or `find` against the printed directory. Treat `./.datastar-docs` as generated scratch documentation, not source to commit.
- When docs output will be quoted in an answer, keep quotes short and summarize the rest.
