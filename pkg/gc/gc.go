package gc

import (
	"sync"
	"time"

	"github.com/ish-xyz/registry-cache/pkg/cache"
	"github.com/ish-xyz/registry-cache/pkg/metrics"
	"github.com/inhies/go-bytesize"
	"github.com/sirupsen/logrus"
)

func NewGarbageCollector(
	ch cache.Cache,
	idx cache.Index,
	maxSize bytesize.ByteSize,
	checkSHA bool,
	interval,
	mMaxAge,
	mMaxUnused,
	lMaxAge,
	lMaxUnused time.Duration) *GarbageCollector {

	return &GarbageCollector{
		cache:    ch,
		index:    idx,
		checkSHA: checkSHA,
		interval: interval,
		disk: struct {
			maxSize bytesize.ByteSize
		}{
			maxSize: maxSize,
		},
		manifests: struct {
			maxUnused time.Duration
			maxAge    time.Duration
		}{
			maxUnused: mMaxUnused,
			maxAge:    mMaxAge,
		},
		layers: struct {
			maxUnused time.Duration
			maxAge    time.Duration
		}{
			maxUnused: lMaxUnused,
			maxAge:    lMaxAge,
		},
		log: logrus.WithField("name", "gc"),
		mu:  sync.Mutex{},
	}
}

func (gc *GarbageCollector) Start() {

	// indipendent gc
	go gc.reduceDiskUsage()

	for {

		func() {
			gc.mu.Lock()
			defer gc.mu.Unlock()

			gc.index.Print() // works only in debug mode
			metrics.TotalGCRuns.Inc()
			gc.cleanUndesiredFiles()
			gc.cleanOrphanFiles()
			gc.cleanCacheKeys()
			if gc.checkSHA {
				gc.cleanCorruptLayerFiles()
			}
			gc.checkStalePartialFiles()
		}()

		time.Sleep(gc.interval)

	}
}

func (gc *GarbageCollector) Try() {
	if gc.mu.TryLock() {
		func() {
			gc.index.Print() // works only in debug mode
			metrics.TotalGCRuns.Inc()
			gc.cleanUndesiredFiles()
			gc.cleanCacheKeys()
			gc.cleanOrphanFiles()
			if gc.checkSHA {
				gc.cleanCorruptLayerFiles()
			}
		}()
		gc.mu.Unlock()
	}
}
