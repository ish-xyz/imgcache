package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateEntry(t *testing.T) {
	myindex := NewMemoryIndex()
	err := myindex.Put("key", "")
	err2 := myindex.Put("key", "datafile")

	_, existsInStore := myindex.store["key"]
	_, existsInMeta := myindex.meta["key"]
	_, existsInDataRef := myindex.dataref["datafile"]

	assert.NotNil(t, err)
	assert.Nil(t, err2)
	assert.True(t, existsInStore)
	assert.True(t, existsInMeta)
	assert.True(t, existsInDataRef)
}

func TestDeleteEntry(t *testing.T) {
	myindex := NewMemoryIndex()
	myindex.Put("key", "datafile")
	myindex.Delete("key")

	_, existsInStore := myindex.store["key"]
	_, existsInMeta := myindex.meta["key"]
	_, existsInDataRef := myindex.dataref["datafile"]

	assert.False(t, existsInStore)
	assert.False(t, existsInMeta)
	assert.False(t, existsInDataRef)
}

func TestList(t *testing.T) {
	myindex := NewMemoryIndex()
	myindex.Put("key", "datafile")
	keys := myindex.ListCacheKeys()
	assert.Equal(t, 1, len(keys))
}

func TestLen(t *testing.T) {
	myindex := NewMemoryIndex()
	myindex.Put("key", "datafile")
	indexLen := myindex.Len()
	assert.Equal(t, 1, indexLen)
}

func TestSetGetMetaFile(t *testing.T) {
	myindex := NewMemoryIndex()
	respfile := NewResponseFile(1000, 200, nil, "key")
	myindex.Put("key", "datafile")

	err := myindex.SetResponseFile("key", respfile)
	getMeta, _ := myindex.GetResponseFile("key")
	assert.Nil(t, err)
	assert.Equal(t, getMeta, respfile)
}

func TestSetGetStatus(t *testing.T) {
	myindex := NewMemoryIndex()
	err := myindex.SetStatus("key", STATUS_AVAILABLE)
	assert.NotNil(t, err)

	myindex.Put("key", "datafile")
	err2 := myindex.SetStatus("key", STATUS_AVAILABLE)
	status := myindex.GetStatus("key")

	assert.Nil(t, err2)
	assert.Equal(t, STATUS_AVAILABLE, status)
}

func TestGetWorker(t *testing.T) {
	currentWorkerID := 11
	myindex := NewMemoryIndex()
	myindex.Put("key", "datafile")

	before := myindex.GetWorker("key")
	myindex.SetWorker("key", currentWorkerID, false)
	after := myindex.GetWorker("key")

	assert.Equal(t, NO_WORKER, before)
	assert.Equal(t, currentWorkerID, after)
}

func TestSetWorker(t *testing.T) {

	myindex := NewMemoryIndex()
	myindex.Put("key", "datafile")

	myindex.SetWorker("key", 10, false)
	first := myindex.GetWorker("key")

	myindex.SetWorker("key", 11, true)
	second := myindex.GetWorker("key")

	myindex.SetWorker("key", 12, false)
	third := myindex.GetWorker("key")

	assert.Equal(t, 10, first)
	assert.Equal(t, 11, second)
	assert.NotEqual(t, 12, third)
	assert.Equal(t, 11, third)
}

func TestSetGetAtime(t *testing.T) {
	myindex := NewMemoryIndex()
	myindex.Put("key", "datafile")

	timePreUpdate, _ := myindex.GetATime("key")
	time.Sleep(1 * time.Second)
	myindex.SetATime("key")
	timePostUpdate, _ := myindex.GetATime("key")

	assert.NotEqual(t, timePreUpdate, timePostUpdate)
}

func TestGetCtime(t *testing.T) {
	myindex := NewMemoryIndex()
	myindex.Put("key", "datafile")

	ctime, _ := myindex.GetCTime("key")

	assert.NotEqual(t, 0, ctime) // this is as good as it gets for this test
}

func TestGetDataRef(t *testing.T) {
	myindex := NewMemoryIndex()
	myindex.Put("key", "datafile")

	retrievedDF, _ := myindex.GetDatafile("key")

	assert.Equal(t, CacheKey("key"), myindex.GetDataRef("datafile"))
	assert.Equal(t, DataFile("datafile"), retrievedDF)
}
