package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"
)

func filesystemCache(opts generateMapConfig) (io.ReadCloser, error) {
	cacheKey := opts.getCacheKey()
	cacheFileName := path.Join(cfg.CacheDir, cacheKey[0:2], cacheKey+".png")

	if info, err := os.Stat(cacheFileName); err == nil && info.ModTime().Add(cfg.ForceCache).After(time.Now()) {
		return os.Open(cacheFileName)
	}

	// No cache hit, generate a new map
	mapReader, err := generateMap(opts)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if _, err = io.Copy(buf, mapReader); err != nil {
		return nil, err
	}

	if err = os.MkdirAll(path.Dir(cacheFileName), 0700); err != nil {
		return nil, err
	}

	if err = ioutil.WriteFile(cacheFileName, buf.Bytes(), 0644); err != nil {
		return nil, err
	}

	return ioutil.NopCloser(buf), err
}
