package scholar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
		"number": "issue", "pages": "page", "isbn": "ISBN", "issn": "ISSN", "bibtex_type": "genre",
		"abstract": "abstract", "language": "language", "edition": "edition", "chapter": "chapter-number",
		"series": "collection-title", "note": "note", "keywords": "keyword",
	} {
		if value := Clean(entry[bibtex]); value != "" {
			item[csl] = value
		}
	}
	for bibtex, csl := range map[string]string{"doi": "DOI", "url": "URL"} {
		if value := cleanURL(entry[bibtex]); value != "" {
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

func formatWithCSL(options Options, entries []Record) error {
	if len(entries) == 0 {
		return nil
	}
	var citations []cslCitation
	for _, record := range entries {
		citations = append(citations, cslCitation{Items: []cslCitationItem{{ID: record.Entry["key"]}}})
	}
	result, err := runCiteproc(options.CiteprocPath, options.Style, options.Locale, entries, citations)
	if err != nil {
		return err
	}
	if len(result.Bibliography) == 0 {
		return fmt.Errorf("CSL style %q produced no bibliography; the generator's default style must provide bibliography output", options.Style)
	}
	if err := applyLocaleOverrides(options, options.Style, entries, citations, &result); err != nil {
		return err
	}
	linkifyCSLReferences(&result, entries)
	references := make(map[string]string)
	order := make(map[string]int)
	for index, pair := range result.Bibliography {
		if len(pair) != 2 {
			return fmt.Errorf("citeproc returned malformed bibliography entry")
		}
		references[pair[0]] = pair[1]
		order[pair[0]] = index + 1
	}
	for index := range entries {
		key := entries[index].Entry["key"]
		entries[index].Entry["csl_citation"] = result.Citations[index]
		entries[index].CSLOrder = order[key]
		if reference, ok := references[key]; ok {
			entries[index].Entry["reference"] = reference
		} else {
			return fmt.Errorf("citeproc omitted bibliography entry %q", key)
		}
	}
	return nil
}

func linkifyCSLReferences(result *cslResult, entries []Record) {
	byKey := make(map[string]Entry, len(entries))
	for _, record := range entries {
		byKey[record.Entry["key"]] = record.Entry
	}
	for _, pair := range result.Bibliography {
		if len(pair) == 2 {
			pair[1] = linkifyCSLReference(pair[1], byKey[pair[0]])
		}
	}
}

// Link only URLs supplied by the entry, and only where citeproc printed them
// as text. Preserve any markup or links already present in the CSL output.
func linkifyCSLReference(reference string, entry Entry) string {
	var urls []string
	if doi := cleanURL(entry["doi"]); doi != "" {
		if !strings.HasPrefix(doi, "http://") && !strings.HasPrefix(doi, "https://") {
			doi = "https://doi.org/" + doi
		}
		urls = append(urls, doi)
	}
	if address := cleanURL(entry["url"]); address != "" {
		urls = append(urls, address)
	}
	valid := urls[:0]
	for _, address := range urls {
		parsed, err := url.Parse(address)
		if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			valid = append(valid, address)
		}
	}
	if len(valid) == 0 {
		return reference
	}

	var output strings.Builder
	inAnchor := false
	writeText := func(source string) {
		if inAnchor {
			output.WriteString(source)
			return
		}
		decoded := html.UnescapeString(source)
		if !strings.Contains(decoded, "http") {
			output.WriteString(source)
			return
		}
		for len(decoded) > 0 {
			position, target := -1, ""
			for _, address := range valid {
				if index := strings.Index(decoded, address); index >= 0 && (position < 0 || index < position || (index == position && len(address) > len(target))) {
					position, target = index, address
				}
			}
			if position < 0 {
				output.WriteString(html.EscapeString(decoded))
				break
			}
			output.WriteString(html.EscapeString(decoded[:position]))
			output.WriteString(`<a href="`)
			output.WriteString(html.EscapeString(target))
			output.WriteString(`">`)
			output.WriteString(html.EscapeString(target))
			output.WriteString(`</a>`)
			decoded = decoded[position+len(target):]
		}
	}
	for len(reference) > 0 {
		start := strings.IndexByte(reference, '<')
		if start < 0 {
			writeText(reference)
			break
		}
		writeText(reference[:start])
		reference = reference[start:]
		end := strings.IndexByte(reference, '>')
		if end < 0 {
			writeText(reference)
			break
		}
		tag := reference[:end+1]
		lower := strings.ToLower(tag)
		if strings.HasPrefix(lower, "<a ") || lower == "<a>" {
			inAnchor = true
		} else if strings.HasPrefix(lower, "</a") {
			inAnchor = false
		}
		output.WriteString(tag)
		reference = reference[end+1:]
	}
	return output.String()
}

func applyLocaleOverrides(options Options, style string, entries []Record, citations []cslCitation, result *cslResult) error {
	if !options.AllowLocaleOverrides {
		return nil
	}
	byLocale := make(map[string]map[string]bool)
	for _, record := range entries {
		locale := strings.TrimSpace(record.Entry["language"])
		if locale == "" || strings.EqualFold(locale, options.Locale) {
			continue
		}
		if byLocale[locale] == nil {
			byLocale[locale] = make(map[string]bool)
		}
		byLocale[locale][record.Entry["key"]] = true
	}
	var locales []string
	for locale := range byLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	indices := make(map[string]int, len(result.Bibliography))
	for index, pair := range result.Bibliography {
		if len(pair) != 2 {
			return fmt.Errorf("citeproc returned malformed bibliography entry")
		}
		indices[pair[0]] = index
	}
	for _, locale := range locales {
		localized, err := runCiteproc(options.CiteprocPath, style, locale, entries, citations)
		if err != nil {
			return fmt.Errorf("locale %q: %w", locale, err)
		}
		for _, pair := range localized.Bibliography {
			if len(pair) != 2 {
				return fmt.Errorf("citeproc returned malformed bibliography entry for locale %q", locale)
			}
			if byLocale[locale][pair[0]] {
				index, ok := indices[pair[0]]
				if !ok {
					return fmt.Errorf("citeproc omitted bibliography entry %q", pair[0])
				}
				result.Bibliography[index][1] = pair[1]
			}
		}
	}
	return nil
}
