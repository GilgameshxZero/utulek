package main

import (
	"fmt"
	"strings"
)

func optionClass(key string) (string, bool) {
	switch key {
	case "-h", "--help":
		return "help", false
	case "-v", "--version":
		return "version", false
	case "-c", "--config":
		return "config", true
	case "--debug":
		return "debug", false
	case "--skip-unparsable":
		return "skip", false
	case "-a", "--addressbook":
		return "book", true
	case "-A", "--target-addressbook":
		return "targetbooks", true
	case "-t", "--target-contact", "--target":
		return "target", true
	case "-i", "--input-file":
		return "input", true
	case "-o", "--output-file":
		return "output", true
	case "--format":
		return "format", true
	case "-d", "--display":
		return "display", true
	case "-s", "--sort":
		return "sort", true
	case "-F", "--fields":
		return "fields", true
	case "--vcard-version":
		return "vcardversion", true
	case "-p", "--parsable":
		return "parsable", false
	case "--mutt":
		return "mutt", false
	case "--force":
		return "force", false
	case "--open-editor", "--edit":
		return "editor", false
	case "-r", "--reverse":
		return "reverse", false
	case "-g", "--group-by-addressbook":
		return "group", false
	case "-f", "--search-in-source-files":
		return "source", false
	case "-H", "--headers":
		return "headers", true
	case "--skip-already-added":
		return "skipadded", false
	case "--patch":
		return "patch", false
	case "--remove-first-line":
		return "firstline", false
	}
	return "", false
}
func expandShortOptions(args []string) []string {
	var expanded []string
	for i, a := range args {
		if a == "--" {
			expanded = append(expanded, args[i:]...)
			break
		}
		if len(a) <= 2 || a[0] != '-' || a[1] == '-' || strings.ContainsRune(a, '=') {
			expanded = append(expanded, a)
			continue
		}
		known := true
		for pos := 1; pos < len(a); pos++ {
			key := "-" + a[pos:pos+1]
			class, value := optionClass(key)
			if class == "" {
				known = false
				break
			}
			if value {
				break
			}
		}
		if !known {
			expanded = append(expanded, a)
			continue
		}
		for pos := 1; pos < len(a); pos++ {
			key := "-" + a[pos:pos+1]
			expanded = append(expanded, key)
			_, value := optionClass(key)
			if value {
				if pos+1 < len(a) {
					expanded = append(expanded, a[pos+1:])
				}
				break
			}
		}
	}
	return expanded
}
func validateCommandOptions(o Options) error {
	sortCommands := "list show edit remove copy move merge email phone postaddress filename"
	allows := func(commands string) bool { return contains(strings.Fields(commands), o.Command) }
	for _, key := range o.Provided {
		class, _ := optionClass(key)
		allowed := false
		switch class {
		case "help", "version", "config", "debug", "skip":
			allowed = true
		case "book":
			allowed = !allows("template addressbooks")
		case "source":
			allowed = !allows("template addressbooks new")
		case "display":
			allowed = allows(sortCommands + " birthdays add-email")
		case "sort", "reverse", "group":
			allowed = allows(sortCommands + " add-email")
		case "targetbooks":
			allowed = allows("copy move merge")
		case "target":
			allowed = allows("merge")
		case "input", "editor":
			allowed = allows("new edit")
			if class == "input" {
				allowed = allows("new edit add-email")
			}
		case "output":
			allowed = allows("show")
		case "format":
			allowed = allows("show new edit")
		case "fields":
			allowed = allows("list")
		case "vcardversion":
			allowed = allows("new add-email")
		case "parsable":
			allowed = allows("list email phone postaddress birthdays")
		case "mutt", "firstline":
			allowed = allows("email")
		case "force":
			allowed = allows("remove")
		case "headers", "skipadded":
			allowed = allows("add-email")
		case "patch":
			allowed = allows("edit")
		}
		if !allowed {
			return fmt.Errorf("%s does not apply to %s", key, o.Command)
		}
	}
	return nil
}
