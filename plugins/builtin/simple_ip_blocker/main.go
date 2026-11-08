// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package main

import (
	"HomeWaf/plugin/proto"
	"HomeWaf/plugin/shared"
	"log"
)

func main() {
	if err := shared.Serve(&proto.GRPCServer{Impl: NewSimpleIPBlockerPlugin()}); err != nil {
		log.Fatal(err)
	}
}
