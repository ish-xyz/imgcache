package proxy

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var (
	REGEX_LAYER    = regexp.MustCompile("^/.*/blobs/sha256:(.+)$")
	REGEX_MANIFEST = regexp.MustCompile("^/.*/manifests/sha256:(.+)$")
)

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

func getLabelsFromPath(path string) (string, string, error) {

	if REGEX_LAYER.MatchString(path) {
		parts := REGEX_LAYER.FindStringSubmatch(path)
		return parts[1], "layer", nil

	} else if REGEX_MANIFEST.MatchString(path) {
		parts := REGEX_MANIFEST.FindStringSubmatch(path)
		return parts[1], "manifest", nil
	}

	return "", "", fmt.Errorf("not a layer or manifest request")
}
