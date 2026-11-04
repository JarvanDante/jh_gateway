package backend

import "testing"

func TestSniffImageType(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	cases := []struct {
		name, declared string
		data           []byte
		mime, ext      string
		ok             bool
	}{
		{"png declared jpeg", "image/jpeg", png, "image/png", ".png", true},
		{"jpeg", "", []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00"), "image/jpeg", ".jpg", true},
		{"gif", "image/png", []byte("GIF89a\x01\x00"), "image/gif", ".gif", true},
		{"webp", "", []byte("RIFF\x24\x00\x00\x00WEBPVP8 "), "image/webp", ".webp", true},
		{"text as png", "image/png", []byte("plain text"), "", "", false},
		{"svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "image/svg+xml", ".svg", true},
		{"svg as png", "image/png", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "", "", false},
		{"text contains svg", "image/svg+xml", []byte(`not an image but has <svg inside`), "", "", false},
	}
	for _, c := range cases {
		m, e, ok := sniffImageType(c.data, c.declared)
		if ok != c.ok || (ok && (m != c.mime || e != c.ext)) {
			t.Errorf("%s: got (%s,%s,%v)", c.name, m, e, ok)
		}
	}
}

func TestReplaceExt(t *testing.T) {
	for in, want := range map[string]string{"a.txt": "a.png", "a.jpg": "a.png", "x/../b.php": "b.png", "": "image.png", "c": "c.png"} {
		if got := replaceExt(in, ".png"); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}
