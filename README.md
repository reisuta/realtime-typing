# realtime-typing

文豪の名文を題材にした、リアルタイム対戦タイピングゲーム。

二人がオンラインで同じ名文（ローマ字打鍵）を「よーいドン」で打ち、相手のカーソル進捗がリアルタイムに見え、先に打ち切った方が勝ち。
現在は **ローカルで2人・費用0・クラウド非依存** で動きます。

## 構成（monorepo）

| ディレクトリ | 役割 | スタック |
|---|---|---|
| `frontend/` | 対戦UI（静的SPA） | Solid.js（TypeScript） |
| `backend/` | WebSocketゲームサーバー（メモリ内ルーム） | Go（Echo + gorilla/websocket） |
| `infra/` | クラウド公開用の Terraform（今後追加） | Terraform |

安定したコア（Goのゲームロジック）と、差し替え可能なエッジ（接続・配信・永続化）を分離する設計。Goバイナリは `embed.FS` でフロントを自己配信し、対戦ルームは goroutine + channel でメモリ管理する。

## 必要なもの

- Go 1.22+
- Node.js 20+ / pnpm 11+

## 遊び方（ローカル単一バイナリ）

フロントをビルドしてGoバイナリに同梱し、1プロセスでUIもWebSocketも配信します。

```sh
make run        # frontend をビルド → embed → :8080 で起動
# ブラウザで http://localhost:8080 を2タブ開く（= 2人）と対戦が始まる
```

### 別PCの相手と遊ぶ（Tailscale・費用0）

クラウド・ドメイン・ポート開放・NAT越えは一切不要です。

1. ホストPCで [Tailscale](https://tailscale.com/) を入れる（個人利用は無料）。
2. `make run` で起動（`BIND_ADDR=0.0.0.0` がデフォルトなので Tailscale 経由で届く）。
3. 相手はブラウザで `http://<ホストのTailscale IP>:8080`（例 `http://100.x.x.x:8080`）を開く。

> 接続先IPはコミットしない設計です。コードに私的情報は含まれません。

## 開発（ホットリロード）

2つのターミナルで：

```sh
make backend    # Go WSサーバー :8080（go run）
make frontend   # Vite devサーバー :3000（/ws を :8080 へプロキシ）
# http://localhost:3000 を開く
```

## 品質

```sh
make test       # Go テスト + フロント（ローマ字エンジン）テスト
make lint       # go vet + tsc 型チェック
```

- **サーバー権威・チート検証**: 進捗はサーバーが単調増加・上限を検証し、`finish` は満了到達時のみ受理（クライアント申告のWPMは信用しない）。
- **DoS面の制限**: WSメッセージサイズ上限・pongタイムアウト・同時接続数上限。
- **供給網対策**: 依存のライフサイクルスクリプトは無効（`ignore-scripts`）、lockfile固定（`--frozen-lockfile`）、CIで `pnpm audit`。

## コスト方針

ローカルのみで遊ぶ限り費用は0円。クラウド公開時もアイドル≒0円を目標とし、`terraform destroy` で0円、デモ時に `terraform apply` で復活させる運用を想定。ローカル完結は恒久的に維持する（クラウドはデプロイ先であって依存先ではない）。

> 今後の移行方針・ロードマップは Issue で管理します。
