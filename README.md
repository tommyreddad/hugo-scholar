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

4. **Generate and build.** Run the generator whenever a reference or citation
   changes, before running Hugo:

   ```sh
   go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar
   hugo
   ```

The page now has a linked `[1]` citation and a bibliography entry with the
paper's title linked to its URL. The generator writes `data/scholar.json`; Hugo
reads that file when it builds the page.

## Build and deploy

If your deployment builds from source, run both commands in its build step:

```sh
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar && hugo --minify
```

Hugo writes the site to `public/`. For
[Cloudflare Pages](https://developers.cloudflare.com/pages/framework-guides/deploy-a-hugo-site/),
use the command above as the build command and `public/` as the output
directory. For
[GitHub Pages](https://docs.github.com/en/pages/getting-started-with-github-pages/configuring-a-publishing-source-for-your-github-pages-site),
run it in your Actions workflow before uploading `public/`. If your deployment
runs only Hugo, generate `data/scholar.json` locally and commit it with your
content changes. The default style needs no `citeproc` installation in either
workflow.

## Shortcodes

- `{{< bibliography >}}` lists every entry in `references.bib`.
- `{{< bibliography cited=true >}}` lists only entries cited on that page.
- `{{< cite keys="sample another-key" >}}` cites multiple entries together.

## Bibliography styles

### Basic

The default `basic` style numbers references on each page. It formats journal
volume, issue, and pages when those fields are present. Add `shortjournal` to a
BibTeX entry to display an abbreviated journal name; otherwise it uses
`journal`. A `url` links the title, with `doi` as a fallback.

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

## More examples

See the [example site](example/) for more shortcodes and the author's
[publications page](https://tommy.reddad.net/publications/) for a rendered
bibliography. Add `--help` to the generator command to see all options.

## Credits and license

Inspired by [Jekyll Scholar](https://github.com/inukshuk/jekyll-scholar).

Hugo Scholar is [MIT licensed](LICENSE). Bundled CSL styles have separate terms
in the [third-party notices](THIRD_PARTY_NOTICES.md).
