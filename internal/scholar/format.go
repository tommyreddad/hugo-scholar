package scholar

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var latexCommands = strings.NewReplacer(
	`\&`, "&", `\%`, "%", `\_`, "_", `\ss`, "ß", `\ae`, "æ", `\oe`, "œ", `\aa`, "å", `\o`, "ø",
)
var latexAccent = regexp.MustCompile(`\\(['\x60"^~ck])\s*\{?([A-Za-z])\}?`)
var latexCommand = regexp.MustCompile(`\\[A-Za-z]+\s*`)

func Clean(value string) string {
	value = latexCommands.Replace(value)
	value = latexAccent.ReplaceAllStringFunc(value, func(raw string) string {
		parts := latexAccent.FindStringSubmatch(raw)
		accent := parts[1]
		letter := parts[2]
		composed := map[string]string{
			"'a": "á", "'e": "é", "'i": "í", "'o": "ó", "'u": "ú", "'A": "Á", "'E": "É", "'I": "Í", "'O": "Ó", "'U": "Ú",
			"`a": "à", "`e": "è", "`i": "ì", "`o": "ò", "`u": "ù", "^a": "â", "^e": "ê", "^i": "î", "^o": "ô", "^u": "û",
			"\"a": "ä", "\"e": "ë", "\"i": "ï", "\"o": "ö", "\"u": "ü", "\"A": "Ä", "\"O": "Ö", "\"U": "Ü",
			"~a": "ã", "~n": "ñ", "~o": "õ", "~N": "Ñ", "ka": "ą", "kA": "Ą", "cc": "ç", "cC": "Ç",
		}[accent+letter]
		if composed != "" {
			return composed
		}
		return letter
	})
	value = latexCommand.ReplaceAllString(value, "")
	value = strings.NewReplacer("{", "", "}", "", "~", " ").Replace(value)
	return strings.TrimSpace(value)
}

type personName struct {
	family  string
	given   string
	suffix  string
	literal string
}

// Interpret names before cleaning away braces that distinguish literal authors.
func parseNames(value string) []personName {
	var names []personName
	for _, raw := range splitNames(value) {
		name := Clean(raw)
		if name == "" {
			continue
		}
		if isLiteralName(raw) {
			names = append(names, personName{literal: name})
			continue
		}
		parts := strings.SplitN(name, ",", 3)
		person := personName{}
		switch len(parts) {
		case 1:
			words := strings.Fields(name)
			person.family = words[len(words)-1]
			person.given = strings.Join(words[:len(words)-1], " ")
		case 2:
			person.family = strings.TrimSpace(parts[0])
			person.given = strings.TrimSpace(parts[1])
		case 3:
			person.family = strings.TrimSpace(parts[0])
			person.suffix = strings.TrimSpace(parts[1])
			person.given = strings.TrimSpace(parts[2])
		}
		names = append(names, person)
	}
	return names
}

func isLiteralName(raw string) bool {
	if !strings.HasPrefix(raw, "{") || !strings.HasSuffix(raw, "}") {
		return false
	}
	depth := 0
	for index, char := range raw {
		switch char {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index == len(raw)-1
			}
		}
	}
	return false
}

func splitNames(value string) []string {
	var names []string
	start, depth := 0, 0
	runes := []rune(value)
	for index := 0; index < len(runes); index++ {
		switch runes[index] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && index > 0 && index+3 < len(runes) &&
			unicode.IsSpace(runes[index-1]) && unicode.IsSpace(runes[index+3]) &&
			strings.EqualFold(string(runes[index:index+3]), "and") {
			names = append(names, strings.TrimSpace(string(runes[start:index])))
			index += 2
			start = index + 1
		}
	}
	names = append(names, strings.TrimSpace(string(runes[start:])))
	return names
}

func nameSort(entry Entry) string {
	var names []string
	for _, name := range parseNames(firstNonempty(entry["author"], entry["editor"])) {
		if name.literal != "" {
			names = append(names, name.literal)
		} else {
			key := name.family + ", " + name.given
			if name.suffix != "" {
				key += ", " + name.suffix
			}
			names = append(names, key)
		}
	}
	return strings.ToLower(firstNonempty(strings.Join(names, "; "), Clean(firstNonempty(entry["bibtex_key"], entry["institution"], entry["organization"], entry["publisher"]))))
}

// URL syntax is not TeX prose: in particular, a literal tilde is not a space.
func cleanURL(value string) string {
	return strings.TrimSpace(strings.NewReplacer(`\&`, "&", `\%`, "%", `\_`, "_", `\#`, "#", `\~{}`, "~", `\~`, "~").Replace(value))
}

func authorCitation(entry Entry) string {
	var authors []string
	for _, name := range parseNames(firstNonempty(entry["author"], entry["editor"], entry["organization"])) {
		authors = append(authors, firstNonempty(name.literal, name.family))
	}
	switch len(authors) {
	case 0:
		return Clean(firstNonempty(entry["title"], entry["key"]))
	case 1:
		return authors[0]
	case 2:
		return authors[0] + " & " + authors[1]
	default:
		return authors[0] + " et al."
	}
}

func authorReference(entry Entry) string {
	authors := parseNames(firstNonempty(entry["author"], entry["editor"]))
	var formatted []string
	for _, author := range authors {
		if author.literal != "" {
			formatted = append(formatted, author.literal)
			continue
		}
		var initials []string
		for _, part := range strings.Fields(author.given) {
			var pieces []string
			for _, piece := range strings.Split(part, "-") {
				if piece != "" {
					pieces = append(pieces, string([]rune(piece)[0])+".")
				}
			}
			initials = append(initials, strings.Join(pieces, "-"))
		}
		name := strings.TrimSpace(strings.Join(initials, " ") + " " + author.family)
		if author.suffix != "" {
			suffix := author.suffix
			if strings.EqualFold(suffix, "Jr") || strings.EqualFold(suffix, "Sr") {
				suffix += "."
			}
			name += ", " + suffix
		}
		formatted = append(formatted, name)
	}
	switch len(formatted) {
	case 0:
		return Clean(entry["organization"])
	case 1:
		return formatted[0]
	case 2:
		return formatted[0] + " and " + formatted[1]
	default:
		return strings.Join(formatted[:len(formatted)-1], ", ") + ", and " + formatted[len(formatted)-1]
	}
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func Reference(entry Entry) string {
	escape := func(field string) string { return html.EscapeString(Clean(entry[field])) }
	var parts []string
	if author := authorReference(entry); author != "" {
		if !strings.HasSuffix(author, ".") {
			author += "."
		}
		parts = append(parts, html.EscapeString(author))
	}
	if title := escape("title"); title != "" {
		if url := referenceURL(entry); url != "" {
			title = `<a href="` + html.EscapeString(url) + `">` + title + `</a>`
		}
		if entry["type"] == "book" || strings.HasSuffix(entry["type"], "thesis") {
			parts = append(parts, "<i>"+title+"</i>.")
		} else {
			parts = append(parts, title+".")
		}
	}
	venue := html.EscapeString(Clean(firstNonempty(entry["shortjournal"], entry["journal_abbrev"], entry["journal"], entry["booktitle"], entry["publisher"], entry["school"])))
	if venue != "" {
		if entry["type"] == "article" {
			venue = "<i>" + venue + "</i>"
			var details string
			if volume := escape("volume"); volume != "" {
				details = volume
				if issue := escape("number"); issue != "" {
					details += "(" + issue + ")"
				}
			}
			if pages := html.EscapeString(strings.ReplaceAll(Clean(entry["pages"]), "--", "–")); pages != "" {
				if details != "" {
					details += ":" + pages
				} else {
					details = "pp. " + pages
				}
			}
			if details != "" {
				venue += ", " + details
			} else if strings.EqualFold(Clean(entry["journal"]), "arXiv e-prints") && entry["eprint"] != "" {
				venue += ", abs/" + escape("eprint")
			}
			parts = append(parts, venue+",")
		} else {
			parts = append(parts, venue+".")
		}
	}
	year := escape("year")
	if year == "" {
		year = "n.d"
	}
	parts = append(parts, year+".")
	return strings.Join(parts, " ")
}

func referenceURL(entry Entry) string {
	if url := cleanURL(entry["url"]); strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return url
	}
	if doi := cleanURL(entry["doi"]); doi != "" {
		if strings.HasPrefix(doi, "https://") || strings.HasPrefix(doi, "http://") {
			return doi
		}
		return "https://doi.org/" + doi
	}
	return ""
}

type Data struct {
	Bibliographies map[string][]Record                   `json:"bibliographies"`
	Pages          map[string]PageRender                 `json:"pages,omitempty"`
	QueryMatches   map[string]map[string]map[string]bool `json:"query_matches,omitempty"`
	Style          string                                `json:"style"`
	Bibliography   string                                `json:"bibliography"`
	files          map[string][]byte
}

type Record struct {
	Entry     Entry
	Links     map[string]string
	DetailURL string
	CSLOrder  int
}

func (r Record) MarshalJSON() ([]byte, error) {
	fields := make(map[string]any, len(r.Entry)+2)
	for key, value := range r.Entry {
		fields[key] = value
	}
	if len(r.Links) > 0 {
		fields["links"] = r.Links
	}
	if r.DetailURL != "" {
		fields["detail_url"] = r.DetailURL
	}
	if r.CSLOrder > 0 {
		fields["csl_order"] = r.CSLOrder
	}
	return json.Marshal(fields)
}

type Options struct {
	Source               string
	DefaultBibliography  string
	Repository           string
	RepositoryURL        string
	RepositoryDelimiter  string
	DetailsDir           string
	DetailsPermalink     string
	Style                string
	RemoveDuplicates     bool
	Query                string
	Locale               string
	AllowLocaleOverrides bool
	CiteprocPath         string
	ContentDir           string
}

func Prepare(source string) (Data, error) {
	return PrepareWithOptions(Options{Source: source})
}

func PrepareWithOptions(options Options) (Data, error) {
	if options.Source == "" {
		options.Source = "_bibliography"
	}
	if options.DefaultBibliography == "" {
		options.DefaultBibliography = "references"
	}
	if options.Style == "" {
		options.Style = "basic"
	}
	if options.ContentDir == "" {
		options.ContentDir = "content"
	}
	data := Data{Bibliographies: map[string][]Record{}, Style: options.Style, Bibliography: options.DefaultBibliography, QueryMatches: map[string]map[string]map[string]bool{}, files: map[string][]byte{}}
	sourceInfo, err := os.Stat(options.Source)
	if err != nil && !os.IsNotExist(err) {
		return Data{}, err
	}
	var files []string
	if sourceInfo != nil {
		err = filepath.WalkDir(options.Source, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && !isEditorLock(path) && (filepath.Ext(path) == ".bib" || filepath.Ext(path) == ".bibtex") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return Data{}, err
		}
	}
	sort.Strings(files)
	for _, path := range files {
		relative, err := filepath.Rel(options.Source, path)
		if err != nil {
			return Data{}, err
		}
		if !sourceInfo.IsDir() {
			relative = filepath.Base(path)
		}
		name := strings.TrimSuffix(filepath.ToSlash(relative), filepath.Ext(relative))
		if _, exists := data.Bibliographies[name]; exists {
			return Data{}, fmt.Errorf("multiple BibTeX files named %q", name)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return Data{}, err
		}
		entries, err := Parse(string(content))
		if err != nil {
			return Data{}, fmt.Errorf("%s: %w", path, err)
		}
		records, err := prepareRecords(options, name, entries)
		if err != nil {
			return Data{}, fmt.Errorf("bibliography %q: %w", name, err)
		}
		data.Bibliographies[name] = records
	}
	if err := convertContentBib(options, &data); err != nil {
		return Data{}, err
	}
	if len(data.Bibliographies) == 0 {
		return Data{}, fmt.Errorf("no .bib or .bibtex files in %s or %s", options.Source, firstNonempty(options.ContentDir, "content"))
	}
	if options.DetailsDir != "" {
		entries, exists := data.Bibliographies[options.DefaultBibliography]
		if !exists {
			return Data{}, fmt.Errorf("default bibliography %q is missing", options.DefaultBibliography)
		}
		if err := prepareDetails(options.ContentDir, options.DetailsDir, entries, data.files); err != nil {
			return Data{}, err
		}
	}
	if err := renderPages(options, &data); err != nil {
		return Data{}, err
	}
	return data, nil
}

func prepareRecords(options Options, name string, entries []Entry) ([]Record, error) {
	records := make([]Record, 0, len(entries))
	for _, entry := range entries {
		if month := entry["month"]; month != "" {
			entry["month_numeric"] = month
		}
		year := Clean(entry["year"])
		if year == "" {
			year = "n.d."
		}
		entry["name_sort"] = nameSort(entry)
		entry["citation"] = authorCitation(entry) + ", " + year
		entry["basic_citation"] = fmt.Sprintf("[%d]", len(records)+1)
		if options.Style == "basic" {
			entry["citation"] = entry["basic_citation"]
		}
		entry["basic_reference"] = fmt.Sprintf("[%d] %s", len(records)+1, Reference(entry))
		entry["reference"] = entry["basic_reference"]
		record := Record{Entry: entry}
		if options.Repository != "" {
			links, err := repositoryLinks(options, entry["key"])
			if err != nil {
				return nil, err
			}
			record.Links = links
		}
		if options.DetailsDir != "" && name == options.DefaultBibliography {
			record.DetailURL = detailURL(options, entry)
		}
		records = append(records, record)
	}
	if options.Style != "" && options.Style != "basic" {
		if err := formatWithCSL(options, records); err != nil {
			return nil, err
		}
	}
	return records, nil
}

// Write commits the prepared JSON and generated content as one operation.
func Write(path string, data Data) error {
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	output, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	files := make(map[string][]byte, len(data.files)+1)
	for filename, source := range data.files {
		filename, err = filepath.Abs(filename)
		if err != nil {
			return err
		}
		if filename == output {
			return fmt.Errorf("JSON output %s overlaps generated content", path)
		}
		files[filename] = source
	}
	files[output] = append(content, '\n')
	return writeFiles(files)
}
