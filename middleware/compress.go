package middleware

import (
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5/middleware/compress"
)

// Compress is a middleware that compresses the response body using
// the best available compression algorithm based on the Accept-Encoding header.
// It respects q-values (quality values) as per RFC 9110 Section 12.5.3.
// Encodings with q=0 are considered not acceptable and will be excluded.
func Compress(level int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse Accept-Encoding header
			acceptEncoding := r.Header.Get("Accept-Encoding")
			if acceptEncoding == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Parse the header into a list of acceptable encodings with their quality values
			encodings := parseAcceptEncoding(acceptEncoding)
			if len(encodings) == 0 {
				// No acceptable encodings, serve uncompressed
				next.ServeHTTP(w, r)
				return
			}

			// Select the best encoding that we support
			var selectedEncoding string
			var selectedQuality float64
			for _, e := range encodings {
				if e.quality > selectedQuality {
					// Check if we support this encoding
					if compress.IsSupportedEncoding(e.name) {
						selectedEncoding = e.name
						selectedQuality = e.quality
					}
				}
			}

			if selectedEncoding == "" {
				// No supported encoding found, serve uncompressed
				next.ServeHTTP(w, r)
				return
			}

			// Create a compressing response writer
			cw, err := compress.NewWriter(w, selectedEncoding, level)
			if err != nil {
				// If we can't create a writer for the selected encoding, fall back to no compression
				next.ServeHTTP(w, r)
				return
			}
			defer cw.Close()

			// Set the Content-Encoding header
			w.Header().Set("Content-Encoding", selectedEncoding)
			// Remove Content-Length because the compressed size is different
			w.Header().Del("Content-Length")

			next.ServeHTTP(cw, r)
		})
	}
}

// encoding represents a compression encoding with its quality value.
type encoding struct {
	name    string
	quality float64
}

// parseAcceptEncoding parses the Accept-Encoding header value into a slice of encodings,
// sorted by quality descending (highest quality first).
// It follows RFC 9110 Section 12.5.3: q=0 means "not acceptable".
func parseAcceptEncoding(header string) []encoding {
	var result []encoding
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by ';' to separate encoding from parameters
		subParts := strings.Split(part, ";")
		if len(subParts) == 0 {
			continue
		}

		encodingName := strings.TrimSpace(subParts[0])
		if encodingName == "" {
			continue
		}

		quality := 1.0 // default quality
		for i := 1; i < len(subParts); i++ {
			param := strings.TrimSpace(subParts[i])
			if strings.HasPrefix(param, "q=") {
				qStr := strings.TrimPrefix(param, "q=")
				qStr = strings.TrimSpace(qStr)
				// Parse the quality value
				var q float64
				n, err := fmt.Sscanf(qStr, "%f", &q)
				if err == nil && n == 1 {
					quality = q
				}
				break // only first q parameter counts
			}
		}

		// According to RFC 9110, q=0 means "not acceptable"
		if quality <= 0.0 {
			continue // skip this encoding
		}

		result = append(result, encoding{name: encodingName, quality: quality})
	}

	// Sort by quality descending (highest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].quality > result[j].quality
	})

	return result
}
