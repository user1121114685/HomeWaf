// Generates isolated UI fixtures from the real built-in policies.
package main

import (
	"HomeWaf/wafpipeline"
	"encoding/json"
	"os"
)

func main() {
	if err := json.NewEncoder(os.Stdout).Encode(wafpipeline.BuiltinTemplates()); err != nil {
		panic(err)
	}
}
