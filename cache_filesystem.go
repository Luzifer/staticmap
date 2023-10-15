package main

import (
	"bytes"
	"io"
	"os"
	"path"
	"time"

	"github.com/pkg/errors"
)

const (
	cacheDirMode  = 0o700
	cacheFileMode = 0o600
)

func filesystemCache(opts generateMapConfig) (io.ReadCloser, error) {
	cacheKey := opts.getCacheKey()
	cacheFileName := path.Join(cfg.CacheDir, cacheKey[0:2], cacheKey+".png")

	if info, err := os.Stat(cacheFileName); err == nil && info.ModTime().Add(cfg.ForceCache).After(time.Now()) {
		f, err := os.Open(cacheFileName) //#nosec:G304 // Intended to open a variable file
		return f, errors.Wrap(err, "opening file")
	}

	// No cache hit, generate a new map
	mapReader, err := generateMap(opts)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if _, err = io.Copy(buf, mapReader); err != nil {
		return nil, errors.Wrap(err, "writing file")
	}

	if err = os.MkdirAll(path.Dir(cacheFileName), cacheDirMode); err != nil {
		return nil, errors.Wrap(err, "creating cache dir")
	}

	if err = os.WriteFile(cacheFileName, buf.Bytes(), cacheFileMode); err != nil {
		return nil, errors.Wrap(err, "writing cache file")
	}

	return io.NopCloser(buf), err
}
