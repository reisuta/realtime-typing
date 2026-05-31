package room

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 // バイト。メッセージは極小なので、巨大ペイロードを抑止する上限
	sendBuffer     = 16
)

// Player は1本の WebSocket 接続をラップする。ゲーム状態はすべて Hub / Room の
// goroutine 側にあり、Player は送信チャネルとライフサイクルだけを持つ。ゲーム
// データにロックはなく、協調はもっぱらチャネル経由で行う。
type Player struct {
	id      string
	name    string
	conn    *websocket.Conn
	hub     *Hub
	onClose func()

	out  chan []byte
	quit chan struct{}
	once sync.Once
}

func newPlayer(id string, conn *websocket.Conn, hub *Hub) *Player {
	return &Player{
		id:   id,
		conn: conn,
		hub:  hub,
		out:  make(chan []byte, sendBuffer),
		quit: make(chan struct{}),
	}
}

// Close は冪等。接続を破棄し、送受信両ポンプに終了を通知する。onClose
// （接続数の計上）はちょうど一度だけ実行される。
func (p *Player) Close() {
	p.once.Do(func() {
		close(p.quit)
		_ = p.conn.Close()
		if p.onClose != nil {
			p.onClose()
		}
	})
}

// trySend は呼び出し元（Hub/Room の goroutine）を決してブロックしない。追従でき
// ないほど遅い受信者は、ゲームループを止めさせる代わりに切断する。
func (p *Player) trySend(b []byte) {
	select {
	case p.out <- b:
	case <-p.quit:
	default:
		p.Close()
	}
}

func (p *Player) sendJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	p.trySend(b)
}

// readPump はソケットからフレームを読み、Hub に渡す。読み取りサイズ上限と pong
// デッドライン（アイドル / DoS 対策）を課す。エラー時はプレイヤーを登録解除し、
// 接続を閉じる。
func (p *Player) readPump() {
	defer func() {
		p.hub.unregister <- p
		p.Close()
	}()

	p.conn.SetReadLimit(maxMessageSize)
	_ = p.conn.SetReadDeadline(time.Now().Add(pongWait))
	p.conn.SetPongHandler(func(string) error {
		return p.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := p.conn.ReadMessage()
		if err != nil {
			return
		}
		select {
		case p.hub.inbound <- inbound{player: p, data: data}:
		case <-p.quit:
			return
		}
	}
}

// writePump はソケットへの全書き込みを所有し、定期的に ping を送る。
func (p *Player) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		p.Close()
	}()

	for {
		select {
		case <-p.quit:
			_ = p.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(writeWait),
			)
			return
		case msg := <-p.out:
			_ = p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := p.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
