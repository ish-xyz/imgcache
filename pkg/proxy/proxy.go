package proxy

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/ish-xyz/registry-cache/pkg/cache"
	"github.com/ish-xyz/registry-cache/pkg/metrics"

	"github.com/ish-xyz/registry-cache/pkg/worker"
	"github.com/sirupsen/logrus"
)

func NewProxy(
	wk *worker.Worker,
	addr,
	dp,
	defaultBackendHost,
	defaultBackendScheme,
	cPath,
	kPath string,
	urules []*UpstreamRule,
) *Proxy {

	return &Proxy{
		worker:   wk,
		address:  addr,
		dataPath: dp,
		defaultBackend: struct {
			Host   string
			Schema string
		}{
			Host:   defaultBackendHost,
			Schema: defaultBackendScheme,
		},
		upstreamRules: urules,
		tlsCertPath:   cPath,
		tlsKeyPath:    kPath,
		log:           logrus.WithField("name", "proxy"),
	}
}

func NewUpstreamRule(host, scheme, regex string) (*UpstreamRule, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}

	return &UpstreamRule{
		host:   host,
		scheme: scheme,
		regex:  re,
	}, nil
}

// Rewrite request from client for the upstream registry
func (p *Proxy) rewriteRequest(r *http.Request) {

	// DO NOT REMOVE
	// http: Request.RequestURI can't be set in client/proxy requests.
	// http://golang.org/src/pkg/net/http/client.go

	// make sure upstream request is correct
	r.RequestURI = ""

	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		appendHostToXForwardHeader(r.Header, clientIP)
	}

	r.Header.Set(HEADER_ORIGINAL_HOST, r.Host)

	for i, cfg := range p.upstreamRules {

		if !cfg.regex.MatchString(r.Host) {
			p.log.Debugf("requested host '%s' doesn't match regex '%s' in rule '%d', skipping.", r.Host, cfg.regex, i)
			continue
		}

		upstreamHost := cfg.host
		groups := cfg.regex.FindStringSubmatch(r.Host)
		for i, g := range groups {
			groupPlaceHolder := fmt.Sprintf("%s%d", HOST_PLACEHOLDER_PREFIX, i)
			upstreamHost = strings.Replace(upstreamHost, groupPlaceHolder, g, -1)
		}

		r.URL.Scheme = cfg.scheme
		r.URL.Host = upstreamHost
		r.Host = upstreamHost

		p.log.Debugf("new destination set '%s'", upstreamHost)
		return
	}

	// no match, set default backend
	r.URL.Scheme = p.defaultBackend.Schema
	r.URL.Host = p.defaultBackend.Host
	r.Host = p.defaultBackend.Host
}

// Proxy entrypoint
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Handle health probe
	// TODO: this could be written better
	if r.RequestURI == "/health" {
		fmt.Fprintf(w, "Healthy")
		return
	}

	logrus.Tracef("request: %+v", r)
	p.rewriteRequest(r) // rewrite request for upstream
	logrus.Tracef("rewritten request: %+v", r)

	cr := cache.NewCacheRequest(r, p.dataPath) //TODO: datapath should be in the cache object only
	logrus.Tracef("cache request: %+v", cr)

	p.worker.Push(cr)
	cresp := <-cr.Response

	// Bump cache miss metric
	if cresp.Origin != cache.ORIGIN_CACHE && cr.CacheEnabled {
		sha256, ptype, err := getLabelsFromPath(r.URL.Path)
		if err == nil {
			metrics.TotalCacheMiss.WithLabelValues(ptype, sha256).Inc()
		}
	}

	metrics.ActiveClientsConn.Add(1)
	err := p.streamResponse(w, cresp.Response, cresp.Origin == cache.ORIGIN_CACHE)
	metrics.ActiveClientsConn.Add(-1)

	if err != nil {
		metrics.FailedRequests.WithLabelValues(STREAMING_ERROR, cr.Request.URL.Path).Inc()
		p.log.Errorf(
			"(%s) [%s - %s %s%s, err: %v]",
			cresp.Origin,
			cresp.Response.Status,
			cresp.Response.Request.Method,
			cresp.Response.Request.Host,
			cresp.Response.Request.URL.Path,
			err,
		)
	}

	p.log.Infof(
		"(%s) [%s - %s %s%s]",
		cresp.Origin,
		cresp.Response.Status,
		cresp.Response.Request.Method,
		cresp.Response.Request.Host,
		cresp.Response.Request.URL.Path,
	)
}

// Start proxy
func (p *Proxy) Start(workers int, debug bool, wg *sync.WaitGroup) *http.Server {

	p.log.Infoln("starting workers...")
	p.worker.Start(workers)

	p.log.Info("web server listening on ", p.address)

	srv := http.Server{
		Addr:              p.address,
		Handler:           p,
		ReadHeaderTimeout: 5 * time.Second, // prevent slowloris
	}

	go func() {
		defer wg.Done()

		if err := srv.ListenAndServeTLS(p.tlsCertPath, p.tlsKeyPath); err != http.ErrServerClosed {
			p.log.Fatalln(err)
		}
	}()

	return &srv
}
