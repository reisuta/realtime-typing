package room

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/reisuta/realtime-typing/backend/internal/game"
	"github.com/reisuta/realtime-typing/backend/internal/protocol"
)

func TestParseDifficulty(t *testing.T) {
	cases := map[string]Difficulty{
		"oni":      DiffOni,
		"tatsujin": DiffTatsujin,
		"futsuu":   DiffFutsuu,
		"nyuumon":  DiffNyuumon,
		"":         DiffFutsuu, // 未指定は既定（ふつう）
		"unknown":  DiffFutsuu, // 未知も既定にフォールバック
	}
	for in, want := range cases {
		if got := parseDifficulty(in); got != want {
			t.Errorf("parseDifficulty(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTiersOrdering は、強い順に並んだ7段階で perKey が厳密に単調増加（=下に行くほど
// 遅い）し、各段階が固有のラベルを持つことを保証する。
func TestTiersOrdering(t *testing.T) {
	if len(tiers) != 7 {
		t.Fatalf("expected 7 tiers, got %d", len(tiers))
	}
	seen := map[string]bool{}
	for i := 1; i < len(tiers); i++ {
		if !(tiers[i-1].prof.perKey < tiers[i].prof.perKey) {
			t.Errorf("perKey not strictly increasing at %d: %v then %v",
				i, tiers[i-1].prof.perKey, tiers[i].prof.perKey)
		}
	}
	for _, tr := range tiers {
		if tr.label == "" {
			t.Errorf("tier %q has empty label", tr.key)
		}
		if seen[string(tr.key)] {
			t.Errorf("duplicate tier key %q", tr.key)
		}
		seen[string(tr.key)] = true
	}
}

func TestNextDelayPositiveAndBounded(t *testing.T) {
	// ゆらぎがあっても遅延は常に正で、停滞なし時の上限を超えない範囲に概ね収まる。
	prof := botProfile{perKey: 200 * time.Millisecond, jitter: 0.4, pauseProb: 0} // 停滞は無効化
	for i := 0; i < 1000; i++ {
		d := prof.nextDelay()
		if d <= 0 {
			t.Fatalf("delay must be positive, got %v", d)
		}
		// jitter=0.4 なので最大は perKey*1.4。余裕を見て検査。
		if d > time.Duration(float64(prof.perKey)*1.4)+time.Millisecond {
			t.Fatalf("delay %v exceeds jitter bound", d)
		}
	}
}

// TestBotWinsAgainstIdleHuman は、何もしない人間に対して Bot が試合を完走し、
// 人間側に match_end(lose) が届くことを確認する。Bot が人間と同じイベント経路で
// 動いていることの結合テスト。
func TestBotWinsAgainstIdleHuman(t *testing.T) {
	// カウントダウンを消し、Bot を高速化してテストを即決させる。
	defer func(orig time.Duration) { countdown = orig }(countdown)
	countdown = 0

	lib, err := game.Load([]byte(`[{"id":"x","reading":"あいうえお"}]`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	h := NewHub(lib)
	go h.Run()

	// 実接続を持たない擬似的な人間（送信は out チャネルへ。ここで読み出して検証する）。
	human := &Player{id: "h", name: "ひと", out: make(chan []byte, 64), quit: make(chan struct{})}
	bot := newBot("b", h, DiffOni)
	fast := botProfile{perKey: time.Millisecond, jitter: 0, pauseProb: 0}
	r := newRoomVsBot(h, human, bot, fast)
	go r.run()

	deadline := time.After(2 * time.Second)
	sawStart := false
	for {
		select {
		case raw := <-human.out:
			var env protocol.Envelope
			if json.Unmarshal(raw, &env) != nil {
				continue
			}
			switch env.Type {
			case protocol.TypeMatchStart:
				var m protocol.MatchStartMsg
				_ = json.Unmarshal(raw, &m)
				sawStart = true
				if m.Opponent != "CPU (鬼)" {
					t.Errorf("opponent name = %q, want CPU (鬼)", m.Opponent)
				}
				if m.Total != 5 {
					t.Errorf("total = %d, want 5", m.Total)
				}
			case protocol.TypeMatchEnd:
				var m protocol.MatchEndMsg
				_ = json.Unmarshal(raw, &m)
				if !sawStart {
					t.Error("match_end arrived before match_start")
				}
				if m.Result != "lose" {
					t.Errorf("result = %q, want lose", m.Result)
				}
				return // 成功
			}
		case <-deadline:
			t.Fatal("timed out waiting for bot to finish the match")
		}
	}
}
