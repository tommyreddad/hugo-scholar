# Hugo Scholar

[![CI](https://github.com/tommyreddad/hugo-scholar/actions/workflows/ci.yaml/badge.svg)](https://github.com/tommyreddad/hugo-scholar/actions/workflows/ci.yaml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Add BibTeX citations and bibliographies to Hugo. A Go command prepares your references, and shortcodes display them in your pages.

References use AMS labels such as `[DoKe15]` by default. See [a publications page](https://tommy.reddad.net/publications/) built with Hugo Scholar, or explore the [example site](example/).

## Setup

Requires Go 1.26+, Hugo, and [`citeproc`](https://github.com/jgm/citeproc). For basic formatting without `citeproc`, pass `--style basic` when generating references.

From your site's root directory, add the module. Skip `hugo mod init` if you already have a `go.mod`:

```sh
hugo mod init example.com/my-site
go get github.com/tommyreddad/hugo-scholar@v0.1.2
```

Import it in `hugo.toml`:

```toml
[[module.imports]]
path = 'github.com/tommyreddad/hugo-scholar'
```

Put your BibTeX entries in `_bibliography/references.bib`, add shortcodes to your pages, then generate and build:

```sh
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar
hugo
```

Rerun the generator after changing references, citations, or citation settings. It writes `data/scholar.json`; commit that file if your deployment only runs Hugo. Hugo itself does not need `citeproc`.

To preview the included example from a checkout of this repository, run `cd example && hugo server`. Its reference data is already generated.

## Shortcodes

### Cite sources

Use the keys from your BibTeX file:

```text
One source {{< cite "go-book" >}}.
Several sources {{< cite keys="go-book paper" >}}.
A specific page {{< cite keys="go-book" locator="42" label="page" >}}.
```

For another bibliography file, use named arguments: `{{< cite keys="go-book" file="books" >}}` reads from `_bibliography/books.bib`.

### List references

List all entries, or only those cited earlier on the page:

```text
{{< bibliography >}}
{{< bibliography cited=true >}}
```

Filter, sort, or group a list as needed:

```text
{{< bibliography type="book" >}}
{{< bibliography sort_by="year" order="descending" >}}
{{< bibliography group_by="year" >}}
{{< bibliography file="books" query="@book[year>=2000]" >}}
```

The citation style determines the default order. Use `cited_in_order=true` for cited entries in order of appearance. Other options include `max`, `offset`, and `remove_duplicates`.

### Other shortcodes

| Shortcode                                   | Purpose                                                                             |
| ------------------------------------------- | ----------------------------------------------------------------------------------- |
| `{{< reference "go-book" >}}`               | Print a full reference inline; use `key="go-book" block=true` for a separate block. |
| `{{< bibliography_count type="book" >}}`    | Count matching entries.                                                             |
| `{{< nocite "go-book" >}}`                  | Include an entry in a cited-only bibliography without displaying a citation.        |
| `{{< quote "go-book" >}}Text{{< /quote >}}` | Display a quotation with its citation.                                              |

See the [example pages](example/content/) for more combinations, including separate citation links and multiple bibliographies.

## Citation styles

Choose a bundled style in `hugo.toml`:

```toml
[params.scholar]
style = 'ieee'
```

| Style                | Setting                                      |
| -------------------- | -------------------------------------------- |
| AMS labels (default) | `american-mathematical-society-label`        |
| AMS numbered         | `american-mathematical-society-numeric`      |
| ACM                  | `association-for-computing-machinery`        |
| IEEE                 | `ieee`                                       |
| Springer LNCS        | `springer-lecture-notes-in-computer-science` |
| APA                  | `apa`                                        |
| MLA                  | `modern-language-association`                |

Override the site setting with `--style`, or set a style on individual shortcodes:

```text
{{< cite keys="go-book" style="ieee" >}}
{{< bibliography style="ieee" >}}
```

For another style, download its CSL file from the [CSL styles repository](https://github.com/citation-style-language/styles) and save it as `styles/journal.csl`. Select it with `style = 'journal'` or `--style journal`.

Styles are loaded locally: an explicit file path takes priority, followed by the site's `styles/` directory, `CSL_STYLE_DIR` if set, and the bundled styles. If a style requires a parent, save that file alongside it too. No styles are downloaded automatically.

## Configuration

Set shared bibliography defaults in `hugo.toml`:

```toml
[params.scholar]
bibliography = 'references'
style = 'american-mathematical-society-label'
sort_by = 'name,year'
order = 'ascending,descending'
```

Common generator options:

| Option                | Purpose                                                            |
| --------------------- | ------------------------------------------------------------------ |
| `--source PATH`       | Read a BibTeX file or directory; defaults to `_bibliography`.      |
| `--output PATH`       | Write generated data; defaults to `data/scholar.json`.             |
| `--bibliography NAME` | Override the site's default bibliography.                          |
| `--style NAME`        | Override the site's citation style, or use `basic`.                |
| `--locale LOCALE`     | Set the citation language; defaults to `en-US`.                    |
| `--repository PATH`   | Link attachments named after citation keys, such as `go-book.pdf`. |
| `--details`           | Generate a page for each entry in the default bibliography.        |

Run `go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar --help` for all options.

Detail pages are written to `content/bibliography/`. Link to them with `{{< cite_details "go-book" >}}`, or use `details_link` to get a page URL. BibTeX files placed under `content/` are also converted to Markdown pages. Generated pages are refreshed on each run.

For custom bibliography markup, create `layouts/partials/scholar/NAME.html` and select it with `{{< bibliography template="NAME" >}}`. The `scholar/reference_content.html` partial renders formatted reference text; call it with `dict "reference" .reference`.

## Coming from Jekyll-Scholar

Replace Liquid tags with Hugo shortcodes and run the generator before Hugo. Common BibTeX and query features are supported, but some BibTeX-Ruby queries and LaTeX filters differ.

## Development

- `make check`: check formatting, run tests and vet, and build the example.
- `make fmt`: format Go and Markdown files (requires Prettier).
- `make generate`: regenerate the example (requires `citeproc`).
- `make clean`: remove example build output.

CI runs without `citeproc`. To run all integration tests locally, install Hugo and `citeproc`, then run `HUGO_SCHOLAR_REQUIRE_INTEGRATION=1 make check`.

When updating bundled styles, copy the originals from a CSL [`v1.0.2` checkout](https://github.com/citation-style-language/styles/tree/v1.0.2), preserve their attribution, and record the upstream commit in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Regenerate the example and run the full checks before committing.

## License

Hugo Scholar is [MIT licensed](LICENSE). Bundled CSL styles are [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/); see [third-party notices](THIRD_PARTY_NOTICES.md).
