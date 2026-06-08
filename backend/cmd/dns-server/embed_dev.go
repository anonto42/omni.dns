//go:build !embed

package main

import (
	"net/http"
	"os"
)

func getFileSystem() http.FileSystem {
	// Fallback to local directory if not embedded
	return http.Dir("./static")
}

func isEmbedded() bool {
	return false
}
