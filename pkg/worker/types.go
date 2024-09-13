package worker

import (
	"net/http"

	"github.com/ish-xyz/registry-cache/pkg/cache"
	"github.com/ish-xyz/registry-cache/pkg/gc"
	"github.com/sirupsen/logrus"
)

const (
	REQUEST_USED_FOR_CACHE = "used-for-cache"
	CACHE_READ_ERROR       = "CacheReadError"
	UPSTREAM_ERROR         = "UpstreamError"
)

type ContextKey string

type Worker struct {
	queue  chan *cache.CacheRequest
	cache  cache.Cache
	index  cache.Index
	client *http.Client
	log    *logrus.Entry
	gc     *gc.GarbageCollector
}
