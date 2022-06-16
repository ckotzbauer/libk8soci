package util

import "strings"

// Unescape removes backslashes and double-quotes from strings
func Unescape(s string) string {
	s = strings.ReplaceAll(s, "\\", "")
	s = strings.ReplaceAll(s, "\"", "")
	return s
}
