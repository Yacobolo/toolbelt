#!/usr/bin/env python3
"""Fetch and slice the live Datastar docs markdown."""

from __future__ import annotations

import argparse
import hashlib
import os
import re
import sys
import tempfile
import time
import urllib.request
from dataclasses import dataclass
from pathlib import Path


SOURCE_URL = "https://data-star.dev/docs.md"
CACHE_DIR = Path(tempfile.gettempdir()) / "datastar-docs-skill"
DOWNLOAD_DIRNAME = ".datastar-docs"


@dataclass(frozen=True)
class Article:
    title: str
    body: str

    @property
    def slug(self) -> str:
        slug = self.title.lower()
        slug = slug.replace("&", " and ")
        slug = re.sub(r"[^a-z0-9]+", "-", slug)
        return slug.strip("-") or "article"


def cache_path(url: str) -> Path:
    digest = hashlib.sha256(url.encode("utf-8")).hexdigest()[:16]
    return CACHE_DIR / f"{digest}.md"


def fetch_markdown(url: str, cache_seconds: int = 0) -> str:
    if cache_seconds < 0:
        raise SystemExit("--cache-seconds must be 0 or greater.")

    path = cache_path(url)
    if cache_seconds > 0 and path.exists():
        age = time.time() - path.stat().st_mtime
        if age <= cache_seconds:
            return path.read_text(encoding="utf-8")

    request = urllib.request.Request(url, headers={"User-Agent": "datastar-docs-skill/1.0"})
    with urllib.request.urlopen(request, timeout=30) as response:
        charset = response.headers.get_content_charset() or "utf-8"
        markdown = response.read().decode(charset)

    if cache_seconds > 0:
        CACHE_DIR.mkdir(parents=True, exist_ok=True)
        tmp_path = path.with_suffix(f".{os.getpid()}.tmp")
        tmp_path.write_text(markdown, encoding="utf-8")
        tmp_path.replace(path)

    return markdown


def split_articles(markdown: str) -> list[Article]:
    lines = markdown.replace("\r\n", "\n").replace("\r", "\n").splitlines()
    articles: list[Article] = []
    current_title: str | None = None
    current_lines: list[str] = []
    preamble: list[str] = []
    fence_marker: str | None = None

    for line in lines:
        stripped = line.lstrip()
        if fence_marker is None:
            match = re.match(r"^(```+|~~~+)", stripped)
            if match:
                fence_marker = match.group(1)
        elif stripped.startswith(fence_marker):
            fence_marker = None

        heading = None
        if fence_marker is None:
            match = re.match(r"^# ([^#].*?)\s*$", line)
            if match:
                heading = match.group(1).strip()

        if heading is not None:
            if current_title is not None:
                articles.append(Article(current_title, "\n".join(current_lines).strip() + "\n"))
            current_title = heading
            current_lines = [line]
            if not articles and preamble:
                current_lines = [*preamble, "", line]
            continue

        if current_title is None:
            if line.strip():
                preamble.append(line)
            elif preamble:
                preamble.append(line)
        else:
            current_lines.append(line)

    if current_title is not None:
        articles.append(Article(current_title, "\n".join(current_lines).strip() + "\n"))

    if not articles:
        raise SystemExit("No top-level H1 headings found in the Datastar docs source.")
    return articles


def article_filenames(articles: list[Article]) -> list[str]:
    seen: dict[str, int] = {}
    filenames: list[str] = []

    for idx, article in enumerate(articles, start=1):
        base_slug = article.slug
        count = seen.get(base_slug, 0) + 1
        seen[base_slug] = count
        slug = base_slug if count == 1 else f"{base_slug}-{count}"
        filenames.append(f"{idx:02d}-{slug}.md")

    return filenames


def render_article(article: Article, source_url: str) -> str:
    return (
        "---\n"
        f"title: {article.title}\n"
        f"source: {source_url}\n"
        "fetched: live\n"
        "---\n\n"
        f"{article.body.strip()}\n"
    )


def load_markdown(source_url: str, cache_seconds: int) -> str:
    return fetch_markdown(source_url, cache_seconds=cache_seconds)


def load_articles(source_url: str, cache_seconds: int) -> list[Article]:
    return split_articles(load_markdown(source_url, cache_seconds))


def print_index(articles: list[Article], markdown: bool) -> None:
    filenames = article_filenames(articles)
    for idx, (article, filename) in enumerate(zip(articles, filenames), start=1):
        if markdown:
            print(f"{idx:02d}. [{article.title}]({filename})")
        else:
            print(f"{idx:02d}\t{article.title}\t{filename}")


def find_article(articles: list[Article], query: str) -> Article:
    filenames = article_filenames(articles)
    wanted = normalize(query)
    matches: list[Article] = []

    for article, filename in zip(articles, filenames):
        candidates = {normalize(article.title), normalize(article.slug), normalize(filename)}
        if wanted in candidates:
            return article
        if wanted in normalize(article.title) or wanted in normalize(filename):
            matches.append(article)

    if len(matches) == 1:
        return matches[0]
    if matches:
        titles = "\n".join(f"- {article.title}" for article in matches)
        raise SystemExit(f"Article query matched multiple sections:\n{titles}")
    raise SystemExit(f"No Datastar article matched: {query}")


def normalize(value: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")


def write_split(
    articles: list[Article],
    source_url: str,
    out_dir: Path | None,
    *,
    markdown: str | None = None,
) -> Path:
    if out_dir is None:
        out_dir = Path(tempfile.mkdtemp(prefix="datastar-docs-"))
    out_dir.mkdir(parents=True, exist_ok=True)

    filenames = article_filenames(articles)
    index = ["# Datastar Docs Index", "", f"Source: {source_url}", ""]
    if markdown is not None:
        (out_dir / "docs.md").write_text(markdown, encoding="utf-8")
        index.extend(["Raw source: docs.md", ""])
    for idx, (article, filename) in enumerate(zip(articles, filenames), start=1):
        index.append(f"{idx:02d}. [{article.title}]({filename})")
        (out_dir / filename).write_text(render_article(article, source_url), encoding="utf-8")
    (out_dir / "index.md").write_text("\n".join(index) + "\n", encoding="utf-8")
    return out_dir


def search_articles(articles: list[Article], query: str, context: int) -> None:
    needle = query.lower()
    for article in articles:
        lines = article.body.splitlines()
        for line_no, line in enumerate(lines, start=1):
            if needle not in line.lower():
                continue
            start = max(1, line_no - context)
            end = min(len(lines), line_no + context)
            print(f"## {article.title}:{line_no}")
            for idx in range(start, end + 1):
                print(f"{idx}: {lines[idx - 1]}")
            print()


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-url", default=SOURCE_URL, help="Markdown URL to fetch.")
    parser.add_argument(
        "--cache-seconds",
        type=int,
        default=0,
        help="Reuse a temp-cache copy younger than this many seconds. Default: 0, always fetch live.",
    )
    subcommands = parser.add_subparsers(dest="command", required=True)

    index = subcommands.add_parser("index", help="List current top-level docs sections.")
    index.add_argument("--plain", action="store_true", help="Print tab-separated rows instead of Markdown.")

    article = subcommands.add_parser("article", help="Print one current H1 article by title, slug, or filename.")
    article.add_argument("query", help="Article title, slug, filename, or unique partial match.")

    split = subcommands.add_parser("split", help="Write current H1 articles to files.")
    split.add_argument("--out-dir", type=Path, help="Output directory. Defaults to a new temp directory.")

    download = subcommands.add_parser(
        "download",
        help="Download current docs into a persistent directory for rg/read/list workflows.",
    )
    download.add_argument(
        "--out-dir",
        type=Path,
        default=Path.cwd() / DOWNLOAD_DIRNAME,
        help=f"Output directory. Default: ./{DOWNLOAD_DIRNAME}",
    )

    search = subcommands.add_parser("search", help="Search current articles for text.")
    search.add_argument("query", help="Case-insensitive text query.")
    search.add_argument("--context", type=int, default=1, help="Context lines around each match.")

    subcommands.add_parser("raw", help="Print the full current upstream markdown.")

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    markdown = load_markdown(args.source_url, args.cache_seconds)

    if args.command == "raw":
        sys.stdout.write(markdown)
        return 0

    articles = split_articles(markdown)

    if args.command == "index":
        print_index(articles, markdown=not args.plain)
    elif args.command == "article":
        article = find_article(articles, args.query)
        sys.stdout.write(render_article(article, args.source_url))
    elif args.command == "split":
        out_dir = write_split(articles, args.source_url, args.out_dir)
        print(out_dir)
    elif args.command == "download":
        out_dir = write_split(articles, args.source_url, args.out_dir, markdown=markdown)
        print(out_dir)
    elif args.command == "search":
        search_articles(articles, args.query, args.context)
    else:
        parser.error(f"unknown command: {args.command}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
