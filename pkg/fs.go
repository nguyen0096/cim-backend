package pkg

import (
	"path/filepath"
	"runtime"
)

func TranslateCallerRelativePath(rel string) string {
	_, file, _, _ := runtime.Caller(1)
	dir := filepath.Dir(file)
	return filepath.Join(dir, rel)
}
