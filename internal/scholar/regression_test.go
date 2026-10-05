package scholar

import (
	"reflect"
	"strings"
	"testing"
)

func TestBibTeXFieldsPreserveIdentity(t *testing.T) {
	entries, err := Parse(`@inproceedings{child,crossref=parent,title={Child}}
@techreport{parent,key={Sort key},type={Research Report},year=2024}`)
	if err != nil {
		t.Fatal(err)
	}
	parent, child := entries[1], entries[0]
	if parent["key"] != "parent" || parent["type"] != "techreport" || parent["bibtex_key"] != "Sort key" || parent["bibtex_type"] != "Research Report" {
		t.Fatalf("metadata overwritten: %#v", parent)
	}
	if child["key"] != "child" || child["type"] != "inproceedings" || child["year"] != "2024" {
		t.Fatalf("crossref lost identity or inheritance: %#v", child)
	}
	item := cslItem(parent)
	if item["id"] != "parent" || item["type"] != "report" || item["genre"] != "Research Report" {
		t.Fatalf("incorrect CSL metadata: %#v", item)
	}
}

func TestAuthorWhitespaceAndSort(t *testing.T) {
	for _, separator := range []string{" and ", " and\n", "\tand\t", "\r\nAND\r\n", "\u2003and\u2003"} {
		got := cslNames("Alpha, Ann" + separator + "Beta, Bob")
		want := []map[string]string{{"family": "Alpha", "given": "Ann"}, {"family": "Beta", "given": "Bob"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("separator %q: got %#v", separator, got)
		}
		entry := Entry{"author": "Alpha, Ann" + separator + "Beta, Bob"}
		if got := authorReference(entry); got != "A. Alpha and B. Beta" {
			t.Errorf("separator %q: basic authors = %q", separator, got)
		}
		if got := authorCitation(entry); got != "Alpha & Beta" {
			t.Errorf("separator %q: citation = %q", separator, got)
		}
	}
	names := cslNames("{Research and\nDevelopment} and\nDoe, Jane")
	if len(names) != 2 || names[0]["literal"] != "Research and\nDevelopment" {
		t.Fatalf("corporate author split: %#v", names)
	}
	for _, pair := range [][2]string{{"Zoe Adams", "Adams, Zoe"}, {"Amy Zeller", "Zeller, Amy"}} {
		if nameSort(Entry{"author": pair[0]}) != nameSort(Entry{"author": pair[1]}) {
			t.Errorf("equivalent name formats sort differently: %v", pair)
		}
	}
	if nameSort(Entry{"author": "Zoe Adams"}) >= nameSort(Entry{"author": "Amy Zeller"}) {
		t.Fatal("names are not sorted by family")
	}
}

func TestPersonalNameComponents(t *testing.T) {
	for _, test := range []struct {
		raw       string
		reference string
		citation  string
		sort      string
		csl       map[string]string
	}{
		{"Smith, Jr, John", "J. Smith, Jr.", "Smith", "smith, john, jr", map[string]string{"family": "Smith", "given": "John", "suffix": "Jr"}},
		{"Smith, Jr., John", "J. Smith, Jr.", "Smith", "smith, john, jr.", map[string]string{"family": "Smith", "given": "John", "suffix": "Jr."}},
		{"Smith, III, John", "J. Smith, III", "Smith", "smith, john, iii", map[string]string{"family": "Smith", "given": "John", "suffix": "III"}},
		{"Doe, Jane", "J. Doe", "Doe", "doe, jane", map[string]string{"family": "Doe", "given": "Jane"}},
		{"Jane Doe", "J. Doe", "Doe", "doe, jane", map[string]string{"family": "Doe", "given": "Jane"}},
		{"Élodie Durand", "É. Durand", "Durand", "durand, élodie", map[string]string{"family": "Durand", "given": "Élodie"}},
		{"Jean-Paul Sartre", "J.-P. Sartre", "Sartre", "sartre, jean-paul", map[string]string{"family": "Sartre", "given": "Jean-Paul"}},
		{"Doe, {Jane}", "J. Doe", "Doe", "doe, jane", map[string]string{"family": "Doe", "given": "Jane"}},
		{"{Jean-Paul} {Sartre}", "J.-P. Sartre", "Sartre", "sartre, jean-paul", map[string]string{"family": "Sartre", "given": "Jean-Paul"}},
		{"{Doe, Jane}", "Doe, Jane", "Doe, Jane", "doe, jane", map[string]string{"literal": "Doe, Jane"}},
	} {
		t.Run(test.raw, func(t *testing.T) {
			entries, err := Parse("@book{a,author={" + test.raw + "},title={Book}}")
			if err != nil {
				t.Fatal(err)
			}
			entry := entries[0]
			if got := authorReference(entry); got != test.reference {
				t.Errorf("basic authors = %q, want %q", got, test.reference)
			}
			if got := Reference(entry); !strings.HasPrefix(got, strings.TrimSuffix(test.reference, ".")+". ") {
				t.Errorf("basic reference author punctuation = %q", got)
			}
			if got := authorCitation(entry); got != test.citation {
				t.Errorf("citation = %q, want %q", got, test.citation)
			}
			if got := nameSort(entry); got != test.sort {
				t.Errorf("sort key = %q, want %q", got, test.sort)
			}
			if got := cslItem(entry)["author"]; !reflect.DeepEqual(got, []map[string]string{test.csl}) {
				t.Errorf("CSL name = %#v, want %#v", got, test.csl)
			}
		})
	}
	if nameSort(Entry{"author": "Smith, Jr, Adam"}) >= nameSort(Entry{"author": "Smith, Amy"}) {
		t.Fatal("suffix was sorted as a given name")
	}
	if nameSort(Entry{"author": "Smith, Jr, John"}) == nameSort(Entry{"author": "Smith, John"}) {
		t.Fatal("suffix was lost from the sort key")
	}
}

func TestURLConversionPreservesSyntax(t *testing.T) {
	address := `https://example.org/~alice/paper?x=1\&y=2\#section`
	want := "https://example.org/~alice/paper?x=1&y=2#section"
	item := cslItem(Entry{"key": "a", "type": "book", "url": address, "doi": "10.1234/~paper"})
	if item["URL"] != want || item["DOI"] != "10.1234/~paper" {
		t.Fatalf("URL syntax changed: %#v", item)
	}
	if got := linkifyCSLReference(want, Entry{"url": address}); !strings.Contains(got, `href="https://example.org/~alice/paper?x=1&amp;y=2#section"`) {
		t.Fatalf("wrong link: %s", got)
	}
}

func TestShortcodeScannerMatchesNesting(t *testing.T) {
	source := `{{< cite "a" >}} {{< wrap >}} {{< wrap >}} {{< cite "b" >}} {{< /wrap >}} {{< cite "c" >}} {{< /wrap >}} {{< bibliography >}}`
	want := []string{"0", "1", "1/0", "1/0/0", "1/1", "2"}
	calls := scanShortcodes(source)
	var got []string
	for _, call := range calls {
		got = append(got, call.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
	// Quoted delimiters and escaped examples do not become shortcode calls.
	source = "{{</* cite \"unused\" */>}} {{< wrap label=`>}} {{< ignored >}}` >}}{{< cite keys=`b\na` >}}{{< /wrap >}} {{< cite \"c\" >}}"
	calls = scanShortcodes(source)
	if len(calls) != 3 || calls[1].ID != "0/0" || calls[1].Args["keys"] != "b\na" || calls[2].ID != "1" {
		t.Fatalf("quoted syntax affected parsing: %#v", calls)
	}
	if got := parseShortcodeArgs(`"a=b"`); got["0"] != "a=b" {
		t.Fatalf("positional argument mistaken for named argument: %v", got)
	}
}

func TestFrontMatterIsExcludedFromCitations(t *testing.T) {
	for _, source := range []string{
		"---\ndescription: '{{< cite \"ignored\" >}}'\n---\n",
		"+++\ndescription = '{{< cite \"ignored\" >}}'\n+++\n",
		"{\"description\":\"{{< cite \\\"ignored\\\" >}}\"}\n",
		"\ufeff---\r\ndescription: '{{< cite \"ignored\" >}}'\r\n---\r\n",
	} {
		_, body, err := splitFrontMatter(source + `{{< cite "real" >}}`)
		if err != nil {
			t.Fatal(err)
		}
		calls := scanShortcodes(body)
		if len(calls) != 1 || calls[0].Args["0"] != "real" || calls[0].ID != "0" {
			t.Fatalf("front matter treated as content: %#v", calls)
		}
	}
}

func TestMalformedEscapesReturnErrors(t *testing.T) {
	for _, source := range []string{"@book{x,title={abc\\", "@book{x,title=\"abc\\", "@comment{abc\\"} {
		if _, err := Parse(source); err == nil {
			t.Errorf("expected error for %q", source)
		}
	}
}

func TestCitationKeyBoundaries(t *testing.T) {
	for _, source := range []string{
		"@book{ à-key \n, title={One}}",
		"@book(à-key\t, title={One})",
	} {
		entries, err := Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0]["key"] != "à-key" || entries[0]["title"] != "One" {
			t.Fatalf("incorrect key/field boundary: %#v", entries)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, source := range []string{"@book{x,title={abc\\", "@book{x,title=\"abc\\", "@book{x,title={A},author={Doe, Jane}}", "@book{x,crossref=y} @book{y,crossref=x}", ""} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		_, _ = Parse(source)
	})
}
