package cache

import (
	"fmt"
	"path/filepath"
)

func ComputeAuthKey(authorization string) AuthKey {
	if authorization == "" {
		return DEFAULT_AUTH_KEY
	}

	return AuthKey(fmt.Sprintf("[%s]", authorization))
}

func ComputeResponseFilePath(filePath string) string {
	return fmt.Sprintf("%s%s", filePath, SUFFIX_META_FILE)
}

// Computes the SHA256 of the requestKey and joins that to the configured DataPath
func ComputeDataFile(datapath, name string, suffix string) (DataFile, error) {
	if name == "" {
		return "", fmt.Errorf("invalid file name")
	}
	return DataFile(filepath.Join(datapath, fmt.Sprintf("%s%s", name, suffix))), nil
}

func ComputeLayerFile(datapath, name string) (DataFile, error) {
	return ComputeDataFile(datapath, name, SUFFIX_LAYER_FILE)
}

func ComputeManifestFile(datapath, name string) (DataFile, error) {
	return ComputeDataFile(datapath, name, SUFFIX_MANIFEST_FILE)
}
