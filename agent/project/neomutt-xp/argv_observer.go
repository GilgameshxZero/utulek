// Test fixture only: records argv instead of reading an address book.
package main

import (
	"encoding/json"
	"os"
)

func main() {
	json.NewEncoder(os.Stdout).Encode(os.Args[1:])
}
