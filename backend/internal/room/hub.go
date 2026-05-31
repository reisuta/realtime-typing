// Package room は goroutine とチャネルでマッチングとリアルタイムのゲームループを
// 実装する。これが本プロジェクトの主役である Go の並行処理ショーケース。Hub は
// 単一の goroutine で全てのマッチング状態を所有するため、状態にロックは不要で、
// あらゆる変更はチャネル経由で届く。
//
// これは「安定したコア」であり、HTTP・AWS・永続化を一切知らない。将来メモリ実装を
// Redis に差し替える際も、同じチャネルの背後に実装を「追加」するだけで、書き直しではない。
package room

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/gorilla/websocket"

	"github.com/reisuta/realtime-typing/backend/internal/game"
	"github.com/reisuta/realtime-typing/backend/internal/protocol"
)

const maxNameLen = 24

type inbound struct {
	player *Player
	data   []byte
}

// Hub は全てのマッチングを単一の Run goroutine に直列化する。
type Hub struct {
	lib *game.Library

	register   chan *Player
	unregister chan *Player
	inbound    chan inbound
	roomDone   chan *Room

	// 以下は Run goroutine だけが触れる。ロックは不要。
	waiting *Player           // 対戦相手を待って並んでいるプレイヤー
	rooms   map[*Player]*Room // 値が nil = 接続済みだがまだロビーにいる

	idSeq uint64
}

func NewHub(lib *game.Library) *Hub {
	return &Hub{
		lib:        lib,
		register:   make(chan *Player),
		unregister: make(chan *Player),
		inbound:    make(chan inbound, 256),
		roomDone:   make(chan *Room),
		rooms:      make(map[*Player]*Room),
	}
}

func (h *Hub) nextID() string {
	return strconv.FormatUint(atomic.AddUint64(&h.idSeq, 1), 10)
}

// Add は Upgrade 直後の接続を登録し、その送受信ポンプを起動する。onClose は接続が
// 破棄されるときに一度だけ呼ばれる（接続数の計上に使う）。
func (h *Hub) Add(conn *websocket.Conn, onClose func()) {
	p := newPlayer(h.nextID(), conn, h)
	p.onClose = onClose
	h.register <- p
	go p.writePump()
	go p.readPump()
}

// Run はマッチング状態の唯一の所有者。決して return しない。
func (h *Hub) Run() {
	for {
		select {
		case p := <-h.register:
			h.rooms[p] = nil // ロビーにいるがまだルームなし

		case p := <-h.unregister:
			h.handleLeave(p)

		case in := <-h.inbound:
			h.handleInbound(in)

		case r := <-h.roomDone:
			// 試合が終わったプレイヤーをロビーへ戻す（接続は維持し、再戦を可能にする）。
			// 切断済みのプレイヤーは handleLeave で既に map から消えているため触れない。
			// Bot は最初から map に居ない。
			for _, p := range r.players {
				if h.rooms[p] == r {
					h.rooms[p] = nil
				}
			}
		}
	}
}

func (h *Hub) handleLeave(p *Player) {
	if h.waiting == p {
		h.waiting = nil
	}
	r, known := h.rooms[p]
	if !known {
		return
	}
	delete(h.rooms, p)
	if r != nil {
		// ルームに相手への通知と終了処理をさせる。done は既に終了済みのルームへ
		// 送ろうとして詰まるのを防ぐガード。
		select {
		case r.events <- event{kind: evLeave, player: p}:
		case <-r.done:
		}
	}
}

func (h *Hub) handleInbound(in inbound) {
	p := in.player
	r, known := h.rooms[p]
	if !known {
		return // プレイヤーは既に退出済み
	}

	if r != nil {
		// 対戦中のプレイヤー。そのルームの goroutine へ振り分ける。
		if ev, ok := parseInMatch(p, in.data); ok {
			select {
			case r.events <- ev:
			case <-r.done:
			}
		}
		return
	}

	// ロビーにいるプレイヤー。受け付けるのは "join" のみ。
	var env protocol.Envelope
	if json.Unmarshal(in.data, &env) != nil || env.Type != protocol.TypeJoin {
		return
	}
	var jm protocol.JoinMsg
	_ = json.Unmarshal(in.data, &jm)
	p.name = sanitizeName(jm.Name)
	if jm.Mode == protocol.ModeCPU {
		h.startCPUMatch(p, parseDifficulty(jm.Difficulty))
		return
	}
	h.matchmake(p)
}

// startCPUMatch は待機キューを介さず、その場で人間 vs CPU の試合を始める。
func (h *Hub) startCPUMatch(p *Player, d Difficulty) {
	bot := newBot(h.nextID(), h, d)
	r := newRoomVsBot(h, p, bot, profileFor(d))
	h.rooms[p] = r // Bot は接続管理対象外なので map に入れない
	go r.run()
}

func (h *Hub) matchmake(p *Player) {
	switch {
	case h.waiting == nil:
		h.waiting = p
		p.sendJSON(protocol.WaitingMsg{Type: protocol.TypeWaiting})
	case h.waiting == p:
		// 待機中の同一プレイヤーからの重複 join。無視する。
	default:
		opp := h.waiting
		h.waiting = nil
		r := newRoom(h, opp, p)
		h.rooms[opp] = r
		h.rooms[p] = r
		go r.run()
	}
}

// parseInMatch は対戦中の生メッセージをルームイベントに変換する。未知・不正な
// メッセージは破棄する。
func parseInMatch(p *Player, data []byte) (event, bool) {
	var env protocol.Envelope
	if json.Unmarshal(data, &env) != nil {
		return event{}, false
	}
	switch env.Type {
	case protocol.TypeProgress:
		var m protocol.ProgressMsg
		if json.Unmarshal(data, &m) != nil {
			return event{}, false
		}
		return event{kind: evProgress, player: p, index: m.Index}, true
	case protocol.TypeFinish:
		return event{kind: evFinish, player: p}, true
	}
	return event{}, false
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	// 相手の端末/UI を壊しうる制御文字を除去する。
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	if name == "" {
		return "名無し"
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		runes := []rune(name)
		name = string(runes[:maxNameLen])
	}
	return name
}
