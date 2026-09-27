package scholar

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type Entry map[string]string

type parser struct {
	source  string
	pos     int
	strings map[string]string
}

var identifierRE = regexp.MustCompile(`^[\pL\pN_:./+\-]+`)

func Parse(source string) ([]Entry, error) {
	p := &parser{source: source, strings: map[string]string{}}
	for index, month := range []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"} {
		p.strings[month] = fmt.Sprint(index + 1)
	}
	var entries []Entry
	for p.pos < len(source) {
		at := strings.IndexByte(source[p.pos:], '@')
		if at < 0 {
			break
		}
		p.pos += at + 1
		entryStart := p.pos - 1
		kind, err := p.identifier()
		if err != nil {
			continue
		}
		kind = strings.ToLower(kind)
		p.space()
		if p.pos >= len(source) || (source[p.pos] != '{' && source[p.pos] != '(') {
			continue
		}
		opener := source[p.pos]
		closer := byte('}')
		if opener == '(' {
			closer = ')'
		}
		if kind == "comment" || kind == "preamble" {
			if _, err := p.wrapped(opener, closer); err != nil {
				return nil, err
			}
			continue
		}
		p.pos++
		if kind == "string" {
			name, err := p.identifier()
			if err != nil {
				return nil, err
			}
			if err := p.expect('='); err != nil {
				return nil, err
			}
			value, err := p.value()
			if err != nil {
				return nil, err
			}
			p.strings[strings.ToLower(name)] = value
			if err := p.expect(closer); err != nil {
				return nil, err
			}
			continue
		}
		start := p.pos
		for p.pos < len(source) && source[p.pos] != ',' && source[p.pos] != closer {
			p.pos++
		}
		key := strings.TrimSpace(source[start:p.pos])
		if key == "" {
			return nil, p.fail("empty citation key")
		}
		entry := Entry{}
		p.space()
		if p.pos < len(source) && source[p.pos] == ',' {
			p.pos++
		}
		for {
			p.space()
			if p.pos >= len(source) {
				return nil, p.fail("unclosed entry")
			}
			if source[p.pos] == closer {
				p.pos++
				break
			}
			field, err := p.identifier()
			if err != nil {
				return nil, err
			}
			if err := p.expect('='); err != nil {
				return nil, err
			}
			value, err := p.value()
			if err != nil {
				return nil, err
			}
			entry[strings.ToLower(field)] = value
			p.space()
			if p.pos < len(source) && source[p.pos] == ',' {
				p.pos++
			} else if p.pos >= len(source) {
				return nil, p.fail("unclosed entry")
			} else if source[p.pos] != closer {
				return nil, p.fail("expected comma or end of entry")
			}
		}
		// BibTeX's key and type fields are not the citation ID or entry kind.
		// Reserve the public metadata names and retain those fields separately.
		if value, ok := entry["key"]; ok {
			entry["bibtex_key"] = value
		}
		if value, ok := entry["type"]; ok {
			entry["bibtex_type"] = value
		}
		entry["key"], entry["type"] = key, kind
		entries = append(entries, entry)
		entry["bibtex"] = source[entryStart:p.pos]
	}
	lookup := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		if _, exists := lookup[entry["key"]]; exists {
			return nil, fmt.Errorf("duplicate citation key %q", entry["key"])
		}
		lookup[entry["key"]] = entry
	}
	state := make(map[string]uint8, len(entries))
	var inherit func(Entry) error
	inherit = func(entry Entry) error {
		key := entry["key"]
		if state[key] == 2 {
			return nil
		}
		if state[key] == 1 {
			return fmt.Errorf("crossref cycle involving %q", key)
		}
		state[key] = 1
		if parent, ok := lookup[entry["crossref"]]; ok {
			if err := inherit(parent); err != nil {
				return err
			}
			for field, value := range parent {
				if field == "key" || field == "type" || field == "bibtex" || field == "crossref" {
					continue
				}
				if _, exists := entry[field]; !exists {
					entry[field] = value
				}
			}
		}
		state[key] = 2
		return nil
	}
	for _, entry := range entries {
		if err := inherit(entry); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func (p *parser) fail(message string) error {
	return fmt.Errorf("line %d: %s", strings.Count(p.source[:min(p.pos, len(p.source))], "\n")+1, message)
}

func (p *parser) space() {
	for p.pos < len(p.source) {
		r := rune(p.source[p.pos])
		if unicode.IsSpace(r) {
			p.pos++
		} else if p.source[p.pos] == '%' {
			end := strings.IndexByte(p.source[p.pos:], '\n')
			if end < 0 {
				p.pos = len(p.source)
			} else {
				p.pos += end + 1
			}
		} else {
			break
		}
	}
}

func (p *parser) identifier() (string, error) {
	p.space()
	match := identifierRE.FindString(p.source[p.pos:])
	if match == "" {
		return "", p.fail("expected identifier")
	}
	p.pos += len(match)
	return match, nil
}

func (p *parser) expect(char byte) error {
	p.space()
	if p.pos >= len(p.source) || p.source[p.pos] != char {
		return p.fail(fmt.Sprintf("expected %q", char))
	}
	p.pos++
	return nil
}

func (p *parser) wrapped(opener, closer byte) (string, error) {
	if err := p.expect(opener); err != nil {
		return "", err
	}
	depth := 1
	start := p.pos
	for p.pos < len(p.source) {
		char := p.source[p.pos]
		if char == '\\' {
			p.pos += min(2, len(p.source)-p.pos)
			continue
		}
		if char == opener {
			depth++
		} else if char == closer {
			depth--
			if depth == 0 {
				value := p.source[start:p.pos]
				p.pos++
				return value, nil
			}
		}
		p.pos++
	}
	return "", p.fail("unclosed value")
}

func (p *parser) valuePart() (string, error) {
	p.space()
	if p.pos >= len(p.source) {
		return "", p.fail("expected value")
	}
	switch p.source[p.pos] {
	case '{':
		return p.wrapped('{', '}')
	case '"':
		p.pos++
		start := p.pos
		depth := 0
		for p.pos < len(p.source) {
			char := p.source[p.pos]
			if char == '\\' {
				p.pos += min(2, len(p.source)-p.pos)
				continue
			}
			if char == '{' {
				depth++
			} else if char == '}' {
				depth--
			} else if char == '"' && depth == 0 {
				value := p.source[start:p.pos]
				p.pos++
				return value, nil
			}
			p.pos++
		}
		return "", p.fail("unclosed quoted value")
	default:
		word, err := p.identifier()
		if err != nil {
			return "", err
		}
		if value, ok := p.strings[strings.ToLower(word)]; ok {
			return value, nil
		}
		return word, nil
	}
}

func (p *parser) value() (string, error) {
	part, err := p.valuePart()
	if err != nil {
		return "", err
	}
	var result strings.Builder
	result.WriteString(part)
	p.space()
	for p.pos < len(p.source) && p.source[p.pos] == '#' {
		p.pos++
		part, err = p.valuePart()
		if err != nil {
			return "", err
		}
		result.WriteString(part)
		p.space()
	}
	return result.String(), nil
}
