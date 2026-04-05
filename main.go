package main

import (
	"os"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func main() {
	port := os.Getenv("PROXY_PORT")
	
	if port == "" {
		port = "4848"
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/", proxyHandler)

	log.Printf("YouTube Stream Proxy starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func healthHandler (w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for all origins (adjust in production)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type")
	
	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	// Only allow GET requests
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Extract the target host from query parameter
	targetHost := r.URL.Query().Get("host")
	if targetHost == "" {
		http.Error(w, "Missing 'host' query parameter", http.StatusBadRequest)
		return
	}
	
	// Validate it's a googlevideo.com domain
	if !strings.HasSuffix(targetHost, ".googlevideo.com") {
		http.Error(w, "Invalid host - must be googlevideo.com", http.StatusBadRequest)
		return
	}
	
	// Build the YouTube URL
	targetURL := buildYouTubeURL(targetHost, r.URL)
	
	log.Printf("Proxying request to: %s", targetURL)
	
	// Create request to YouTube
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Set minimal headers that YouTube expects
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.youtube.com/")
	req.Header.Set("Origin", "https://www.youtube.com")
	
	// Make the request
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}
	
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to fetch from YouTube: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	
	// Copy response headers to client
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	
	// Ensure CORS headers are set (might be overwritten above)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Content-Type")
	
	// Write status code
	w.WriteHeader(resp.StatusCode)
	
	// Stream the response body
	written, err := io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error streaming response: %v (wrote %d bytes)", err, written)
		return
	}
	
	log.Printf("Successfully proxied %d bytes (status: %d)", written, resp.StatusCode)
}

func buildYouTubeURL(host string, requestURL *url.URL) string {
	// Start with https and the target host
	targetURL := &url.URL{
		Scheme: "https",
		Host:   host,
		Path:   requestURL.Path,
	}
	
	// Copy all query parameters except 'host'
	query := url.Values{}
	for key, values := range requestURL.Query() {
		if key != "host" {
			for _, value := range values {
				query.Add(key, value)
			}
		}
	}
	
	targetURL.RawQuery = query.Encode()
	return targetURL.String()
}
