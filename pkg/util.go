package pkg

import (
	"strings"
)

func TrimSpaces(s string) string {
	return strings.Trim(s, "\r\n\t\v\f ")
}

func StandardizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
