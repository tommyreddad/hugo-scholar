package scholar

import (
	"bytes"
	"embed"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed styles/*.csl
var bundledStyles embed.FS

func loadBundledStyle(name string) (string, error) {
	content, err := bundledStyles.ReadFile("styles/" + name)
	return string(content), err
}

type styleSource struct {
	content  string
	filename string // Empty for an embedded style.
}

func loadStyleWithParents(style string, depth int) (string, error) {
	return resolveStyle(style, "", depth)
}

func resolveStyle(style, relativeTo string, depth int) (string, error) {
	if depth > 8 {
		return "", fmt.Errorf("CSL style parent chain is cyclic or too deep at %q", style)
	}
	source, err := findStyle(style, relativeTo)
	if err != nil {
		return "", err
	}
	decoder := xml.NewDecoder(strings.NewReader(source.content))
	var defaultLocale string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return source.content, nil
		}
		if err != nil {
			return "", fmt.Errorf("parse CSL style %q: %w", style, err)
		}
		element, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if element.Name.Local == "style" {
			for _, attribute := range element.Attr {
				if attribute.Name.Local == "default-locale" && attribute.Name.Space == "" {
					defaultLocale = attribute.Value
				}
			}
		}
		if element.Name.Local != "link" {
			continue
		}
		var relation, href string
		for _, attribute := range element.Attr {
			switch attribute.Name.Local {
			case "rel":
				relation = attribute.Value
			case "href":
				href = attribute.Value
			}
		}
		if relation == "independent-parent" {
			parent, err := localParentName(href)
			if err != nil {
				return "", fmt.Errorf("parent of CSL style %q: %w", style, err)
			}
			resolved, err := resolveStyle(parent, source.filename, depth+1)
			if err != nil {
				return "", fmt.Errorf("parent %q of CSL style %q: %w", href, style, err)
			}
			if defaultLocale == "" {
				return resolved, nil
			}
			return styleWithLocale(resolved, defaultLocale)
		}
	}
}

// Parent URLs identify styles; they are never fetched. A URL ending in
// /styles/apa or /apa.csl resolves to a locally available apa.csl.
func localParentName(href string) (string, error) {
	if href == "" {
		return "", fmt.Errorf("missing independent-parent identifier")
	}
	if styleURL(href) {
		parsed, err := url.Parse(href)
		if err != nil || (parsed.Scheme != "" && parsed.Scheme != "http" && parsed.Scheme != "https") {
			return "", fmt.Errorf("invalid parent identifier %q; use a local CSL filename or HTTP(S) style identifier", href)
		}
		name := path.Base(parsed.Path)
		if name == "." || name == "/" || name == ".." {
			return "", fmt.Errorf("parent identifier %q has no style name", href)
		}
		return name, nil
	}
	return href, nil
}

func styleURL(style string) bool {
	return strings.Contains(style, "://") || strings.HasPrefix(style, "//")
}

// Match complete attributes so text inside another quoted value cannot be
// mistaken for default-locale. Only the XML decoder's root opening tag is edited.
var styleAttribute = regexp.MustCompile(`([^\s=<>/]+)\s*=\s*("[^"]*"|'[^']*')`)

func styleWithLocale(content, locale string) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(content))
	for {
		before := decoder.InputOffset()
		token, err := decoder.Token()
		if err != nil {
			return "", fmt.Errorf("read parent style root: %w", err)
		}
		root, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attribute := range root.Attr {
			if attribute.Name.Local == "default-locale" && attribute.Name.Space == "" && attribute.Value == locale {
				return content, nil
			}
		}
		var escaped bytes.Buffer
		if err := xml.EscapeText(&escaped, []byte(locale)); err != nil {
			return "", err
		}
		attribute := `default-locale="` + escaped.String() + `"`
		after := decoder.InputOffset()
		header := content[before:after]
		for _, match := range styleAttribute.FindAllStringSubmatchIndex(header, -1) {
			if header[match[2]:match[3]] == "default-locale" {
				header = header[:match[0]] + attribute + header[match[1]:]
				return content[:before] + header + content[after:], nil
			}
		}
		insert := len(header) - 1
		if strings.HasSuffix(header, "/>") {
			insert--
		}
		header = header[:insert] + " " + attribute + header[insert:]
		return content[:before] + header + content[after:], nil
	}
}

func loadStyle(style string) (string, error) {
	source, err := findStyle(style, "")
	return source.content, err
}

func findStyle(style, relativeTo string) (styleSource, error) {
	if styleURL(style) {
		return styleSource{}, fmt.Errorf("CSL URL %q is not supported: Hugo Scholar does not download CSL files; obtain the style independently and pass its local .csl path", style)
	}
	name := style
	if filepath.Ext(name) == "" {
		name += ".csl"
	}
	var paths []string
	if relativeTo != "" && !filepath.IsAbs(style) {
		directory := filepath.Dir(relativeTo)
		paths = append(paths, filepath.Join(directory, name))
		// Official CSL collections keep dependent styles one level below their parents.
		if filepath.Base(directory) == "dependent" {
			paths = append(paths, filepath.Join(directory, "..", name))
		}
	}
	paths = append(paths, style)
	if !filepath.IsAbs(style) {
		paths = append(paths, filepath.Join("styles", name))
		if directory := os.Getenv("CSL_STYLE_DIR"); directory != "" {
			paths = append(paths, filepath.Join(directory, name))
		}
	}
	var searched []string
	seen := map[string]bool{}
	for _, filename := range paths {
		filename = filepath.Clean(filename)
		if seen[filename] {
			continue
		}
		seen[filename] = true
		searched = append(searched, filename)
		content, err := os.ReadFile(filename)
		if err == nil {
			return styleSource{content: string(content), filename: filename}, nil
		}
		if !os.IsNotExist(err) {
			return styleSource{}, fmt.Errorf("read CSL style %q: %w", filename, err)
		}
	}
	if fs.ValidPath(filepath.ToSlash(name)) {
		content, err := loadBundledStyle(filepath.ToSlash(name))
		if err == nil {
			return styleSource{content: content}, nil
		}
		if !os.IsNotExist(err) {
			return styleSource{}, fmt.Errorf("read bundled CSL style %q: %w", style, err)
		}
	}
	return styleSource{}, fmt.Errorf("CSL style %q not found; searched %q and bundled styles (apa, ieee, modern-language-association, american-mathematical-society-label, american-mathematical-society-numeric, association-for-computing-machinery, springer-lecture-notes-in-computer-science); obtain the CSL file independently and put it in styles/ or CSL_STYLE_DIR, or pass its local path", style, searched)
}
