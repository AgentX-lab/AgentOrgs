package matrix

import (
	"strings"
	"testing"
)

func TestFormatVisibleMentions(t *testing.T) {
	plain, html := formatVisibleMentions([]string{"@lead:matrix.local", "@be-1:matrix.local"}, "please start")
	if !strings.Contains(plain, "@lead:matrix.local") || !strings.Contains(plain, "@be-1:matrix.local") {
		t.Fatalf("plain=%q", plain)
	}
	if !strings.HasSuffix(plain, "please start") {
		t.Fatalf("plain missing text: %q", plain)
	}
	if !strings.Contains(html, `href="https://matrix.to/#/`) {
		t.Fatalf("html missing matrix.to: %q", html)
	}
	if !strings.Contains(html, "@lead:matrix.local") || !strings.Contains(html, "please start") {
		t.Fatalf("html=%q", html)
	}
}

func TestFormatVisibleMentionsEmptyText(t *testing.T) {
	plain, html := formatVisibleMentions([]string{"@lead:matrix.local"}, "")
	if plain != "@lead:matrix.local" {
		t.Fatalf("plain=%q", plain)
	}
	if !strings.Contains(html, "@lead:matrix.local") {
		t.Fatalf("html=%q", html)
	}
}
