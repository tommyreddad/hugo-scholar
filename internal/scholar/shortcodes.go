package scholar

import (
	"strconv"
	"strings"
)

type shortcode struct {
	Name string
	Args map[string]string
	ID   string
}

func shortcodeSpace(char byte) bool {
	return char == ' ' || char == '\n' || char == '\r' || char == '\t'
}

func parseShortcodeArgs(source string) map[string]string {
	args := make(map[string]string)
	positional := 0
	for pos := 0; pos < len(source); {
		for pos < len(source) && shortcodeSpace(source[pos]) {
			pos++
		}
		if pos == len(source) || source[pos:] == "/" {
			break
		}
		var token strings.Builder
		name := ""
		quote := byte(0)
		for pos < len(source) {
			char := source[pos]
			pos++
			if quote != 0 {
				if char == '\\' && quote != '`' && pos < len(source) && (source[pos] == quote || source[pos] == '\\') {
					token.WriteByte(source[pos])
					pos++
				} else if char == quote {
					quote = 0
				} else {
					token.WriteByte(char)
				}
			} else if char == '"' || char == '\'' || char == '`' {
				quote = char
			} else if shortcodeSpace(char) {
				break
			} else if char == '=' && name == "" {
				name = token.String()
				token.Reset()
			} else {
				token.WriteByte(char)
			}
		}
		if name == "" {
			name = strconv.Itoa(positional)
			positional++
		}
		args[name] = token.String()
	}
	return args
}

// scanShortcodes uses Hugo's ordinal path: "2/0" is the first child of the
// third top-level shortcode. Quoted arguments may contain shortcode delimiters.
func scanShortcodes(source string) []shortcode {
	var calls []shortcode
	var parents []int
	var closed []bool
	var stack []int
	for pos := 0; pos < len(source); {
		at := strings.Index(source[pos:], "{{")
		if at < 0 {
			break
		}
		pos += at + 2
		if pos >= len(source) || (source[pos] != '<' && source[pos] != '%') {
			continue
		}
		closer := ">}}"
		if source[pos] == '%' {
			closer = "%}}"
		}
		pos++
		if strings.HasPrefix(source[pos:], "/*") {
			if end := strings.Index(source[pos:], "*/"+closer); end >= 0 {
				pos += end + 2 + len(closer)
			}
			continue
		}
		start := pos
		quote := byte(0)
		for pos < len(source) {
			char := source[pos]
			if quote != 0 {
				if char == '\\' && quote != '`' && pos+1 < len(source) {
					pos += 2
					continue
				}
				if char == quote {
					quote = 0
				}
			} else if strings.HasPrefix(source[pos:], closer) {
				break
			} else if char == '"' || char == '\'' || char == '`' {
				quote = char
			}
			pos++
		}
		if pos == len(source) {
			break // Hugo reports malformed shortcode syntax at build time.
		}
		tag := strings.TrimSpace(source[start:pos])
		pos += len(closer)
		if tag == "" {
			continue
		}
		name := strings.Fields(tag)[0]
		if strings.HasPrefix(name, "/") {
			name = strings.TrimPrefix(name, "/")
			for i := len(stack) - 1; i >= 0; i-- {
				parent := stack[i]
				if !closed[parent] && calls[parent].Name == name {
					for _, child := range stack[i+1:] {
						parents[child] = parent
					}
					// A completed pair can itself be the child of an outer pair.
					stack = stack[:i+1]
					closed[parent] = true
					break
				}
			}
			continue
		}
		calls = append(calls, shortcode{Name: name, Args: parseShortcodeArgs(strings.TrimSpace(tag[len(name):]))})
		parents = append(parents, -1)
		closed = append(closed, strings.HasSuffix(tag, "/"))
		stack = append(stack, len(calls)-1)
	}
	counts := make(map[int]int)
	for index := range calls {
		parent := parents[index]
		id := strconv.Itoa(counts[parent])
		counts[parent]++
		if parent >= 0 {
			id = calls[parent].ID + "/" + id
		}
		calls[index].ID = id
	}
	return calls
}
