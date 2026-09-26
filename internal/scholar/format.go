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

func people(value string) []string {
	var result []string
	for _, person := range splitNames(value) {
		if person = Clean(person); person != "" {
			result = append(result, person)
		}
	}
	return result
}

func splitNames(value string) []string {
	var names []string
	start, depth := 0, 0
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && strings.HasPrefix(value[index:], " and ") {
			names = append(names, strings.TrimSpace(value[start:index]))
			index += len(" and ") - 1
			start = index + 1
		}
	}
	names = append(names, strings.TrimSpace(value[start:]))
	return names
}

func surname(person string) string {
	if family, _, ok := strings.Cut(person, ","); ok {
		return strings.TrimSpace(family)
	}
	parts := strings.Fields(person)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func authorCitation(entry Entry) string {
	authors := people(firstNonempty(entry["author"], entry["editor"], entry["organization"]))
	switch len(authors) {
	case 0:
		return Clean(firstNonempty(entry["title"], entry["key"]))
	case 1:
		return surname(authors[0])
	case 2:
		return surname(authors[0]) + " & " + surname(authors[1])
	default:
		return surname(authors[0]) + " et al."
	}
}

func authorReference(entry Entry) string {
	authors := people(firstNonempty(entry["author"], entry["editor"]))
	var formatted []string
	for _, author := range authors {
		family, given, hasComma := strings.Cut(author, ",")
		if !hasComma {
			parts := strings.Fields(author)
			if len(parts) == 0 {
				continue
			}
			family = parts[len(parts)-1]
			given = strings.Join(parts[:len(parts)-1], " ")
		}
		var initials []string
		for _, part := range strings.Fields(strings.ReplaceAll(given, "-", " ")) {
			initials = append(initials, string([]rune(part)[0])+".")
		}
		name := strings.TrimSpace(family)
		if len(initials) > 0 {
			name += ", " + strings.Join(initials, " ")
		}
		formatted = append(formatted, name)
	}
	switch len(formatted) {
	case 0:
		return Clean(entry["organization"])
	case 1:
		return formatted[0]
	default:
		return strings.Join(formatted[:len(formatted)-1], ", ") + ", & " + formatted[len(formatted)-1]
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
		parts = append(parts, html.EscapeString(strings.TrimSuffix(author, "."))+".")
	}
	year := escape("year")
	if year == "" {
		year = "n.d."
	}
	parts = append(parts, "("+year+").")
	if title := escape("title"); title != "" {
		if entry["type"] == "book" || strings.HasSuffix(entry["type"], "thesis") {
			parts = append(parts, "<i>"+title+"</i>.")
		} else {
			parts = append(parts, title+".")
		}
	}
	venue := html.EscapeString(Clean(firstNonempty(entry["journal"], entry["booktitle"], entry["publisher"], entry["school"])))
	if venue != "" {
		if entry["type"] == "article" {
			parts = append(parts, "<i>"+venue+"</i>.")
		} else {
			parts = append(parts, venue+".")
		}
	}
	if doi := entry["doi"]; doi != "" {
		url := "https://doi.org/" + doi
		parts = append(parts, "<a href=\""+html.EscapeString(url)+"\">"+html.EscapeString(url)+"</a>")
	} else if url := entry["url"]; strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		parts = append(parts, "<a href=\""+html.EscapeString(url)+"\">"+html.EscapeString(url)+"</a>")
	}
	return strings.Join(parts, " ")
}

type Data struct {
	Bibliographies map[string][]Record   `json:"bibliographies"`
	Pages          map[string]PageRender `json:"pages,omitempty"`
	Style          string                `json:"style"`
}

type Record struct {
	Entry     Entry
	Links     map[string]string
	DetailURL string
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
	data := Data{Bibliographies: map[string][]Record{}, Style: options.Style}
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
			if !entry.IsDir() && (filepath.Ext(path) == ".bib" || filepath.Ext(path) == ".bibtex") {
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
	if err := renderPages(options, &data); err != nil {
		return Data{}, err
	}
	return data, nil
}

func prepareRecords(options Options, name string, entries []Entry) ([]Record, error) {
	var records []Record
	for _, entry := range entries {
		if month := entry["month"]; month != "" {
			entry["month_numeric"] = month
		}
		year := Clean(entry["year"])
		if year == "" {
			year = "n.d."
		}
		entry["name_sort"] = strings.ToLower(Clean(firstNonempty(entry["author"], entry["editor"], entry["institution"], entry["organization"], entry["publisher"])))
		entry["citation"] = authorCitation(entry) + ", " + year
		entry["basic_reference"] = Reference(entry)
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

func WriteJSON(path string, data Data) error {
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(content, '\n'), 0644)
}
