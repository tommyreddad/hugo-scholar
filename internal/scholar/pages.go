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
	Citations        map[string]string                       `json:"citations"`
	References       map[string]map[string]string            `json:"references"`
	Cited            map[string][]string                     `json:"cited"`
	CitedAt          map[string]CitedSnapshot                `json:"cited_at,omitempty"`
	Separate         map[string][]string                     `json:"separate,omitempty"`
	StyledCitations  map[string]map[string]string            `json:"styled_citations,omitempty"`
	StyledReferences map[string]map[string]map[string]string `json:"styled_references,omitempty"`
	StyledSeparate   map[string]map[string][]string          `json:"styled_separate,omitempty"`
	Orders           map[string][]string                     `json:"orders,omitempty"`
	StyledOrders     map[string]map[string][]string          `json:"styled_orders,omitempty"`
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
	if content == "" {
		content = "content"
	}
	data.Pages = make(map[string]PageRender)
	return filepath.WalkDir(content, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) && filename == content {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || isEditorLock(filename) || !isHugoContent(filename) {
			return nil
		}
		source, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		_, body, err := splitFrontMatter(string(source))
		if err != nil {
			return fmt.Errorf("%s: %w", filename, err)
		}
		relative, err := filepath.Rel(content, filename)
		if err != nil {
			return err
		}
		page := PageRender{
			Citations:        map[string]string{},
			References:       map[string]map[string]string{},
			Cited:            map[string][]string{},
			CitedAt:          map[string]CitedSnapshot{},
			Separate:         map[string][]string{},
			StyledCitations:  map[string]map[string]string{},
			StyledReferences: map[string]map[string]map[string]string{},
			StyledSeparate:   map[string]map[string][]string{},
			Orders:           map[string][]string{},
			StyledOrders:     map[string]map[string][]string{},
		}
		grouped := map[string][]shortcode{}
		stylesByFile := map[string]map[string]bool{}
		active := map[string][]string{}
		for _, call := range scanShortcodes(body) {
			file := firstNonempty(call.Args["file"], options.DefaultBibliography, "references")
			if style := call.Args["style"]; style != "" && style != options.Style && style != "basic" && (call.Name == "cite" || call.Name == "quote" || call.Name == "bibliography" || call.Name == "reference") {
				if stylesByFile[file] == nil {
					stylesByFile[file] = map[string]bool{}
				}
				stylesByFile[file][style] = true
				if _, exists := grouped[file]; !exists {
					grouped[file] = nil
				}
			}
			switch call.Name {
			case "cite", "quote", "nocite":
				grouped[file] = append(grouped[file], call)
				for _, key := range strings.Fields(firstNonempty(call.Args["keys"], call.Args["key"], call.Args["0"])) {
					if !contains(active[file], key) {
						active[file] = append(active[file], key)
					}
				}
			case "bibliography", "bibliography_count":
				page.CitedAt[call.ID] = CitedSnapshot{Keys: append([]string{}, active[file]...)}
				if call.Args["clear"] == "true" {
					active[file] = nil
				}
			}
		}
		for file, calls := range grouped {
			records, ok := data.Bibliographies[file]
			if !ok {
				return fmt.Errorf("%s: bibliography %q not found", filename, file)
			}
			if len(records) == 0 {
				continue
			}
			known := make(map[string]bool, len(records))
			for _, record := range records {
				known[record.Entry["key"]] = true
			}
			var citations []cslCitation
			var validCalls []shortcode
			cited := make(map[string]bool)
			for _, call := range calls {
				citation, err := citationFromShortcode(call)
				if err != nil {
					return fmt.Errorf("%s: %w", filename, err)
				}
				valid := true
				for _, item := range citation.Items {
					if !known[item.ID] {
						valid = false
					}
				}
				if !valid {
					continue
				}
				for _, item := range citation.Items {
					cited[item.ID] = true
					if !contains(page.Cited[file], item.ID) {
						page.Cited[file] = append(page.Cited[file], item.ID)
					}
				}
				citations = append(citations, citation)
				validCalls = append(validCalls, call)
			}
			// Include uncited entries after real citations so the processor can
			// produce a full bibliography when a page asks for one.
			for _, record := range records {
				key := record.Entry["key"]
				if !cited[key] {
					citations = append(citations, cslCitation{Items: []cslCitationItem{{ID: key}}})
				}
			}
			var styles []string
			if options.Style != "" && options.Style != "basic" {
				styles = append(styles, options.Style)
			}
			for style := range stylesByFile[file] {
				styles = append(styles, style)
			}
			sort.Strings(styles)
			multiple := false
			for _, citation := range citations[:len(validCalls)] {
				if len(citation.Items) > 1 {
					multiple = true
					break
				}
			}
			var singles []cslCitation
			if multiple {
				for _, citation := range citations {
					for _, item := range citation.Items {
						singles = append(singles, cslCitation{Items: []cslCitationItem{item}})
					}
				}
			}
			for _, style := range styles {
				result, err := runCiteproc(options.CiteprocPath, style, options.Locale, records, citations)
				if err != nil {
					return fmt.Errorf("%s: style %q: %w", filename, style, err)
				}
				if err := applyLocaleOverrides(options, style, records, citations, &result); err != nil {
					return fmt.Errorf("%s: style %q: %w", filename, style, err)
				}
				linkifyCSLReferences(&result, records)
				citationOutput := page.Citations
				referenceOutput := page.References
				separateOutput := page.Separate
				orderOutput := page.Orders
				if style != options.Style {
					if page.StyledCitations[style] == nil {
						page.StyledCitations[style] = map[string]string{}
						page.StyledReferences[style] = map[string]map[string]string{}
						page.StyledSeparate[style] = map[string][]string{}
						page.StyledOrders[style] = map[string][]string{}
					}
					citationOutput = page.StyledCitations[style]
					referenceOutput = page.StyledReferences[style]
					separateOutput = page.StyledSeparate[style]
					orderOutput = page.StyledOrders[style]
				}
				for index, call := range validCalls {
					citationOutput[call.ID] = result.Citations[index]
				}
				if multiple {
					individual, err := runCiteproc(options.CiteprocPath, style, options.Locale, records, singles)
					if err != nil {
						return fmt.Errorf("%s: separate citations with style %q: %w", filename, style, err)
					}
					position := 0
					for index, call := range validCalls {
						count := len(citations[index].Items)
						if count > 1 {
							separateOutput[call.ID] = individual.Citations[position : position+count]
						}
						position += count
					}
				}
				referenceOutput[file] = make(map[string]string)
				for _, pair := range result.Bibliography {
					if len(pair) == 2 {
						referenceOutput[file][pair[0]] = pair[1]
						orderOutput[file] = append(orderOutput[file], pair[0])
					}
				}
			}
		}
		if len(grouped) > 0 || len(page.CitedAt) > 0 {
			data.Pages[filepath.ToSlash(relative)] = page
		}
		return nil
	})
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
