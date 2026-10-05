package scholar

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PageRender struct {
	Cited    map[string][]string           `json:"cited"`
	CitedAt  map[string]CitedSnapshot      `json:"cited_at,omitempty"`
	Contexts map[string]BibliographyRender `json:"contexts,omitempty"`
}

type BibliographyRender struct {
	Citations    map[string]string   `json:"citations,omitempty"`
	References   map[string]string   `json:"references,omitempty"`
	BasicNumbers map[string]int      `json:"basic_numbers,omitempty"`
	Separate     map[string][]string `json:"separate,omitempty"`
	Order        []string            `json:"order,omitempty"`
}

type CitedSnapshot struct {
	Keys []string `json:"keys"`
}

func citationFromShortcode(call shortcode) (cslCitation, error) {
	keys := strings.Fields(firstNonempty(call.Args["keys"], call.Args["key"], call.Args["0"]))
	if len(keys) == 0 {
		return cslCitation{}, fmt.Errorf("%s shortcode has no citation key", call.Name)
	}
	citation := cslCitation{}
	locators := strings.Split(firstNonempty(call.Args["locators"], call.Args["locator"]), "|")
	labels := strings.Split(firstNonempty(call.Args["labels"], call.Args["label"]), "|")
	for index, key := range keys {
		item := cslCitationItem{ID: key}
		if call.Args["suppress_author"] == "true" {
			item.Type = "suppress-author"
		}
		if index < len(locators) {
			item.Locator = locators[index]
		}
		if index < len(labels) {
			item.Label = labels[index]
		}
		citation.Items = append(citation.Items, item)
	}
	return citation, nil
}

func renderPages(options Options, data *Data) error {
	content := options.ContentDir
	data.Pages = make(map[string]PageRender)
	render := func(filename string, source []byte) error {
		_, body, err := splitFrontMatter(string(source))
		if err != nil {
			return fmt.Errorf("%s: %w", filename, err)
		}
		relative, err := filepath.Rel(content, filename)
		if err != nil {
			return err
		}
		page, err := renderPage(options, data.Bibliographies, data.QueryMatches, scanShortcodes(body))
		if err != nil {
			return fmt.Errorf("%s: %w", filename, err)
		}
		if len(page.Contexts) > 0 || len(page.CitedAt) > 0 {
			data.Pages[filepath.ToSlash(relative)] = page
		}
		return nil
	}
	seen := map[string]bool{}
	err := filepath.WalkDir(content, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) && filename == content {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || isEditorLock(filename) || !isHugoContent(filename) {
			return nil
		}
		seen[filename] = true
		source, planned := data.files[filename]
		if planned && source == nil {
			return nil
		}
		if !planned {
			var err error
			source, err = os.ReadFile(filename)
			if err != nil {
				return err
			}
		}
		return render(filename, source)
	})
	if err != nil {
		return err
	}
	var newPages []string
	for filename, source := range data.files {
		if source != nil && !seen[filename] && isHugoContent(filename) {
			newPages = append(newPages, filename)
		}
	}
	sort.Strings(newPages)
	for _, filename := range newPages {
		if err := render(filename, data.files[filename]); err != nil {
			return err
		}
	}
	return nil
}

func isHugoContent(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".md", ".markdown", ".html":
		return true
	default:
		return false
	}
}

func isEditorLock(filename string) bool {
	return strings.HasPrefix(filepath.Base(filename), ".#")
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
