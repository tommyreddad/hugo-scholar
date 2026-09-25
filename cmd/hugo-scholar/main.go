package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tommyreddad/hugo-scholar/internal/scholar"
)

func main() {
	source := flag.String("source", "_bibliography", "directory containing BibTeX files")
	output := flag.String("output", "data/scholar.json", "generated Hugo data file")
	bibliography := flag.String("bibliography", "references", "default bibliography file name without extension")
	repository := flag.String("repository", "", "directory containing files named after citation keys")
	repositoryURL := flag.String("repository-url", "", "public URL prefix for repository files")
	repositoryDelimiter := flag.String("repository-delimiter", ".", "delimiter between a citation key and a repository file suffix")
	details := flag.Bool("details", false, "generate one Hugo content page per entry in the default bibliography")
	detailsDir := flag.String("details-dir", "bibliography", "content directory and URL path for detail pages")
	detailsPermalink := flag.String("details-permalink", "", "detail page URL template, such as /bibliography/:year/:key/")
	style := flag.String("style", "apa", "CSL style name, file, or HTTPS URL; use basic to skip citeproc")
	locale := flag.String("locale", "en-US", "CSL locale")
	allowLocaleOverrides := flag.Bool("allow-locale-overrides", false, "format each bibliography entry in its BibTeX language when present")
	citeproc := flag.String("citeproc", "citeproc", "path to the citeproc executable")
	content := flag.String("content", "content", "Hugo content directory to scan for page citations")
	flag.Parse()
	options := scholar.Options{Source: *source, DefaultBibliography: *bibliography, Repository: *repository, RepositoryURL: *repositoryURL, RepositoryDelimiter: *repositoryDelimiter, Style: *style, Locale: *locale, AllowLocaleOverrides: *allowLocaleOverrides, CiteprocPath: *citeproc, ContentDir: *content}
	if *details {
		options.DetailsDir = *detailsDir
		options.DetailsPermalink = *detailsPermalink
	}
	data, err := scholar.PrepareWithOptions(options)
	if err == nil && *details {
		entries, exists := data.Bibliographies[*bibliography]
		if !exists {
			err = fmt.Errorf("default bibliography %q is missing", *bibliography)
		} else {
			err = scholar.WriteDetails(*content, *detailsDir, entries)
		}
	}
	if err == nil {
		err = scholar.WriteJSON(*output, data)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hugo-scholar:", err)
		os.Exit(1)
	}
	count := 0
	for _, entries := range data.Bibliographies {
		count += len(entries)
	}
	fmt.Printf("Prepared %d references in %s\n", count, *output)
}
