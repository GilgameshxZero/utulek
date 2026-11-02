package main

import (
	"errors"
	"strings"
)

func splitCommand(input string) ([]string, error) {
	args := []string{}
	var buffer strings.Builder
	quote := rune(0)
	escaped := false
	active := false
	for _, r := range input {
		if escaped {
			buffer.WriteRune(r)
			escaped = false
			active = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			active = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				buffer.WriteRune(r)
			}
			active = true
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			active = true
			continue
		}
		if r == ' ' || r == '\t' {
			if active {
				args = append(args, buffer.String())
				buffer.Reset()
				active = false
			}
			continue
		}
		buffer.WriteRune(r)
		active = true
	}
	if quote != 0 || escaped {
		return nil, errors.New("invalid EDITOR quoting")
	}
	if active {
		args = append(args, buffer.String())
	}
	if len(args) == 0 || args[0] == "" {
		return nil, errors.New("EDITOR is empty")
	}
	return args, nil
}
