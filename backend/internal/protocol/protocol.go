// Package protocol は Go サーバーと Solid.js クライアント間の WebSocket メッセージ
// 仕様を定義する。これが正（source of truth）であり、frontend/src/protocol.ts が
// 同じ形を手作業でミラーする（同期を保つこと。将来は Go の構造体から TS を生成したい）。
package protocol

// クライアント -> サーバー のメッセージ種別。
const (
	TypeJoin     = "join"     // {type, name}
	TypeProgress = "progress" // {type, index}
	TypeFinish   = "finish"   // {type}
)

// サーバー -> クライアント のメッセージ種別。
const (
	TypeWaiting          = "waiting"           // マッチング待ち（対戦相手待ち）
	TypeMatchStart       = "match_start"       // {type, quote, opponent, startAt, total}
	TypeOpponentProgress = "opponent_progress" // {type, index}
	TypeMatchEnd         = "match_end"         // {type, result}
	TypeOpponentLeft     = "opponent_left"     // 相手が切断した
	TypeError            = "error"             // {type, message}
)

// Envelope は本格デコード前にメッセージの type だけを覗くために使う。
type Envelope struct {
	Type string `json:"type"`
}

// ---- クライアント -> サーバー ----

type JoinMsg struct {
	Type string `json:"type"`
	Name string `json:"name"`
	// Mode は "human"（既定・対人マッチング）か "cpu"（即CPU対戦）。
	Mode string `json:"mode,omitempty"`
	// Difficulty は Mode=="cpu" のときの強さ "easy" | "normal" | "hard"。
	Difficulty string `json:"difficulty,omitempty"`
}

// 対戦モード。
const (
	ModeHuman = "human"
	ModeCPU   = "cpu"
)

// ProgressMsg は名文の読みを何文字（rune）打ち終えたかを報告する。サーバーは
// これを「申告」として扱い、検証する（チート対策）。
type ProgressMsg struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

type FinishMsg struct {
	Type string `json:"type"`
}

// ---- サーバー -> クライアント ----

// Quote は1つの名文。Reading はプレイヤーが打つ純粋なひらがな文字列で、
// Text は表示用（漢字・句読点を含みうる）。
type Quote struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Reading string `json:"reading"`
}

type WaitingMsg struct {
	Type string `json:"type"`
}

type MatchStartMsg struct {
	Type     string `json:"type"`
	Quote    Quote  `json:"quote"`
	Opponent string `json:"opponent"`
	StartAt  int64  `json:"startAt"` // エポックミリ秒。クライアントはこの時刻までカウントダウンする
	Total    int    `json:"total"`   // Reading の打鍵対象 rune 総数
}

type OpponentProgressMsg struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// MatchEndMsg の result は受信者から見た "win" | "lose" | "draw"。
type MatchEndMsg struct {
	Type   string `json:"type"`
	Result string `json:"result"`
}

type OpponentLeftMsg struct {
	Type string `json:"type"`
}

type ErrorMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
