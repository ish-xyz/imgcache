package cache

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

func NewMemoryIndex() *MemoryIndex {
	return &MemoryIndex{
		meta:    make(map[CacheKey]*CacheKeyMetadata),
		store:   make(map[CacheKey]DataFile),
		dataref: make(map[DataFile]CacheKey),
	}
}

// ###############
// *** GETTERS ***
// ###############

// Get meta file from in-memory index
func (i *MemoryIndex) GetResponseFile(ckey CacheKey) (*ResponseFile, error) {
	i.metaLock.RLock()
	defer i.metaLock.RUnlock()

	if _, ok := i.meta[ckey]; ok {
		// create clone of resp meta
		return i.meta[ckey].ResponseFile, nil
	}
	return &ResponseFile{}, fmt.Errorf("cache key not found")
}

func (i *MemoryIndex) GetWorker(ckey CacheKey) int {
	i.metaLock.RLock()
	defer i.metaLock.RUnlock()

	if data, ok := i.meta[ckey]; ok {

		return data.WorkerID
	}

	return NO_WORKER
}

/*
Get status of cached item:

	STATUS_NOT_FOUND   = -1
	STATUS_AVAILABLE   = 2
	STATUS_IN_PROGRESS = 1
*/
func (i *MemoryIndex) GetStatus(ckey CacheKey) int {

	i.metaLock.RLock()
	defer i.metaLock.RUnlock()

	if data, ok := i.meta[ckey]; ok {
		return data.Status
	}
	return STATUS_NOT_FOUND
}

// Get atime from in-memory index
func (i *MemoryIndex) GetATime(ckey CacheKey) (int64, error) {

	i.metaLock.RLock()
	defer i.metaLock.RUnlock()

	if data, ok := i.meta[ckey]; ok {
		return data.Atime, nil
	}
	return -1, fmt.Errorf("cache key not found")
}

// Get ctime from in-memory index
func (i *MemoryIndex) GetCTime(ckey CacheKey) (int64, error) {

	i.metaLock.RLock()
	defer i.metaLock.RUnlock()

	if data, ok := i.meta[ckey]; ok {
		return data.Ctime, nil
	}
	return -1, fmt.Errorf("cache key not found")
}

// Get CacheKey from DataFile path
func (i *MemoryIndex) GetDataRef(df DataFile) CacheKey {

	i.datarefLock.RLock()
	defer i.datarefLock.RUnlock()

	if _, ok := i.dataref[df]; ok {
		return i.dataref[df]
	}
	return ""
}

func (i *MemoryIndex) GetDatafile(ckey CacheKey) (DataFile, error) {

	i.storeLock.RLock()
	defer i.storeLock.RUnlock()

	if _, ok := i.store[ckey]; ok {
		return i.store[ckey], nil
	}
	return "", fmt.Errorf("auth key does not exists")
}

// ###############
// *** SETTERS ***
// ###############
func (i *MemoryIndex) SetResponseFile(ckey CacheKey, rf *ResponseFile) error {
	i.metaLock.Lock()
	defer i.metaLock.Unlock()

	if _, ok := i.meta[ckey]; ok {
		i.meta[ckey].ResponseFile = rf
		return nil
	}
	return fmt.Errorf("cache key not found")
}

func (i *MemoryIndex) SetATime(ckey CacheKey) error {
	i.metaLock.Lock()
	defer i.metaLock.Unlock()

	if data, ok := i.meta[ckey]; ok {
		data.Atime = int64(time.Now().Unix())
	}
	return fmt.Errorf("invalid cache key")
}

// set status for cache key, fails only if the cache key doesn't exists
func (i *MemoryIndex) SetStatus(ckey CacheKey, status int) error {
	i.metaLock.Lock()
	defer i.metaLock.Unlock()

	if data, ok := i.meta[ckey]; ok {
		data.Status = status
		return nil
	}
	return fmt.Errorf("failed to update cache key status, not found")
}

func (i *MemoryIndex) SetWorker(ckey CacheKey, id int, force bool) error {
	i.metaLock.Lock()
	defer i.metaLock.Unlock()

	if data, ok := i.meta[ckey]; ok {

		if force || data.WorkerID == NO_WORKER {
			data.WorkerID = id
		}

		return nil
	}
	return fmt.Errorf("failed to update cache key status, not found")
}

// ###############
// ** Delete/List Methods
// ###############

// return list for keys from the cache store
func (i *MemoryIndex) ListCacheKeys() []CacheKey {

	i.storeLock.RLock()
	defer i.storeLock.RUnlock()

	return maps.Keys(i.store)
}

//TODO: add methods for CacheKeyMetadata

// Insert cache entry if doesn't exists
// Sets all values to defaults
// This method must be idempotent
func (i *MemoryIndex) Put(ckey CacheKey, df DataFile) error {

	if ckey == "" || df == "" {
		return fmt.Errorf("invalid value for cache key or datafile")
	}

	i.globalLock()
	defer i.globalUnLock()

	if _, metaOK := i.meta[ckey]; !metaOK {
		now := int64(time.Now().Unix())
		i.meta[ckey] = &CacheKeyMetadata{
			Atime:        now,
			Ctime:        now,
			Status:       STATUS_NOT_FOUND,
			ResponseFile: nil,
			WorkerID:     NO_WORKER,
		}
	}

	if _, storeOK := i.store[ckey]; !storeOK {
		// using the DEFAULT_AUTH_KEY
		// 		to initialise the structure
		i.store[ckey] = df
	}

	if _, datarefOK := i.dataref[df]; !datarefOK {
		i.dataref[df] = ckey
	}
	return nil
}

func (i *MemoryIndex) Delete(ckey CacheKey) {

	i.globalLock()
	defer i.globalUnLock()

	if df, ok := i.store[ckey]; ok {
		delete(i.dataref, df)
	}
	delete(i.store, ckey)
	delete(i.meta, ckey)

}

// ###############
// ** Others
// ###############

func (i *MemoryIndex) Len() int {

	i.storeLock.RLock()
	defer i.storeLock.RUnlock()

	return len(i.store)
}

func (i *MemoryIndex) globalLock() {
	i.metaLock.Lock()
	i.datarefLock.Lock()
	i.storeLock.Lock()
}

func (i *MemoryIndex) globalUnLock() {
	i.metaLock.Unlock()
	i.datarefLock.Unlock()
	i.storeLock.Unlock()
}

func (i *MemoryIndex) Print() {
	logrus.Debugln("----------------------------------------")
	logrus.Debugln("store:", i.store)
	logrus.Debugln("----------------------------------------")
}
