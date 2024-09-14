package main

import (
	"io"
	"regexp"
)

var respace = regexp.MustCompile(`\s+`)

type zero = struct{}

func readfull(r io.Reader, destination []byte) (n int, err error) {
	remaining := destination
	for len(remaining) > 0 {
		if nn, err := r.Read(remaining); err != nil {
			return n, err
		} else {
			n += nn
			remaining = remaining[nn:]
		}
	}
	return n, err
}
