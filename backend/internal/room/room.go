package room

import (
	"time"

	"github.com/reisuta/realtime-typing/backend/internal/protocol"
)

type eventKind int

const (
	evProgress eventKind = iota
	evFinish
	evLeave
)

type event struct {
	kind   eventKind
	player *Player
	index  int
}

// countdown は打鍵開始前の「よーいドン」の助走。テストで 0 に差し替えられるよう var。
var countdown = 3 * time.Second

const matchTimeout = 10 * time.Minute // 放置対策。これを超える試合は存在させない

// Room は1対1の1試合。自身の goroutine (run) で動き、それが下記の試合状態の
// 唯一の所有者となる（ここでもロックなし）。進捗の申告はここで検証され、
// サーバーを権威とする（チート対策）。
type Room struct {
	hub     *Hub
	players [2]*Player

	quote protocol.Quote
	total int

	progress [2]int
	finished bool

	// CPU対戦のときだけ設定される。bot は players の一方でもある。
	bot     *Player
	botProf botProfile

	events chan event
	done   chan struct{}
}

func newRoom(h *Hub, a, b *Player) *Room {
	q, total := h.lib.Pick()
	return &Room{
		hub:     h,
		players: [2]*Player{a, b},
		quote:   q,
		total:   total,
		events:  make(chan event, 64),
		done:    make(chan struct{}),
	}
}

// newRoomVsBot は人間 human と CPU bot の1試合を作る。human が players[0]。
func newRoomVsBot(h *Hub, human, bot *Player, prof botProfile) *Room {
	r := newRoom(h, human, bot)
	r.bot = bot
	r.botProf = prof
	return r
}

func (r *Room) other(p *Player) *Player {
	if r.players[0] == p {
		return r.players[1]
	}
	return r.players[0]
}

func (r *Room) idx(p *Player) int {
	if r.players[0] == p {
		return 0
	}
	return 1
}

func (r *Room) run() {
	// 先に done を閉じて Hub からの送信途中をアンブロックし、その後で Hub に
	// このルームのプレイヤー対応表を回収させる。
	defer func() {
		close(r.done)
		r.hub.roomDone <- r
	}()

	startAt := time.Now().Add(countdown)
	for i, p := range r.players {
		p.sendJSON(protocol.MatchStartMsg{
			Type:     protocol.TypeMatchStart,
			Quote:    r.quote,
			Opponent: r.players[1-i].name,
			StartAt:  startAt.UnixMilli(),
			Total:    r.total,
		})
	}

	// CPU対戦なら、カウントダウン明けから Bot が自走する。
	if r.bot != nil {
		go r.runBot(r.bot, r.botProf, startAt, r.total)
	}

	timeout := time.NewTimer(matchTimeout)
	defer timeout.Stop()

	for {
		select {
		case ev := <-r.events:
			if r.handleEvent(ev) {
				return
			}
		case <-timeout.C:
			return
		}
	}
}

// handleEvent は1つのイベントを適用し、試合が終了したかどうかを返す。
func (r *Room) handleEvent(ev event) bool {
	switch ev.kind {
	case evLeave:
		if !r.finished {
			r.other(ev.player).sendJSON(protocol.OpponentLeftMsg{Type: protocol.TypeOpponentLeft})
		}
		return true

	case evProgress:
		i := r.idx(ev.player)
		// サーバー側検証。クライアントは先飛ばしや名文超過ができない。
		// 進捗は厳密に単調増加で、実際の total を上限とする。
		if !validProgress(r.progress[i], ev.index, r.total) {
			return false // 不正な申告は無視する
		}
		r.progress[i] = ev.index
		r.other(ev.player).sendJSON(protocol.OpponentProgressMsg{
			Type:  protocol.TypeOpponentProgress,
			Index: ev.index,
		})
		if ev.index == r.total && !r.finished {
			r.finish(ev.player)
			return true
		}
		return false

	case evFinish:
		// finish の申告は、検証済みの進捗が実際に末尾へ到達している場合のみ受理する。
		// 全進捗を送らずに勝つことはできない。
		if r.progress[r.idx(ev.player)] != r.total {
			return false
		}
		if !r.finished {
			r.finish(ev.player)
			return true
		}
		return false
	}
	return false
}

func (r *Room) finish(winner *Player) {
	r.finished = true
	winner.sendJSON(protocol.MatchEndMsg{Type: protocol.TypeMatchEnd, Result: "win"})
	r.other(winner).sendJSON(protocol.MatchEndMsg{Type: protocol.TypeMatchEnd, Result: "lose"})
}

// validProgress はチート対策のルール。テストしやすいよう切り出している。
func validProgress(prev, idx, total int) bool {
	return idx > prev && idx <= total
}
