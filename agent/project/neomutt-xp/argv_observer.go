// Test fixture only: records argv instead of reading an address book.
package main

import (
	"encoding/json"
	"os"
)

func main() {
	if path := os.Getenv("NEOMUTT_TEST_ARGV_OUTPUT"); path != "" {
		data, _ := json.Marshal(os.Args[1:])
		os.WriteFile(path, data, 0600)
	}
	json.NewEncoder(os.Stdout).Encode(os.Args[1:])
}
