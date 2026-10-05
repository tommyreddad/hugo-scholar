package scholar

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// A prefix identifies a bibliography's anchor namespace on its page.
func renderContextKey(file, style, prefix string) string {
	encoded, _ := json.Marshal([]string{file, style, prefix})
	return string(encoded)
}

type renderSpec struct {
	file        string
	style       string
	full        bool
	inOrder     bool
	duplicates  string
	preferences string
}

type pageFile struct {
	records   []Record
	calls     []shortcode
	citations []cslCitation
	cited     map[string]bool
	order     []string
}

func renderPage(options Options, bibliographies map[string][]Record, queryMatches map[string]map[string]map[string]bool, calls []shortcode) (PageRender, error) {
	page := PageRender{
		Cited: map[string][]string{}, CitedAt: map[string]CitedSnapshot{},
		Contexts: map[string]BibliographyRender{},
	}
	files := map[string]*pageFile{}
	specs := map[string]renderSpec{}
	policies := map[string][]string{}
	selections := map[string][]shortcode{}
	active := map[string][]string{}
	for _, call := range calls {
		file := firstNonempty(call.Args["file"], options.DefaultBibliography, "references")
		style := firstNonempty(call.Args["style"], options.Style, "basic")
		if call.Name == "bibliography" || call.Name == "bibliography_count" {
			if query := firstNonempty(call.Args["query"], options.Query); query != "" {
				records, exists := bibliographies[file]
				if !exists {
					return PageRender{}, fmt.Errorf("bibliography %q not found", file)
				}
				if _, err := prepareQuery(queryMatches, file, strings.TrimSpace(query), records); err != nil {
					return PageRender{}, err
				}
			}
		}
		switch call.Name {
		case "cite", "quote", "nocite", "bibliography", "reference":
			records, exists := bibliographies[file]
			if !exists {
				return PageRender{}, fmt.Errorf("bibliography %q not found", file)
			}
			if files[file] == nil {
				files[file] = &pageFile{records: records}
			}
			key := renderContextKey(file, style, call.Args["prefix"])
			spec := specs[key]
			spec.file, spec.style = file, style
			if (call.Name == "bibliography" && call.Args["cited"] != "true" && call.Args["cited_in_order"] != "true") || (call.Name == "reference" && style != options.Style) {
				spec.full = true
			}
			if call.Name == "bibliography" {
				spec.inOrder = spec.inOrder || call.Args["cited_in_order"] == "true"
				removeDuplicates := options.RemoveDuplicates
				if value, exists := call.Args["remove_duplicates"]; exists {
					removeDuplicates = value == "true"
				}
				if removeDuplicates {
					fields := strings.Split(firstNonempty(call.Args["match_fields"], "year,title"), ",")
					for index := range fields {
						fields[index] = strings.TrimSpace(fields[index])
					}
					policy := strings.Join(fields, ",")
					if !contains(policies[key], policy) {
						policies[key] = append(policies[key], policy)
					}
					selections[key] = append(selections[key], call)
				}
			}
			specs[key] = spec
		}
		switch call.Name {
		case "cite", "quote", "nocite":
			files[file].calls = append(files[file].calls, call)
			for _, key := range strings.Fields(firstNonempty(call.Args["keys"], call.Args["key"], call.Args["0"])) {
				if !contains(active[file], key) {
					active[file] = append(active[file], key)
				}
			}
		case "bibliography", "bibliography_count":
			page.CitedAt[call.ID] = CitedSnapshot{Keys: append([]string{}, active[file]...)}
			if call.Args["clear"] == "true" {
				active[file] = nil
			}
		}
	}
	for file, input := range files {
		known := make(map[string]bool, len(input.records))
		for _, record := range input.records {
			known[record.Entry["key"]] = true
		}
		input.cited = map[string]bool{}
		validCalls := make([]shortcode, 0, len(input.calls))
		for _, call := range input.calls {
			citation, err := citationFromShortcode(call)
			if err != nil {
				return PageRender{}, err
			}
			valid := true
			for _, item := range citation.Items {
				if !known[item.ID] {
					valid = false
				}
			}
			if !valid {
				continue
			}
			for _, item := range citation.Items {
				if !input.cited[item.ID] {
					input.order = append(input.order, item.ID)
				}
				input.cited[item.ID] = true
			}
			input.citations = append(input.citations, citation)
			validCalls = append(validCalls, call)
		}
		input.calls = validCalls
		page.Cited[file] = input.order
	}
	keys := make([]string, 0, len(specs))
	for key := range specs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	cache := map[renderSpec]BibliographyRender{}
	for _, key := range keys {
		spec := specs[key]
		sort.Strings(policies[key])
		spec.duplicates = strings.Join(policies[key], "\x00")
		var ranks map[string]int
		if len(policies[key]) > 0 && len(files[spec.file].records) > 1 {
			ranks = bibliographyRanks(options, page, spec.file, files[spec.file], selections[key], queryMatches)
			encoded, _ := json.Marshal(ranks)
			spec.preferences = string(encoded)
		}
		rendered, exists := cache[spec]
		if !exists {
			var err error
			rendered, err = renderBibliography(options, spec, files[spec.file], policies[key], ranks)
			if err != nil {
				return PageRender{}, fmt.Errorf("style %q: %w", spec.style, err)
			}
			cache[spec] = rendered
		}
		page.Contexts[key] = rendered
	}
	return page, nil
}

// The first bibliography selecting a group determines its representative.
// Prefer records cited in that selection, not later or filtered-out citations.
func bibliographyRanks(options Options, page PageRender, file string, input *pageFile, selections []shortcode, queryMatches map[string]map[string]map[string]bool) map[string]int {
	ranks := make(map[string]int, len(input.records))
	for _, record := range input.records {
		key := record.Entry["key"]
		rank := len(selections)*2 + 1
		if input.cited[key] {
			rank--
		}
		ranks[key] = rank
	}
	for index, call := range selections {
		cited := page.Cited[file]
		restricted := call.Args["cited"] == "true" || call.Args["cited_in_order"] == "true"
		if restricted {
			cited = page.CitedAt[call.ID].Keys
		}
		query := strings.TrimSpace(firstNonempty(call.Args["query"], options.Query))
		for _, record := range input.records {
			key := record.Entry["key"]
			if kind := call.Args["type"]; kind != "" && record.Entry["type"] != kind {
				continue
			}
			if query != "" && !queryMatches[file][query][key] {
				continue
			}
			isCited := contains(cited, key)
			if restricted && !isCited {
				continue
			}
			rank := index*2 + 1
			if isCited {
				rank--
			}
			if rank < ranks[key] {
				ranks[key] = rank
			}
		}
	}
	return ranks
}

var digestWhitespace = regexp.MustCompile(`\s+`)

// Match the normalization used by scholar/digest.html. Eligible representatives
// take precedence over records excluded by a bibliography's selection.
func canonicalRecords(records []Record, cited map[string]bool, policies []string, ranks map[string]int) ([]Record, map[string]string) {
	if len(policies) == 0 || len(records) < 2 {
		return records, nil
	}
	parents := make([]int, len(records))
	for index := range parents {
		parents[index] = index
	}
	root := func(index int) int {
		for parents[index] != index {
			parents[index] = parents[parents[index]]
			index = parents[index]
		}
		return index
	}
	for _, policy := range policies {
		fields := strings.Split(policy, ",")
		values := make([]string, len(fields))
		seen := make(map[string]int, len(records))
		for index, record := range records {
			for fieldIndex, field := range fields {
				values[fieldIndex] = digestWhitespace.ReplaceAllString(strings.ToLower(record.Entry[field]), "")
			}
			encoded, _ := json.Marshal(values)
			digest := string(encoded)
			if previous, exists := seen[digest]; exists {
				left, right := root(previous), root(index)
				if left > right {
					left, right = right, left
				}
				parents[right] = left
			} else {
				seen[digest] = index
			}
		}
	}
	representatives := make([]int, len(records))
	for index := range representatives {
		representatives[index] = -1
	}
	preference := func(key string) int {
		if rank, exists := ranks[key]; exists {
			return rank
		}
		if cited[key] {
			return 0
		}
		return 1
	}
	groups := 0
	for index, record := range records {
		group := root(index)
		previous := representatives[group]
		if previous < 0 {
			representatives[group] = index
			groups++
		} else if preference(record.Entry["key"]) < preference(records[previous].Entry["key"]) {
			representatives[group] = index
		}
	}
	if groups == len(records) {
		return records, nil
	}
	canonical := make([]Record, 0, groups)
	for _, index := range representatives {
		if index >= 0 {
			canonical = append(canonical, records[index])
		}
	}
	aliases := make(map[string]string, len(records))
	for index, record := range records {
		aliases[record.Entry["key"]] = records[representatives[root(index)]].Entry["key"]
	}
	return canonical, aliases
}

func renderBibliography(options Options, spec renderSpec, input *pageFile, policies []string, ranks map[string]int) (BibliographyRender, error) {
	rendered := BibliographyRender{BasicNumbers: map[string]int{}}
	records, aliases := canonicalRecords(input.records, input.cited, policies, ranks)
	canonicalKey := func(key string) string {
		if aliases != nil {
			return aliases[key]
		}
		return key
	}
	cited := input.cited
	if aliases != nil {
		cited = make(map[string]bool, len(input.cited))
		for key := range input.cited {
			cited[canonicalKey(key)] = true
		}
	}
	numbers := rendered.BasicNumbers
	if spec.full {
		for _, record := range records {
			numbers[record.Entry["key"]] = len(numbers) + 1
		}
	} else if spec.inOrder {
		for _, key := range input.order {
			key = canonicalKey(key)
			if numbers[key] == 0 {
				numbers[key] = len(numbers) + 1
			}
		}
	} else {
		for _, record := range input.records {
			key := record.Entry["key"]
			if input.cited[key] {
				key = canonicalKey(key)
				if numbers[key] == 0 {
					numbers[key] = len(numbers) + 1
				}
			}
		}
	}
	if aliases != nil {
		for key, canonical := range aliases {
			if number := numbers[canonical]; number != 0 {
				numbers[key] = number
			}
		}
	}
	if spec.style == "basic" {
		return rendered, nil
	}
	citations := input.citations
	if aliases != nil || spec.full {
		citations = append([]cslCitation(nil), citations...)
	}
	if aliases != nil {
		for index, citation := range citations {
			items := append([]cslCitationItem(nil), citation.Items...)
			for itemIndex := range items {
				items[itemIndex].ID = canonicalKey(items[itemIndex].ID)
			}
			citations[index].Items = items
		}
	}
	styleRecords := records
	if spec.full {
		for _, record := range records {
			key := record.Entry["key"]
			if !cited[key] {
				citations = append(citations, cslCitation{Items: []cslCitationItem{{ID: key}}})
			}
		}
	} else {
		styleRecords = make([]Record, 0, len(cited))
		for _, record := range records {
			if cited[record.Entry["key"]] {
				styleRecords = append(styleRecords, record)
			}
		}
	}
	if len(citations) == 0 {
		return rendered, nil
	}
	result, err := runCiteproc(options.CiteprocPath, spec.style, options.Locale, styleRecords, citations)
	if err != nil {
		return BibliographyRender{}, err
	}
	if err := applyLocaleOverrides(options, spec.style, styleRecords, citations, &result); err != nil {
		return BibliographyRender{}, err
	}
	linkifyCSLReferences(&result, styleRecords)
	rendered.Citations = make(map[string]string, len(input.calls))
	multiple := false
	for index, call := range input.calls {
		rendered.Citations[call.ID] = result.Citations[index]
		multiple = multiple || len(citations[index].Items) > 1
	}
	if multiple {
		var singles []cslCitation
		for _, citation := range citations {
			for _, item := range citation.Items {
				singles = append(singles, cslCitation{Items: []cslCitationItem{item}})
			}
		}
		individual, err := runCiteproc(options.CiteprocPath, spec.style, options.Locale, styleRecords, singles)
		if err != nil {
			return BibliographyRender{}, fmt.Errorf("separate citations: %w", err)
		}
		rendered.Separate = map[string][]string{}
		position := 0
		for index, call := range input.calls {
			count := len(citations[index].Items)
			if count > 1 {
				rendered.Separate[call.ID] = individual.Citations[position : position+count]
			}
			position += count
		}
	}
	members := map[string][]string{}
	if aliases != nil {
		for _, record := range input.records {
			key := record.Entry["key"]
			canonical := aliases[key]
			members[canonical] = append(members[canonical], key)
		}
	}
	rendered.References = make(map[string]string, len(input.records))
	for _, pair := range result.Bibliography {
		if len(pair) != 2 {
			return BibliographyRender{}, fmt.Errorf("citeproc returned malformed bibliography entry")
		}
		keys := members[pair[0]]
		if aliases == nil {
			keys = []string{pair[0]}
		}
		for _, key := range keys {
			rendered.References[key] = pair[1]
			rendered.Order = append(rendered.Order, key)
		}
	}
	return rendered, nil
}
