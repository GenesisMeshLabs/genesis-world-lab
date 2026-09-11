// Command bridge runs the GenesisMesh Game Bridge: a small HTTP service that
// translates Luanti game requests into GenesisMesh identity, capability,
// delegation, boundary, and audit operations. See section 4.3 of
// GenesisMesh-Luanti-Server-Requirements.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/GenesisMeshLabs/genesis-world-lab/bridge/internal/world"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	config := flag.String("config", "", "private lab configuration file")
	provision := flag.Bool("provision", false, "issue missing approved identities and authority memberships")
	keyFile := flag.String("keygen", "", "create or read a private player seed and print its public key")
	flag.Parse()
	if *keyFile != "" {
		pub, e := world.PlayerKey(*keyFile)
		if e != nil {
			log.Fatal(e)
		}
		fmt.Println(pub)
		return
	}
	c, e := world.LoadConfig(*config)
	if e != nil {
		log.Fatal(e)
	}
	if *provision {
		if e = world.Provision(context.Background(), c); e != nil {
			log.Fatal(e)
		}
		return
	}
	s, e := world.New(c)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	srv := &http.Server{Addr: c.Address, Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	go func() {
		<-ctx.Done()
		stop, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = srv.Shutdown(stop)
	}()
	log.Printf("GenesisMesh world bridge listening on %s", c.Address)
	if e = srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		cancel()
		<-done
		log.Fatal(e)
	}
	cancel()
	<-done
}
