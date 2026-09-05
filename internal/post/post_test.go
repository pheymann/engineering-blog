package post

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	validBody := `---
date: 2026-09-04
redirect:
  - /previous-title/
  - /older-title/
---
# A Local-First Engineering Blog
#blog #engineering #preview #notes
first line
second line
third line
fourth line
fifth line
sixth line`

	tests := []struct {
		name        string
		path        string
		markdown    string
		wantNil     bool
		wantTitle   string
		wantDate    time.Time
		wantSlug    string
		wantTags    []string
		wantRedirs  []string
		wantExcerpt string
		wantBody    string
		wantError   string
	}{
		{
			name:        "valid preview post",
			path:        "Engineering Blog/A Local-First Engineering Blog.md",
			markdown:    validBody,
			wantTitle:   "A Local-First Engineering Blog",
			wantDate:    time.Date(2026, time.September, 4, 0, 0, 0, 0, time.UTC),
			wantSlug:    "a-local-first-engineering-blog",
			wantTags:    []string{"blog", "engineering", "preview", "notes"},
			wantRedirs:  []string{"/previous-title/", "/older-title/"},
			wantExcerpt: "first line\nsecond line\nthird line\nfourth line\nfifth line",
			wantBody:    "first line\nsecond line\nthird line\nfourth line\nfifth line\nsixth line",
		},
		{
			name:     "irrelevant tags are ignored without metadata",
			path:     "notes/ordinary.md",
			markdown: "# An ordinary note\n#blog #engineering",
			wantNil:  true,
		},
		{
			name:        "filename supplies title without H1",
			path:        "posts/Title From Filename.md",
			markdown:    "---\ndate: 2026-09-04\n---\n#blog #engineering #preview\nparagraph",
			wantTitle:   "Title From Filename",
			wantSlug:    "title-from-filename",
			wantDate:    time.Date(2026, time.September, 4, 0, 0, 0, 0, time.UTC),
			wantTags:    []string{"blog", "engineering", "preview"},
			wantRedirs:  []string{},
			wantExcerpt: "paragraph",
			wantBody:    "paragraph",
		},
		{
			name:      "missing date",
			path:      "posts/no-date.md",
			markdown:  "# Title\n#blog #engineering #preview",
			wantError: "posts/no-date.md: date property is required",
		},
		{
			name:      "invalid date",
			path:      "posts/bad-date.md",
			markdown:  "---\ndate: 04-09-2026\n---\n# Title\n#blog #engineering #preview",
			wantError: "posts/bad-date.md: date property must use YYYY-MM-DD",
		},
		{
			name:      "invalid redirect scalar",
			path:      "posts/bad-redirect.md",
			markdown:  "---\ndate: 2026-09-04\nredirect: /old/\n---\n# Title\n#blog #engineering #preview",
			wantError: "posts/bad-redirect.md: redirect property must be a list",
		},
		{
			name:      "invalid redirect URL",
			path:      "posts/external-redirect.md",
			markdown:  "---\ndate: 2026-09-04\nredirect:\n  - https://example.com/old/\n---\n# Title\n#blog #engineering #preview",
			wantError: "posts/external-redirect.md: redirect \"https://example.com/old/\" must be an absolute site path",
		},
		{
			name:      "unusable slug",
			path:      "posts/!!!.md",
			markdown:  "---\ndate: 2026-09-04\n---\n# !!!\n#blog #engineering #preview",
			wantError: "posts/!!!.md: title \"!!!\" does not produce a usable slug",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := Parse(test.path, test.markdown)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("Parse() error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if test.wantNil {
				if parsed != nil {
					t.Fatalf("Parse() = %#v, want ignored post", parsed)
				}
				return
			}
			if parsed == nil {
				t.Fatal("Parse() = nil, want parsed post")
			}
			if parsed.Title != test.wantTitle || !parsed.Date.Equal(test.wantDate) || parsed.Slug != test.wantSlug || parsed.Excerpt != test.wantExcerpt || parsed.Body != test.wantBody {
				t.Fatalf("Parse() = %#v, want title/date/slug/excerpt/body values", parsed)
			}
			if !reflect.DeepEqual(parsed.Tags, test.wantTags) || !reflect.DeepEqual(parsed.Redirects, test.wantRedirs) {
				t.Fatalf("Parse() tags/redirects = %#v/%#v, want %#v/%#v", parsed.Tags, parsed.Redirects, test.wantTags, test.wantRedirs)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{title: "  C++ / Go  ", want: "c-go"},
		{title: "Über engineering", want: "über-engineering"},
		{title: "!!!", want: ""},
	}
	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			if got := Slug(test.title); got != test.want {
				t.Fatalf("Slug(%q) = %q, want %q", test.title, got, test.want)
			}
		})
	}
}

func TestParseRejectsRedirectThatEscapesGeneratedSite(t *testing.T) {
	markdown := "---\ndate: 2026-09-05\nredirect:\n  - /../outside/\n---\n# Unsafe redirect\n#blog #engineering #preview\nBody"

	_, err := Parse("unsafe.md", markdown)
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("Parse() error = %v, want redirect validation error", err)
	}
}

func TestParseRemovesMatchingH1AfterBodyTags(t *testing.T) {
	markdown := "---\ndate: 2026-09-05\n---\n#blog #engineering #preview\n\n# Title From Filename\n\nBody"

	parsed, err := Parse("Title From Filename.md", markdown)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if strings.Contains(parsed.Body, "# Title From Filename") {
		t.Fatalf("Parse() body retains a duplicate title heading: %q", parsed.Body)
	}
}

func TestParseExcerptSkipsTagsImagesAndBlankLines(t *testing.T) {
	markdown := "---\ndate: 2026-09-05\n---\n#blog #engineering #preview\n\n![[hero image.png]]\n\nFirst text line.\n\nSecond text line.\nThird text line.\nFourth text line.\nFifth text line.\nSixth text line."

	parsed, err := Parse("Preview.md", markdown)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if strings.Contains(parsed.Body, "#blog") {
		t.Fatalf("Parse() body retains deployment tags: %q", parsed.Body)
	}
	want := "First text line.\nSecond text line.\nThird text line.\nFourth text line.\nFifth text line."
	if parsed.Excerpt != want {
		t.Fatalf("Parse() excerpt = %q, want %q", parsed.Excerpt, want)
	}
}
