// Command bridge runs the GenesisMesh Game Bridge: a small HTTP service that
// translates Luanti game requests into GenesisMesh identity, capability,
// delegation, boundary, and audit operations. See section 4.3 of
// GenesisMesh-Luanti-Server-Requirements.md.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/GenesisMeshLabs/genesis-world-lab/bridge/internal/server"
)

func main() {
	addr := os.Getenv("BRIDGE_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	s := server.New()
	log.Printf("genesismesh game bridge listening on %s", addr)
	if err := http.ListenAndServe(addr, s); err != nil {
		log.Fatal(err)
	}
}
