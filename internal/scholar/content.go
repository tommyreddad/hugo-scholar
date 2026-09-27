package scholar

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const contentMarker = "<!-- Generated from BibTeX by Hugo Scholar. Do not edit. -->"

// convertContentBib gives Hugo a Markdown counterpart for each content BibTeX
// page. Text outside entries stays in place; each entry becomes a reference.
func convertContentBib(options Options, data *Data) error {
	root := options.ContentDir
	if root == "" {
		root = "content"
	}
	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) && filename == root {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || isEditorLock(filename) || (filepath.Ext(filename) != ".bib" && filepath.Ext(filename) != ".bibtex") {
			return nil
		}
		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		source := strings.ReplaceAll(string(content), "\r\n", "\n")
		frontMatter, body, err := splitFrontMatter(source)
		if err != nil {
			return fmt.Errorf("%s: %w", filename, err)
		}
		entries, err := Parse(body)
		if err != nil {
			return fmt.Errorf("%s: %w", filename, err)
		}
		name := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		if _, exists := data.Bibliographies[name]; exists {
			return fmt.Errorf("content bibliography %q conflicts with another bibliography", name)
		}
		records, err := prepareRecords(options, name, entries)
		if err != nil {
			return fmt.Errorf("%s: %w", filename, err)
		}
		data.Bibliographies[name] = records
		var rendered strings.Builder
		rendered.WriteString(frontMatter)
		rendered.WriteString(contentMarker)
		rendered.WriteString("\n")
		position := 0
		for _, item := range entries {
			raw := item["bibtex"]
			index := strings.Index(body[position:], raw)
			if index < 0 {
				return fmt.Errorf("%s: cannot locate entry %q", filename, item["key"])
			}
			index += position
			rendered.WriteString(strings.TrimRight(stripBibtexMeta(body[position:index]), "\n"))
			fmt.Fprintf(&rendered, "\n\n{{< reference key=%q file=%q block=true >}}\n\n", item["key"], name)
			position = index + len(raw)
		}
		rendered.WriteString(strings.TrimLeft(stripBibtexMeta(body[position:]), "\n"))
		target := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".md"
		if old, err := os.ReadFile(target); err == nil {
			if !strings.Contains(string(old), contentMarker) {
				return fmt.Errorf("refusing to overwrite non-generated page %s", target)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		return os.WriteFile(target, []byte(rendered.String()), 0644)
	})
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) && filename == root {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || isEditorLock(filename) || filepath.Ext(filename) != ".md" {
			return nil
		}
		for _, extension := range []string{".bib", ".bibtex"} {
			if _, err := os.Stat(strings.TrimSuffix(filename, ".md") + extension); err == nil {
				return nil
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), contentMarker) {
			return os.Remove(filename)
		}
		return nil
	})
}

func stripBibtexMeta(source string) string {
	var output strings.Builder
	position := 0
	for position < len(source) {
		at := strings.IndexByte(source[position:], '@')
		if at < 0 {
			output.WriteString(source[position:])
			break
		}
		at += position
		p := parser{source: source, pos: at + 1}
		kind, err := p.identifier()
		if err == nil && (strings.EqualFold(kind, "string") || strings.EqualFold(kind, "comment") || strings.EqualFold(kind, "preamble")) {
			p.space()
			if p.pos < len(source) && (source[p.pos] == '{' || source[p.pos] == '(') {
				opener := source[p.pos]
				closer := byte('}')
				if opener == '(' {
					closer = ')'
				}
				if _, err := p.wrapped(opener, closer); err == nil {
					output.WriteString(source[position:at])
					position = p.pos
					continue
				}
			}
		}
		output.WriteString(source[position : at+1])
		position = at + 1
	}
	return output.String()
}

func splitFrontMatter(source string) (string, string, error) {
	text := strings.TrimPrefix(source, "\ufeff")
	offset := len(source) - len(text)
	if strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "{{") {
		decoder := json.NewDecoder(strings.NewReader(text))
		var value map[string]json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return "", "", fmt.Errorf("invalid JSON front matter: %w", err)
		}
		end := offset + int(decoder.InputOffset())
		return source[:end], source[end:], nil
	}
	firstEnd := strings.IndexByte(text, '\n')
	if firstEnd < 0 {
		return "", source, nil
	}
	opening := strings.TrimSpace(text[:firstEnd])
	if opening != "---" && opening != "+++" {
		return "", source, nil
	}
	for start := offset + firstEnd + 1; start < len(source); {
		end := strings.IndexByte(source[start:], '\n')
		if end < 0 {
			end = len(source) - start
		}
		line := strings.TrimSpace(source[start : start+end])
		if line == opening || (opening == "---" && line == "...") {
			end += start
			if end < len(source) {
				end++
			}
			return source[:end], source[end:], nil
		}
		start += end + 1
	}
	return "", "", fmt.Errorf("unclosed front matter (%s)", opening)
}
