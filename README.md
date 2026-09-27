# Hugo Scholar

[![CI](https://github.com/tommyreddad/hugo-scholar/actions/workflows/ci.yaml/badge.svg)](https://github.com/tommyreddad/hugo-scholar/actions/workflows/ci.yaml)
[![Last commit](https://img.shields.io/github/last-commit/tommyreddad/hugo-scholar)](https://github.com/tommyreddad/hugo-scholar/commits)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

BibTeX citations and bibliographies for Hugo. A Go command generates the data; Hugo shortcodes render it.

References use APA by default, formatted by [`citeproc`](https://github.com/jgm/citeproc). Use `--style basic` for formatting without `citeproc`. Hugo builds from the generated data and does not need `citeproc`.

## Try it

Requires Go 1.26+, Hugo, and `citeproc` for CSL styles.

```sh
cd example
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar --repository static/repository --details
hugo server
```

The example includes generated data, so `hugo server` also works on its own. To regenerate without `citeproc`, add `--style basic`.

Example: [my publications page](https://tommy.reddad.net/publications/).

## Add it to a site

Import the module in `hugo.toml`:

```toml
[[module.imports]]
path = 'github.com/tommyreddad/hugo-scholar'
```

Add it to the site's `go.mod`:

```go
module example.com/my-site

go 1.26

require github.com/tommyreddad/hugo-scholar v0.1.1
```

Put BibTeX in `_bibliography/references.bib` (or `.bibtex`). From the site root, run:

```sh
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar
hugo
```

The generator writes `data/scholar.json`. Commit it if deployment runs only Hugo. Rerun it after changing BibTeX, citations, styles, or generator options. `hugo server` does not run it automatically.

## Shortcodes

```text
This claim is supported {{< cite "go-book" >}}.
Several sources agree {{< cite keys="go-book paper" >}}.
See {{< cite keys="go-book" locator="42" label="page" >}}.

{{< bibliography >}}
{{< bibliography cited=true >}}
{{< bibliography file="books" query="@book[year>=2000]" >}}

Count: {{< bibliography_count type="book" >}}
Full reference: {{< reference "go-book" >}}
{{< quote "go-book" >}}A quoted passage.{{< /quote >}}
{{< nocite "go-book" >}}
```

`cite` takes one or more BibTeX keys. `cited=true` lists only cited entries; `cited_in_order=true` lists them in first citation order. `nocite` marks an entry as cited without printing a citation. Use `file="books"` for `_bibliography/books.bib`.

Bibliographies accept `sort_by`, `order`, `group_by`, `max`, `offset`, `remove_duplicates`, and `query`. CSL styles set the default order; `sort_by` and `cited_in_order` override it. Use `sort_by="none"` for BibTeX input order.

`offset` and `max` apply before grouping; `max=0` gives an empty list. Duplicate removal prefers cited entries and keeps anchors for all equivalent keys. Missing months are grouped as `(unknown)`. Nonnumeric values do not match numeric query comparisons.

CSL bibliographies use `ul` to avoid numbering citation labels twice. Basic bibliographies use `ol`. Override this with `params.scholar.bibliography_list_tag`. DOI and URL text links to valid HTTP(S) addresses supplied by the entry.

Use `separate_links=true` to link each key in a grouped citation. For multiple lists on a page, use matching `prefix` values on citations and bibliographies.

## Citation styles

Three CSL styles are included as plain files (about 148 KB total): `apa` (default), `ieee`, and `modern-language-association` (MLA). Style processing works offline once the generator and `citeproc` are installed.

Set your site's default in `hugo.toml`:

```toml
[params.scholar]
style = 'ieee'
```

`--style` overrides the site default:

```sh
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar --style modern-language-association
```

Shortcodes can override the style. `cite` requires named arguments for this:

```text
{{< cite keys="go-book" style="ieee" >}}
{{< bibliography style="ieee" >}}
```

### Additional styles

Place other styles, such as files from the [CSL styles repository](https://github.com/citation-style-language/styles), in a `styles/` directory at the site root. This is a Hugo Scholar convention, not a [standard Hugo directory](https://gohugo.io/getting-started/directory-structure/). The generator reads it directly; Hugo does not process or publish it by default. Select `styles/journal.csl` as `journal` or by path:

```sh
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar --style styles/journal.csl
```

To share styles across sites, set `CSL_STYLE_DIR` to a local directory of CSL files or a checkout of the CSL repository:

```sh
export CSL_STYLE_DIR=/path/to/my/csl-styles
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar --style journal
```

Styles are checked in this order:

1. Supplied path
2. Site's `styles/` directory
3. `CSL_STYLE_DIR`, if set
4. Bundled styles

Local files override bundled styles. The `.csl` suffix is optional. Relative paths start from the generator's working directory, normally the site root.

Hugo Scholar does not download CSL files or accept style URLs. Missing-style errors list the paths searched. Go and Hugo may download modules during setup.

### Dependent journal styles

Some journal styles require a parent style. If it is not bundled, place it beside the journal style, in `styles/`, or in `CSL_STYLE_DIR`.

For a style in `dependent/`, the parent is also sought in the directory above it. Select these styles as `dependent/journal`.

Parent HTTP(S) URLs identify local files: `http://www.zotero.org/styles/apa` means `apa.csl`, and a URL ending in `custom.csl` means that filename. Nothing is fetched. Relative parent paths start from the journal file's directory. Missing parents cause an error. The journal style's language settings are preserved.

The default style must define a bibliography. Styles that define only notes cause an error when used as the default.

## Generator options

| Option                     | Default                        | Purpose                                               |
| -------------------------- | ------------------------------ | ----------------------------------------------------- |
| `--source`                 | `_bibliography`                | BibTeX file or directory                              |
| `--output`                 | `data/scholar.json`            | Generated Hugo data                                   |
| `--bibliography`           | Hugo config, then `references` | Default BibTeX file name                              |
| `--style`                  | Hugo config, then `apa`        | Local CSL name, `.csl` path, or `basic`               |
| `--locale`                 | `en-US`                        | CSL language                                          |
| `--allow-locale-overrides` | false                          | Use an entry's BibTeX `language` for its reference    |
| `--citeproc`               | `citeproc`                     | Path to the citation processor                        |
| `--repository`             | unset                          | Attachment directory; files start with a citation key |
| `--details`                | false                          | Generate a page for each entry                        |

For all options, run `go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar --help`. Entry locale overrides affect references only; citations use the site locale.

`--repository static/repository` links each entry to files named `KEY.SUFFIX`. `--details` writes pages under `content/bibliography/`. BibTeX files directly under `content/` also get sibling Markdown pages. The next run overwrites generated pages.

Shortcode options can also be set under `[params.scholar]` in `hugo.toml`:

```toml
[params.scholar]
bibliography = 'references'
style = 'apa'
sort_by = 'name,year'
order = 'ascending,descending'
```

The generator reads `params.scholar.bibliography` and `params.scholar.style` through `hugo config --format json`, including configuration directories and environment overrides. CLI flags take precedence. Supply both `--bibliography` and `--style` to skip reading Hugo configuration. Without site configuration, the defaults are `references` and `apa`.

Templates use the generated style. The generated default bibliography must match the site configuration; rerun the generator after changing it.

Generated entries reserve `key` for the citation ID and `type` for the entry kind. BibTeX fields with those names become `bibtex_key` and `bibtex_type`. Names can span lines. Citations support nested shortcodes and backtick-delimited arguments.

To customize bibliography items, add `layouts/partials/scholar/NAME.html` and use `{{< bibliography template="NAME" >}}`.

## Jekyll-Scholar compatibility

When moving from [Jekyll-Scholar](https://github.com/inukshuk/jekyll-scholar), use Hugo shortcodes in place of Liquid tags, and run the generator before Hugo. Query and BibTeX support cover common cases, but not all BibTeX-Ruby queries or LaTeX filters.

## Development

- `make check`: check Go formatting, run tests and vet, and build the example (requires Hugo).
- `make fmt`: format code and documentation (requires Prettier).
- `make generate`: regenerate example data and pages (requires `citeproc`).
- `make clean`: remove example build output.

Integration tests check the generator and Hugo output. Tests skip when a required tool is missing; set `HUGO_SCHOLAR_REQUIRE_INTEGRATION=1` to fail instead. CI installs Hugo, runs tests that do not require `citeproc`, and builds the example from committed data. To check CSL formatting locally, install `citeproc` and run `make generate` and `HUGO_SCHOLAR_REQUIRE_INTEGRATION=1 make check`.

### Updating bundled styles

1. Review a CSL [`v1.0.2` checkout](https://github.com/citation-style-language/styles/tree/v1.0.2) and record its full commit SHA.
2. Copy `apa.csl`, `ieee.csl`, and `modern-language-association.csl` into `internal/scholar/styles/`, preserving the XML and attribution.
3. Update the commit, SHA-256 checksums, and byte counts in [manifest.json](internal/scholar/styles/manifest.json). Calculate them from the repository root:

```sh
sha256sum internal/scholar/styles/*.csl
wc -c internal/scholar/styles/*.csl
```

Run `make generate` and `HUGO_SCHOLAR_REQUIRE_INTEGRATION=1 make check`. Review and commit the styles, manifest, and changed example output. Tests check file hashes, offline style lookup, URL rejection, and `citeproc` formatting.

## License

Hugo Scholar is [MIT licensed](LICENSE). The bundled CSL styles are [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/); see [third-party notices](THIRD_PARTY_NOTICES.md).
