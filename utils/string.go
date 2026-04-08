package utils

import "strings"

func EclipseString(s string, max int) string {
	var tmp = strings.TrimSpace(s)
	if len(s) > max {
		tmp = s[:max] + "..."
	}
	return tmp
}
