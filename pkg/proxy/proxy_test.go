package proxy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ish-xyz/registry-cache/pkg/cache"
	"github.com/ish-xyz/registry-cache/pkg/worker"
	"github.com/stretchr/testify/assert"
)

func getUpstreamRules(rules []map[string]string) ([]*UpstreamRule, error) {
	var urules = make([]*UpstreamRule, 0)
	for _, r := range rules {

		u, err := NewUpstreamRule(r["host"], r["scheme"], r["regex"])
		if err != nil {
			return nil, err
		}
		urules = append(urules, u)
	}
	return urules, nil
}

func TestIntegrationFullOK(t *testing.T) {

	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`OK`))
	}))
	defer testServer.Close()

	testServerUrl, _ := url.Parse(testServer.URL)
	dataPath := "/tmp/registry-cache-data"
	urulesMap := []map[string]string{
		{
			"regex":  "^localhost:8000$",
			"host":   testServerUrl.Host,
			"scheme": testServerUrl.Scheme,
		},
	}

	err := os.MkdirAll("/tmp/registry-cache-data", 0777)
	if err != nil {
		panic(err)
	}

	indexObj := cache.NewMemoryIndex()
	cacheObj := cache.NewCache(indexObj, dataPath)
	workerObj := worker.NewWorker(cacheObj, indexObj, testServer.Client(), nil)
	urules, _ := getUpstreamRules(urulesMap)

	proxyObj := NewProxy(
		workerObj,
		"0.0.0.0:8000",
		dataPath,
		testServerUrl.Host,
		testServerUrl.Scheme,
		fmt.Sprintf("%s/../../config/localhost.crt", baseDir),
		fmt.Sprintf("%s/../../config/localhost.key", baseDir),
		urules,
	)

	proxyDone := &sync.WaitGroup{}

	go proxyObj.Start(10, false, proxyDone)

	time.Sleep(2 * time.Second)

	cclient := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}

	req, err := http.NewRequest("GET", "https://localhost:8000/v2/nvidia/cudagl/blobs/sha256:8bd98d4761dc30931a35b249051f59e5deb9a7a3b3dee384fd3b99ca03e792eb", nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", "Baerer somethingsomething")
	resp, respErr := cclient.Do(req)
	resp2, respErr2 := cclient.Do(req)

	ckey := cache.CacheKey("8bd98d4761dc30931a35b249051f59e5deb9a7a3b3dee384fd3b99ca03e792eb")
	dataFile := cache.DataFile(fmt.Sprintf("%s/%s", dataPath, "8bd98d4761dc30931a35b249051f59e5deb9a7a3b3dee384fd3b99ca03e792eb.layer"))
	fetchedDF, fetchDFErr := indexObj.GetDatafile(ckey)
	_, statErr := os.Stat(string(dataFile))

	fmt.Println(resp2)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp2)
	assert.Nil(t, respErr)
	assert.Nil(t, respErr2)
	assert.Equal(t, dataFile, fetchedDF)
	assert.Nil(t, fetchDFErr)
	assert.Nil(t, statErr)
	// TODO get prometheus metrics
	// clean up data
}
