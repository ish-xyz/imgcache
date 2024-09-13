package gc

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/ish-xyz/registry-cache/pkg/cache"
)

const (
	_2h = 7200
)

func (gc *GarbageCollector) cleanCacheKey(k cache.CacheKey, df cache.DataFile, cfgMaxAge, cfgMaxU time.Duration) {

	//***  checking max unused
	atimeInt, err := gc.index.GetATime(k)
	if err != nil {
		gc.log.Errorln("failed to read times from index")
	} else {
		atime := time.Unix(atimeInt, 0)
		uvalue := time.Duration(int(cfgMaxU.Seconds()-cfgMaxU.Seconds()*2)) * time.Second
		unused := time.Now().Add(uvalue)
		if unused.After(atime) {
			gc.log.Infoln("deleting unused file: ", df)
			gc.cache.Delete(df, k, false)
			return
		}
	}

	//*** checking max age
	ctimeInt, err := gc.index.GetCTime(k)
	if err != nil {
		gc.log.Errorln("failed to read times from index")
	} else {
		ctime := time.Unix(ctimeInt, 0)
		mvalue := time.Duration(int(cfgMaxAge.Seconds()-cfgMaxAge.Seconds()*2)) * time.Second
		maxAge := time.Now().Add(mvalue)
		if maxAge.After(ctime) {
			gc.log.Infoln("deleting file max age reached: ", df)
			gc.cache.Delete(df, k, false)
			return
		}

	}

	//***  checking missing files
	// key exists but not the underlying file
	if _, err := os.Stat(string(df)); errors.Is(err, os.ErrNotExist) {
		gc.log.Infoln("cleaning up cache key: ", k)
		gc.cache.Delete(df, k, false)
	}
}

func (gc *GarbageCollector) cleanCacheKeys() {

	for _, k := range gc.index.ListCacheKeys() {
		// check cached file max age
		df, _ := gc.index.GetDatafile(k)
		if strings.HasSuffix(string(df), cache.SUFFIX_LAYER_FILE) {
			gc.cleanCacheKey(k, df, gc.layers.maxAge, gc.layers.maxUnused)
			continue
		} else if strings.HasSuffix(string(df), cache.SUFFIX_MANIFEST_FILE) {
			gc.cleanCacheKey(k, df, gc.manifests.maxAge, gc.manifests.maxUnused)
			continue
		}
	}
}
