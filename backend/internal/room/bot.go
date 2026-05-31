package room

import (
	"math/rand"
	"time"
)

// Bot は AI ではなく、一定の打鍵特性に従って進捗イベントを送るだけの goroutine。
// 人間のプレイヤーと同じ経路（room の events チャネル）に進捗を流すので、ゲーム
// ロジック（検証・中継・勝敗判定）は人間相手とまったく同じものを再利用できる。

// Difficulty は CPU の強さ。
type Difficulty string

const (
	DiffOni        Difficulty = "oni"        // 鬼（最強）
	DiffTatsujin   Difficulty = "tatsujin"   // 達人
	DiffJoukyuu    Difficulty = "joukyuu"    // 上級
	DiffFutsuu     Difficulty = "futsuu"     // ふつう
	DiffYasashii   Difficulty = "yasashii"   // やさしい
	DiffShoshinsha Difficulty = "shoshinsha" // 初心者
	DiffNyuumon    Difficulty = "nyuumon"    // 入門（最弱）
)

// botProfile はある難易度の打鍵特性。perKey を基準に、jitter でゆらぎを与え、
// pauseProb の確率で「考え込む/詰まる」一拍を入れて人間らしさを出す。
// 弱いほど perKey が大きく（遅い）、ゆらぎ・詰まりも大きい（雑になる）。
type botProfile struct {
	perKey    time.Duration // 1かなあたりの基準間隔（小さいほど速い）
	jitter    float64       // 基準に対するゆらぎ幅の割合（0.0〜1.0）
	pauseProb float64       // 各かなで停滞する確率（0.0〜1.0）
	pauseMin  time.Duration // 停滞時間の下限
	pauseMax  time.Duration // 停滞時間の上限
}

// difficultyTier は1段階ぶんの定義。強い順（先頭が最強）に並べる。
type difficultyTier struct {
	key   Difficulty
	label string
	prof  botProfile
}

// tiers は7段階の難易度表。強い順。
// 換算の目安: かな1文字 ≒ 約2打鍵なので、KPM ≒ (60000/perKey[ms]) * 2。
var tiers = []difficultyTier{
	{DiffOni, "鬼", botProfile{80 * time.Millisecond, 0.25, 0.015, 100 * time.Millisecond, 300 * time.Millisecond}},
	{DiffTatsujin, "達人", botProfile{110 * time.Millisecond, 0.30, 0.020, 120 * time.Millisecond, 350 * time.Millisecond}},
	{DiffJoukyuu, "上級", botProfile{160 * time.Millisecond, 0.35, 0.030, 150 * time.Millisecond, 450 * time.Millisecond}},
	{DiffFutsuu, "ふつう", botProfile{220 * time.Millisecond, 0.40, 0.040, 200 * time.Millisecond, 600 * time.Millisecond}},
	{DiffYasashii, "やさしい", botProfile{380 * time.Millisecond, 0.50, 0.060, 300 * time.Millisecond, 900 * time.Millisecond}},
	{DiffShoshinsha, "初心者", botProfile{520 * time.Millisecond, 0.55, 0.090, 400 * time.Millisecond, 1100 * time.Millisecond}},
	{DiffNyuumon, "入門", botProfile{700 * time.Millisecond, 0.60, 0.120, 500 * time.Millisecond, 1400 * time.Millisecond}},
}

// defaultTier は未知の難易度が来たときのフォールバック（ふつう）。
const defaultDifficulty = DiffFutsuu

func tierFor(d Difficulty) difficultyTier {
	for _, t := range tiers {
		if t.key == d {
			return t
		}
	}
	for _, t := range tiers {
		if t.key == defaultDifficulty {
			return t
		}
	}
	return tiers[0]
}

// parseDifficulty は文字列を Difficulty に変換する。未知の値は既定（ふつう）。
func parseDifficulty(s string) Difficulty {
	for _, t := range tiers {
		if t.key == Difficulty(s) {
			return t.key
		}
	}
	return defaultDifficulty
}

// botName は難易度に応じた表示名を返す（相手プレイヤーに見える名前）。
func botName(d Difficulty) string {
	return "CPU (" + tierFor(d).label + ")"
}

// profileFor は難易度ごとの打鍵特性を返す。
func profileFor(d Difficulty) botProfile {
	return tierFor(d).prof
}

// nextDelay は次の1打鍵までの待ち時間を、ゆらぎと停滞込みで決める。
func (p botProfile) nextDelay() time.Duration {
	base := float64(p.perKey)
	// ±jitter のゆらぎ（factor は 1-jitter 〜 1+jitter）。
	factor := 1 + (rand.Float64()*2-1)*p.jitter
	d := time.Duration(base * factor)
	if d <= 0 {
		d = p.perKey
	}
	// たまに考え込む。
	if p.pauseProb > 0 && rand.Float64() < p.pauseProb {
		span := int64(p.pauseMax - p.pauseMin)
		extra := p.pauseMin
		if span > 0 {
			extra += time.Duration(rand.Int63n(span + 1))
		}
		d += extra
	}
	return d
}

// runBot は1かなずつ進捗を送り、最後まで到達したら finish を送る。カウントダウンが
// 明けるまで待ち、人間が先に終わって試合が終了したら（r.done）即座に止まる。
func (r *Room) runBot(bot *Player, prof botProfile, startAt time.Time, total int) {
	// 「よーいドン」まで待つ。
	select {
	case <-time.After(time.Until(startAt)):
	case <-r.done:
		return
	}

	for i := 1; i <= total; i++ {
		select {
		case <-time.After(prof.nextDelay()):
		case <-r.done:
			return
		}
		select {
		case r.events <- event{kind: evProgress, player: bot, index: i}:
		case <-r.done:
			return
		}
	}

	select {
	case r.events <- event{kind: evFinish, player: bot}:
	case <-r.done:
	}
}
