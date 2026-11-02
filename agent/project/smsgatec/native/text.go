package main

func width(s string) int {
	n := 0
	for _, r := range s {
		n += runeWidth(r)
	}
	return n
}
func clip(s string, limit int) string {
	result := []rune{}
	used := 0
	for _, r := range clean(s) {
		if r == '\n' || r == '\t' {
			r = ' '
		}
		n := runeWidth(r)
		if used+n > limit {
			break
		}
		result = append(result, r)
		used += n
	}
	return string(result)
}
func wrapped(s string, limit int) []string {
	if limit < 1 {
		limit = 1
	}
	result := []string{}
	line := []rune{}
	used := 0
	for _, r := range clean(s) {
		if r == '\n' {
			result = append(result, string(line))
			line = nil
			used = 0
			continue
		}
		if r == '\t' {
			r = ' '
		}
		n := runeWidth(r)
		if used+n > limit && len(line) > 0 {
			result = append(result, string(line))
			line = nil
			used = 0
		}
		line = append(line, r)
		used += n
	}
	return append(result, string(line))
}
