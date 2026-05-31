package webui

import "testing"

func TestNeutralizeContentType(t *testing.T) {
	tests := []struct {
		name       string
		upstream   string
		wantType   string
		wantInline bool
	}{
		// Active/dangerous types -> inert octet-stream attachment.
		{"html", "text/html", octetStream, false},
		{"html with charset", "text/html; charset=utf-8", octetStream, false},
		{"html uppercase", "TEXT/HTML", octetStream, false},
		{"xhtml", "application/xhtml+xml", octetStream, false},
		{"svg", "image/svg+xml", octetStream, false},
		{"application xml", "application/xml", octetStream, false},
		{"text xml", "text/xml", octetStream, false},
		{"arbitrary +xml suffix", "application/vnd.custom+xml", octetStream, false},
		{"rss", "application/rss+xml", octetStream, false},

		// Safe types -> served inline with original type.
		{"plain text", "text/plain", "text/plain", true},
		{"plain text charset", "text/plain; charset=utf-8", "text/plain; charset=utf-8", true},
		{"png", "image/png", "image/png", true},
		{"jpeg", "image/jpeg", "image/jpeg", true},
		{"mp4", "video/mp4", "video/mp4", true},
		{"json", "application/json", "application/json", true},
		{"pdf", "application/pdf", "application/pdf", true},
		{"octet-stream stays inline", "application/octet-stream", "application/octet-stream", true},

		// Empty/unknown upstream -> inert octet-stream, but inline (not active).
		{"empty", "", octetStream, true},
		{"whitespace", "   ", octetStream, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := neutralizeContentType(tt.upstream)
			if got.ContentType != tt.wantType {
				t.Errorf("ContentType = %q, want %q", got.ContentType, tt.wantType)
			}
			if got.Inline != tt.wantInline {
				t.Errorf("Inline = %v, want %v", got.Inline, tt.wantInline)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"simple", "file.txt", "file.txt"},
		{"strips path", "dir/sub/file.txt", "file.txt"},
		{"strips backslash path", "dir\\sub\\file.txt", "file.txt"},
		// A lone CR/LF truncates the name (single-line rule), so the bytes after
		// it are dropped rather than spliced in.
		{"truncates at CR", "file\r.txt", "file"},
		{"truncates at LF", "fi\nle.txt", "fi"},
		{"truncates at header injection", "a.txt\r\nContent-Type: text/html", "a.txt"},
		{"strips double quote", `na"me.txt`, "name.txt"},
		// Backslash is treated as a path separator (Windows-style).
		{"backslash is a path separator", `na\me.txt`, "me.txt"},
		{"unicode kept", "café€.txt", "café€.txt"},
		{"trailing slash", "dir/", "dir"},
		{"empty -> download", "", "download"},
		{"dot -> download", ".", "download"},
		{"dotdot -> download", "..", "download"},
		{"only quotes -> download", `"`, "download"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeFilename(tt.key); got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
