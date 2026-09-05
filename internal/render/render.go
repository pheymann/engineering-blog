// Package render turns validated preview posts into the blog's static HTML.
package render

import (
	"fmt"
	"html"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/pheymann/engineering-blog/internal/post"
)

const canonicalBase = "https://engineering.paulheymann.de"

// Post renders a complete article page using the site's checked-in assets and
// layout. The caller is responsible for resolving vault links and assets first.
func Post(value *post.Post) (string, error) {
	if value == nil {
		return "", fmt.Errorf("post is required")
	}
	if strings.TrimSpace(value.Title) == "" || strings.TrimSpace(value.Slug) == "" {
		return "", fmt.Errorf("post title and slug are required")
	}
	body, err := markdown(value.Body)
	if err != nil {
		return "", fmt.Errorf("render %q: %w", value.Path, err)
	}
	canonical := postURL(value.Slug)
	description := plainText(value.Excerpt)
	return document(value.Title+" — Paul's Engineering Blog", description, "article", canonical,
		`<main class="site-content"><article class="post"><header class="post-header"><h1>`+html.EscapeString(value.Title)+`</h1><time datetime="`+value.Date.Format("2006-01-02")+`">`+html.EscapeString(value.Date.Format("January 2, 2006"))+`</time></header>`+body+`</article></main>`), nil
}

// Home renders all supplied posts newest first. Each card preserves the first
// five parsed body lines as the post package derived them.
func Home(posts []*post.Post) (string, error) {
	ordered := append([]*post.Post(nil), posts...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Date.After(ordered[j].Date) })

	var cards strings.Builder
	for _, value := range ordered {
		if value == nil || strings.TrimSpace(value.Title) == "" || strings.TrimSpace(value.Slug) == "" {
			return "", fmt.Errorf("homepage contains a post without title or slug")
		}
		cards.WriteString(`<a class="post-card" href="/` + html.EscapeString(value.Slug) + `/"><h2>`)
		cards.WriteString(html.EscapeString(value.Title))
		cards.WriteString(`</h2><p class="post-excerpt">`)
		cards.WriteString(excerptHTML(value.Excerpt))
		cards.WriteString(`</p><span class="post-card-link">Read note <span aria-hidden="true">→</span></span></a>`)
	}
	description := "Notes on building calm, useful software by Paul Heymann."
	return document("Paul's Engineering Blog", description, "website", canonicalBase+"/",
		`<main class="site-content"><h1 class="section-title">Latest engineering notes</h1><div class="post-list">`+cards.String()+`</div></main>`), nil
}

func document(title, description, kind, canonical, main string) string {
	return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="description" content="` + html.EscapeString(description) + `">
    <title>` + html.EscapeString(title) + `</title>
    <link rel="canonical" href="` + html.EscapeString(canonical) + `">
    <meta property="og:title" content="` + html.EscapeString(title) + `">
    <meta property="og:type" content="` + kind + `">
    <meta property="og:url" content="` + html.EscapeString(canonical) + `">
    <meta property="og:description" content="` + html.EscapeString(description) + `">
    <link rel="stylesheet" href="/assets/styles.css">
  </head>
  <body>
    <header class="site-header"><a class="site-logo" href="/" aria-label="Paul's Engineering Blog home"><img class="site-logo-image" src="/assets/images/pauls-engineering-blog.svg" alt="Paul's Engineering Blog"></a></header>
    ` + main + `
    <footer class="site-footer"><div class="site-footer-inner"><span>Paul's Engineering Blog</span><nav class="footer-links" aria-label="Legal links"><a href="/impressum/">Impressum</a><a href="/datenschutzerklaerung/">Datenschutzerklärung</a></nav></div></footer>
  </body>
</html>
`
}

func postURL(slug string) string { return canonicalBase + "/" + slug + "/" }

func excerptHTML(excerpt string) string {
	return strings.ReplaceAll(html.EscapeString(excerpt), "\n", "<br>")
}

func plainText(markdown string) string {
	plain := strings.Join(strings.Fields(markdown), " ")
	if plain == "" {
		return "Paul's Engineering Blog"
	}
	return plain
}

func markdown(source string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	var output strings.Builder
	for index := 0; index < len(lines); {
		line := lines[index]
		if strings.TrimSpace(line) == "" {
			index++
			continue
		}
		if strings.HasPrefix(line, "```") {
			index = fencedCode(lines, index, &output)
			continue
		}
		if level, text, ok := heading(line); ok {
			output.WriteString(fmt.Sprintf("<h%d>%s</h%d>", level, inline(text), level))
			index++
			continue
		}
		if strings.HasPrefix(line, ">") {
			var quote []string
			for index < len(lines) && strings.HasPrefix(lines[index], ">") {
				quote = append(quote, strings.TrimSpace(strings.TrimPrefix(lines[index], ">")))
				index++
			}
			output.WriteString("<blockquote><p>" + inline(strings.Join(quote, " ")) + "</p></blockquote>")
			continue
		}
		if ordered, item, ok := listItem(line); ok {
			index = renderList(lines, index, ordered, item, &output)
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "!") {
			if image, ok, err := imageLine(strings.TrimSpace(line)); err != nil {
				return "", err
			} else if ok {
				output.WriteString(image)
				index++
				continue
			}
		}
		var paragraph []string
		for index < len(lines) && strings.TrimSpace(lines[index]) != "" && !strings.HasPrefix(lines[index], "```") && !strings.HasPrefix(lines[index], ">") {
			if len(paragraph) > 0 {
				if _, _, ok := heading(lines[index]); ok {
					break
				}
				if _, _, ok := listItem(lines[index]); ok {
					break
				}
			}
			paragraph = append(paragraph, strings.TrimSpace(lines[index]))
			index++
		}
		output.WriteString("<p>" + inline(strings.Join(paragraph, " ")) + "</p>")
	}
	return output.String(), nil
}

func fencedCode(lines []string, index int, output *strings.Builder) int {
	info := strings.TrimSpace(strings.TrimPrefix(lines[index], "```"))
	index++
	var code []string
	for index < len(lines) && !strings.HasPrefix(lines[index], "```") {
		code = append(code, lines[index])
		index++
	}
	class := ""
	if info != "" {
		class = ` class="language-` + html.EscapeString(info) + `"`
	}
	output.WriteString("<pre><code" + class + ">" + html.EscapeString(strings.Join(code, "\n")) + "</code></pre>")
	if index < len(lines) {
		index++
	}
	return index
}

func heading(line string) (int, string, bool) {
	trimmed := strings.TrimSpace(line)
	for level := 6; level >= 1; level-- {
		prefix := strings.Repeat("#", level) + " "
		if strings.HasPrefix(trimmed, prefix) {
			return level, strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, prefix), "#")), true
		}
	}
	return 0, "", false
}

func listItem(line string) (bool, string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return false, strings.TrimSpace(trimmed[2:]), true
	}
	for index, character := range trimmed {
		if character == '.' && index > 0 && allDigits(trimmed[:index]) && index+1 < len(trimmed) && trimmed[index+1] == ' ' {
			return true, strings.TrimSpace(trimmed[index+2:]), true
		}
		if character < '0' || character > '9' {
			break
		}
	}
	return false, "", false
}

func allDigits(value string) bool { return value != "" && strings.Trim(value, "0123456789") == "" }

func renderList(lines []string, index int, ordered bool, first string, output *strings.Builder) int {
	tag := "ul"
	if ordered {
		tag = "ol"
	}
	output.WriteString("<" + tag + "><li>" + inline(first) + "</li>")
	index++
	for index < len(lines) {
		isOrdered, item, ok := listItem(lines[index])
		if !ok || isOrdered != ordered {
			break
		}
		output.WriteString("<li>" + inline(item) + "</li>")
		index++
	}
	output.WriteString("</" + tag + ">")
	return index
}

func imageLine(line string) (string, bool, error) {
	if !strings.HasPrefix(line, "![") {
		return "", false, nil
	}
	endAlt := strings.Index(line, "](")
	if endAlt < 2 || !strings.HasSuffix(line, ")") {
		return "", false, nil
	}
	alt, destination := line[2:endAlt], strings.TrimSuffix(line[endAlt+2:], ")")
	source, caption := splitDestination(destination)
	if err := safeURL(source, true); err != nil {
		return "", false, err
	}
	image := `<img src="` + html.EscapeString(source) + `" alt="` + html.EscapeString(alt) + `">`
	if caption != "" {
		return "<figure>" + image + "<figcaption>" + html.EscapeString(caption) + "</figcaption></figure>", true, nil
	}
	return "<figure>" + image + "</figure>", true, nil
}

func inline(text string) string {
	var output strings.Builder
	for len(text) > 0 {
		if strings.HasPrefix(text, "[") {
			if endText := strings.Index(text, "]("); endText > 0 {
				if endURL := strings.Index(text[endText+2:], ")"); endURL >= 0 {
					label, destination := text[1:endText], text[endText+2:endText+2+endURL]
					href, title := splitDestination(destination)
					if safeURL(href, false) == nil {
						output.WriteString(`<a href="` + html.EscapeString(href) + `"`)
						if title != "" {
							output.WriteString(` title="` + html.EscapeString(title) + `"`)
						}
						output.WriteString(">" + html.EscapeString(label) + "</a>")
					} else {
						output.WriteString(html.EscapeString(label))
					}
					text = text[endText+3+endURL:]
					continue
				}
			}
		}
		if strings.HasPrefix(text, "~") {
			if end := strings.Index(text[1:], "~"); end > 0 {
				value := text[1 : end+1]
				if allDigits(value) {
					output.WriteString("<sub>" + html.EscapeString(value) + "</sub>")
					text = text[end+2:]
					continue
				}
			}
		}
		_, size := utf8.DecodeRuneInString(text)
		output.WriteString(html.EscapeString(text[:size]))
		text = text[size:]
	}
	return output.String()
}

func splitDestination(destination string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(destination), " ", 2)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Trim(strings.TrimSpace(parts[1]), `"'`)
}

func safeURL(value string, image bool) error {
	parsed, err := url.Parse(value)
	if err != nil || value == "" {
		return fmt.Errorf("invalid URL %q", value)
	}
	if parsed.Host != "" && image {
		return fmt.Errorf("image URL %q must be local", value)
	}
	if parsed.Scheme != "" && parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "mailto" {
		return fmt.Errorf("unsafe URL %q", value)
	}
	if image && parsed.Scheme != "" {
		return fmt.Errorf("image URL %q must be local", value)
	}
	return nil
}
