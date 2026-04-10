package proxy

import (
	"net/url"
	"strings"
)

var duplicatedAPIPrefixes = []string{
	"/v1beta",
	"/v1alpha",
	"/v1",
}

func joinUpstreamPath(basePath, requestPath string) string {
	req := strings.TrimSpace(requestPath)
	if req == "" {
		req = "/"
	}
	if !strings.HasPrefix(req, "/") {
		req = "/" + req
	}

	base := strings.TrimSpace(basePath)
	if base == "" || base == "/" {
		return req
	}
	base = "/" + strings.Trim(strings.TrimSuffix(base, "/"), "/")

	for _, prefix := range duplicatedAPIPrefixes {
		if strings.HasSuffix(base, prefix) && (req == prefix || strings.HasPrefix(req, prefix+"/")) {
			req = strings.TrimPrefix(req, prefix)
			if req == "" {
				req = "/"
			}
			break
		}
	}

	if req == "/" {
		return deduplicateV1Prefix(base)
	}
	return deduplicateV1Prefix(base + req)
}

func buildHealthProbeURL(baseURL, requestPath string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		trimmedBase := strings.TrimRight(baseURL, "/")
		return trimmedBase + joinUpstreamPath("", requestPath)
	}
	parsed.Path = joinUpstreamPath(parsed.Path, requestPath)
	parsed.RawPath = parsed.Path
	return parsed.String()
}
