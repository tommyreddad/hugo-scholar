package scholar

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed styles/apa.csl
var bundledAPA string

var styleCache sync.Map

type cslInput struct {
	Citations  []cslCitation    `json:"citations"`
	References []map[string]any `json:"references"`
	Style      string           `json:"style"`
	Lang       string           `json:"lang,omitempty"`
}

type cslCitation struct {
	Items []cslCitationItem `json:"citationItems"`
}

type cslCitationItem struct {
	ID      string `json:"id"`
	Type    string `json:"type,omitempty"`
	Label   string `json:"label,omitempty"`
	Locator string `json:"locator,omitempty"`
	Prefix  string `json:"prefix,omitempty"`
	Suffix  string `json:"suffix,omitempty"`
}

type cslResult struct {
	Citations    []string          `json:"citations"`
	Bibliography [][]string        `json:"bibliography"`
	Warnings     []json.RawMessage `json:"warnings"`
}

var cslTypes = map[string]string{
	"article": "article-journal", "book": "book", "booklet": "pamphlet", "conference": "paper-conference",
	"inbook": "chapter", "incollection": "chapter", "inproceedings": "paper-conference", "manual": "book",
	"mastersthesis": "thesis", "phdthesis": "thesis", "thesis": "thesis", "proceedings": "book",
	"techreport": "report", "unpublished": "manuscript", "online": "webpage", "misc": "article",
}

func cslNames(raw string) []map[string]string {
	var names []map[string]string
	for _, rawName := range splitNames(raw) {
		name := Clean(rawName)
		if name == "" {
			continue
		}
		if strings.HasPrefix(rawName, "{") && strings.HasSuffix(rawName, "}") {
			names = append(names, map[string]string{"literal": name})
			continue
		}
		family, given, comma := strings.Cut(name, ",")
		if !comma {
			parts := strings.Fields(name)
			if len(parts) == 0 {
				continue
			}
			family = parts[len(parts)-1]
			given = strings.Join(parts[:len(parts)-1], " ")
		}
		person := map[string]string{"family": strings.TrimSpace(family)}
		if given = strings.TrimSpace(given); given != "" {
			person["given"] = given
		}
		names = append(names, person)
	}
	return names
}

func cslItem(entry Entry) map[string]any {
	item := map[string]any{"id": entry["key"], "type": cslTypes[entry["type"]]}
	if item["type"] == "" {
		item["type"] = "article"
	}
	for bibtex, csl := range map[string]string{
		"publisher": "publisher", "volume": "volume",
		"number": "issue", "pages": "page", "doi": "DOI", "url": "URL", "isbn": "ISBN", "issn": "ISSN",
		"abstract": "abstract", "language": "language", "edition": "edition", "chapter": "chapter-number",
		"series": "collection-title", "note": "note", "keywords": "keyword",
	} {
		if value := Clean(entry[bibtex]); value != "" {
			item[csl] = value
		}
	}
	if title := Clean(entry["title"]); title != "" {
		if subtitle := Clean(entry["subtitle"]); subtitle != "" {
			title += ": " + subtitle
		}
		item["title"] = title
	}
	if venue := Clean(firstNonempty(entry["journal"], entry["booktitle"])); venue != "" {
		item["container-title"] = venue
	}
	if place := Clean(firstNonempty(entry["address"], entry["location"])); place != "" {
		item["publisher-place"] = place
	}
	if _, present := item["publisher"]; !present {
		if publisher := Clean(firstNonempty(entry["institution"], entry["school"], entry["organization"])); publisher != "" {
			item["publisher"] = publisher
		}
	}
	for bibtex, csl := range map[string]string{"author": "author", "editor": "editor", "translator": "translator"} {
		if names := cslNames(entry[bibtex]); len(names) > 0 {
			item[csl] = names
		}
	}
	if year, err := strconv.Atoi(strings.TrimSpace(entry["year"])); err == nil {
		date := []int{year}
		if month, err := strconv.Atoi(strings.TrimSpace(entry["month"])); err == nil && month >= 1 && month <= 12 {
			date = append(date, month)
		}
		item["issued"] = map[string]any{"date-parts": [][]int{date}}
	}
	return item
}

func runCiteproc(binary, stylePath, locale string, entries []Record, citations []cslCitation) (cslResult, error) {
	style, err := loadStyleWithParents(stylePath, 0)
	if err != nil {
		return cslResult{}, err
	}
	input := cslInput{Style: style, Lang: locale, Citations: citations}
	for _, record := range entries {
		input.References = append(input.References, cslItem(record.Entry))
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return cslResult{}, err
	}
	if binary == "" {
		binary = "citeproc"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary)
	command.Stdin = bytes.NewReader(encoded)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return cslResult{}, fmt.Errorf("citeproc timed out: %w", ctx.Err())
		}
		return cslResult{}, fmt.Errorf("citeproc failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var result cslResult
	if err := json.Unmarshal(output, &result); err != nil {
		return cslResult{}, fmt.Errorf("decode citeproc output: %w", err)
	}
	if len(result.Citations) != len(citations) {
		return cslResult{}, fmt.Errorf("citeproc returned %d citations for %d requests", len(result.Citations), len(citations))
	}
	return result, nil
}

func loadStyleWithParents(style string, depth int) (string, error) {
	if depth > 8 {
		return "", fmt.Errorf("CSL style parent chain is too deep")
	}
	content, err := loadStyle(style)
	if err != nil {
		return "", err
	}
	decoder := xml.NewDecoder(strings.NewReader(content))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return content, nil
		}
		if err != nil {
			return "", fmt.Errorf("parse CSL style %q: %w", style, err)
		}
		link, ok := token.(xml.StartElement)
		if !ok || link.Name.Local != "link" {
			continue
		}
		var relation, href string
		for _, attribute := range link.Attr {
			switch attribute.Name.Local {
			case "rel":
				relation = attribute.Value
			case "href":
				href = attribute.Value
			}
		}
		if relation == "independent-parent" {
			parent := href
			if strings.HasPrefix(href, "http://www.zotero.org/styles/") || strings.HasPrefix(href, "https://www.zotero.org/styles/") {
				parent = href[strings.LastIndex(href, "/")+1:]
			}
			if parent == style || parent == "" {
				return "", fmt.Errorf("invalid parent in CSL style %q", style)
			}
			return loadStyleWithParents(parent, depth+1)
		}
	}
}

func loadStyle(style string) (string, error) {
	if style == "apa" {
		return bundledAPA, nil
	}
	if cached, ok := styleCache.Load(style); ok {
		return cached.(string), nil
	}
	if strings.HasPrefix(style, "https://") {
		content, err := downloadStyle(style)
		if err == nil {
			styleCache.Store(style, content)
		}
		return content, err
	}
	paths := []string{style}
	if filepath.Ext(style) == "" {
		paths = append(paths, filepath.Join("styles", style+".csl"))
		if directory := os.Getenv("CSL_STYLE_DIR"); directory != "" {
			paths = append(paths, filepath.Join(directory, style+".csl"))
		}
	}
	for _, path := range paths {
		if content, err := os.ReadFile(path); err == nil {
			return string(content), nil
		}
	}
	if filepath.Ext(style) == "" && !strings.ContainsAny(style, `/\`) {
		content, err := downloadStyle("https://raw.githubusercontent.com/citation-style-language/styles/master/" + url.PathEscape(style) + ".csl")
		if err == nil {
			styleCache.Store(style, content)
			return content, nil
		}
		return "", fmt.Errorf("CSL style %q not found locally or in the official style repository: %w", style, err)
	}
	return "", fmt.Errorf("CSL style %q not found; provide a .csl file, HTTPS URL, or style in styles/", style)
}

func downloadStyle(address string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("download CSL style: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download CSL style: HTTP %d", response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil {
		return "", err
	}
	if len(content) > 2<<20 {
		return "", fmt.Errorf("CSL style exceeds 2 MiB")
	}
	return string(content), nil
}

func formatWithCSL(options Options, entries []Record) error {
	var citations []cslCitation
	for _, record := range entries {
		citations = append(citations, cslCitation{Items: []cslCitationItem{{ID: record.Entry["key"]}}})
	}
	result, err := runCiteproc(options.CiteprocPath, options.Style, options.Locale, entries, citations)
	if err != nil {
		return err
	}
	references := make(map[string]string)
	for _, pair := range result.Bibliography {
		if len(pair) != 2 {
			return fmt.Errorf("citeproc returned malformed bibliography entry")
		}
		references[pair[0]] = pair[1]
	}
	for index := range entries {
		key := entries[index].Entry["key"]
		entries[index].Entry["csl_citation"] = result.Citations[index]
		if reference, ok := references[key]; ok {
			entries[index].Entry["reference"] = reference
		} else {
			return fmt.Errorf("citeproc omitted bibliography entry %q", key)
		}
	}
	return nil
}
