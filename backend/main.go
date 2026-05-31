// realtime-typing のゲーム本体を動かす単一の Go バイナリ。
//
// ローカルファースト設計。このバイナリ自身が同梱した Solid.js フロントエンド
// (embed.FS) を配信し、メモリ上の goroutine ベースのルームを動かす。プレイに
// クラウドは不要で、同じマシン / LAN / Tailscale 上の2人が直接接続できる。
package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/reisuta/realtime-typing/backend/internal/config"
	"github.com/reisuta/realtime-typing/backend/internal/game"
	"github.com/reisuta/realtime-typing/backend/internal/room"
	"github.com/reisuta/realtime-typing/backend/internal/server"
)

// webFS はフロントエンドを保持する。web/placeholder はコミットしてあるので
// バイナリは常にビルド・起動できる。web/dist は `pnpm build` で生成され
// (gitignore対象)、存在する場合は placeholder より優先して配信される。
//
//go:embed all:web
var webFS embed.FS

// quotesData は同梱した名文データセット（submodule ではなくコピーで持つ）。
//
//go:embed quotes.json
var quotesData []byte

func main() {
	cfg := config.Load()

	lib, err := game.Load(quotesData)
	if err != nil {
		log.Fatalf("load quotes: %v", err)
	}
	log.Printf("loaded %d quotes", lib.Count())

	staticFS, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("web fs: %v", err)
	}

	hub := room.NewHub(lib)
	go hub.Run()

	srv := server.New(cfg, hub, staticFS)
	log.Printf("realtime-typing listening on %s (open http://localhost%s)", cfg.BindAddr, portSuffix(cfg.BindAddr))
	if err := srv.Start(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func portSuffix(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[i:]
		}
	}
	return ""
}
