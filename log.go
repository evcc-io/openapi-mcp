package openapi2mcp

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// logHTTPRequest logs an HTTP request in human-readable format
func logHTTPRequest(req *http.Request, body []byte) {
	timestamp := time.Now().Format("2006-01-02 15:04:05 MST")

	log.Printf("┌─ HTTP REQUEST ────────────────────────────────────────────────────────────────")
	log.Printf("│ 🕐 %s", timestamp)
	log.Printf("│ 🌐 %s %s", req.Method, req.URL.String())

	// Log headers (excluding sensitive auth headers in detail)
	if len(req.Header) > 0 {
		log.Printf("│ 📋 Headers:")
		for name, values := range req.Header {
			if strings.ToLower(name) == "authorization" {
				log.Printf("│    %s: [REDACTED]", name)
			} else if strings.ToLower(name) == "cookie" {
				log.Printf("│    %s: [REDACTED]", name)
			} else {
				log.Printf("│    %s: %s", name, strings.Join(values, ", "))
			}
		}
	}

	// Log body if present and not too large
	if len(body) > 0 {
		if len(body) > 1000 {
			log.Printf("│ 📄 Body: %s... (%d bytes)", string(body[:1000]), len(body))
		} else {
			log.Printf("│ 📄 Body: %s", string(body))
		}
	}

	log.Printf("└───────────────────────────────────────────────────────────────────────────────")
}

// logHTTPResponse logs an HTTP response in human-readable format
func logHTTPResponse(resp *http.Response, body []byte) {
	timestamp := time.Now().Format("2006-01-02 15:04:05 MST")

	// Status icon based on response code
	var statusIcon string
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		statusIcon = "✅"
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		statusIcon = "🔄"
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		statusIcon = "❌"
	case resp.StatusCode >= 500:
		statusIcon = "💥"
	default:
		statusIcon = "❓"
	}

	log.Printf("┌─ HTTP RESPONSE ───────────────────────────────────────────────────────────────")
	log.Printf("│ 🕐 %s", timestamp)
	log.Printf("│ %s %d %s", statusIcon, resp.StatusCode, resp.Status)

	// Log important headers
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		log.Printf("│ 📋 Content-Type: %s", contentType)
	}
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		log.Printf("│ 📋 Content-Length: %s", contentLength)
	}

	// Log body if present and not too large
	if len(body) > 0 {
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "json") || strings.Contains(contentType, "text") {
			if len(body) > 1000 {
				log.Printf("│ 📄 Body: %s... (%d bytes)", string(body[:1000]), len(body))
			} else {
				log.Printf("│ 📄 Body: %s", string(body))
			}
		} else {
			log.Printf("│ 📄 Body: [Binary content, %d bytes, type: %s]", len(body), contentType)
		}
	}

	log.Printf("└───────────────────────────────────────────────────────────────────────────────")
}
