package internal

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Version defines the version of apoco.
const Version = "v0.0.6"
const PStep = "recognition/post-correction"

// IDFromFilePath generates an id based on the file group and the file
// path.
func IDFromFilePath(path, fg string) string {
	// Use base path and remove file extensions.
	path = filepath.Base(path)
	path = path[0 : len(path)-len(filepath.Ext(path))]
	// Split everything after the last `_`.
	splits := strings.Split(path, "_")
	return fmt.Sprintf("%s_%s", fg, splits[len(splits)-1])
}
