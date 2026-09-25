package scholar

import (
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
		if entry.IsDir() || (filepath.Ext(filename) != ".bib" && filepath.Ext(filename) != ".bibtex") {
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
			rendered.WriteString(strings.TrimRight(body[position:index], "\n"))
			fmt.Fprintf(&rendered, "\n\n{{< reference key=%q file=%q block=true >}}\n\n", item["key"], name)
			position = index + len(raw)
		}
		rendered.WriteString(strings.TrimLeft(body[position:], "\n"))
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
		if entry.IsDir() || filepath.Ext(filename) != ".md" {
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

func splitFrontMatter(source string) (string, string, error) {
	if !strings.HasPrefix(source, "---\n") {
		return "", source, nil
	}
	for start := 4; start < len(source); {
		end := strings.IndexByte(source[start:], '\n')
		if end < 0 {
			end = len(source) - start
		}
		line := strings.TrimSpace(source[start : start+end])
		if line == "---" || line == "..." {
			end += start
			if end < len(source) {
				end++
			}
			return source[:end], source[end:], nil
		}
		start += end + 1
	}
	return "", "", fmt.Errorf("unclosed YAML front matter")
}
