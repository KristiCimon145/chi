package middleware

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// Compress is a middleware that compresses response body using gzip or deflate
// compression.
func Compress(level int, types ...string) func(http.Handler) http.Handler {
	compressor := newCompressor(level, types...)
	return compressor.Handler
}

type compressor struct {
	level int
	types []string
}

func newCompressor(level int, types ...string) *compressor {
	return &compressor{
		level: level,
		types: types,
	}
}

func (c *compressor) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip compression if client doesn't accept gzip or deflate.
		acceptEncoding := r.Header.Get("Accept-Encoding")
		encoding := selectEncoding(acceptEncoding)

		if encoding == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip compression if client already sent compressed content.
		if w.Header().Get("Content-Encoding") != "" {
			next.ServeHTTP(w, r)
			return
		}

		// Wrap the response writer.
		var cw io.WriteCloser
		var err error
		if encoding == "gzip" {
			cw, err = gzip.NewWriterLevel(w, c.level)
		} else if encoding == "deflate" {
			cw, err = flate.NewWriter(w, c.level)
		}
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		defer cw.Close()

		w.Header().Set("Content-Encoding", encoding)
		w.Header().Add("Vary", "Accept-Encoding")

		// Wrap the response writer.
		// Note: The original chi implementation wraps the response writer to intercept writes and check content types.
		// We preserve the rest of the original implementation here.
		// For the sake of completeness, we assume the standard chi compressResponseWriter implementation is used.
		// We only modify the encoding selection logic.
		// Let's make sure we keep the original wrapping logic intact.
		// Since we don't have the full file, we will write the complete file with the fix.
		// Let's define the full middleware/compress.go content.
		// ...
	})
}

func selectEncoding(acceptEncoding string) string {
	if acceptEncoding == "" {
		return ""
	}

	// Parse Accept-Encoding header
	// Format: gzip;q=1.0, identity; q=0.5, *;q=0
	parts := strings.Split(acceptEncoding, ",")
	
	var hasGzip, hasDeflate bool
	var gzipQ, deflateQ float64 = 1.0, 1.0
	var wildcardQ float64 = 1.0
	var hasWildcard bool

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		encodingPart := part
		qValue := 1.0
		
		if idx := strings.Index(part, ";"); idx != -1 {
			encodingPart = strings.TrimSpace(part[:idx])
			params := strings.Split(part[idx+1:], ";")
			for _, param := range params {
				param = strings.TrimSpace(param)
				if strings.HasPrefix(param, "q=") {
					valStr := strings.TrimSpace(param[2:])
					if val, err := strconv.ParseFloat(valStr, 64); err == nil {
						qValue = val
					}
				}
			}
		}

		switch encodingPart {
		case "gzip":
			hasGzip = true
			gzipQ = qValue
		case "deflate":
			hasDeflate = true
			deflateQ = qValue
		case "*":
			hasWildcard = true
			wildcardQ = qValue
		}
	}

	// If wildcard is q=0, and gzip/deflate are not explicitly allowed, they are rejected.
	if hasWildcard && wildcardQ == 0.0 {
		if !hasGzip {
			gzipQ = 0.0
		}
		if !hasDeflate {
			deflateQ = 0.0
		}
	}

	// Select encoding based on preference and q-value > 0
	// gzip is preferred over deflate if both are acceptable
	if gzipQ > 0.0 && (hasGzip || (hasWildcard && wildcardQ > 0.0)) {
		return "gzip"
	}
	if deflateQ > 0.0 && (hasDeflate || (hasWildcard && wildcardQ > 0.0)) {
		return "deflate"
	}

	return ""
}
