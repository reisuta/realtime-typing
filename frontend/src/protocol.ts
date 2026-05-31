// WebSocket メッセージの仕様。
//
// 正（source of truth）は backend/internal/protocol/protocol.go。当面はこの形を
// 手作業で同期する（将来は Go の構造体から生成したい）。

// ---- クライアント -> サーバー ----
// CPU の強さ（7段階）。サーバーの内部キーと一致させること。
export type Difficulty =
  | "oni"
  | "tatsujin"
  | "joukyuu"
  | "futsuu"
  | "yasashii"
  | "shoshinsha"
  | "nyuumon";
export type JoinMsg = {
  type: "join";
  name: string;
  mode?: "human" | "cpu";
  difficulty?: Difficulty;
};
export type ProgressMsg = { type: "progress"; index: number };
export type FinishMsg = { type: "finish" };
export type ClientMsg = JoinMsg | ProgressMsg | FinishMsg;

// ---- 共通 ----
export type Quote = {
  id: string;
  author: string;
  title: string;
  text: string;
  reading: string;
};

// ---- サーバー -> クライアント ----
export type WaitingMsg = { type: "waiting" };
export type MatchStartMsg = {
  type: "match_start";
  quote: Quote;
  opponent: string;
  startAt: number; // epoch ms
  total: number;
};
export type OpponentProgressMsg = { type: "opponent_progress"; index: number };
export type MatchEndMsg = { type: "match_end"; result: "win" | "lose" | "draw" };
export type OpponentLeftMsg = { type: "opponent_left" };
export type ErrorMsg = { type: "error"; message: string };

export type ServerMsg =
  | WaitingMsg
  | MatchStartMsg
  | OpponentProgressMsg
  | MatchEndMsg
  | OpponentLeftMsg
  | ErrorMsg;
