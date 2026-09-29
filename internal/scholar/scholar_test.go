package scholar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("HUGO_SCHOLAR_TEST_CITEPROC") == "1" {
		var input cslInput
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		result := cslResult{}
		order := map[string]int{}
		for _, citation := range input.Citations {
			var numbers []string
			for _, item := range citation.Items {
				if order[item.ID] == 0 {
					order[item.ID] = len(order) + 1
				}
				number := fmt.Sprint(order[item.ID])
				if item.Locator != "" {
					number += " " + item.Label + " " + item.Locator
				}
				numbers = append(numbers, number)
			}
			result.Citations = append(result.Citations, "["+strings.Join(numbers, ", ")+"]")
		}
		for _, reference := range input.References {
			key := reference["id"].(string)
			formatted := fmt.Sprintf("[%d] %s", order[key], key)
			if os.Getenv("HUGO_SCHOLAR_TEST_LOCALE") == "1" {
				formatted += " " + input.Lang
			}
			result.Bibliography = append(result.Bibliography, []string{key, formatted})
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestParseStringsCrossrefsAndUnicode(t *testing.T) {
	source := `
@string{venue = "Proceedings of " # {A {Great} Meeting}}
@proceedings{parent, booktitle=venue, year=2020}
@inproceedings{child,
  author={Garc{\'i}a, Ana and Lovelace, Ada},
  title="A {Protected} Title",
  crossref=parent,
}`
	entries, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if entries[1]["booktitle"] != "Proceedings of A {Great} Meeting" || entries[1]["year"] != "2020" {
		t.Fatalf("crossref or string macro failed: %#v", entries[1])
	}
	if got := Clean(entries[1]["author"]); got != "García, Ana and Lovelace, Ada" {
		t.Fatalf("accent conversion = %q", got)
	}
}

func TestParseRejectsDuplicateAndMalformedEntries(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{`@book{same,title={One}} @book{same,title={Two}}`, "duplicate citation key"},
		{`@book{missing,title={One}`, "unclosed entry"},
		{`@book{a,crossref=b} @book{b,crossref=a}`, "crossref cycle"},
	} {
		_, err := Parse(test.source)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("Parse(%q) error = %v, want %q", test.source, err, test.want)
		}
	}
}

func TestTransitiveCrossref(t *testing.T) {
	entries, err := Parse(`@book{child,crossref=parent} @book{parent,crossref=grand} @book{grand,year=2020}`)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0]["year"] != "2020" {
		t.Fatalf("transitive crossref missing: %#v", entries[0])
	}
}

func TestPrepareFormatsAndEscapes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "references.bib"), []byte(`@book{x,author={Doe, Jane},title={A <Book>},year=2022,url={https://example.org}}`), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	entry := data.Bibliographies["references"][0].Entry
	if entry["citation"] != "[1]" || !strings.Contains(entry["reference"], "&lt;Book&gt;") || strings.Contains(entry["reference"], "<Book>") {
		t.Fatalf("unsafe or unexpected format: %#v", entry)
	}
	output := filepath.Join(dir, "data", "scholar.json")
	if err := WriteJSON(output, data); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(output); err != nil || !strings.Contains(string(content), `"bibliographies"`) {
		t.Fatalf("JSON output failed: %v", err)
	}
}

func TestBasicReferenceJournalDetails(t *testing.T) {
	entry := Entry{
		"type": "article", "author": "Lee, Morgan and Chen, Riley",
		"title": "A sample study of trees", "journal": "Journal of Sample Studies",
		"shortjournal": "J. Sample Stud.", "volume": "12", "number": "3",
		"pages": "101--118", "year": "2024", "url": "https://example.org/articles/sample-trees",
	}
	want := `M. Lee and R. Chen. <a href="https://example.org/articles/sample-trees">A sample study of trees</a>. <i>J. Sample Stud.</i>, 12(3):101–118, 2024.`
	if got := Reference(entry); got != want {
		t.Fatalf("basic reference = %q, want %q", got, want)
	}
}

func TestBasicReferenceArxivAndDOI(t *testing.T) {
	for _, test := range []struct {
		entry Entry
		want  string
	}{
		{Entry{"type": "article", "title": "A sample preprint", "journal": "arXiv e-prints", "eprint": "1234.56789", "year": "2023", "url": "https://example.org/preprints/sample"}, `<a href="https://example.org/preprints/sample">A sample preprint</a>. <i>arXiv e-prints</i>, abs/1234.56789, 2023.`},
		{Entry{"type": "article", "title": "A & B", "journal": "Journal of Sample Computing", "volume": "4", "number": "1", "pages": "1--11", "year": "2024", "doi": "10.1234/sample.2024"}, `<a href="https://doi.org/10.1234/sample.2024">A &amp; B</a>. <i>Journal of Sample Computing</i>, 4(1):1–11, 2024.`},
	} {
		if got := Reference(test.entry); got != test.want {
			t.Errorf("basic reference = %q, want %q", got, test.want)
		}
	}
}

func TestPrepareIgnoresDanglingEditorLocks(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "_bibliography")
	content := filepath.Join(dir, "content")
	for _, folder := range []string{source, content} {
		if err := os.Mkdir(folder, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "references.bib"), []byte(`@book{one,title={One}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(content, "_index.md"), []byte(`{{< cite "one" >}}`), 0644); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{
		filepath.Join(source, ".#references.bib"),
		filepath.Join(content, ".#reading.bib"),
		filepath.Join(content, ".#index.md"),
		filepath.Join(content, ".#_index.md"),
	} {
		if err := os.Symlink("missing-editor-buffer", filename); err != nil {
			t.Fatal(err)
		}
	}
	data, err := PrepareWithOptions(Options{Source: source, ContentDir: content, Style: "basic"})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Bibliographies) != 1 || len(data.Pages) != 1 || data.Pages["_index.md"].Cited["references"][0] != "one" {
		t.Fatalf("editor locks affected generated data: %#v", data)
	}
}

func TestCiteprocPageOrderAndLocators(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{"_bibliography", "content"} {
		if err := os.Mkdir(filepath.Join(dir, path), 0755); err != nil {
			t.Fatal(err)
		}
	}
	bib := `@book{first,title={First},author={Doe, Jane},year=2020}
@article{second,title={Second},author={Roe, Richard},year=2021}`
	if err := os.WriteFile(filepath.Join(dir, "_bibliography", "references.bib"), []byte(bib), 0644); err != nil {
		t.Fatal(err)
	}
	otherStyle := filepath.Join(dir, "other.csl")
	page := fmt.Sprintf(`---
title: Test
---
{{< bibliography_count >}}
{{< cite keys="second first" locator="42" label="page" >}}
{{< cite "first" >}}
{{< bibliography style=%q >}}
{{< cite keys="second" style=%q >}}`, otherStyle, otherStyle)
	if err := os.WriteFile(filepath.Join(dir, "content", "article.md"), []byte(page), 0644); err != nil {
		t.Fatal(err)
	}
	style := filepath.Join(dir, "style.csl")
	if err := os.WriteFile(style, []byte("<style></style>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherStyle, []byte("<style></style>"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HUGO_SCHOLAR_TEST_CITEPROC", "1")
	data, err := PrepareWithOptions(Options{
		Source: filepath.Join(dir, "_bibliography"), ContentDir: filepath.Join(dir, "content"),
		Style: style, Locale: "en-US", CiteprocPath: os.Args[0],
	})
	if err != nil {
		t.Fatal(err)
	}
	render := data.Pages["article.md"]
	if render.Citations["1"] != "[1 page 42, 2]" || render.Citations["2"] != "[2]" {
		t.Fatalf("wrong page citations: %#v", render.Citations)
	}
	if render.References["references"]["second"] != "[1] second" {
		t.Fatalf("wrong page references: %#v", render.References)
	}
	if render.StyledReferences[otherStyle]["references"]["second"] != "[1] second" {
		t.Fatalf("missing style override references: %#v", render.StyledReferences)
	}
	if render.StyledCitations[otherStyle]["4"] != "[1]" {
		t.Fatalf("missing style override citation: %#v", render.StyledCitations)
	}
}

func TestRepositoryAndDetailPages(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "_bibliography")
	repository := filepath.Join(dir, "static", "repository")
	for _, path := range []string{source, repository} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "references.bib"), []byte(`@book{paper,title={A Paper}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "paper.slides.pdf"), []byte("slides"), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := PrepareWithOptions(Options{Source: source, Repository: repository, RepositoryURL: "/repository", DetailsDir: "bibliography"})
	if err != nil {
		t.Fatal(err)
	}
	record := data.Bibliographies["references"][0]
	if record.Links["slides.pdf"] != "/repository/paper.slides.pdf" || record.DetailURL != "/bibliography/paper/" {
		t.Fatalf("wrong links: %#v", record)
	}
	content := filepath.Join(dir, "content")
	if err := WriteDetails(content, "bibliography", data.Bibliographies["references"]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(content, "bibliography", "paper.md")); err != nil {
		t.Fatal(err)
	}
	if err := WriteDetails(content, "bibliography", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(content, "bibliography", "paper.md")); !os.IsNotExist(err) {
		t.Fatalf("stale generated page remains: %v", err)
	}
	url := detailURL(Options{DetailsDir: "bibliography", DetailsPermalink: "/:details_dir/:year/:key/"}, Entry{"key": "Paper One", "year": "2024"})
	if url != "/bibliography/2024/paper-one/" {
		t.Fatalf("custom detail URL = %q", url)
	}
}

func TestCorporateCSLName(t *testing.T) {
	names := cslNames(`{World Health Organization} and Doe, Jane`)
	if len(names) != 2 || names[0]["literal"] != "World Health Organization" || names[1]["family"] != "Doe" {
		t.Fatalf("corporate and personal names: %#v", names)
	}
}

func TestBasicCitedOrderIncludesQuotes(t *testing.T) {
	dir := t.TempDir()
	for _, folder := range []string{"_bibliography", "content"} {
		if err := os.Mkdir(filepath.Join(dir, folder), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "_bibliography", "references.bib"), []byte(`@book{one,title={One}} @book{two,title={Two}}`), 0644); err != nil {
		t.Fatal(err)
	}
	page := `{{< bibliography cited=true >}} {{< quote "two" >}}quoted{{< /quote >}} {{< bibliography cited_in_order=true clear=true >}} {{< cite keys="one two" >}} {{< bibliography cited=true >}}`
	if err := os.WriteFile(filepath.Join(dir, "content", "page.md"), []byte(page), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := PrepareWithOptions(Options{Source: filepath.Join(dir, "_bibliography"), ContentDir: filepath.Join(dir, "content"), Style: "basic"})
	if err != nil {
		t.Fatal(err)
	}
	got := data.Pages["page.md"].Cited["references"]
	if strings.Join(got, ",") != "two,one" {
		t.Fatalf("cited order = %q", got)
	}
	if len(data.Pages["page.md"].CitedAt["0"].Keys) != 0 || strings.Join(data.Pages["page.md"].CitedAt["2"].Keys, ",") != "two" || strings.Join(data.Pages["page.md"].CitedAt["4"].Keys, ",") != "one,two" {
		t.Fatalf("cited snapshots = %#v", data.Pages["page.md"].CitedAt)
	}
}

func TestContentBibConversion(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "_bibliography")
	content := filepath.Join(dir, "content")
	for _, folder := range []string{source, content} {
		if err := os.Mkdir(folder, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "references.bib"), []byte(`@book{base,title={Base}}`), 0644); err != nil {
		t.Fatal(err)
	}
	page := "---\ntitle: Reading\n---\nContact reader@example.org.\n\n@string{booktitle={A {Book}}}\n@book{item,title=booktitle,year=2024}\n\nConclusion.\n"
	if err := os.WriteFile(filepath.Join(content, "reading.bib"), []byte(page), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := PrepareWithOptions(Options{Source: source, ContentDir: content, Style: "basic"})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Bibliographies["reading"]) != 1 {
		t.Fatalf("content bibliography missing: %#v", data.Bibliographies)
	}
	generated, err := os.ReadFile(filepath.Join(content, "reading.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "reader@example.org") || strings.Contains(string(generated), "@string") || !strings.Contains(string(generated), `{{< reference key="item" file="reading" block=true >}}`) || !strings.Contains(string(generated), "Conclusion.") {
		t.Fatalf("bad generated content: %s", generated)
	}
	if err := os.WriteFile(filepath.Join(content, "reading.md"), []byte("user page"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareWithOptions(Options{Source: source, ContentDir: content, Style: "basic"}); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected overwrite guard, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(content, "reading.md"), generated, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(content, "reading.bib")); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareWithOptions(Options{Source: source, ContentDir: content, Style: "basic"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(content, "reading.md")); !os.IsNotExist(err) {
		t.Fatalf("stale converted page remains: %v", err)
	}
}

func TestRealCiteprocNumericStyle(t *testing.T) {
	requireIntegrationTool(t, "citeproc")
	entries := []Record{
		{Entry: Entry{"key": "first", "type": "book", "title": "First", "author": "Doe, Jane", "year": "2020"}},
		{Entry: Entry{"key": "second", "type": "book", "title": "Second", "author": "Roe, Richard", "year": "2021"}},
	}
	result, err := runCiteproc("citeproc", filepath.Join("..", "..", "example", "styles", "numeric.csl"), "en-US", entries, []cslCitation{
		{Items: []cslCitationItem{{ID: "second"}}},
		{Items: []cslCitationItem{{ID: "first"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Citations[0] != "[1]" || result.Citations[1] != "[2]" {
		t.Fatalf("numeric citations: %#v", result.Citations)
	}
	if len(result.Bibliography) != 2 || result.Bibliography[0][0] != "second" {
		t.Fatalf("numeric bibliography: %#v", result.Bibliography)
	}
}

func TestRealCiteprocAPALinks(t *testing.T) {
	requireIntegrationTool(t, "citeproc")
	dir := t.TempDir()
	source := filepath.Join(dir, "_bibliography")
	content := filepath.Join(dir, "content")
	for _, folder := range []string{source, content} {
		if err := os.Mkdir(folder, 0755); err != nil {
			t.Fatal(err)
		}
	}
	bib := `@book{urlcase,title={URL Test},author={Doe, Jane},year={2024},url={https://example.org/full-text?x=1&y=2}}
@article{doicase,title={DOI Test},author={Roe, Ray},year={2023},doi={10.1234/a.b}}`
	if err := os.WriteFile(filepath.Join(source, "references.bib"), []byte(bib), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(content, "_index.md"), []byte(`{{< cite "urlcase" >}} {{< cite "doicase" >}} {{< bibliography >}}`), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := PrepareWithOptions(Options{Source: source, ContentDir: content, Style: "apa", Locale: "en-US"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ key, link string }{
		{"urlcase", `<a href="https://example.org/full-text?x=1&amp;y=2">https://example.org/full-text?x=1&amp;y=2</a>`},
		{"doicase", `<a href="https://doi.org/10.1234/a.b">https://doi.org/10.1234/a.b</a>`},
	} {
		var entryReference string
		for _, record := range data.Bibliographies["references"] {
			if record.Entry["key"] == test.key {
				entryReference = record.Entry["reference"]
			}
		}
		for _, reference := range []string{data.Pages["_index.md"].References["references"][test.key], entryReference} {
			if !strings.Contains(reference, test.link) {
				t.Fatalf("%s reference has no APA link: %s", test.key, reference)
			}
		}
	}
}

func TestLinkifyCSLReferenceKeepsExistingLinks(t *testing.T) {
	address := "https://example.org/article"
	reference := `<a href="https://example.org/article">https://example.org/article</a> and https://example.org/article`
	got := linkifyCSLReference(reference, Entry{"url": address})
	if strings.Count(got, `<a href="https://example.org/article">`) != 2 || strings.Contains(got, "<a href=\"https://example.org/article\"><a") {
		t.Fatalf("existing link was nested or plain URL remained: %s", got)
	}
}

func TestLocaleOverridesKeepCitationOrder(t *testing.T) {
	style := filepath.Join(t.TempDir(), "style.csl")
	if err := os.WriteFile(style, []byte("<style></style>"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HUGO_SCHOLAR_TEST_CITEPROC", "1")
	t.Setenv("HUGO_SCHOLAR_TEST_LOCALE", "1")
	entries := []Record{
		{Entry: Entry{"key": "english", "type": "book"}},
		{Entry: Entry{"key": "french", "type": "book", "language": "fr-FR"}},
	}
	citations := []cslCitation{{Items: []cslCitationItem{{ID: "french"}}}, {Items: []cslCitationItem{{ID: "english"}}}}
	result, err := runCiteproc(os.Args[0], style, "en-US", entries, citations)
	if err != nil {
		t.Fatal(err)
	}
	options := Options{AllowLocaleOverrides: true, CiteprocPath: os.Args[0], Locale: "en-US"}
	if err := applyLocaleOverrides(options, style, entries, citations, &result); err != nil {
		t.Fatal(err)
	}
	if result.Citations[0] != "[1]" || result.Bibliography[0][1] != "[2] english en-US" || result.Bibliography[1][1] != "[1] french fr-FR" {
		t.Fatalf("locale override changed order or wrong language: %#v", result)
	}
}

func TestNestedBibliographyNames(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "topics")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(nested, "books.bib")
	if err := os.WriteFile(file, []byte(`@book{one,title={One}}`), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := PrepareWithOptions(Options{Source: root, ContentDir: filepath.Join(root, "missing"), Style: "basic"})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Bibliographies["topics/books"]) != 1 {
		t.Fatalf("nested file missing: %#v", data.Bibliographies)
	}
	data, err = PrepareWithOptions(Options{Source: file, ContentDir: filepath.Join(root, "missing"), Style: "basic"})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Bibliographies["books"]) != 1 {
		t.Fatalf("single source file missing: %#v", data.Bibliographies)
	}
}

func TestContentOnlyBibliography(t *testing.T) {
	root := t.TempDir()
	content := filepath.Join(root, "content")
	if err := os.Mkdir(content, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(content, "reading.bib"), []byte(`@book{one,title={One}}`), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := PrepareWithOptions(Options{Source: filepath.Join(root, "missing"), ContentDir: content, Style: "basic"})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Bibliographies["reading"]) != 1 {
		t.Fatalf("content bibliography missing: %#v", data.Bibliographies)
	}
}

func TestEmptyBibContentFrontMatter(t *testing.T) {
	front, body, err := splitFrontMatter("---\n---\n@book{one,title={One}}")
	if err != nil || front != "---\n---\n" || body != "@book{one,title={One}}" {
		t.Fatalf("front matter split = %q, %q, %v", front, body, err)
	}
}
