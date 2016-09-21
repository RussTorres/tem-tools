package utils

import (
	"io"
	"net/http"
	"os"
	"strings"
)

// ReadFromURL returns a reader for the given URL. If the URL contains a local drive
// that is mapped to a URL it replaces the driver with the corresponding URL.
func ReadFromURL(url string, driverMapping map[string]string) (io.ReadCloser, error) {
	if strings.HasPrefix(url, "file:///") {
		url = url[len("file://"):]
	}
	url = remapURL(url, driverMapping)
	if strings.HasPrefix(url, "/") {
		f, err := os.Open(url)

		return f, err
	}
	resp, err := http.Get(url)

	return resp.Body, err
}

func remapURL(url string, driverMapping map[string]string) string {
	mappedURL := url
	for k, v := range driverMapping {
		if strings.HasPrefix(url, "/"+k+"/") {
			mappedURL = v + "/" + url[len("/"+k+"/"):]
			break
		} else if strings.HasPrefix(url, k+":/") {
			mappedURL = v + "/" + url[len(k+":/"):]
			break
		}
	}
	return mappedURL
}
