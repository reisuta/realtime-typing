// Package config は実行時の設定を環境変数から読み込む。これにより同一バイナリが
// ローカルでもクラウドでも書き換えなしで動く。値はハードコードせず、プライベートIPや
// オリジンをコミットする必要がない。
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// BindAddr は HTTP/WS サーバーが待ち受ける host:port。
	BindAddr string
	// AllowedOrigins は WebSocket の Origin 許可リスト。空なら許容（permissive）で、
	// ローカル / Tailscale 対戦ではこれが望ましい。公開時は明示的に指定する。
	AllowedOrigins []string
	// MaxConns は WebSocket 同時接続数の上限（DoS 対策）。
	MaxConns int
}

// Load は任意の .env ファイル（リポジトリ直下または backend/）を読み、その後に
// プロセスの環境変数を読む。実際の環境変数は常に .env より優先される。
func Load() Config {
	loadDotEnv(".env")
	loadDotEnv("../.env")

	host := getenv("BIND_ADDR", "0.0.0.0")
	port := getenv("PORT", "8080")

	var origins []string
	if raw := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS")); raw != "" {
		for _, o := range strings.Split(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
	}

	return Config{
		BindAddr:       host + ":" + port,
		AllowedOrigins: origins,
		MaxConns:       getenvInt("MAX_CONNS", 200),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// loadDotEnv は依存ライブラリなしの KEY=VALUE 読み取り。ファイルが無ければ無視する。
// 既に環境変数に存在するキーは上書きしない。
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") { // 空行・コメント行は飛ばす
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key != "" {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, val)
			}
		}
	}
}
