package fetch_test

import (
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/tools/web/fetch"
)

func TestExtractMarkdown_Headings(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "h1 to h6 tags",
			html:     "<h1>Heading 1</h1><h2>Heading 2</h2><h3>Heading 3</h3><h4>Heading 4</h4><h5>Heading 5</h5><h6>Heading 6</h6>",
			expected: "# Heading 1\n\n## Heading 2\n\n### Heading 3\n\n#### Heading 4\n\n##### Heading 5\n\n###### Heading 6",
		},
		{
			name:     "heading with inline tags",
			html:     "<h1>Welcome to <strong>AGIS</strong></h1>",
			expected: "# Welcome to **AGIS**",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetch.ExtractMarkdown(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("ExtractMarkdown() =\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}

func TestExtractMarkdown_ParagraphsAndFormatting(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "paragraphs with spacing",
			html:     "<p>First paragraph.</p><p>Second paragraph with text.</p>",
			expected: "First paragraph.\n\nSecond paragraph with text.",
		},
		{
			name:     "bold, italic, strong, em",
			html:     "<p><b>Bold</b> and <strong>Strong</strong>, <i>Italic</i> and <em>Emphasized</em>.</p>",
			expected: "**Bold** and **Strong**, *Italic* and *Emphasized*.",
		},
		{
			name:     "nested formatting",
			html:     "<p><strong><em>Bold and italic</em></strong></p>",
			expected: "***Bold and italic***",
		},
		{
			name:     "line break and horizontal rule",
			html:     "<p>Line 1<br/>Line 2</p><hr/><p>Line 3</p>",
			expected: "Line 1\nLine 2\n\n---\n\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetch.ExtractMarkdown(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("ExtractMarkdown() =\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}

func TestExtractMarkdown_Links(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "anchor with text and href",
			html:     `<p>Visit <a href="https://example.com">Example</a> today.</p>`,
			expected: "Visit [Example](https://example.com) today.",
		},
		{
			name:     "anchor without text",
			html:     `<p>Check <a href="https://example.com"></a></p>`,
			expected: "Check [https://example.com](https://example.com)",
		},
		{
			name:     "anchor without href",
			html:     `<p>Just <a>Plain Anchor</a></p>`,
			expected: "Just Plain Anchor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetch.ExtractMarkdown(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("ExtractMarkdown() =\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}

func TestExtractMarkdown_Lists(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name: "unordered list",
			html: "<ul><li>Item 1</li><li>Item 2</li><li>Item 3</li></ul>",
			expected: "- Item 1\n- Item 2\n- Item 3",
		},
		{
			name: "ordered list",
			html: "<ol><li>First</li><li>Second</li><li>Third</li></ol>",
			expected: "1. First\n2. Second\n3. Third",
		},
		{
			name: "list with inline formatting",
			html: "<ul><li><strong>Important:</strong> Read docs</li><li><code>go test</code></li></ul>",
			expected: "- **Important:** Read docs\n- `go test`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetch.ExtractMarkdown(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("ExtractMarkdown() =\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}

func TestExtractMarkdown_CodeAndBlockquotes(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "inline code",
			html:     "<p>Use the <code>go build</code> command.</p>",
			expected: "Use the `go build` command.",
		},
		{
			name:     "pre code block",
			html:     "<pre><code>func main() {\n\tfmt.Println(\"hello\")\n}</code></pre>",
			expected: "```\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```",
		},
		{
			name:     "blockquote",
			html:     "<blockquote>This is a quote from an article.</blockquote>",
			expected: "> This is a quote from an article.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetch.ExtractMarkdown(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("ExtractMarkdown() =\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}

func TestExtractMarkdown_BoilerplateStripping(t *testing.T) {
	htmlDoc := `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
	<script>alert("xss");</script>
	<style>body { color: red; }</style>
	<noscript>Please enable JS</noscript>
</head>
<body>
	<header>
		<nav><a href="/">Home</a> | <a href="/about">About</a></nav>
	</header>
	<main>
		<h1>Main Content Title</h1>
		<p>This is the actual article content.</p>
		<aside>Related articles sidebar</aside>
	</main>
	<footer>
		<p>Copyright 2026</p>
	</footer>
</body>
</html>`

	expected := "# Main Content Title\n\nThis is the actual article content."

	got, err := fetch.ExtractMarkdown(strings.NewReader(htmlDoc))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expected {
		t.Errorf("ExtractMarkdown() =\n%q\nwant:\n%q", got, expected)
	}
}

func TestExtractMarkdown_WhitespaceNormalization(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "multiple spaces and newlines in text node",
			html:     "<p>Hello     \n\n   world!   How    are you?</p>",
			expected: "Hello world! How are you?",
		},
		{
			name:     "empty html or whitespace only",
			html:     "   <div>\n\t<p></p>  </div>   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetch.ExtractMarkdown(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("ExtractMarkdown() =\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}
