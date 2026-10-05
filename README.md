# Hugo Scholar

[![CI](https://github.com/tommyreddad/hugo-scholar/actions/workflows/ci.yaml/badge.svg)](https://github.com/tommyreddad/hugo-scholar/actions/workflows/ci.yaml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

BibTeX citations for Hugo. Keep references in a `.bib` file, cite them by key in
Markdown, and generate the bibliography before Hugo builds your site.

## Quick start

You need [Go 1.26 or later](https://go.dev/dl/) and
[Hugo](https://gohugo.io/installation/). Run these steps from your Hugo site's
root directory.

1. **Add the module.** If your site has no `go.mod`, initialize it first:

   ```sh
   hugo mod init example.com/my-site
   ```

   Then add Hugo Scholar:

   ```sh
   go get github.com/tommyreddad/hugo-scholar@latest
   ```

   Add the import to `hugo.toml`:

   ```toml
   [[module.imports]]
     path = 'github.com/tommyreddad/hugo-scholar'
   ```

2. **Add a reference.** Create `_bibliography/references.bib`:

   ```bibtex
   @article{sample,
     author = {Doe, Jane},
     title = {A useful paper},
     journal = {Journal of Examples},
     year = {2024},
     url = {https://example.org/paper}
   }
   ```

3. **Cite it.** In a page such as `content/reading.md`:

   ```md
   +++
   title = 'Reading'
   +++

   This paper is useful {{< cite "sample" >}}.

   ## References

   {{< bibliography cited=true >}}
   ```

4. **Generate and build.** Rerun after bibliography, shortcode, or Scholar
   configuration changes:

   ```sh
   go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar
   hugo
   ```

   Add `--help` to the generator command to list generator options.

The page now has a linked `[1]` citation and a bibliography entry with the
paper's title linked to its URL. The generator writes `data/scholar.json`; Hugo
reads that file when it builds the page.

## Shortcodes

| Example | Purpose |
| --- | --- |
| `{{< cite "sample" >}}` | A linked in-text citation. |
| `{{< cite keys="sample another-key" >}}` | Several citations together. |
| `{{< bibliography >}}` | All entries in the default bibliography. |
| `{{< bibliography cited=true >}}` | Entries cited before this shortcode. |
| `{{< bibliography_count >}}` | The number of matching entries. |

**Citation options:** `locator="42"` adds a shared page locator;
`separate_links=true` links each key individually.

**Bibliography options:**

- `query="@book[year>=2020]"` filters entries. Use `,` between selectors and
  `&&` between conditions; predicate values can contain commas.
- `remove_duplicates=true` merges matching `year,title` entries, ignoring case
  and whitespace. Use `match_fields="doi"` to match by DOI instead.
- `clear=true` resets cited selection for subsequent lists.
- Use the same `prefix` on citations and their bibliography, with distinct
  prefixes for independently configured lists.

## Bibliography styles

### Basic

The default `basic` style numbers references per page and links titles using
`url`, falling back to `doi`. Set `shortjournal` to override `journal` with an
abbreviated name.

### CSL styles

To use a CSL style such as IEEE or APA, install
[`citeproc`](https://github.com/jgm/citeproc) and set the style in `hugo.toml`:

```toml
[params.scholar]
  style = 'ieee'
```

For a style that is not bundled, download its `.csl` file from the
[CSL styles repository](https://github.com/citation-style-language/styles), save
it in your site's `styles/` directory (for example,
`styles/chicago-author-date.csl`), and set `style = 'chicago-author-date'`.
Commit the file so your deployment can use it. If the style depends on a parent
style that is also not bundled, save that `.csl` file there too.

## Build and deploy

- **Generate during deployment:** run the generator before Hugo.
- **Generate locally:** commit `data/scholar.json` so deployment only needs Hugo.
  Also commit generated Markdown for `--details` and BibTeX pages in `content/`.

Publish `public/`; see the deployment guides for
[Cloudflare Pages](https://developers.cloudflare.com/pages/framework-guides/deploy-a-hugo-site/) and
[GitHub Pages](https://docs.github.com/en/pages/getting-started-with-github-pages/configuring-a-publishing-source-for-your-github-pages-site).

Content `.bib` and `.bibtex` files may be published verbatim; omit private
annotations.

## More examples

See the [example site](example/) for more shortcodes and the author's
[publications page](https://tommy.reddad.net/publications/) for a rendered
bibliography.

## Credits and license

Inspired by [Jekyll Scholar](https://github.com/inukshuk/jekyll-scholar).

Hugo Scholar is [MIT licensed](LICENSE). Bundled CSL styles have separate terms
in the [third-party notices](THIRD_PARTY_NOTICES.md).
