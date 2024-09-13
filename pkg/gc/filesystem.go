package gc

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ish-xyz/registry-cache/pkg/cache"
)

func (gc *GarbageCollector) cleanUndesiredFiles() {

	files, err := gc.cache.List()
	if err != nil {
		gc.log.Errorf("failed to list files")
		return
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), cache.SUFFIX_META_FILE) {
			continue
		}
		if strings.HasSuffix(f.Name(), cache.SUFFIX_LAYER_FILE) {
			continue
		}
		if strings.HasSuffix(f.Name(), cache.SUFFIX_MANIFEST_FILE) {
			continue
		}
		if strings.HasSuffix(f.Name(), cache.SUFFIX_PARTIAL_FILE) {
			continue
		}
		df, err := cache.ComputeDataFile(gc.cache.GetDataPath(), f.Name(), "")
		if err != nil {
			gc.log.Errorln("can't compute filename for", f.Name())
			continue
		}
		gc.log.Infoln("deleting undesired file ", df)
		os.Remove(string(df))
	}

}

func (gc *GarbageCollector) cleanOrphanFiles() {

	orphans := map[string]struct{}{}
	files, err := gc.cache.List()
	if err != nil {
		gc.log.Errorf("failed to list files")
		return
	}

	for _, f := range files {

		if strings.HasSuffix(f.Name(), cache.SUFFIX_META_FILE) {
			dataFile := strings.Replace(f.Name(), cache.SUFFIX_META_FILE, "", -1)
			if _, ok := orphans[dataFile]; ok {
				delete(orphans, dataFile)
				continue
			}
			orphans[f.Name()] = struct{}{}

		} else if strings.HasSuffix(f.Name(), cache.SUFFIX_PARTIAL_FILE) {
			continue
		} else {
			respFile := cache.ComputeResponseFilePath(f.Name())

			if _, ok := orphans[respFile]; ok {
				delete(orphans, respFile)
				continue
			}
			orphans[f.Name()] = struct{}{}
		}

	}

	for fname := range orphans {
		f, _ := cache.ComputeDataFile(gc.cache.GetDataPath(), fname, "")
		gc.log.Infoln("deleting orphan file ", f)
		gc.cache.Delete(f, "", false)
	}
}

func (gc *GarbageCollector) checkStalePartialFiles() {
	files, err := gc.cache.List()
	if err != nil {
		gc.log.Errorf("failed to list files")
		return
	}
	for _, i := range files {
		if !strings.HasSuffix(i.Name(), cache.SUFFIX_PARTIAL_FILE) {
			// skipping checksum on metafiles
			continue
		}
		fpath := fmt.Sprintf("%s%s", gc.cache.GetDataPath(), i.Name())

		gc.log.Infof("checking file: '%s'", fpath)
		toDelete := true

		f, err := os.Stat(fpath)
		if err != nil {
			gc.log.Warnf("cannot stat() file: '%s'", fpath)
			continue
		}

		prevfsize := f.Size()
		currfsize := int64(0)
		for counter := 0; counter < 4; counter++ {

			f, err := os.Stat(fpath)
			if err != nil {
				gc.log.Warnf("cannot stat() file: '%s'", fpath)
				continue
			}

			currfsize = f.Size()
			if prevfsize < currfsize {
				toDelete = false
				break
			}
			time.Sleep(1 * time.Second)
		}

		if toDelete {
			gc.log.Infoln("removing stale partial file", fpath)
			os.Remove(fpath)
		}
	}
}

func (gc *GarbageCollector) cleanCorruptLayerFiles() {
	files, err := gc.cache.List()
	if err != nil {
		gc.log.Errorf("failed to list files")
		return
	}
	for _, i := range files {

		if !strings.HasSuffix(i.Name(), cache.SUFFIX_LAYER_FILE) {
			// skipping checksum on metafiles
			continue
		}

		df, _ := cache.ComputeDataFile(gc.cache.GetDataPath(), i.Name(), "")
		f, _ := os.Open(string(df))
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			gc.log.Errorf("%s failed to calculate sha256 %v", df, err)
			continue
		}

		expectedSHA256 := strings.ReplaceAll(i.Name(), cache.SUFFIX_LAYER_FILE, "")
		actualSHA256 := fmt.Sprintf("%x", h.Sum(nil))
		if actualSHA256 != expectedSHA256 {
			gc.log.Infoln("deleting corrupted file:", i.Name())
			gc.cache.Delete(df, "", false)
		}
	}
}
