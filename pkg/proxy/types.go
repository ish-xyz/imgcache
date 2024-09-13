package proxy

import (
	"net/http"
	"regexp"

	"github.com/ish-xyz/registry-cache/pkg/worker"
	"github.com/sirupsen/logrus"
)

const (
	HOST_PLACEHOLDER_PREFIX = "$group"
	HEADER_ORIGINAL_HOST    = "X-ORIGINAL-HOST"
	STREAMING_ERROR         = "StreamingError"
)

type Proxy struct {
	worker      *worker.Worker
	address     string
	dataPath    string
	tlsCertPath string
	tlsKeyPath  string

	upstreamRules  []*UpstreamRule
	defaultBackend struct {
		Host   string
		Schema string
	}
	streamers      int
	streamingQueue chan *StreamingMessage
	log            *logrus.Entry
}

type UpstreamRule struct {
	regex  *regexp.Regexp
	host   string
	scheme string
}

type StreamingMessage struct {
	Writer        http.ResponseWriter
	CacheResponse *http.Response
	OriginCache   bool
	Error         chan error
}
