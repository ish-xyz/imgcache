package gc

import (
	"sync"
	"time"

	"github.com/ish-xyz/registry-cache/pkg/cache"
	"github.com/inhies/go-bytesize"
	"github.com/sirupsen/logrus"
)

type GarbageCollector struct {
	interval time.Duration
	disk     struct {
		maxSize bytesize.ByteSize
	}
	layers struct {
		maxUnused time.Duration
		maxAge    time.Duration
	}
	manifests struct {
		maxUnused time.Duration
		maxAge    time.Duration
	}
	checkSHA bool
	cache    cache.Cache
	index    cache.Index
	log      *logrus.Entry
	mu       sync.Mutex
}
