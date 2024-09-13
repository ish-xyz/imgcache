package cache

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func NewCache(idx Index, dp string) *LocalCache {
	return &LocalCache{
		dataPath:    dp,
		index:       idx,
		LRUQueue:    list.New(),
		LRUElements: make(map[CacheKey]*list.Element),
		log:         logrus.WithField("name", "cache"),
	}
}

// TODO: make it nicer
// and review
func NewCacheRequest(r *http.Request, dataPath string) *CacheRequest {

	var df DataFile
	var err error

	var cr = &CacheRequest{
		CacheEnabled: false,
		Request:      r.Clone(context.TODO()), // TODO: improve context usage
		Response:     make(chan *CacheResponse, 1),
	}

	// create cache request for layers
	if REGEX_LAYER.MatchString(r.URL.Path) && r.Method == http.MethodGet {
		groups := REGEX_LAYER.FindStringSubmatch(r.URL.Path)
		df, err = ComputeLayerFile(dataPath, groups[1])
		if err != nil {
			return cr
		}
		cr.CacheEnabled = true
		cr.DataFile = df
		cr.AuthKey = ComputeAuthKey(r.Header.Get("Authorization"))
		cr.CacheKey = CacheKey(groups[1])
		cr.ResponseFilePath = ComputeResponseFilePath(string(df))
		cr.ItemType = "layer"
	}

	if REGEX_MANIFEST.MatchString(r.URL.Path) && r.Method == http.MethodGet {
		groups := REGEX_MANIFEST.FindStringSubmatch(r.URL.Path)
		df, err = ComputeManifestFile(dataPath, groups[1])
		if err != nil {
			return cr
		}
		cr.CacheEnabled = true
		cr.DataFile = df
		cr.AuthKey = ComputeAuthKey(r.Header.Get("Authorization"))
		cr.CacheKey = CacheKey(groups[1])
		cr.ResponseFilePath = ComputeResponseFilePath(string(df))
		cr.ItemType = "manifest"
	}

	return cr
}

func (c *LocalCache) GetDataPath() string {
	return c.dataPath
}

func (c *LocalCache) Restore() error {

	files, err := c.List()
	if err != nil {
		return fmt.Errorf("failed to restore: %v", err)
	}

	for _, f := range files {
		rf := &ResponseFile{}
		df := filepath.Join(c.dataPath, f.Name())
		metapath := ComputeResponseFilePath(df)
		err := rf.Load(metapath)
		if err != nil {
			return err
		}
		c.index.Put(rf.CacheKey, DataFile(df))
		c.index.SetStatus(rf.CacheKey, STATUS_AVAILABLE)
		c.index.SetResponseFile(rf.CacheKey, rf)

		// avoid the cache key to stay in progress
		err = c.index.SetStatus(rf.CacheKey, STATUS_AVAILABLE)
		if err != nil {
			c.index.Delete(rf.CacheKey)
		}
	}

	return nil
}

func (c *LocalCache) List() ([]fs.DirEntry, error) {

	files, err := os.ReadDir(c.GetDataPath())
	if err != nil {
		return nil, err
	}
	return files, nil
}

// Create files on disk and add response file in index
func (c *LocalCache) Create(cr *CacheRequest, respfile *ResponseFile, content io.ReadCloser) error {

	err := c.index.SetResponseFile(cr.CacheKey, respfile)
	if err != nil {
		// we don't want to write the layer if we can't write the meta file
		return err
	}

	// using name ending with .partial so that GC ignores the files while are getting downloaded
	partialdf := fmt.Sprintf("%s%s", string(cr.DataFile), SUFFIX_PARTIAL_FILE)
	dst, err := os.Create(partialdf)
	if err != nil {
		return fmt.Errorf("failed to create cache file '%v' '%v'", cr.DataFile, err)
	}

	err = os.Rename(partialdf, string(cr.DataFile))
	if err != nil {
		os.Remove(partialdf) // try to remove. If fails, will be removed by GC
		return fmt.Errorf("failed to rename partial cache file '%s' to '%v'", partialdf, cr.DataFile)
	}

	// try to dump ResponseFile on disk for restore
	_ = respfile.Dump(cr.ResponseFilePath)
	buf := make([]byte, 4*1024)
	_, err = io.CopyBuffer(dst, content, buf)

	c.LRUElements[cr.CacheKey] = c.LRUQueue.PushFront(cr.CacheKey)

	return err
}

func (c *LocalCache) Read(cr *CacheRequest) (io.ReadCloser, *ResponseFile, error) {

	meta, err := c.index.GetResponseFile(cr.CacheKey)
	if err != nil {
		return nil, nil, err
	}
	// update last access time for cache key
	c.index.SetATime(cr.CacheKey)
	//TODOX: move up in the LRUqueue

	file, err := os.Open(string(cr.DataFile))
	if err != nil {
		return nil, nil, err
	}

	if el, ok := c.LRUElements[cr.CacheKey]; ok {
		c.LRUQueue.MoveToFront(el)
	}

	return file, meta, nil
}

// Deletes file from disk and index entries
// TODO: possible data race condition here as there's no locking
func (c *LocalCache) Delete(filepath DataFile, ckey CacheKey, atomic bool) error {

	var err error

	if filepath == "" && ckey == "" {
		return fmt.Errorf("empty cache key and empty datafile")
	}

	if filepath == "" {
		filepath, _ = c.index.GetDatafile(ckey)
	}

	if ckey == "" {
		// uses ResponseFile so get this value before deleteing the ResponseFile
		ckey = c.index.GetDataRef(filepath)
		if err != nil && atomic {
			return err
		}
	}

	// remove ckey if it exists
	if ckey != "" {
		// TODOX: remove from LRUqueue
		if le, ok := c.LRUElements[ckey]; ok {
			c.LRUQueue.Remove(le)
		}
		c.index.Delete(ckey)
	}

	// remove underlying file if it exists
	if filepath != "" {
		err = os.Remove(string(filepath))
		if !errors.Is(err, os.ErrNotExist) && atomic {
			return err
		}
		err = os.Remove(string(ComputeResponseFilePath(string(filepath))))
		if !errors.Is(err, os.ErrNotExist) && atomic {
			return err
		}
	}

	return nil
}

func (c *LocalCache) GetLeastUsedFile() (DataFile, error) {
	el := c.LRUQueue.Back()

	ckey, ok := el.Value.(CacheKey)
	if !ok {
		return "", fmt.Errorf("can't cast LRU element into CacheKey %v", el.Value)
	}

	return c.index.GetDatafile(ckey)
}
