package core

import "net/http"

// Get the root part of the URL of the request
func GetRootURLFromRequest(r *http.Request) string {
	// Find the protocol (http ou https)
	var protocol string
	if r.TLS != nil {
		protocol = "https://"
	} else {
		protocol = "http://"
	}

	// Build root URL
	rootUrl := protocol + r.Host
	return rootUrl
}
