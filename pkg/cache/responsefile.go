package cache

import (
	"encoding/json"
	"net/http"
	"os"
)

func NewResponseFile(cLen int, status int, headers http.Header, ck CacheKey) *ResponseFile {

	return &ResponseFile{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Proto:         DEFAULT_PROTO,
		ContentLength: cLen,
		Header:        headers,
		Uncompressed:  DEFAULT_UNCOMPRESSED,
		CacheKey:      ck,
	}
}

func (m *ResponseFile) Dump(path string) error {

	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(jsonBytes)

	return err
}

func (m *ResponseFile) Load(path string) error {

	jsonBytes, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	err = json.Unmarshal(jsonBytes, m)
	if err != nil {
		return err
	}

	return nil
}
