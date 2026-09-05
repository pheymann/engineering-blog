package render

import (
	"strings"
	"testing"
	"time"

	"github.com/pheymann/engineering-blog/internal/post"
)

func TestPostRendersSupportedMarkdownAndMVPChrome(t *testing.T) {
	value := &post.Post{
		Path: "Engineering Blog/example.md", Title: `A <safe> & useful note`, Slug: "a-safe-useful-note",
		Date: time.Date(2026, time.September, 4, 0, 0, 0, 0, time.UTC), Excerpt: "First <line>\nSecond line",
		Body: "## A heading\n\nA paragraph with [a link](/other/) and ~1~.\n\n- one\n- two\n\n1. first\n2. second\n\n![Logo](/assets/images/logo.svg \"A caption\")\n\n> A useful quote\n\n```html\n<p>escaped</p>\n```",
	}
	html, err := Post(value)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	for _, want := range []string{
		`<h1>A &lt;safe&gt; &amp; useful note</h1><time datetime="2026-09-04">September 4, 2026</time>`,
		`<h2>A heading</h2>`, `<p>A paragraph with <a href="/other/">a link</a> and <sub>1</sub>.</p>`,
		`<ul><li>one</li><li>two</li></ul>`, `<ol><li>first</li><li>second</li></ol>`,
		`<figure><img src="/assets/images/logo.svg" alt="Logo"><figcaption>A caption</figcaption></figure>`,
		`<blockquote><p>A useful quote</p></blockquote>`, `<pre><code class="language-html">&lt;p&gt;escaped&lt;/p&gt;</code></pre>`,
		`<link rel="canonical" href="https://engineering.paulheymann.de/a-safe-useful-note/">`,
		`<meta property="og:title" content="A &lt;safe&gt; &amp; useful note — Paul&#39;s Engineering Blog">`,
		`<meta property="og:type" content="article">`, `<meta property="og:description" content="First &lt;line&gt; Second line">`,
		`href="/assets/styles.css"`, `src="/assets/images/pauls-engineering-blog.svg"`, `href="/impressum/"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Post() output missing %q\n%s", want, html)
		}
	}
	if strings.Contains(html, "<script") || strings.Contains(html, "javascript:") {
		t.Error("Post() rendered executable content")
	}
}

func TestPostEscapesTextAndDropsUnsafeLink(t *testing.T) {
	value := &post.Post{Path: "post.md", Title: "Über", Slug: "uber", Date: time.Now(), Body: `<img src=x> [click](javascript:alert(1))`}
	html, err := Post(value)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if !strings.Contains(html, `&lt;img src=x&gt; click`) {
		t.Fatalf("Post() did not escape unsafe input: %s", html)
	}
	if strings.Contains(html, `href="javascript:`) {
		t.Error("Post() retained an unsafe URL")
	}
}

func TestPostRejectsRemoteImages(t *testing.T) {
	value := &post.Post{Path: "post.md", Title: "Title", Slug: "title", Date: time.Now(), Body: "![alt](https://example.com/image.png)"}
	if _, err := Post(value); err == nil || !strings.Contains(err.Error(), "must be local") {
		t.Fatalf("Post() error = %v, want local-image error", err)
	}
}

func TestPostLinksMarkdownFootnotesWithSafeUniqueReferenceIDs(t *testing.T) {
	value := &post.Post{
		Path: "post.md", Title: "Footnotes", Slug: "footnotes", Date: time.Now(),
		Body: "First reference[^source one], repeated[^source one], and another[^source-one].\n\n## Footnotes\n\n[^source-one]: Second source\n[^source one]: [First source](https://example.com/one)",
	}
	html, err := Post(value)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	for _, want := range []string{
		`<sup><a href="#footnote-1" id="footnote-ref-1-1" aria-describedby="footnote-1">1</a></sup>`,
		`<sup><a href="#footnote-1" id="footnote-ref-1-2" aria-describedby="footnote-1">1</a></sup>`,
		`<sup><a href="#footnote-2" id="footnote-ref-2-1" aria-describedby="footnote-2">2</a></sup>`,
		`<ol class="footnotes"><li id="footnote-1"><a href="https://example.com/one">First source</a> <a class="footnote-backref" href="#footnote-ref-1-1" aria-label="Back to reference 1">↩</a> <a class="footnote-backref" href="#footnote-ref-1-2" aria-label="Back to reference 1">↩</a></li><li id="footnote-2">Second source <a class="footnote-backref" href="#footnote-ref-2-1" aria-label="Back to reference 2">↩</a></li></ol>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Post() output missing %q\n%s", want, html)
		}
	}
	if strings.Contains(html, `[^source one]:`) || strings.Contains(html, `[^source-one]:`) {
		t.Fatalf("Post() rendered footnote definition syntax: %s", html)
	}
}

func TestPostRendersFootnotesReferencedFromDefinitions(t *testing.T) {
	value := &post.Post{
		Path: "post.md", Title: "Footnotes", Slug: "footnotes", Date: time.Now(),
		Body: "Reference[^first].\n\n[^first]: First definition references[^second].\n[^second]: Second definition.",
	}
	html, err := Post(value)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if !strings.Contains(html, `<li id="footnote-2">Second definition.`) {
		t.Fatalf("Post() emitted a nested footnote link without its target definition: %s", html)
	}
}

func TestHomeOrdersPostsAndEscapesThreeSentenceExcerpts(t *testing.T) {
	older := &post.Post{Title: "Older", Slug: "older", Date: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Excerpt: "one.\ntwo!\nthree?"}
	newer := &post.Post{Title: "Newer", Slug: "newer", Date: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), Excerpt: "<one>.\ntwo!\nthree?"}
	html, err := Home([]*post.Post{older, newer})
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	if strings.Index(html, "Newer") > strings.Index(html, "Older") {
		t.Error("Home() did not order newest posts first")
	}
	if !strings.Contains(html, `&lt;one&gt;.<br>two!<br>three?`) {
		t.Error("Home() did not safely retain the three-sentence excerpt")
	}
	if strings.Contains(html, "Latest engineering notes") || strings.Contains(html, `class="section-title"`) {
		t.Error("Home() retained the removed latest-post heading")
	}
	for _, want := range []string{`<meta property="og:type" content="website">`, `href="https://engineering.paulheymann.de/"`, `href="/newer/"`} {
		if !strings.Contains(html, want) {
			t.Errorf("Home() output missing %q", want)
		}
	}
}
