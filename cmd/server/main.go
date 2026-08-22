package main

import (
	"context"
	"example.com/gflow/internal/api"
	"example.com/gflow/internal/config"
	"example.com/gflow/internal/engine"
	"example.com/gflow/internal/executor"
	"example.com/gflow/internal/store"
	"example.com/gflow/internal/web"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
)

func main() {
	c := config.Load()
	config.Flags(&c)
	flag.Parse()
	s := store.NewMemory()
	e := engine.New(s)
	x := executor.New(s, e, c.Interval)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	x.Start(ctx)
	mux := http.NewServeMux()
	mux.Handle("/api/", api.New(s, e).Routes())
	mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(web.FS))))
	mux.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/ui/", http.StatusFound) })
	srv := &http.Server{Addr: c.Addr, Handler: mux}
	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()); x.Stop() }()
	log.Printf("gflow listening on %s", c.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
