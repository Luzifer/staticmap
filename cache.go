package main

import (
	"io"
)

type cacheFunction func(generateMapConfig) (io.ReadCloser, error)
