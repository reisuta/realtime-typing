import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

// ビルド成果物は backend/web/dist に出力し、Go バイナリが embed して自己配信できる
// ようにする。フロントの配信にはローカル等価（バイナリ自身）があり、クラウドでは
// S3+CloudFront に差し替え可能。
// `base: "./"` で asset の URL を相対にし、バイナリのルート配信でも CDN のサブパス
// でも解決できるようにする。
export default defineConfig({
  plugins: [solid()],
  base: "./",
  build: {
    outDir: "../backend/web/dist",
    emptyOutDir: true,
  },
  server: {
    port: 3000,
    // 開発時は Vite サーバーが WebSocket を :8080 の Go バックエンドへプロキシする。
    // これにより、どの環境でもクライアントは相対パス "/ws" に接続できる。
    proxy: {
      "/ws": { target: "ws://localhost:8080", ws: true },
      "/healthz": "http://localhost:8080",
    },
  },
});
