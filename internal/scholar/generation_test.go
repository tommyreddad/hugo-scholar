package scholar

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generationFixture(t *testing.T) (Options, string) {
	t.Helper()
	root := t.TempDir()
	options := Options{Source: filepath.Join(root, "_bibliography"), ContentDir: filepath.Join(root, "content"), DetailsDir: "bibliography", Style: "basic"}
	for name, content := range map[string]string{
		"_bibliography/references.bib": "@book{paper,title={Original}}",
		"content/reading.bib":          "---\ntitle: Reading\n---\n@book{old,title={Original Reading}}",
		"content/obsolete.bib":         "@book{obsolete,title={Obsolete}}",
		"content/manual.md":            "---\ntitle: Manual\n---\nA user-authored page.\n",
	} {
		writeGenerationInput(t, filepath.Join(root, name), content)
	}
	output := filepath.Join(root, "data", "scholar.json")
	data, err := PrepareWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(output, data); err != nil {
		t.Fatal(err)
	}
	return options, output
}

func writeGenerationInput(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func changeGenerationInputs(t *testing.T, options Options) {
	t.Helper()
	writeGenerationInput(t, filepath.Join(options.Source, "references.bib"), "@book{new,title={Changed}}")
	writeGenerationInput(t, filepath.Join(options.ContentDir, "reading.bib"), "---\ntitle: Reading\n---\n@book{new-reading,title={Changed Reading}}")
	if err := os.Remove(filepath.Join(options.ContentDir, "obsolete.bib")); err != nil {
		t.Fatal(err)
	}
}

func generationSnapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if strings.HasPrefix(entry.Name(), ".hugo-scholar-") {
			t.Errorf("temporary output remains: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".md" || filepath.Ext(path) == ".json" {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] = content
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertGenerationUnchanged(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	after := generationSnapshot(t, root)
	for path, content := range before {
		if !bytes.Equal(after[path], content) {
			t.Errorf("output changed after failed generation: %s", path)
		}
	}
	for path := range after {
		if _, existed := before[path]; !existed {
			t.Errorf("output created after failed generation: %s", path)
		}
	}
}

func TestFailedPreparationPreservesGeneration(t *testing.T) {
	for _, failure := range []string{"missing bibliography", "later malformed content", "detail collision", "user detail page", "invalid query"} {
		t.Run(failure, func(t *testing.T) {
			options, output := generationFixture(t)
			changeGenerationInputs(t, options)
			switch failure {
			case "missing bibliography":
				writeGenerationInput(t, filepath.Join(options.ContentDir, "bad.md"), "{{< cite key=\"x\" file=\"absent\" >}}")
			case "later malformed content":
				writeGenerationInput(t, filepath.Join(options.ContentDir, "z-broken.bib"), "@book{x title={Malformed}}")
			case "detail collision":
				writeGenerationInput(t, filepath.Join(options.Source, "references.bib"), "@book{new,title={One}} @book{New,title={Two}}")
			case "user detail page":
				writeGenerationInput(t, filepath.Join(options.ContentDir, "bibliography", "new.md"), "A user-authored detail page.")
			case "invalid query":
				writeGenerationInput(t, filepath.Join(options.ContentDir, "bad.md"), "{{< bibliography query=\"@book[year=]\" >}}")
			}
			root := filepath.Dir(filepath.Dir(output))
			before := generationSnapshot(t, root)
			if _, err := PrepareWithOptions(options); err == nil {
				t.Fatal("invalid generation was accepted")
			}
			assertGenerationUnchanged(t, root, before)
		})
	}
}

func TestFailedWritePreservesGeneration(t *testing.T) {
	for _, failure := range []string{"parent is a file", "output is a directory"} {
		t.Run(failure, func(t *testing.T) {
			options, output := generationFixture(t)
			changeGenerationInputs(t, options)
			data, err := PrepareWithOptions(options)
			if err != nil {
				t.Fatal(err)
			}
			root := filepath.Dir(filepath.Dir(output))
			before := generationSnapshot(t, root)
			blocked := filepath.Join(root, "z-blocked")
			if failure == "parent is a file" {
				writeGenerationInput(t, blocked, "not a directory")
				blocked = filepath.Join(blocked, "scholar.json")
			} else if err := os.Mkdir(blocked, 0755); err != nil {
				t.Fatal(err)
			}
			if err := Write(blocked, data); err == nil {
				t.Fatal("invalid output destination was accepted")
			}
			assertGenerationUnchanged(t, root, before)
		})
	}
}

func TestSuccessfulWriteCommitsGeneration(t *testing.T) {
	options, output := generationFixture(t)
	changeGenerationInputs(t, options)
	writeGenerationInput(t, filepath.Join(options.ContentDir, "new-page.bib"), `---
title: New Page
---
{{< cite key="late" file="new-page" >}}
@book{first,title={First}}
@book{late,title={Late}}`)
	root := filepath.Dir(filepath.Dir(output))
	before := generationSnapshot(t, root)
	data, err := PrepareWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	assertGenerationUnchanged(t, root, before)
	render := data.Pages["new-page.md"].Contexts[renderContextKey("new-page", "basic", "")]
	if render.BasicNumbers["late"] != 1 {
		t.Fatalf("new content page did not receive cited-only numbering: %#v", render.BasicNumbers)
	}
	if err := Write(output, data); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"obsolete.md", "bibliography/paper.md"} {
		if _, err := os.Stat(filepath.Join(options.ContentDir, path)); !os.IsNotExist(err) {
			t.Errorf("stale generated page remains: %s, %v", path, err)
		}
	}
	for _, path := range []string{"reading.md", "new-page.md", "bibliography/new.md"} {
		if _, err := os.Stat(filepath.Join(options.ContentDir, path)); err != nil {
			t.Errorf("new generated page missing: %s, %v", path, err)
		}
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var published struct {
		Bibliographies map[string][]map[string]any
	}
	if err := json.Unmarshal(content, &published); err != nil {
		t.Fatal(err)
	}
	if published.Bibliographies["references"][0]["key"] != "new" || published.Bibliographies["reading"][0]["key"] != "new-reading" {
		t.Fatalf("JSON did not commit the new bibliography records: %s", content)
	}
	manual, err := os.ReadFile(filepath.Join(options.ContentDir, "manual.md"))
	if err != nil || string(manual) != "---\ntitle: Manual\n---\nA user-authored page.\n" {
		t.Fatalf("manual content was changed: %q, %v", manual, err)
	}
	generationSnapshot(t, root)
}
