package main

import (
	"os"
	"regexp"
	"runtime"
	"strings"
)

var bookPathVariables = regexp.MustCompile(`\$\{[^}]+\}|\$[A-Za-z_][A-Za-z_0-9]*|%[^%]+%`)

// Match expandvars for native book paths; preserve unknown references rather
// than silently deleting part of a path. Do not expand editor arguments.
func expandBookPath(s string) (string, error) {
	s = bookPathVariables.ReplaceAllStringFunc(s, func(token string) string {
		name := token[1:]
		if token[0] == '%' {
			if runtime.GOOS != "windows" {
				return token
			}
			name = token[1 : len(token)-1]
		} else if strings.HasPrefix(name, "{") {
			name = name[1 : len(name)-1]
		}
		if value, ok := os.LookupEnv(name); ok {
			return value
		}
		return token
	})
	return expandPath(s)
}
