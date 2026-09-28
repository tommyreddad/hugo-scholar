package scholar

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireIntegrationTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		if os.Getenv("HUGO_SCHOLAR_REQUIRE_INTEGRATION") == "1" {
			t.Fatalf("required integration tool %s is unavailable: %v", name, err)
		}
		t.Skipf("%s is not installed", name)
	}
}

const integrationBibliography = `@book{a,author={Alpha, Ann},title={First},year=2020}
@book{b,author={Beta, Bob},title={Second},year=2021}
@book{c,author={Gamma, Gil},title={Third},year=2022}`

type integrationCase struct {
	name          string
	bib           string
	page          string
	pagePath      string
	style         string
	useSiteStyle  bool
	needsCiteproc bool
	generateError string
	config        string
	baseURL       string
	files         map[string]string
	flags         []string
	want          []string
	absent        []string
	ordered       []string
	countItems    int // -1 means no count assertion
}

func TestGeneratorHugoIntegration(t *testing.T) {
	requireIntegrationTool(t, "hugo")
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "hugo-scholar")
	build := exec.Command("go", "build", "-o", binary, "./cmd/hugo-scholar")
	build.Dir = repo
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build generator: %v\n%s", err, output)
	}
	numeric, err := os.ReadFile(filepath.Join(repo, "example", "styles", "numeric.csl"))
	if err != nil {
		t.Fatal(err)
	}
	const duplicates = `@book{a,title={Same},year=2020}
@book{b,title={Same},year=2020}`
	tests := []integrationCase{
		{
			name: "default AMS numbered", useSiteStyle: true,
			page:    `{{< cite "b" >}} {{< cite "a" >}} {{< bibliography cited=true >}}`,
			want:    []string{`href="#b">[1]</a>`, `href="#a">[2]</a>`},
			ordered: []string{`id="b"`, `id="a"`}, countItems: 2,
		},
		{
			name: "single late entry on post", style: "association-for-computing-machinery",
			bib: `@book{a,author={Alpha, Ann},title={First},year=2020}
@book{b,author={Beta, Bob},title={Second},year=2020}
@book{c,author={Charlie, Cal},title={Third},year=2020}
@book{d,author={Delta, Dan},title={Fourth},year=2020}
@book{e,author={Echo, Ed},title={Fifth},year=2020}
@book{f,author={Foxtrot, Fran},title={Sixth},year=2020}
@book{g,author={Golf, Gil},title={Seventh},year=2020}
@book{h,author={Hotel, Hal},title={Eighth},year=2020}
@book{devroye2019discrete,author={Zulu, Zoe},title={Discrete},year=2019}`,
			pagePath: "posts/example.md",
			page: `{{< cite "devroye2019discrete" >}}

{{< bibliography cited=true >}}`,
			want:       []string{`href="#devroye2019discrete">[1]</a>`, `id="devroye2019discrete"><div class="scholar-csl"><span class="csl-left-margin">[1]</span>`},
			absent:     []string{`[9]`},
			countItems: 1,
		},
		{
			name: "ACM full bibliography retains uncited entries", style: "association-for-computing-machinery",
			page:       `{{< cite "b" >}} {{< bibliography >}}`,
			want:       []string{`href="#b">[2]</a>`, `id="a"`, `id="b"`, `id="c"`},
			countItems: 3,
		},
		{
			name: "Hugo style default", useSiteStyle: true,
			config: "[params.scholar]\nstyle = 'ieee'\n",
			page:   `{{< cite "b" >}} {{< cite "a" >}} {{< bibliography cited=true >}}`,
			want:   []string{`href="#b">[1]</a>`, `href="#a">[2]</a>`}, countItems: 2,
		},
		{
			name: "Hugo style configuration directory", useSiteStyle: true,
			files: map[string]string{"config/_default/params.toml": "[scholar]\nstyle = 'ieee'\n"},
			page:  `{{< cite "a" >}} {{< bibliography cited=true >}}`,
			want:  []string{`href="#a">[1]</a>`}, countItems: 1,
		},
		{
			name: "explicit style overrides site", style: "apa",
			config: "[params.scholar]\nstyle = 'ieee'\n",
			page:   `{{< cite "a" >}} {{< bibliography cited=true >}}`,
			want:   []string{`href="#a">(Alpha, 2020)</a>`}, countItems: 1,
		},
		{
			name: "configured local journal style", useSiteStyle: true,
			config: "[params.scholar]\nstyle = 'dependent/journal'\n",
			files:  map[string]string{"styles/dependent/journal.csl": dependentStyle("https://www.zotero.org/styles/numeric", "en-US")},
			page:   `{{< cite "a" >}} {{< bibliography cited=true >}}`,
			want:   []string{`href="#a">[1]</a>`}, countItems: 1,
		},
		{
			name: "bundled shortcode style override", style: "apa",
			page: `{{< cite keys="a" style="ieee" >}} {{< bibliography style="ieee" cited=true >}}`,
			want: []string{`href="#a">[1]</a>`}, countItems: 1,
		},
		{
			name: "remote styles rejected", style: "https://example.test/style.csl",
			generateError: "does not download CSL files",
		},
		{
			name: "missing local parent", style: "journal.csl",
			files:         map[string]string{"journal.csl": dependentStyle("https://example.test/uninstalled-parent", "")},
			generateError: "uninstalled-parent.csl",
		},
		{
			name: "safe IDs and prefixes", bib: `@book{x"><img src=x onerror=alert(1)>,title={Ordinary}}`,
			page: "{{< bibliography prefix=`pre\"><script>alert(2)</script>` >}}",
			want: []string{"&lt;img", "&lt;script"}, absent: []string{"<img", "<script"}, countItems: 1,
		},
		{
			name: "BibTeX identity", bib: `@techreport{original,key={sort-key},type={Research Report},title={Report},year=2024}`,
			page: `{{< cite "original" >}} {{< bibliography type="techreport" >}}`,
			want: []string{`href="#original"`, `id="original"`}, absent: []string{"missing reference"}, countItems: 1,
		},
		{
			name: "nested ordinals", style: "styles/numeric.csl",
			page: `{{< cite "a" >}} {{< wrap >}}{{< wrap >}}{{< cite "b" >}}{{< /wrap >}}{{< cite "c" >}}{{< /wrap >}} {{< bibliography cited_in_order=true >}}`,
			want: []string{`href="#a">[1]</a>`, `href="#b">[2]</a>`, `href="#c">[3]</a>`}, countItems: 3,
		},
		{
			name: "front matter citations", style: "styles/numeric.csl",
			page: "---\ntitle: Review\ndescription: '{{< cite \"a\" >}}'\n---\n{{< cite \"b\" >}}\n{{< bibliography cited=true >}}",
			want: []string{`href="#b">[1]</a>`, `id="b">[1] Second`}, absent: []string{`id="a"`}, countItems: 1,
		},
		{
			name: "shared style override", needsCiteproc: true,
			page: `{{< cite keys="b a" style="styles/numeric.csl" separate_links=true >}} {{< bibliography style="styles/numeric.csl" >}}
{{< cite keys="x" file="other" style="styles/numeric.csl" >}} {{< bibliography file="other" style="styles/numeric.csl" >}}`,
			files: map[string]string{"_bibliography/other.bib": `@book{x,title={Other},year=2023}`},
			want:  []string{`href="#b">1</a>`, `href="#a">2</a>`, `href="#x">[1]</a>`, `id="x">[1] Other`}, countItems: 4,
		},
		{
			name: "author whitespace", style: "apa", bib: "@book{a,author={Alpha, Ann and\nBeta, Bob},title={Book},year=2024}",
			want: []string{"Alpha, A.,", "Beta, B."}, absent: []string{"Beta., Bob."}, countItems: 1,
		},

		{
			name: "URL syntax", style: "apa", bib: `@book{a,title={URL},url={https://example.org/~alice/paper?x=1\&y=2}}`,
			want: []string{`href="https://example.org/~alice/paper?x=1&amp;y=2"`}, absent: []string{"/ alice"}, countItems: 1,
		},

		{
			name: "numeric bibliography order", style: "styles/numeric.csl",
			page:    `{{< cite "b" >}} {{< cite "a" >}} {{< bibliography >}}`,
			ordered: []string{`id="b">[1] Second`, `id="a">[2] First`, `id="c">[3] Third`},
			absent:  []string{"<ol"}, countItems: 3,
		},
		{
			name: "APA bibliography order", style: "apa",
			bib:     `@book{z,author={Zeller, Amy},title={Last},year=2020} @book{a,author={Adams, Zoe},title={First},year=2021}`,
			ordered: []string{`id="a"`, `id="z"`}, countItems: 2,
		},
		{
			name: "explicit input order", style: "styles/numeric.csl",
			page:    `{{< cite "b" >}} {{< bibliography sort_by="none" >}}`,
			ordered: []string{`id="a"`, `id="b"`, `id="c"`}, countItems: 3,
		},
		{
			name: "deduplicate cited", bib: duplicates,
			page: `{{< cite "b" >}} {{< bibliography cited=true remove_duplicates=true >}}`,
			want: []string{`id="b"`, `id="a"`}, countItems: 1,
		},
		{
			name: "deduplicate full", bib: duplicates,
			page: `{{< cite "a" >}} {{< cite "b" >}} {{< bibliography remove_duplicates=true >}}`,
			want: []string{`id="a"`, `id="b"`}, countItems: 1,
		},
		{
			name: "missing month", page: `{{< bibliography group_by="month_numeric" >}}`,
			want: []string{"(unknown)"}, countItems: 3,
		},
		{
			name: "nonnumeric year", bib: integrationBibliography + ` @book{upcoming,title={Upcoming},year={in press}}`,
			page: `{{< bibliography query="@book[year>=2000]" >}}`, absent: []string{"Upcoming"}, countItems: 3,
		},
		{
			name: "raw string keys", style: "styles/numeric.csl", page: "{{< cite keys=`b\na` >}} {{< bibliography cited=true >}}",
			want: []string{`href="#b">[1, 2]</a>`}, ordered: []string{`id="b"`, `id="a"`}, countItems: 2,
		},
		{
			name: "group pagination", page: `{{< bibliography group_by="year" offset=1 max=1 >}}`,
			want: []string{`id="b"`}, absent: []string{`id="a"`, `id="c"`}, countItems: 1,
		},
		{
			name: "zero limit", page: `{{< bibliography max=0 >}} {{< bibliography group_by="year" max=0 >}}`, countItems: 0,
		},
		{
			name: "family sorting", bib: `@book{z,author={Amy Zeller},title={Last},year=2020} @book{a,author={Zoe Adams},title={First},year=2021}`,
			page: `{{< bibliography sort_by="name" >}}`, ordered: []string{`id="a"`, `id="z"`}, countItems: 2,
		},
		{
			name: "empty CSL bibliography", style: "apa", bib: "% Empty bibliography\n",
			page:  `{{< cite "missing" >}} {{< bibliography >}} {{< bibliography file="other" style="styles/numeric.csl" >}}`,
			files: map[string]string{"_bibliography/other.bib": ""}, want: []string{"missing reference"}, countItems: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runHugoIntegration(t, binary, repo, numeric, test)
		})
	}

	// Exercise CSL layout fields without requiring citeproc in CI.
	t.Run("CSL display fields", func(t *testing.T) {
		runHugoIntegration(t, binary, repo, numeric, integrationCase{
			files: map[string]string{"layouts/index.html": `
{{ $reference := ` + "`" + `<div class="csl-left-margin">[1]</div><div class="csl-right-inline"><i>Title</i> <a href="https://example.org/?a=1&amp;b=2">Link</a><div class="csl-block">Block</div><div class="csl-indent">Indented</div>End</div>` + "`" + ` }}
<p>Before {{ partial "scholar/reference_content.html" (dict "reference" $reference "inline" true) }} After.</p>
{{ partial "scholar/reference.html" (dict "entry" (dict "key" "a") "reference" $reference "referenceTag" "p" "prefix" "") }}
`},
			want: []string{
				`<p>Before <span class="csl-left-margin">[1]</span> <span class="csl-right-inline">`,
				`</a> <span class="csl-block">Block</span> <span class="csl-indent">Indented</span> End</span> After.</p>`,
				`<div id="a"><div class="scholar-csl"`,
				`<div class="scholar-csl"><span class="csl-left-margin">[1]</span> <span class="csl-right-inline">`,
				`style="display:block"`, `style="display:block;margin-inline-start:2em"`,
				`<i>Title</i> <a href="https://example.org/?a=1&amp;b=2">Link</a>`,
			},
			absent: []string{`<p id="a">`, `<span id="a">`, `float:`, `margin-inline-end:`},
		})
	})

	styles, err := bundledStyles.ReadDir("styles")
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range styles {
		t.Run("reference rendering/"+style.Name(), func(t *testing.T) {
			root := runHugoIntegration(t, binary, repo, numeric, integrationCase{
				style: strings.TrimSuffix(style.Name(), ".csl"),
				page:  "Before {{< reference \"a\" >}} After.\n\n{{< reference key=\"a\" block=true >}}\n\n{{< bibliography >}}",
				flags: []string{"--details"}, countItems: 3,
				want:   []string{`<div class="scholar-reference">`},
				absent: []string{`<p><div`, `<span id="a"><div`},
			})
			output, err := os.ReadFile(filepath.Join(root, "public", "index.html"))
			if err != nil {
				t.Fatal(err)
			}
			_, after, found := strings.Cut(string(output), "<p>Before ")
			inline, _, ended := strings.Cut(after, " After.</p>")
			if !found || !ended || strings.Contains(inline, "<div") || !strings.Contains(inline, "First") {
				t.Fatalf("reference interrupted its paragraph: %s", output)
			}
			details, err := os.ReadFile(filepath.Join(root, "public", "bibliography", "a", "index.html"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(details), `<div class="scholar-reference">`) || strings.Contains(string(details), "<p><div") {
				t.Fatalf("invalid detail reference block: %s", details)
			}
		})
	}

	t.Run("subdirectory links", func(t *testing.T) {
		root := runHugoIntegration(t, binary, repo, numeric, integrationCase{
			baseURL: "https://example.org/project/", flags: []string{"--details", "--repository", "static/repository"},
			files: map[string]string{"static/repository/a.pdf": "attachment"},
			page:  `{{< cite_details "a" >}} {{< details_link "a" >}} {{< bibliography >}}`,
			want:  []string{`href="/project/bibliography/a/"`}, absent: []string{`href="/bibliography/`}, countItems: 3,
		})
		details, err := os.ReadFile(filepath.Join(root, "public", "bibliography", "a", "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(details), `href="/project/repository/a.pdf"`) {
			t.Errorf("wrong attachment URL: %s", details)
		}
	})

	t.Run("external attachments", func(t *testing.T) {
		root := runHugoIntegration(t, binary, repo, numeric, integrationCase{
			baseURL: "https://example.org/project/",
			flags:   []string{"--details", "--repository", "static/repository", "--repository-url", "https://files.example.org/papers"},
			files:   map[string]string{"static/repository/a.pdf": "attachment"}, countItems: 3,
		})
		details, err := os.ReadFile(filepath.Join(root, "public", "bibliography", "a", "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(details), `href="https://files.example.org/papers/a.pdf"`) {
			t.Errorf("wrong attachment URL: %s", details)
		}
	})

	t.Run("Hugo bibliography default", func(t *testing.T) {
		root := runHugoIntegration(t, binary, repo, numeric, integrationCase{
			style: "apa", config: "[params.scholar]\nbibliography = 'books'\n",
			page:  `{{< cite "b" >}} {{< bibliography cited=true >}}`,
			files: map[string]string{"_bibliography/books.bib": `@book{b,author={Yves, Y},title={Other},year=2001}`},
			want:  []string{`href="#b">(Yves, 2001)</a>`, "Yves, Y. (2001)"}, absent: []string{"Beta"}, countItems: 1,
		})
		data, err := os.ReadFile(filepath.Join(root, "data", "scholar.json"))
		if err != nil {
			t.Fatal(err)
		}
		var generated struct{ Bibliography string }
		if err := json.Unmarshal(data, &generated); err != nil || generated.Bibliography != "books" {
			t.Fatalf("default not persisted: %s (%v)", data, err)
		}
		configPath := filepath.Join(root, "hugo.toml")
		config, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		changed := strings.Replace(string(config), "bibliography = 'books'", "bibliography = 'references'", 1)
		if err := os.WriteFile(configPath, []byte(changed), 0644); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("hugo")
		command.Dir = root
		if output, err := command.CombinedOutput(); err == nil || !strings.Contains(string(output), "differs from generated bibliography") {
			t.Fatalf("configuration mismatch not rejected: %v\n%s", err, output)
		}
	})
}

func runHugoIntegration(t *testing.T, binary, repo string, numeric []byte, test integrationCase) string {
	t.Helper()
	if test.needsCiteproc || test.useSiteStyle || (test.style != "" && test.style != "basic") {
		requireIntegrationTool(t, "citeproc")
	}
	root := t.TempDir()
	page := firstNonempty(test.page, `{{< bibliography >}}`)
	if !strings.HasPrefix(page, "---") {
		page = "---\ntitle: Review\n---\n\n" + page
	}
	files := map[string]string{
		"go.mod":             "module integration\n\ngo 1.26\n\nrequire github.com/tommyreddad/hugo-scholar v0.1.4\nreplace github.com/tommyreddad/hugo-scholar => " + filepath.ToSlash(repo) + "\n",
		"hugo.toml":          "baseURL = '" + firstNonempty(test.baseURL, "https://example.org/") + "'\ndisableKinds = ['taxonomy', 'term', 'RSS', 'sitemap']\n[[module.imports]]\npath = 'github.com/tommyreddad/hugo-scholar'\n" + test.config,
		"layouts/index.html": "{{ .Content }}", "layouts/_default/single.html": "{{ .Content }}",
		"layouts/shortcodes/wrap.html":                         "<div>{{ .Inner }}</div>",
		"_bibliography/references.bib":                         firstNonempty(test.bib, integrationBibliography),
		"content/" + firstNonempty(test.pagePath, "_index.md"): page, "styles/numeric.csl": string(numeric),
	}
	for name, content := range test.files {
		files[name] = content
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var args []string
	if !test.useSiteStyle {
		args = []string{"--style", firstNonempty(test.style, "basic")}
	}
	generate := exec.Command(binary, append(args, test.flags...)...)
	generate.Dir = root
	generationOutput, generationErr := generate.CombinedOutput()
	if test.generateError != "" {
		if generationErr == nil || !strings.Contains(string(generationOutput), test.generateError) {
			t.Fatalf("expected generation error %q: %v\n%s", test.generateError, generationErr, generationOutput)
		}
		return root
	}
	if generationErr != nil {
		t.Fatalf("generate: %v\n%s", generationErr, generationOutput)
	}
	hugo := exec.Command("hugo")
	hugo.Dir = root
	if output, err := hugo.CombinedOutput(); err != nil {
		t.Fatalf("hugo: %v\n%s", err, output)
	}
	outputPath := "index.html"
	if test.pagePath != "" {
		outputPath = strings.TrimSuffix(test.pagePath, filepath.Ext(test.pagePath)) + "/index.html"
	}
	output, err := os.ReadFile(filepath.Join(root, "public", outputPath))
	if err != nil {
		t.Fatal(err)
	}
	html := string(output)
	for _, want := range test.want {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in:\n%s", want, html)
		}
	}
	for _, absent := range test.absent {
		if strings.Contains(html, absent) {
			t.Errorf("unexpected %q in:\n%s", absent, html)
		}
	}
	position := -1
	for _, want := range test.ordered {
		index := strings.Index(html, want)
		if index <= position {
			t.Fatalf("missing or out of order %q in:\n%s", want, html)
		}
		position = index
	}
	if test.countItems >= 0 && strings.Count(html, "<li>") != test.countItems {
		t.Errorf("expected %d bibliography items in:\n%s", test.countItems, html)
	}
	return root
}
