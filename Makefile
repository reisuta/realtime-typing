# realtime-typing — 開発用エントリポイント。
# ローカルファースト。どのターゲットもクラウドには触れない。

.PHONY: help dev backend frontend build run test lint tidy clean

help: ## このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## ---- 開発（ターミナル2枚）----
backend: ## Go WS サーバーを :8080 で起動（go run）
	cd backend && go run .

frontend: ## Vite 開発サーバーを :3000 で起動（/ws を :8080 へプロキシ）
	cd frontend && pnpm dev

## ---- 単一の自己配信バイナリ（Tailscale / LAN 対戦）----
build: ## フロントをビルドして1つの Go バイナリに同梱する
	cd frontend && pnpm install --frozen-lockfile --ignore-scripts && pnpm build
	cd backend && go build -o bin/realtime-typing .

run: build ## ビルドして単一バイナリを起動（UI + WS が :8080）
	./backend/bin/realtime-typing

## ---- 品質 ----
test: ## Go とフロントのテストを実行
	cd backend && go test ./...
	cd frontend && pnpm test

lint: ## Go の vet とフロントの型チェック
	cd backend && go vet ./...
	cd frontend && pnpm typecheck

tidy: ## Go モジュールを整理
	cd backend && go mod tidy

clean: ## ビルド成果物を削除（node_modules は残す）
	rm -rf backend/bin backend/web/dist frontend/dist
