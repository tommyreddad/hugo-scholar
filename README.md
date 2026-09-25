# Hugo Scholar

Hugo Scholar adds BibTeX citations and bibliographies to Hugo. A Go command reads BibTeX, formats references, and writes Hugo data. A Hugo module supplies the shortcodes and templates. No Python or Ruby is used.

The default style is APA. For CSL formatting, the Go command calls the [`citeproc` executable](https://github.com/jgm/citeproc#readme) while preparing data. **Site visitors and Hugo builds that use committed generated data do not need `citeproc`.** Use `--style basic` to prepare data without it. The basic formatter is deliberately small and is not APA.

## Try the example

Requirements: Go 1.22 or newer, Hugo (tested with 0.166), and `citeproc` for CSL styles. From this checkout:

```sh
cd example
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar --repository static/repository --details
hugo server
```

The example uses a local Go module replacement. To try it without `citeproc`, add `--style basic` to the `go run` command. Generated example files are included, so `hugo server` alone also works.

## Add it to a Hugo site

Add the Hugo module to `hugo.toml`:

```toml
[[module.imports]]
path = 'github.com/tommyreddad/hugo-scholar'
```

Use the same module in the site's `go.mod`. For an unpublished local checkout:

```go
module example.com/my-site

go 1.22

require github.com/tommyreddad/hugo-scholar v0.0.1
replace github.com/tommyreddad/hugo-scholar => /absolute/path/to/hugo-scholar
```

The initial release version is `v0.0.1`. Once that tag is published, remove `replace` to fetch it as a Go module. Put BibTeX files in `_bibliography/`. The default file is `_bibliography/references.bib` (or `.bibtex`). Run the generator from the site root before Hugo:

```sh
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar
hugo
```

The command writes `data/scholar.json`. Commit this file if your deployment runs only Hugo. Re-run the command after changing BibTeX, citation shortcodes, content BibTeX pages, style, locale, or generator options. During `hugo server`, the command does not run automatically; run it again when any of those inputs change.

The executable named `citeproc` must be on `PATH` when generating CSL data. Set `--citeproc /path/to/citeproc` if needed. [Its input/output protocol](https://github.com/jgm/citeproc/blob/master/man/citeproc.1.md) is JSON; this project calls it directly from Go. `--style apa` uses the bundled APA CSL file. Other names are loaded from `styles/NAME.csl`, `CSL_STYLE_DIR/NAME.csl`, or the [official CSL styles repository](https://github.com/citation-style-language/styles). A local `.csl` path or HTTPS URL also works. Dependent CSL styles resolve to their independent parent. Downloaded named styles require network access only when data is prepared. `--locale` accepts a BCP 47 language tag such as `en-US` or `fr-FR`.

## Write content

Hugo shortcodes replace Jekyll-Scholar's Liquid tags:

```text
This claim is supported {{< cite "ruby" >}}.
Several sources agree {{< cite keys="ruby paper" >}}.
As the authors explain {{< cite "ruby" suppress_author=true locator="42" label="page" >}}.

{{< bibliography >}}
{{< bibliography cited=true >}}
{{< bibliography cited_in_order=true >}}
{{< bibliography file="books" query="@book[year>=2000]" >}}
{{< bibliography sort_by="name,year" order="ascending,descending" max="5" >}}
{{< bibliography group_by="type,year" >}}
{{< bibliography style="styles/numeric.csl" prefix="numeric-" >}}

Count: {{< bibliography_count type="book" >}}
Full reference: {{< reference "ruby" >}}
{{< quote "ruby" >}}A longer passage.{{< /quote >}}
{{< nocite "ruby" >}}

{{< cite_details "ruby" text="Read more" >}}
{{< details_link "ruby" >}}
```

`cite` accepts one or more space-separated keys. `locator` and `label` apply to the first key; `locators="42|3"` and `labels="page|chapter"` apply to a group. `suppress_author=true` is passed to CSL. `nocite` adds a key to cited-only bibliographies without printing a citation. `quote` prints a blockquote and source citation. `reference` prints one formatted entry. `bibliography_count` accepts the same selection options as `bibliography`.

Use `style="styles/numeric.csl"` on `cite`, `quote`, `reference`, or `bibliography` to override the command's default style for that shortcode. The Go command precomputes each style it finds in content. A `cite` style override must use named arguments, such as `{{< cite keys="ruby" style="styles/numeric.csl" prefix="numeric-" >}}`; Hugo does not allow positional and named arguments in one shortcode. `style="basic"` uses the built-in formatter. A CSL override still requires `citeproc` during generation when the command uses `--style basic`.

Set `separate_links=true` on `cite`, or set `params.scholar.separate_links`, to link each key in a grouped citation separately. Some CSL styles collapse citation ranges; those styles may not yield matching individual labels.

`cited=true` selects cited items in bibliography order. `cited_in_order=true` uses first citation order. `query` supports `@type`, `!@type`, `@*`, comma-separated selectors, field-existence tests, `=`, `!=`, `/=`, `>`, `>=`, `<`, `<=`, regular-expression matching (`^=` or `~=`), `!~`, and `&&`/`||` within brackets. Numeric comparisons convert both sides to numbers. Examples: `@*[url]`, `!@book`, `@book, @article`, `@book[year>=2000 && author ^= Doe]`.

Citation selection follows page order. `clear=true` on `bibliography` clears the cited set for later lists. Set `params.scholar.missing_reference` to change the text shown for an unknown key (default: `(missing reference)`).

Set `file="books"` to use `_bibliography/books.bib`, or `file="topics/books"` for a file in a nested directory. The site's `params.scholar.bibliography` sets the default file name; pass the same name to the Go command with `--bibliography books`. For multiple lists on one page, use matching `prefix` values on `cite` and `bibliography` so their HTML IDs are unique. `relative="/references/"` links citations to a list on another page.

## Configure output

Shortcode options override matching values under `[params.scholar]` in `hugo.toml`:

```toml
[params.scholar]
bibliography = 'references'
sort_by = 'name,year'
order = 'ascending,descending'
group_by = 'type'
group_order = 'ascending'
type_order = ['book', 'article']
relative = '/references/'
bibliography_class = 'bibliography'
bibliography_list_tag = 'ol'
bibliography_item_tag = 'li'
bibliography_group_tag = 'h2,h3,h4,h5'
reference_tagname = 'span'
cite_class = 'citation'

[params.scholar.type_names]
book = 'Books'

[params.scholar.type_aliases]
inproceedings = 'article'

[params.scholar.bibliography_list_attributes]
role = 'list'
```

Also available: `query`, `bibliography_item_attributes`, `details_link`, `details_link_class`, and `month_names` (12 names). `sort_by` and `group_by` accept comma-separated fields. `name` sorts by the first available author, editor, institution, organization, or publisher. `order` and `group_order` accept one direction per field; the last direction is reused. `type_order` places named types after unnamed types in the given order. `max` is ignored when grouping, as in Jekyll-Scholar. `offset` skips entries before `max` is applied.

Set `remove_duplicates=true` on a bibliography or in `params.scholar` to retain the first entry for each normalized `year,title` pair. `match_fields="title,doi"` changes the comparison fields for a shortcode. This affects rendered lists and counts; the generated data retains every entry so citations by key still work.
CSL disambiguation still considers those hidden entries. If a style adds year suffixes such as `2020a`, the suffix may remain after list deduplication.

To customize each bibliography item, create `layouts/partials/scholar/NAME.html` in your site and use `{{< bibliography template="NAME" >}}`. The partial receives `entry` (BibTeX fields, `links`, and `detail_url`), `reference` (formatted HTML), `index`, `prefix`, and `referenceTag`. `layouts/partials/scholar/reference.html` is the module's default. Hugo partials replace Liquid bibliography templates.

The Go command has these options:

| Option | Default | Use |
| --- | --- | --- |
| `--source` | `_bibliography` | Directory (searched recursively) or one `.bib`/`.bibtex` file |
| `--output` | `data/scholar.json` | Generated Hugo data |
| `--bibliography` | `references` | Default file name without extension |
| `--content` | `content` | Hugo content directory to scan |
| `--style` | `apa` | CSL name, file, HTTPS URL, or `basic` |
| `--locale` | `en-US` | CSL language |
| `--citeproc` | `citeproc` | Citation processor executable |
| `--repository` | unset | Directory of attachments named `KEY.SUFFIX` |
| `--repository-url` | inferred for `static/…` | Public URL prefix for attachments |
| `--repository-delimiter` | `.` | Separator between key and attachment suffix |
| `--details` | false | Generate detail pages for the default bibliography |
| `--details-dir` | `bibliography` | Detail-page directory under `content` |
| `--details-permalink` | `/:details_dir/:key/` | URL template for detail pages |

For attachments, `--repository static/repository` exposes files such as `static/repository/paper.notes.txt` as `entry.links["notes.txt"]`. Use `--repository-url` if the directory is outside `static/`. `--details` writes generated Markdown under `content/bibliography/`, uses the module's `scholar` layout, and adds `detail_url` to the default bibliography. Only files marked as generated are replaced or removed. A custom `--details-permalink` can use `:details_dir`, `:key`, `:doi`, `:extension` (a trailing slash), and BibTeX fields such as `:year`; field values are URL slugs.

BibTeX files placed directly under `content/` are converted to sibling `.md` pages. They may contain YAML front matter and Markdown text around entries. Each entry is replaced in place by a formatted reference. The generated Markdown is marked and overwritten on the next run; it is removed if its source `.bib` file is deleted. Do not edit it directly.

## Compatibility notes

This module covers the usual Jekyll-Scholar workflow: BibTeX strings and crossrefs, CSL styles and locales, grouped and filtered lists, counts, citations and quotes, cited-only lists, detail pages, attachment links, custom item templates, and BibTeX content pages. [Jekyll-Scholar's README](https://github.com/inukshuk/jekyll-scholar/blob/main/README.md) describes its original behavior.

Some behavior remains different or unsupported:

- Jekyll's Liquid syntax, layouts, and build hooks cannot run in Hugo. Use Hugo shortcodes and partials, and run the Go command before Hugo.
- BibTeX-Ruby's full query language and its configurable LaTeX/HTML filter pipeline are not reproduced. Common LaTeX accents are converted; unfamiliar commands are stripped. Check unusual BibTeX markup in the generated output.
- Per-entry locale overrides are not implemented.
- `--file` selects a named bibliography loaded from `--source` or a converted content file. Arbitrary absolute BibTeX paths in shortcodes are not supported.
- The CSL mapping covers common BibTeX fields. Specialized BibTeX fields may need additional mapping.

Run `go test ./...` and `go vet ./...` for Go checks. Regenerate the example data and run `hugo --source example` to check the Hugo templates. Tests that require `citeproc` skip when it is not installed.

## License

Except for the bundled APA CSL style, Hugo Scholar is licensed under the [MIT License](LICENSE). The [APA CSL style](https://github.com/citation-style-language/styles/blob/master/apa.csl) comes from the [Citation Style Language (CSL) project](https://citationstyles.org/) and is licensed under [CC BY-SA 3.0 Unported](https://creativecommons.org/licenses/by-sa/3.0/). See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for attribution and license details.
