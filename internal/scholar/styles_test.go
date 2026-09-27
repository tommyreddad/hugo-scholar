package scholar

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stylesOffline(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("CSL_STYLE_DIR", "")
}

func writeStyle(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func dependentStyle(parent, locale string) string {
	return fmt.Sprintf(`<style xmlns="http://purl.org/net/xbiblio/csl" version="1.0" default-locale="%s"><info><link rel="independent-parent" href="%s"/></info></style>`, locale, parent)
}

func TestStarterStylesOffline(t *testing.T) {
	stylesOffline(t)
	files, err := bundledStyles.ReadDir("styles")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(file.Name(), ".csl")
		content, err := loadStyleWithParents(name, 0)
		if err != nil {
			t.Fatal(err)
		}
		var style struct {
			XMLName      xml.Name  `xml:"http://purl.org/net/xbiblio/csl style"`
			Citation     *struct{} `xml:"citation"`
			Bibliography *struct{} `xml:"bibliography"`
		}
		if err := xml.Unmarshal([]byte(content), &style); err != nil || style.Citation == nil || style.Bibliography == nil {
			t.Fatalf("invalid starter style %s: %v", name, err)
		}
	}
}

func TestLocalStyleLookupOffline(t *testing.T) {
	stylesOffline(t)
	for _, selection := range []string{"ieee", "ieee.csl"} {
		if _, err := loadStyle(selection); err != nil {
			t.Fatal(err)
		}
	}
	const site = `<style xmlns="http://purl.org/net/xbiblio/csl"><info><title>Site</title></info></style>`
	const shared = `<style xmlns="http://purl.org/net/xbiblio/csl"><info><title>Shared</title></info></style>`
	const direct = `<style xmlns="http://purl.org/net/xbiblio/csl"><info><title>Direct</title></info></style>`
	directory := t.TempDir()
	t.Setenv("CSL_STYLE_DIR", directory)
	writeStyle(t, filepath.Join(directory, "apa.csl"), shared)
	writeStyle(t, filepath.Join(directory, "custom.csl"), shared)
	writeStyle(t, "styles/apa.csl", site)
	writeStyle(t, "explicit.csl", direct)
	for _, test := range []struct{ name, want string }{
		{"apa", site}, {"styles/apa.csl", site}, {"custom", shared}, {"custom.csl", shared},
		{"explicit.csl", direct}, {filepath.Join(directory, "apa.csl"), shared},
	} {
		got, err := loadStyleWithParents(test.name, 0)
		if err != nil || got != test.want {
			t.Fatalf("%s: got %q, error %v", test.name, got, err)
		}
	}
	writeStyle(t, "apa", direct)
	if got, err := loadStyle("apa"); err != nil || got != direct {
		t.Fatalf("explicit path precedence: %v", err)
	}
	for _, missing := range []string{"not-a-real-style", "dependent/not-a-real-style", "styles/missing.csl"} {
		_, err := loadStyleWithParents(missing, 0)
		if err == nil || !strings.Contains(err.Error(), "searched") || !strings.Contains(err.Error(), directory) || !strings.Contains(err.Error(), "obtain the CSL file independently") {
			t.Fatalf("missing style %q: %v", missing, err)
		}
	}
}

func TestRemoteStyleURLsRejected(t *testing.T) {
	stylesOffline(t)
	for _, address := range []string{"https://styles.example.test/apa.csl", "http://styles.example.test/apa.csl", "//styles.example.test/apa.csl", "file:///tmp/apa.csl"} {
		if _, err := loadStyleWithParents(address, 0); err == nil || !strings.Contains(err.Error(), "does not download CSL files") {
			t.Fatalf("remote style %q: %v", address, err)
		}
	}
}

func TestLocalStyleParentsOffline(t *testing.T) {
	stylesOffline(t)
	const parent = `<style xmlns="http://purl.org/net/xbiblio/csl" version="1.0"><info><title>Local parent</title></info></style>`
	shared := t.TempDir()
	t.Setenv("CSL_STYLE_DIR", shared)
	writeStyle(t, filepath.Join(shared, "parent.csl"), parent)
	writeStyle(t, filepath.Join(shared, "dependent/journal.csl"), dependentStyle("https://www.zotero.org/styles/parent", ""))
	writeStyle(t, "outside/parent.csl", parent)
	t.Setenv("CSL_STYLE_DIR", "")
	for _, href := range []string{"parent.csl", "../outside/parent.csl", "https://example.test/styles/parent.csl", "http://www.zotero.org/styles/parent"} {
		// Sibling lookup must work even when this directory isn't a style search root.
		writeStyle(t, "outside/journal.csl", dependentStyle(href, ""))
		got, err := loadStyleWithParents("outside/journal.csl", 0)
		if err != nil || got != parent {
			t.Fatalf("parent %q: %v", href, err)
		}
	}
	t.Setenv("CSL_STYLE_DIR", shared)
	got, err := loadStyleWithParents("dependent/journal", 0)
	if err != nil || got != parent {
		t.Fatalf("shared dependent style: %v", err)
	}
	// A parent stored next to an explicitly chosen child takes precedence over the bundle.
	writeStyle(t, "outside/apa.csl", parent)
	writeStyle(t, "outside/journal.csl", dependentStyle("https://www.zotero.org/styles/apa", ""))
	if got, err := loadStyleWithParents("outside/journal.csl", 0); err != nil || got != parent {
		t.Fatalf("sibling precedence: %v", err)
	}
	// A canonical parent URL may also resolve to a bundled style.
	writeStyle(t, "journal.csl", dependentStyle("http://www.zotero.org/styles/ieee", ""))
	expected, _ := loadBundledStyle("ieee.csl")
	if got, err := loadStyleWithParents("journal.csl", 0); err != nil || got != expected {
		t.Fatalf("bundled parent: %v", err)
	}
	writeStyle(t, "journal.csl", dependentStyle("https://example.test/not-installed.csl", ""))
	if _, err := loadStyleWithParents("journal.csl", 0); err == nil || !strings.Contains(err.Error(), "not-installed.csl") || !strings.Contains(err.Error(), "searched") {
		t.Fatalf("missing remote parent must fail locally: %v", err)
	}
	writeStyle(t, "cycle-a.csl", dependentStyle("cycle-b.csl", ""))
	writeStyle(t, "cycle-b.csl", dependentStyle("cycle-a.csl", ""))
	if _, err := loadStyleWithParents("cycle-a.csl", 0); err == nil || !strings.Contains(err.Error(), "cyclic or too deep") {
		t.Fatalf("parent cycle: %v", err)
	}
}

func TestDependentStyleLocaleOffline(t *testing.T) {
	stylesOffline(t)
	writeStyle(t, "journal.csl", dependentStyle("https://www.zotero.org/styles/apa", "de-DE"))
	resolved, err := loadStyleWithParents("journal.csl", 0)
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Locale string `xml:"default-locale,attr"`
	}
	if err := xml.Unmarshal([]byte(resolved), &root); err != nil || root.Locale != "de-DE" {
		t.Fatalf("lost journal locale: %q, %v", root.Locale, err)
	}
	// Preserve namespace prefixes, comments, body bytes, and quoted attribute values.
	for _, input := range []string{
		`<?xml version="1.0"?><!-- comment --><style xmlns="http://purl.org/net/xbiblio/csl" default-locale='en-US'><info/></style>`,
		`<csl:style xmlns:csl="http://purl.org/net/xbiblio/csl" title='mentions default-locale="en-US"'><csl:info/></csl:style>`,
	} {
		got, err := styleWithLocale(input, "de-DE")
		if err != nil {
			t.Fatal(err)
		}
		root.Locale = ""
		if err := xml.Unmarshal([]byte(got), &root); err != nil || root.Locale != "de-DE" {
			t.Fatalf("invalid inherited locale in %s: %v", got, err)
		}
		if strings.Contains(input, "mentions") && !strings.Contains(got, `title='mentions default-locale="en-US"'`) {
			t.Fatal("rewrote another attribute's value")
		}
	}
}

func TestStarterStylesCiteprocOffline(t *testing.T) {
	requireIntegrationTool(t, "citeproc")
	stylesOffline(t)
	entries := []Record{{Entry: Entry{"key": "a", "type": "book", "author": "Smith, Jane", "title": "Example Book", "year": "2024", "publisher": "Example Press"}}}
	citations := []cslCitation{{Items: []cslCitationItem{{ID: "a"}}}}
	writeStyle(t, "journal.csl", dependentStyle("http://www.zotero.org/styles/ieee", "en-GB"))
	for _, test := range []struct{ style, citation string }{
		{"apa", "Smith, 2024"}, {"ieee", "[1]"}, {"modern-language-association", "Smith"}, {"journal.csl", "[1]"},
		{"american-mathematical-society-label", "[Smit24]"},
		{"american-mathematical-society-numeric", "[1]"},
		{"association-for-computing-machinery", "[1]"},
		{"springer-lecture-notes-in-computer-science", "[1]"},
	} {
		t.Run(test.style, func(t *testing.T) {
			result, err := runCiteproc("citeproc", test.style, "en-US", entries, citations)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Citations) != 1 || !strings.Contains(result.Citations[0], test.citation) {
				t.Fatalf("unexpected citation: %v", result.Citations)
			}
			if len(result.Bibliography) != 1 || len(result.Bibliography[0]) != 2 || result.Bibliography[0][0] != "a" || !strings.Contains(result.Bibliography[0][1], "Example Book") {
				t.Fatalf("unexpected bibliography: %v", result.Bibliography)
			}
		})
	}
	writeStyle(t, "notes.csl", `<style xmlns="http://purl.org/net/xbiblio/csl" class="note" version="1.0"><info><title>Notes only</title><id>https://example.test/notes</id><updated>2026-01-01T00:00:00Z</updated></info><citation><layout><text variable="title"/></layout></citation></style>`)
	err := formatWithCSL(Options{Style: "notes.csl", Locale: "en-US"}, entries)
	if err == nil || !strings.Contains(err.Error(), "default style must provide bibliography output") {
		t.Fatalf("note-only style: %v", err)
	}
}

func TestSiteDefaultsWithoutHugo(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PATH", "")
	defaults, err := ReadSiteDefaults()
	if err != nil || defaults.Bibliography != "references" || defaults.Style != "american-mathematical-society-label" {
		t.Fatalf("standalone defaults: %+v, %v", defaults, err)
	}
}
