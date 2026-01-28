package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"codex-manager/internal/config"
	"codex-manager/internal/render"
	"codex-manager/internal/search"
	"codex-manager/internal/sessions"
	"codex-manager/internal/web"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatalf("config error: %v", err)
	}
	sessions.SetTrimUserRequestEnabled(!cfg.NoTrimRequest)

	idx := sessions.NewIndex(cfg.SessionsDir)
	if err := idx.Refresh(); err != nil {
		log.Printf("initial scan failed: %v", err)
	}

	searchIdx := search.NewIndex()
	if err := searchIdx.RefreshFrom(idx); err != nil {
		log.Printf("initial search index build failed: %v", err)
	}

	go func() {
		ticker := time.NewTicker(cfg.RescanInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := idx.Refresh(); err != nil {
				log.Printf("rescan failed: %v", err)
				continue
			}
			if err := searchIdx.RefreshFrom(idx); err != nil {
				log.Printf("search reindex failed: %v", err)
			}
		}
	}()

	renderer, err := render.New()
	if err != nil {
		log.Fatalf("template error: %v", err)
	}

	server := web.NewServer(idx, searchIdx, renderer, cfg.SessionsDir, cfg.ShareDir, cfg.ShareAddr, cfg.Theme)
	shareServer := web.NewShareServer(cfg.ShareDir)

	log.Printf("Codex sessions server listening on %s", cfg.Addr)
	log.Printf("Share server listening on %s", cfg.ShareAddr)
	log.Printf("Watching sessions in %s", cfg.SessionsDir)
	go func() {
		if err := http.ListenAndServe(cfg.ShareAddr, shareServer); err != nil {
			log.Fatalf("share server error: %v", err)
		}
	}()
	if cfg.UseTailscale {
		host, err := web.SetupTailscale(cfg.ShareAddr)
		if err != nil {
			log.Fatalf("tailscale setup error: %v", err)
		}
		server.EnableTailscale(host)
		log.Printf("Tailscale share host: %s", host)
	} else {
		log.Printf("Not using tailscale share")
	}
	if err := http.ListenAndServe(cfg.Addr, server); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
