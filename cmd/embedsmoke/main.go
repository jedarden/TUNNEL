// Command embedsmoke is a minimal carrier for the embedded web UI. It
// exercises the same Fiber frontend mount used by cmd/tunnel without pulling
// in the application's providers and authentication packages.
//
// Usage:
//
//	tunnel-smoke [-addr 127.0.0.1:0]
//
// It prints the address it is serving on as a single line, "http://<addr>/",
// and serves the embedded frontend with SPA fallback until interrupted.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/jedarden/tunnel/internal/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "address to serve on (port 0 picks a free port)")
	flag.Parse()

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	if err := web.MountFrontend(app); err != nil {
		log.Fatalf("web UI not embedded: %v", err)
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", *addr, err)
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- app.Listener(listener)
	}()

	fmt.Printf("http://%s/\n", listener.Addr().String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-sigCh:
		if err := app.Shutdown(); err != nil {
			log.Printf("shut down web server: %v", err)
		}
	case err := <-serveErr:
		if err != nil {
			log.Fatalf("serve web UI: %v", err)
		}
	}
}
