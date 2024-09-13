package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ish-xyz/registry-cache/pkg/cache"
	"github.com/ish-xyz/registry-cache/pkg/gc"
	"github.com/ish-xyz/registry-cache/pkg/metrics"
	"github.com/sirupsen/logrus"
)

func NewWorker(ch cache.Cache, idx cache.Index, cl *http.Client, gc *gc.GarbageCollector) *Worker {
	return &Worker{
		cache:  ch,
		index:  idx,
		queue:  make(chan *cache.CacheRequest, 100),
		client: cl,
		log:    logrus.WithField("name", "worker"),
		gc:     gc,
	}
}

func (w *Worker) Push(cr *cache.CacheRequest) {
	w.queue <- cr
}

func (w *Worker) Pop() *cache.CacheRequest {
	return <-w.queue
}

// Send HEAD request to check authn and authz to the upstream resource
func (w *Worker) authRequest(r *http.Request) bool {

	r.Method = http.MethodHead
	r.Body = nil
	r.ContentLength = 0
	resp, err := w.client.Do(r)

	if err != nil {
		w.log.Errorln("head request failed:", err)
		return false
	}
	defer resp.Body.Close()

	w.log.Tracef(
		"checkAuth()  status: '%d', path: '%s', auth: '%s'",
		resp.StatusCode, r.URL.Path, r.Header.Get("Authorization"),
	)
	return resp.StatusCode == http.StatusOK
}

func (w *Worker) checkPerms(cr *cache.CacheRequest) error {

	isAuthorised := w.authRequest(cr.Request.Clone(context.TODO()))
	if !isAuthorised {
		return fmt.Errorf("authentication HEAD request failed")
	}

	return nil
}

// Fetch request from upstream registry and return it
func (w *Worker) getResponseFromUpstream(cr *cache.CacheRequest, usedForCache bool) (*http.Response, error) {

	var err error

	r := cr.Request.Clone(context.TODO())
	resp := &http.Response{}
	defaultBadGatewayResponse := &http.Response{
		Status:     http.StatusText(http.StatusBadGateway),
		StatusCode: 502,
		Body:       io.NopCloser(bytes.NewBufferString("Upstream is broken mate!")),
		Header:     make(http.Header),
	}

	//Bump active connections counter
	metrics.UpstreamConn.Add(1)

	if usedForCache {
		resp, err = w.client.Do(r)
	} else {
		// let the real client handle the request and act as reverse proxy
		resp, err = w.client.Transport.RoundTrip(r)
	}
	if err != nil {
		metrics.FailedRequests.WithLabelValues(UPSTREAM_ERROR, cr.Request.URL.Path).Inc()
		return defaultBadGatewayResponse, err
	}

	return resp, nil
}

func (w *Worker) getResponseFromCache(cr *cache.CacheRequest) (*http.Response, error) {

	w.log.Tracef("serving from cache. Load data file %s", cr.DataFile)
	freader, meta, err := w.cache.Read(cr)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", cr.DataFile, err)
	}

	logrus.Tracef("meta file loaded: %+v ", meta)
	resp := &http.Response{
		Status:        meta.Status,
		StatusCode:    meta.StatusCode,
		Proto:         meta.Proto,
		Body:          freader,
		Uncompressed:  meta.Uncompressed,
		ContentLength: int64(meta.ContentLength),
		Header:        meta.Header, // NOTE: headers are read only
		Request:       cr.Request.Clone(context.TODO()),
	}

	return resp, nil
}

func (w *Worker) storeFile(ctx context.Context, cr *cache.CacheRequest) error {

	now := time.Now()
	currWorkerId := ctx.Value(ContextKey("id")).(int)
	err := w.index.SetWorker(cr.CacheKey, currWorkerId, false)
	if err != nil {
		return err
	}

	allocatedWorker := w.index.GetWorker(cr.CacheKey)
	if allocatedWorker != currWorkerId {
		return nil
	}

	respForCache, err := w.getResponseFromUpstream(cr, true)
	if respForCache.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream returned a non-200 response: %v", respForCache.StatusCode)
	}

	if err != nil {
		return fmt.Errorf("error while requesting upstream: %v", err)
	}
	defer respForCache.Body.Close()

	err = w.index.SetStatus(cr.CacheKey, cache.STATUS_IN_PROGRESS)
	if err != nil {
		return fmt.Errorf("failed to set status for cache request: %v", err)
	}

	respfile := cache.NewResponseFile(
		int(respForCache.ContentLength),
		respForCache.StatusCode,
		respForCache.Header,
		cr.CacheKey,
	)
	err = w.cache.Create(cr, respfile, respForCache.Body)
	if err != nil {
		// reset status if download/write failed
		w.index.SetStatus(cr.CacheKey, cache.STATUS_NOT_FOUND)
		return fmt.Errorf("error while writing file to disk: %v", err)
	}

	err = w.index.SetStatus(cr.CacheKey, cache.STATUS_AVAILABLE)
	if err != nil {
		return fmt.Errorf("error while setting status in index for cachekey %v", err)
	}

	w.index.SetWorker(cr.CacheKey, cache.NO_WORKER, true)
	if err != nil {
		return fmt.Errorf("failed to remove allocated worker: %v", err)
	}

	// Update metrics

	// Bump connections counter by -1
	metrics.UpstreamConn.Add(-1)

	// Update Pull Speed metric
	bytesPerSecond := float64(respForCache.ContentLength) / time.Since(now).Seconds()
	metrics.UpstreamPullSpeed.WithLabelValues(
		string(cr.CacheKey),
		cr.ItemType,
	).Set(bytesPerSecond / 1024 / 1024 * 8) //calculate mbps

	w.log.Infof("file %s stored locally", cr.DataFile)

	return nil
}

func (w *Worker) handleFromUpstream(cr *cache.CacheRequest) {
	resp, _ := w.getResponseFromUpstream(cr, false)
	cacheResponse := &cache.CacheResponse{Response: resp, Origin: cache.ORIGIN_UPSTREAM}
	cr.Response <- cacheResponse
}

func (w *Worker) waitForStatusUpdate(ckey cache.CacheKey) (int, error) {

	ckeystatus := w.index.GetStatus(ckey)
	for x := 0; x <= 300; x++ {
		ckeystatus = w.index.GetStatus(ckey)
		if ckeystatus != cache.STATUS_NOT_FOUND {
			return ckeystatus, nil
		}
		time.Sleep(time.Duration(20) * time.Millisecond)
	}
	return ckeystatus, fmt.Errorf("timeout waiting for cache key status")
}

// Run single worker
func (w *Worker) Run(ctx context.Context) {

	w.log.Infoln("start worker", ctx.Value(ContextKey("id")))
	log := logrus.WithField("name", fmt.Sprintf("worker-%d", ctx.Value(ContextKey("id"))))

	for {

		// wait for messages from the queue
		cr := w.Pop()
		if cr.CacheEnabled {

			ckeystatus := w.index.GetStatus(cr.CacheKey)
			if ckeystatus == cache.STATUS_NOT_FOUND {

				// no perms no party
				err := w.checkPerms(cr)
				if err != nil {
					w.handleFromUpstream(cr)
					continue
				}

				// we need the entry in the index
				// 	before selecting the workers/etc
				err = w.index.Put(cr.CacheKey, cr.DataFile)
				if err != nil {
					w.handleFromUpstream(cr)
					continue
				}

				err = w.storeFile(ctx, cr)
				if err != nil {
					log.Warning("failed to store file locally:", err)
					w.handleFromUpstream(cr)
					continue
				}

				ckeystatus, err = w.waitForStatusUpdate(cr.CacheKey)
				if err != nil {
					w.log.Traceln("pushing CR back into queue (status hasn't changed):", cr)
					w.Push(cr)
					continue
				}
			}

			// if another thread is downloading the data, push the request back into the queue
			if ckeystatus == cache.STATUS_IN_PROGRESS {
				w.log.Traceln("pushing CR back into queue:", cr)
				w.Push(cr)
				continue
			}

			if ckeystatus == cache.STATUS_AVAILABLE {
				err := w.checkPerms(cr)
				if err != nil {
					w.handleFromUpstream(cr)
					continue
				}

				resp, err := w.getResponseFromCache(cr)
				if err != nil {
					w.log.Warningln("failed to fetch data from cache:", err)

					metrics.FailedRequests.WithLabelValues(CACHE_READ_ERROR, cr.Request.URL.Path).Inc()

					go w.gc.Try() // try to cleanup bad cache (fails if gc is already running)

					w.handleFromUpstream(cr)
					continue
				}

				metrics.TotalCachedRequests.WithLabelValues(cr.ItemType, string(cr.CacheKey)).Inc()
				cacheResponse := &cache.CacheResponse{Response: resp, Origin: cache.ORIGIN_CACHE}
				cr.Response <- cacheResponse
				continue
			}
		}

		// if no condition is met, serve from upstream
		w.handleFromUpstream(cr)
	}

}

func (w *Worker) Start(nworkers int) {
	for n := 0; n < nworkers; n++ {
		ctx := context.WithValue(context.TODO(), ContextKey("id"), n)
		go w.Run(ctx)
	}
}
