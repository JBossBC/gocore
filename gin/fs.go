package gin

import (
	"net/http"
	"os"
)

type onlyFileFS struct {
	fs http.FileSystem
}

type neuteredReaddirFile struct {
	http.File
}

func Dir(root string, listDirectory bool) http.FileSystem {
	fs := http.Dir(root)
	if listDirectory {
		return fs

	}
	return &onlyFileFS{fs}
}

func (fs onlyFileFS) Open(name string) (http.File, error) {
	f, err := fs.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return neuteredReaddirFile{f}, nil
}
func (f neuteredReaddirFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, nil
}
