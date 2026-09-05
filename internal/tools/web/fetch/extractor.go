package fetch

import (
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	whitespaceRe = regexp.MustCompile(`[ \t\r\n]+`)
	newlinesRe   = regexp.MustCompile(`\n{3,}`)
)

type extractorContext struct {
	inPre      bool
	inCode     bool
	listStack  []listState
	blockquote int
}

type listState struct {
	isOrdered bool
	index     int
}

// ExtractMarkdown parses HTML from an io.Reader and converts readable content into clean Markdown.
func ExtractMarkdown(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	ctx := &extractorContext{}
	renderNode(doc, &sb, ctx)

	// Clean trailing spaces on every line and collapse excessive newlines
	lines := strings.Split(sb.String(), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t\r")
	}
	result := strings.Join(lines, "\n")
	result = newlinesRe.ReplaceAllString(result, "\n\n")
	result = strings.TrimSpace(result)

	return result, nil
}

func renderNode(n *html.Node, sb *strings.Builder, ctx *extractorContext) {
	if n == nil {
		return
	}

	switch n.Type {
	case html.DocumentNode:
		renderChildren(n, sb, ctx)
		return

	case html.CommentNode, html.DoctypeNode:
		return

	case html.TextNode:
		text := n.Data
		if ctx.inPre {
			sb.WriteString(text)
		} else {
			trimmed := whitespaceRe.ReplaceAllString(text, " ")
			if trimmed == " " {
				s := sb.String()
				if s != "" && !strings.HasSuffix(s, "\n") && !strings.HasSuffix(s, " ") {
					sb.WriteString(" ")
				}
			} else {
				sb.WriteString(trimmed)
			}
		}
		return

	case html.ElementNode:
		if isIgnoredTag(n.DataAtom, n.Data) {
			return
		}

		switch n.DataAtom {
		case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
			ensureBlockStart(sb)
			level := int(n.Data[1] - '0')
			sb.WriteString(strings.Repeat("#", level))
			sb.WriteString(" ")
			renderChildren(n, sb, ctx)
			ensureBlockEnd(sb)

		case atom.P:
			ensureBlockStart(sb)
			renderChildren(n, sb, ctx)
			ensureBlockEnd(sb)

		case atom.Blockquote:
			ensureBlockStart(sb)
			var bqSb strings.Builder
			ctx.blockquote++
			renderChildren(n, &bqSb, ctx)
			ctx.blockquote--
			bqContent := strings.TrimSpace(bqSb.String())
			lines := strings.Split(bqContent, "\n")
			for i, line := range lines {
				sb.WriteString("> ")
				sb.WriteString(line)
				if i < len(lines)-1 {
					sb.WriteString("\n")
				}
			}
			ensureBlockEnd(sb)

		case atom.Hr:
			ensureBlockStart(sb)
			sb.WriteString("---")
			ensureBlockEnd(sb)

		case atom.Br:
			sb.WriteString("\n")

		case atom.B, atom.Strong:
			sb.WriteString("**")
			renderChildren(n, sb, ctx)
			sb.WriteString("**")

		case atom.I, atom.Em:
			sb.WriteString("*")
			renderChildren(n, sb, ctx)
			sb.WriteString("*")

		case atom.Code:
			if ctx.inPre {
				renderChildren(n, sb, ctx)
			} else {
				sb.WriteString("`")
				ctx.inCode = true
				renderChildren(n, sb, ctx)
				ctx.inCode = false
				sb.WriteString("`")
			}

		case atom.Pre:
			ensureBlockStart(sb)
			sb.WriteString("```\n")
			ctx.inPre = true
			renderChildren(n, sb, ctx)
			ctx.inPre = false
			if !strings.HasSuffix(sb.String(), "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("```")
			ensureBlockEnd(sb)

		case atom.A:
			href := getAttr(n, "href")
			var linkTextSb strings.Builder
			renderChildren(n, &linkTextSb, ctx)
			linkText := strings.TrimSpace(linkTextSb.String())

			if href == "" {
				sb.WriteString(linkText)
			} else if linkText == "" {
				sb.WriteString("[")
				sb.WriteString(href)
				sb.WriteString("](")
				sb.WriteString(href)
				sb.WriteString(")")
			} else {
				sb.WriteString("[")
				sb.WriteString(linkText)
				sb.WriteString("](")
				sb.WriteString(href)
				sb.WriteString(")")
			}

		case atom.Ul:
			ensureBlockStart(sb)
			ctx.listStack = append(ctx.listStack, listState{isOrdered: false})
			renderChildren(n, sb, ctx)
			ctx.listStack = ctx.listStack[:len(ctx.listStack)-1]
			ensureBlockEnd(sb)

		case atom.Ol:
			ensureBlockStart(sb)
			ctx.listStack = append(ctx.listStack, listState{isOrdered: true, index: 1})
			renderChildren(n, sb, ctx)
			ctx.listStack = ctx.listStack[:len(ctx.listStack)-1]
			ensureBlockEnd(sb)

		case atom.Li:
			ensureLineStart(sb)
			if len(ctx.listStack) > 0 {
				curr := &ctx.listStack[len(ctx.listStack)-1]
				if curr.isOrdered {
					sb.WriteString(strings.Repeat("  ", len(ctx.listStack)-1))
					sb.WriteString(formatInt(curr.index))
					sb.WriteString(". ")
					curr.index++
				} else {
					sb.WriteString(strings.Repeat("  ", len(ctx.listStack)-1))
					sb.WriteString("- ")
				}
			} else {
				sb.WriteString("- ")
			}
			var itemSb strings.Builder
			renderChildren(n, &itemSb, ctx)
			itemText := strings.TrimSpace(itemSb.String())
			sb.WriteString(itemText)
			sb.WriteString("\n")

		default:
			renderChildren(n, sb, ctx)
		}
	}
}

func renderChildren(n *html.Node, sb *strings.Builder, ctx *extractorContext) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderNode(c, sb, ctx)
	}
}

func isIgnoredTag(a atom.Atom, data string) bool {
	switch a {
	case atom.Script, atom.Style, atom.Noscript, atom.Template,
		atom.Svg, atom.Canvas, atom.Iframe, atom.Form,
		atom.Nav, atom.Header, atom.Footer, atom.Aside, atom.Head:
		return true
	}
	tag := strings.ToLower(data)
	return tag == "script" || tag == "style" || tag == "noscript" ||
		tag == "template" || tag == "svg" || tag == "canvas" ||
		tag == "iframe" || tag == "form" || tag == "nav" ||
		tag == "header" || tag == "footer" || tag == "aside" || tag == "head"
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, key) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func ensureBlockStart(sb *strings.Builder) {
	s := sb.String()
	if s == "" {
		return
	}
	if strings.HasSuffix(s, "\n\n") {
		return
	}
	if strings.HasSuffix(s, "\n") {
		sb.WriteString("\n")
		return
	}
	sb.WriteString("\n\n")
}

func ensureBlockEnd(sb *strings.Builder) {
	s := sb.String()
	if strings.HasSuffix(s, "\n\n") {
		return
	}
	if strings.HasSuffix(s, "\n") {
		sb.WriteString("\n")
		return
	}
	sb.WriteString("\n\n")
}

func ensureLineStart(sb *strings.Builder) {
	s := sb.String()
	if s == "" || strings.HasSuffix(s, "\n") {
		return
	}
	sb.WriteString("\n")
}

func formatInt(n int) string {
	var b [20]byte
	i := len(b)
	if n == 0 {
		return "0"
	}
	for n > 0 {
		i--
		b[i] = byte('0' + (n % 10))
		n /= 10
	}
	return string(b[i:])
}
