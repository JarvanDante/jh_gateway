package backend

import (
	"strings"
	"testing"
)

func TestLooksLikeSVG_RejectEmbeddedString(t *testing.T) {
	if looksLikeSVG([]byte(`hello world <svg xmlns="http://www.w3.org/2000/svg"></svg>`)) {
		t.Fatal("plain text that merely contains <svg must be rejected")
	}
	if !looksLikeSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)) {
		t.Fatal("svg root should pass")
	}
}

func TestSanitizeSVG(t *testing.T) {
	dirty := []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><script>alert(1)</script><a href="javascript:alert(1)">x</a></svg>`)
	clean, err := sanitizeSVG(dirty)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	lower := strings.ToLower(string(clean))
	for _, bad := range []string{"<script", "onload=", "javascript:"} {
		if strings.Contains(lower, bad) {
			t.Errorf("still contains %q", bad)
		}
	}
}
