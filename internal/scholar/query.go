package scholar

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var querySelectorPattern = regexp.MustCompile(`^(!?)@([\w*]+)(?:\[(.*)\])?$`)
var queryConditionPattern = regexp.MustCompile(`^([\w]+)\s*(\^=|~=|!~|!=|/=|>=|<=|=|>|<)\s*(.+)$`)
var queryFieldPattern = regexp.MustCompile(`^[\w]+$`)
var queryNumberPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

type queryCondition struct {
	field    string
	operator string
	expected string
	pattern  *regexp.Regexp
}

type querySelector struct {
	kind     string
	negate   bool
	branches [][]queryCondition
}

// Preserve predicate commas, including those in regex character classes.
func querySelectors(query string) []string {
	var selectors []string
	start, depth := 0, 0
	escaped := false
	for index, char := range query {
		if char == ',' && depth == 0 {
			selectors = append(selectors, strings.TrimSpace(query[start:index]))
			start = index + 1
			continue
		}
		if escaped {
			escaped = false
		} else if char == '\\' && depth > 0 {
			escaped = true
		} else if char == '[' {
			depth++
		} else if char == ']' && depth > 0 {
			depth--
		}
	}
	return append(selectors, strings.TrimSpace(query[start:]))
}

func compileQuery(query string) ([]querySelector, error) {
	var selectors []querySelector
	for _, source := range querySelectors(query) {
		parts := querySelectorPattern.FindStringSubmatch(source)
		if parts == nil {
			return nil, fmt.Errorf("invalid bibliography query %q", source)
		}
		selector := querySelector{kind: parts[2], negate: parts[1] == "!"}
		if expression := parts[3]; expression != "" {
			for _, branch := range strings.Split(expression, "||") {
				var conditions []queryCondition
				for _, source := range strings.Split(branch, "&&") {
					source = strings.TrimSpace(source)
					condition := queryCondition{field: source}
					if parts := queryConditionPattern.FindStringSubmatch(source); parts != nil {
						condition.field, condition.operator, condition.expected = parts[1], parts[2], strings.TrimSpace(parts[3])
					} else if !queryFieldPattern.MatchString(source) {
						return nil, fmt.Errorf("invalid query condition %q", source)
					}
					if condition.operator == "^=" || condition.operator == "~=" || condition.operator == "!~" {
						pattern, err := regexp.Compile(condition.expected)
						if err != nil {
							return nil, fmt.Errorf("invalid query pattern %q: %w", condition.expected, err)
						}
						condition.pattern = pattern
					}
					conditions = append(conditions, condition)
				}
				selector.branches = append(selector.branches, conditions)
			}
		}
		selectors = append(selectors, selector)
	}
	return selectors, nil
}

func (condition queryCondition) matches(actual string) bool {
	switch condition.operator {
	case "":
		return actual != ""
	case "=":
		return actual == condition.expected
	case "!=", "/=":
		return actual != condition.expected
	case "^=", "~=":
		return condition.pattern.MatchString(actual)
	case "!~":
		return !condition.pattern.MatchString(actual)
	}
	if !queryNumberPattern.MatchString(actual) || !queryNumberPattern.MatchString(condition.expected) {
		return false
	}
	left, leftErr := strconv.ParseFloat(actual, 64)
	right, rightErr := strconv.ParseFloat(condition.expected, 64)
	if leftErr != nil || rightErr != nil {
		return false
	}
	switch condition.operator {
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	}
	return false
}

func matchesQuery(record Record, selectors []querySelector) bool {
	for _, selector := range selectors {
		include := selector.kind == "*" || record.Entry["type"] == selector.kind
		if selector.negate {
			include = !include
		}
		if !include {
			continue
		}
		if len(selector.branches) == 0 {
			return true
		}
		for _, conditions := range selector.branches {
			all := true
			for _, condition := range conditions {
				actual := record.Entry[condition.field]
				if condition.field == "detail_url" {
					actual = record.DetailURL
				} else if condition.field == "csl_order" && record.CSLOrder > 0 {
					actual = strconv.Itoa(record.CSLOrder)
				}
				if !condition.matches(actual) {
					all = false
					break
				}
			}
			if all {
				return true
			}
		}
	}
	return false
}

func prepareQuery(cache map[string]map[string]map[string]bool, file, query string, records []Record) (map[string]bool, error) {
	if matches, exists := cache[file][query]; exists {
		return matches, nil
	}
	selectors, err := compileQuery(query)
	if err != nil {
		return nil, err
	}
	matches := map[string]bool{}
	for _, record := range records {
		if matchesQuery(record, selectors) {
			matches[record.Entry["key"]] = true
		}
	}
	if cache[file] == nil {
		cache[file] = map[string]map[string]bool{}
	}
	cache[file][query] = matches
	return matches, nil
}
