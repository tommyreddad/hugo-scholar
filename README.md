# Hugo Scholar

[![CI](https://github.com/tommyreddad/hugo-scholar/actions/workflows/ci.yaml/badge.svg)](https://github.com/tommyreddad/hugo-scholar/actions/workflows/ci.yaml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Hugo Scholar adds BibTeX citations and bibliographies to a Hugo site. Keep your references in a `.bib` file, cite them in your pages, and run the generator before building the site.

See the [example project](example/) for a working setup and [a publications page](https://tommy.reddad.net/publications/) for a live site.

## Get started

You need Go 1.26 or later, Hugo, and [`citeproc`](https://github.com/jgm/citeproc). If basic formatting is enough, you can skip `citeproc` and run the generator with `--style basic`.

From your Hugo site's root directory, add the module. If the site already has a `go.mod`, skip the first command.

```sh
hugo mod init example.com/my-site
go get github.com/tommyreddad/hugo-scholar@v0.1.4
```

Add the module to `hugo.toml`:

```toml
[[module.imports]]
path = 'github.com/tommyreddad/hugo-scholar'
```

Put your references in `_bibliography/references.bib`. For example:

```bibtex
@book{go-book,
  title = {The Go Programming Language},
  author = {Donovan, Alan A. A. and Kernighan, Brian W.},
  year = 2015,
  publisher = {Addison-Wesley}
}
```

Use the entry key in a page:

```text
This book is useful {{< cite "go-book" >}}.

## References

{{< bibliography >}}
```

Generate the reference data, then build your site:

```sh
go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar
hugo
```

Run the generator again when you change references or citations. It writes `data/scholar.json`. Commit that file if your deployment runs Hugo without the generator.

## A few options

Use `{{< bibliography cited=true >}}` to list only sources cited on the page. To cite more than one source, use `{{< cite keys="go-book paper" >}}`.

The default style uses numbered AMS citations. To use another bundled style, set it in `hugo.toml`:

```toml
[params.scholar]
style = 'ieee'
```

The [example pages](example/content/) show other shortcodes and options. Run `go run github.com/tommyreddad/hugo-scholar/cmd/hugo-scholar --help` for generator options.

Hugo Scholar is [MIT licensed](LICENSE). Bundled citation styles have separate terms listed in [third-party notices](THIRD_PARTY_NOTICES.md).
