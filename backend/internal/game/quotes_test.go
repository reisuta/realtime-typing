package game

import (
	"os"
	"path/filepath"
	"testing"
	"unicode"
)

// TestLoadVendoredQuotes はバイナリに同梱する実際の quotes.json を読み込み、
// ローマ字エンジンが前提とする不変条件を検証する。すなわち、全ての reading が
// 純粋なひらがな（＋長音符）で、漢字・句読点・空白を含まないこと。
func TestLoadVendoredQuotes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "quotes.json"))
	if err != nil {
		t.Fatalf("read quotes.json: %v", err)
	}
	lib, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if lib.Count() == 0 {
		t.Fatal("expected at least one quote")
	}
	for _, q := range lib.quotes {
		for _, r := range q.Reading {
			if r == 'ー' {
				continue
			}
			if !unicode.In(r, unicode.Hiragana) {
				t.Errorf("quote %q reading contains non-hiragana rune %q", q.ID, string(r))
			}
		}
	}
}

func TestLoadRejectsEmpty(t *testing.T) {
	if _, err := Load([]byte(`[]`)); err == nil {
		t.Fatal("expected error for empty dataset")
	}
	if _, err := Load([]byte(`not json`)); err == nil {
		t.Fatal("expected error for malformed json")
	}
}

func TestPickReturnsRuneCount(t *testing.T) {
	lib, err := Load([]byte(`[{"id":"x","reading":"あいう"}]`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	q, total := lib.Pick()
	if q.ID != "x" {
		t.Fatalf("unexpected quote %q", q.ID)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
}
