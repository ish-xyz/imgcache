package cache

import (
	"container/list"
	"io"
	"io/fs"
	"net/http"
	"regexp"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	REGEX_LAYER    = regexp.MustCompile("^/.*/blobs/sha256:(.+)$")
	REGEX_MANIFEST = regexp.MustCompile("^/.*/manifests/sha256:(.+)$")
)

const (
	ORIGIN_UPSTREAM = "origin-upstream"
	ORIGIN_CACHE    = "origin-cache"

	DEFAULT_PROTO        = "HTTP/1.1"
	DEFAULT_UNCOMPRESSED = false
	DEFAULT_AUTH_KEY     = "[_default]"

	SUFFIX_META_FILE     = ".meta.json"
	SUFFIX_LAYER_FILE    = ".layer"
	SUFFIX_MANIFEST_FILE = ".manifest"
	SUFFIX_PARTIAL_FILE  = ".partial"

	STATUS_NOT_FOUND   = -1
	STATUS_AVAILABLE   = 2
	STATUS_IN_PROGRESS = 1

	NO_WORKER = -1
)

// Interfaces

type Index interface {
	SetResponseFile(ckey CacheKey, rf *ResponseFile) error
	SetStatus(ckey CacheKey, status int) error
	SetWorker(ckey CacheKey, id int, force bool) error
	SetATime(ckey CacheKey) error

	GetResponseFile(ckey CacheKey) (*ResponseFile, error)
	GetStatus(ckey CacheKey) int
	GetWorker(ckey CacheKey) int
	GetATime(ckey CacheKey) (int64, error)

	/* without setter */
	GetCTime(ckey CacheKey) (int64, error)
	GetDataRef(df DataFile) CacheKey
	GetDatafile(ckey CacheKey) (DataFile, error)

	Put(ckey CacheKey, df DataFile) error
	Delete(ckey CacheKey)
	ListCacheKeys() []CacheKey

	Len() int
	Print()
}

type Cache interface {
	Create(cr *CacheRequest, meta *ResponseFile, content io.ReadCloser) error
	Restore() error
	Read(cr *CacheRequest) (io.ReadCloser, *ResponseFile, error)
	Delete(filepath DataFile, ckey CacheKey, atomic bool) error
	GetDataPath() string
	GetLeastUsedFile() (DataFile, error)
	List() ([]fs.DirEntry, error)
}

// Types

type CacheKey string

type AuthKey string

type DataFile string

type LocalCache struct {
	dataPath    string
	index       Index
	LRUQueue    *list.List
	LRUElements map[CacheKey]*list.Element
	log         *logrus.Entry
}

type MemoryIndex struct {
	meta        map[CacheKey]*CacheKeyMetadata
	metaLock    sync.RWMutex
	store       map[CacheKey]DataFile
	storeLock   sync.RWMutex
	dataref     map[DataFile]CacheKey
	datarefLock sync.RWMutex
}

type CacheKeyMetadata struct {
	Status   int
	WorkerID int
	//actual meta file on filesystem with response parameters
	//useful in restore of the cache index, so the proxy know how to respond to clients
	ResponseFile *ResponseFile
	Atime        int64
	Ctime        int64
	Lock         sync.RWMutex
}

type CacheRequest struct {
	CacheEnabled     bool
	CacheKey         CacheKey
	AuthKey          AuthKey
	DataFile         DataFile
	ResponseFilePath string
	Request          *http.Request
	Response         chan *CacheResponse
	ItemType         string
}

type CacheResponse struct {
	Response *http.Response
	Origin   string
}

type ResponseFile struct {
	Status        string              `json:"status"`
	StatusCode    int                 `json:"statusCode"`
	Proto         string              `json:"proto"`
	Header        map[string][]string `json:"header"`
	ContentLength int                 `json:"contentLength"`
	Uncompressed  bool                `json:"uncompressed"`
	CacheKey      CacheKey            `json:"cacheKey"`
}
