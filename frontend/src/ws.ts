import type { ClientMsg, ServerMsg } from "./protocol";

// WebSocket の薄いラッパー。URL は常にページのオリジンと相対パス "/ws" から導く
// ので、ホストをハードコードする必要がない。開発時は Vite が :8080 へプロキシし、
// 単一バイナリのビルドでは同一オリジンがページとソケットの両方を配信する。
// プライベートIPは一切コミットしない。
export type GameSocketHandlers = {
  onOpen?: () => void;
  onMessage: (msg: ServerMsg) => void;
  onClose?: () => void;
};

export class GameSocket {
  private ws: WebSocket;

  constructor(handlers: GameSocketHandlers) {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    this.ws = new WebSocket(`${proto}://${location.host}/ws`);
    this.ws.onopen = () => handlers.onOpen?.();
    this.ws.onclose = () => handlers.onClose?.();
    this.ws.onmessage = (e) => {
      try {
        handlers.onMessage(JSON.parse(e.data) as ServerMsg);
      } catch {
        // 不正なフレームは無視する
      }
    };
  }

  send(msg: ClientMsg): void {
    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  close(): void {
    this.ws.close();
  }
}
