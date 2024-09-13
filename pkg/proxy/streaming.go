package proxy

import (
	"errors"
	"io"
	"net/http"

	"github.com/ish-xyz/registry-cache/pkg/metrics"
	"github.com/sirupsen/logrus"
)

func (p *Proxy) streamResponse(w http.ResponseWriter, resp *http.Response, originCache bool) error {

	defer resp.Body.Close()

	logrus.Tracef("cloning response headers")
	// clone headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// Add status code
	if resp.StatusCode >= 100 {
		w.WriteHeader(resp.StatusCode)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		p.log.Errorf(
			"setting status code to 500, invalid status code received => code: %d, url: %s, origin-cache: %t\n",
			resp.StatusCode,
			resp.Request.URL.String(),
			originCache,
		)
	}

	var err error
	if originCache && resp.ContentLength != -1 {
		metrics.TotalBytesServedFromCache.Add(float64(resp.ContentLength))
		_, err = lazyStream(w, resp.Body, resp.ContentLength)
	} else {
		_, err = io.Copy(w, resp.Body)
		// Bump connections counter by -1
		metrics.UpstreamConn.Add(-1)
	}
	if err != nil {
		return err
	}
	return nil
}

func lazyStream(dst io.Writer, src io.Reader, totalBytes int64) (written int64, err error) {

	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		var er error
		var nr int

		nr, er = src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = errors.New("short write")
				break
			}
		}
		if er != nil && written == totalBytes {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
