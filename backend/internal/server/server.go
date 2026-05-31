// Package server は room.Hub の周りに HTTP/WebSocket のトランスポートを配線する。
// これは「差し替え可能なエッジ」で、ローカルでは同梱フロントを自己配信して WS 接続を
// 直接受け、クラウドでは同じコードが ALB の背後に置かれる。ゲームロジックはここには無い。
package server

import (
	"io/fs"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/reisuta/realtime-typing/backend/internal/config"
	"github.com/reisuta/realtime-typing/backend/internal/room"
)

type Server struct {
	cfg      config.Config
	hub      *room.Hub
	echo     *echo.Echo
	upgrader websocket.Upgrader
	conns    int64
}

func New(cfg config.Config, hub *room.Hub, staticFS fs.FS) *Server {
	s := &Server{cfg: cfg, hub: hub}
	s.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     s.checkOrigin,
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())

	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	e.GET("/ws", s.handleWS)
	s.registerStatic(e, staticFS)

	s.echo = e
	return s
}

func (s *Server) Start() error {
	return s.echo.Start(s.cfg.BindAddr)
}

// checkOrigin は WebSocket の Upgrade を守る。許可リストが空なら許容（permissive）で、
// ローカル / Tailscale 対戦では正しい（ページとソケットが同一オリジン）。公開時は
// ALLOWED_ORIGINS を設定して締める。
func (s *Server) checkOrigin(r *http.Request) bool {
	if len(s.cfg.AllowedOrigins) == 0 {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // ブラウザ以外のクライアント
	}
	for _, allowed := range s.cfg.AllowedOrigins {
		if allowed == origin {
			return true
		}
	}
	return false
}

func (s *Server) handleWS(c echo.Context) error {
	// 同時接続数の上限（DoS 対策）。Upgrade の前に枠を確保する。
	if int(atomic.AddInt64(&s.conns, 1)) > s.cfg.MaxConns {
		atomic.AddInt64(&s.conns, -1)
		return c.String(http.StatusServiceUnavailable, "server full")
	}
	conn, err := s.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		atomic.AddInt64(&s.conns, -1)
		return nil // Upgrade が既にエラーレスポンスを書いている
	}
	// 枠は接続が閉じるときにちょうど一度だけ解放される。
	s.hub.Add(conn, func() { atomic.AddInt64(&s.conns, -1) })
	return nil
}

// registerStatic は同梱の SPA を配信する。ビルド済みの web/dist（本物のフロント）を
// 優先し、無ければ web/placeholder にフォールバックするので、素の `go run .` でも動く。
func (s *Server) registerStatic(e *echo.Echo, staticFS fs.FS) {
	root := "placeholder"
	if _, err := fs.Stat(staticFS, "dist/index.html"); err == nil {
		root = "dist"
	}
	sub, err := fs.Sub(staticFS, root)
	if err != nil {
		return
	}
	fileServer := http.FileServer(http.FS(sub))

	e.GET("/*", func(c echo.Context) error {
		req := c.Request()
		p := strings.TrimPrefix(req.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		// SPA フォールバック。未知のパスは index.html を返し、クライアント側ルーティングを成立させる。
		if _, statErr := fs.Stat(sub, p); statErr != nil {
			req = req.Clone(req.Context())
			req.URL.Path = "/"
		}
		fileServer.ServeHTTP(c.Response(), req)
		return nil
	})
}
