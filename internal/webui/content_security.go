package webui

import (
	"path"
	"strings"
)

// Security response headers applied to every object/meta response. They ensure
// that even a direct browser navigation to an object URL can never execute
// active content in our origin (which would otherwise enable access-token
// theft, since the SPA holds the OIDC token in JS memory).
const (
	// headerContentTypeOptions disables MIME sniffing so the browser honours
	// the Content-Type we send and will not "upgrade" e.g. text/plain to HTML.
	headerXCTO = "nosniff"

	// headerCSP neutralizes any active content if the object URL is opened
	// directly: no resources may load and the document is fully sandboxed
	// (no script execution, no same-origin, no forms, etc.).
	headerCSP = "default-src 'none'; sandbox"

	// octetStream is the inert fallback Content-Type used for active/dangerous
	// content so the browser downloads rather than renders it.
	octetStream = "application/octet-stream"
)

// activeContentTypes is the set of media types a browser may execute or render
// as active markup in our origin. Serving any of these inline is unsafe, so
// they are forced to an attachment download with an inert Content-Type.
//
// Matching is done on the bare media type (parameters such as "; charset=..."
// are stripped) and is case-insensitive.
var activeContentTypes = map[string]struct{}{
	"text/html":              {},
	"application/xhtml+xml":  {},
	"image/svg+xml":          {},
	"application/xml":        {},
	"text/xml":               {},
	"application/xml-dtd":    {},
	"application/mathml+xml": {},
	"application/rss+xml":    {},
	"application/atom+xml":   {},
}

// disposition describes how an object should be served to the browser.
type disposition struct {
	// ContentType is the value to send in the Content-Type header.
	ContentType string
	// Inline is true when the object may be rendered inline (Content-Disposition:
	// inline); false means it must be downloaded as an attachment.
	Inline bool
}

// neutralizeContentType decides how to serve an object given the upstream
// Content-Type. Active/dangerous types (HTML/XHTML/SVG/XML and similar) are
// downgraded to an inert octet-stream attachment; everything else is served
// inline with its original type.
//
// The body bytes are never altered - only headers/disposition change.
func neutralizeContentType(upstream string) disposition {
	media := bareMediaType(upstream)
	if media == "" {
		// No usable type from upstream: serve inline as octet-stream. An
		// unknown type is not active, and nosniff prevents the browser from
		// guessing it into something executable.
		return disposition{ContentType: octetStream, Inline: true}
	}
	if isActiveContentType(media) {
		return disposition{ContentType: octetStream, Inline: false}
	}
	return disposition{ContentType: upstream, Inline: true}
}

// isActiveContentType reports whether a bare (parameter-stripped) media type is
// one a browser may execute or render as active markup. Beyond the explicit
// allowlist it also rejects any "+xml" structured-syntax suffix, which covers
// arbitrary XML-based formats the browser might process.
func isActiveContentType(media string) bool {
	if _, ok := activeContentTypes[media]; ok {
		return true
	}
	// Any "+xml" structured syntax (e.g. application/foo+xml) is XML the
	// browser may parse/execute; treat it as active.
	return strings.HasSuffix(media, "+xml")
}

// bareMediaType lowercases a Content-Type and strips any parameters
// (e.g. "Text/HTML; charset=utf-8" -> "text/html"). Returns "" for an empty
// input.
func bareMediaType(ct string) string {
	ct = strings.TrimSpace(ct)
	if ct == "" {
		return ""
	}
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

// sanitizeFilename derives a safe filename for a Content-Disposition header from
// an object key. It strips any path component and removes characters that could
// break out of the quoted header value or inject a second header
// (CR, LF, double-quote, backslash). The result is always non-empty.
func sanitizeFilename(key string) string {
	// 1. A filename is a single line: drop everything from the first CR or LF
	//    on. Truncating (rather than deleting the breaks) both kills any
	//    "\r\nContent-Type: ..." header-injection attempt AND prevents splicing
	//    the trailing bytes back into the name.
	if i := strings.IndexAny(key, "\r\n"); i >= 0 {
		key = key[:i]
	}
	// 2. Treat backslash as a path separator (Windows-style), then take the
	//    final path segment. path.Base handles trailing slashes and collapses
	//    "." / empty to ".".
	base := path.Base(strings.ReplaceAll(key, `\`, "/"))
	// 3. Drop characters that could break out of the quoted header value or are
	//    otherwise unsafe (double quote, C0 control chars, DEL).
	base = strings.Map(func(r rune) rune {
		if r == '"' || r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, base)
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == ".." {
		return "download"
	}
	return base
}
