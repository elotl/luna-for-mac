package runtimeservice

import (
	"path/filepath"
	"strings"
)

func splitPath(p string) []string {
	parts := strings.Split(filepath.Clean(p), string(filepath.Separator))
	result := make([]string, 0)
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// Check if path p1 is inside p2. Example: /usr/bin is inside /usr.
func isInsidePath(p1, p2 string) bool {
	p1Parts := splitPath(p1)
	p2Parts := splitPath(p2)
	if len(p2Parts) > len(p1Parts) {
		return false
	}
	for i := range p2Parts {
		if p1Parts[i] != p2Parts[i] {
			return false
		}
	}
	return true
}
