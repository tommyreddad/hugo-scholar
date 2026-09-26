# Hugo Scholar

[![CI](https://github.com/tommyreddad/hugo-scholar/actions/workflows/ci.yaml/badge.svg)](https://github.com/tommyreddad/hugo-scholar/actions/workflows/ci.yaml)
[![Last commit](https://img.shields.io/github/last-commit/tommyreddad/hugo-scholar)](https://github.com/tommyreddad/hugo-scholar/commits)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

BibTeX citations and bibliographies for Hugo. A Go command turns BibTeX into Hugo data; the module provides the shortcodes and templates.

References use APA by default through [`citeproc`](https://github.com/jgm/citeproc). The generator links DOI and URL text in CSL references when the entry supplies a valid HTTP(S) address. Use `--style basic` if you don't need CSL formatting. Hugo only needs the generated data, so a site build does not need `citeproc`.

## Try it

Requires Go 1.26+, Hugo, and `citeproc` for CSL styles.

```sh
cd example
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar --repository static/repository --details
hugo server
```

The example includes generated data, so you can also run `hugo server` on its own. Add `--style basic` to the generator command to run it without `citeproc`.

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

require github.com/tommyreddad/hugo-scholar v0.1.0
```

Put BibTeX in `_bibliography/references.bib` (or `.bibtex`). From the site root, run:

```sh
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar
hugo
```

The generator writes `data/scholar.json`. Commit it if deployment runs only Hugo. Run the generator again after changing BibTeX, citations, styles, or generator options; `hugo server` does not run it for you.

## Shortcodes

```text
This claim is supported {{< cite "ruby" >}}.
Several sources agree {{< cite keys="ruby paper" >}}.
See {{< cite keys="ruby" locator="42" label="page" >}}.

{{< bibliography >}}
{{< bibliography cited=true >}}
{{< bibliography file="books" query="@book[year>=2000]" >}}

Count: {{< bibliography_count type="book" >}}
Full reference: {{< reference "ruby" >}}
{{< quote "ruby" >}}A quoted passage.{{< /quote >}}
{{< nocite "ruby" >}}
```

`cite` takes one or more BibTeX keys. `cited=true` limits a bibliography to cited entries; `cited_in_order=true` uses first citation order. `nocite` adds an entry to those lists without printing a citation. Use `file="books"` for `_bibliography/books.bib`. Bibliographies also accept `sort_by`, `order`, `group_by`, `max`, `offset`, `remove_duplicates`, and `query`.

Set `style` on a shortcode to override the default, for example `{{< cite keys="ruby" style="styles/numeric.csl" >}}`. A style override on `cite` needs named arguments. For a grouped citation, `separate_links=true` links each key separately. Use matching `prefix` values on citations and bibliographies when a page has multiple lists.

## Generator options

| Option                     | Default             | Purpose                                               |
| -------------------------- | ------------------- | ----------------------------------------------------- |
| `--source`                 | `_bibliography`     | BibTeX file or directory                              |
| `--output`                 | `data/scholar.json` | Generated Hugo data                                   |
| `--bibliography`           | `references`        | Default BibTeX file name                              |
| `--style`                  | `apa`               | CSL name, `.csl` path, HTTPS URL, or `basic`          |
| `--locale`                 | `en-US`             | CSL language                                          |
| `--allow-locale-overrides` | false               | Use an entry's BibTeX `language` for its reference    |
| `--citeproc`               | `citeproc`          | Path to the citation processor                        |
| `--repository`             | unset               | Attachment directory; files start with a citation key |
| `--details`                | false               | Generate a page for each entry                        |

Run `go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar --help` for all options. The bundled APA style works offline. Other named styles may be downloaded from the [CSL styles repository](https://github.com/citation-style-language/styles) during generation. Per-entry locale overrides affect references; citations continue to use the site locale.

`--repository static/repository` links files named `KEY.SUFFIX` from each entry. `--details` writes generated pages under `content/bibliography/`. BibTeX files directly under `content/` are also converted to sibling Markdown pages. Generated pages are overwritten on the next run.

Shortcode options can also be set under `[params.scholar]` in `hugo.toml`:

```toml
[params.scholar]
bibliography = 'references'
sort_by = 'name,year'
order = 'ascending,descending'
```

To customize bibliography items, add `layouts/partials/scholar/NAME.html` and use `{{< bibliography template="NAME" >}}`.

## Notes

Hugo cannot run Jekyll-Scholar's Liquid tags or build hooks. Use these shortcodes and run the generator before Hugo. The query and BibTeX conversion cover common cases, but do not reproduce BibTeX-Ruby's full query or LaTeX filter behavior.

Run `make check` for Go formatting, tests, vetting, and the example build. Hugo is a development dependency for the build check; Prettier is one for `make fmt`. `make clean` removes the example build output. Tests that need `citeproc` skip if it is unavailable.

## License

Hugo Scholar is [MIT licensed](LICENSE). The bundled APA CSL style is [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/); see [third-party notices](THIRD_PARTY_NOTICES.md).
