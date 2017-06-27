package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/golang/geo/s2"
)

func filesystemCache(center s2.LatLng, zoom int, marker []marker, x, y int) (io.ReadCloser, error) {
	cacheKey := cacheKeyHelper(center, zoom, marker, x, y)
	cacheFileName := path.Join(cfg.CacheDir, cacheKey[0:2], cacheKey+".png")

	if info, err := os.Stat(cacheFileName); err == nil && info.ModTime().Add(cfg.ForceCache).After(time.Now()) {
		return os.Open(cacheFileName)
	}

	// No cache hit, generate a new map
	mapReader, err := generateMap(center, zoom, marker, x, y)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, mapReader); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(path.Dir(cacheFileName), 0755); err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(cacheFileName, buf.Bytes(), 0644); err != nil {
		return nil, err
	}

	return ioutil.NopCloser(buf), err
}
