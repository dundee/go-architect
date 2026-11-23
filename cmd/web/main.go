package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/go-architect/go-architect/internal/server"
	"github.com/go-architect/go-architect/web"
)

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			*s = append(*s, trimmed)
		}
	}
	return nil
}

func main() {
	var (
		projectPath = flag.String("path", ".", "Path to the Go Project to analyze")
		addr        = flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
		openBrowser = flag.Bool("open", true, "Open the web UI in the default browser")
		orgPackages stringSliceFlag
	)
	flag.Var(&orgPackages, "org", "Organization package prefix (repeatable or comma-separated)")
	flag.Parse()

	if flag.NArg() > 0 && *projectPath == "." {
		*projectPath = flag.Arg(0)
	}

	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatalf("embedded frontend error: %v", err)
	}

	srv, err := server.New(*projectPath, orgPackages, distFS)
	if err != nil {
		log.Fatalf("startup failed: %v", err)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", *addr, err)
	}

	url := fmt.Sprintf("http://%s", ln.Addr().String())
	ctxInfo := srv.ProjectContext()
	log.Printf("Go Architect serving %s (%s)", ctxInfo.Project.Package, ctxInfo.Project.Path)
	if len(ctxInfo.Project.OrganizationPackages) > 0 {
		log.Printf("Organization package prefixes: %v", ctxInfo.Project.OrganizationPackages)
	}
	log.Printf("Listening on %s", url)

	httpServer := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	if *openBrowser {
		go launchBrowser(url)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-sigCh:
		log.Printf("Received %s, shutting down...", sig)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown error: %v", err)
		}
	}
}

func launchBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("could not open browser automatically: %v", err)
	}
}
