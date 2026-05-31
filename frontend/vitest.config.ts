import { defineConfig } from "vitest/config";

// 専用の Vitest 設定。ユニットテストが Solid/vite プラグインや DOM 環境を
// 引き込まないようにする。エンジンのテストは純粋な TypeScript ロジックで Node 上で
// 走るため、テストのツールチェーン（と依存範囲）を最小に保てる。
export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
