package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"strings"

	"github.com/golang/geo/s2"
)

type cacheFunction func(center s2.LatLng, zoom int, marker []marker, x, y int, disableAttribution bool) (io.ReadCloser, error)

func cacheKeyHelper(center s2.LatLng, zoom int, marker []marker, x, y int, disableAttribution bool) string {
	markerString := []string{}
	for _, m := range marker {
		markerString = append(markerString, m.String())
	}
	hashString := fmt.Sprintf("%s|%d|%s|%dx%d|%v", center.String(), zoom, strings.Join(markerString, "+"), x, y, disableAttribution)

	return fmt.Sprintf("%x", sha256.Sum256([]byte(hashString)))
}
