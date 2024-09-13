package gc

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ish-xyz/registry-cache/pkg/metrics"
)

// should watch disk usage
// use LRU queue to remove files

func getFiles(path string) ([]fs.FileInfo, error) {
	var files []fs.FileInfo

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			files = append(files, info)
		}
		return err
	})

	return files, err
}

func getDirSize(path string) (int64, error) {

	var dirSize int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			dirSize += info.Size()
		}
		return err
	})

	return dirSize, err
}

func (gc *GarbageCollector) reduceDiskUsage() {

	datapath := gc.cache.GetDataPath()
	for {
		size, err := getDirSize(datapath)
		if err != nil {
			gc.log.Errorln("failed to calculate dir size:", err)
		}

		metrics.CacheSize.Set(float64(size))

		tolerance := int64(gc.disk.maxSize) / 100 * 15
		if size < int64(gc.disk.maxSize)-tolerance {
			gc.log.Debugln("disk space is under the limit")
		} else {
			luf, err := gc.cache.GetLeastUsedFile()
			if err != nil {
				gc.log.Errorln("failed to fetch least used file from queue")
				continue
			}

			err = gc.cache.Delete(luf, "", false)
			if err != nil {
				gc.log.Errorln("error trying to cleanup file ", luf)
			} else {
				gc.log.Infof("deleted file %s (least used)", luf)
			}
			continue
		}
		time.Sleep(100 * time.Second)
	}
}
