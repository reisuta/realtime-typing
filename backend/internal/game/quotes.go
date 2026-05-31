// Package game は名文データセットと、試合に依存しないゲームデータを持つ。
package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"unicode/utf8"

	"github.com/reisuta/realtime-typing/backend/internal/protocol"
)

// Library は起動時に一度だけ読み込む、不変・メモリ上の名文集合。
type Library struct {
	quotes []protocol.Quote
}

// Load は同梱の quotes.json をパースする。壊れたデータや空データでは即座に失敗し、
// 名文ゼロの状態で起動してしまうのを防ぐ。
func Load(data []byte) (*Library, error) {
	var qs []protocol.Quote
	if err := json.Unmarshal(data, &qs); err != nil {
		return nil, fmt.Errorf("parse quotes: %w", err)
	}
	if len(qs) == 0 {
		return nil, fmt.Errorf("quotes dataset is empty")
	}
	for i, q := range qs {
		if q.Reading == "" {
			return nil, fmt.Errorf("quote %d (%q) has an empty reading", i, q.ID)
		}
	}
	return &Library{quotes: qs}, nil
}

// Count は読み込んだ名文の数を返す。
func (l *Library) Count() int { return len(l.quotes) }

// Pick はランダムな名文と、その打鍵対象の長さ（Reading の rune 数）を返す。
// この長さが、勝つために到達すべき正規の total になる。
func (l *Library) Pick() (protocol.Quote, int) {
	q := l.quotes[rand.Intn(len(l.quotes))]
	return q, utf8.RuneCountInString(q.Reading)
}
