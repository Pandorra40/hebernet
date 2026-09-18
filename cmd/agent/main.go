package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/hebernet/hebernet/internal/agent"
)

func main() {
	socket := flag.String("socket", envOr("HEBERNET_AGENT_SOCKET", "data/agent.sock"), "unix socket path")
	dataDir := flag.String("data", envOr("HEBERNET_DATA", "data"), "data directory (dry-run artefacts)")
	dryRun := flag.Bool("dry-run", envOr("HEBERNET_DRY_RUN", "1") != "0", "simulate privileged ops (demo / no root)")
	flag.Parse()

	absData, _ := filepath.Abs(*dataDir)
	absSock, _ := filepath.Abs(*socket)
	srv := &agent.Server{SocketPath: absSock, DataDir: absData, DryRun: *dryRun}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		_ = os.Remove(absSock)
		os.Exit(0)
	}()

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
