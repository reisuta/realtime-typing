import { createSignal, onCleanup, onMount, Show, For, type Component } from "solid-js";
import type { Quote, ServerMsg } from "./protocol";
import { GameSocket } from "./ws";
import { TypingSession } from "./romaji";

type Screen = "lobby" | "waiting" | "playing" | "finished";
type Result = "win" | "lose" | "draw" | "left";

const App: Component = () => {
  const [screen, setScreen] = createSignal<Screen>("lobby");
  const [name, setName] = createSignal("");
  const [connError, setConnError] = createSignal("");

  const [quote, setQuote] = createSignal<Quote | null>(null);
  const [opponent, setOpponent] = createSignal("");
  const [total, setTotal] = createSignal(0);
  const [startAt, setStartAt] = createSignal(0);
  const [now, setNow] = createSignal(0);

  const [myIndex, setMyIndex] = createSignal(0);
  const [oppIndex, setOppIndex] = createSignal(0);
  const [status, setStatus] = createSignal<("done" | "active" | "todo")[]>([]);
  const [hint, setHint] = createSignal("");

  const [result, setResult] = createSignal<Result | null>(null);
  const [finalStats, setFinalStats] = createSignal<{ wpm: number; acc: number } | null>(null);

  let socket: GameSocket | null = null;
  let session: TypingSession | null = null;
  let clock: ReturnType<typeof setInterval> | undefined;

  const started = () => now() >= startAt();
  const countdownLeft = () => Math.max(0, Math.ceil((startAt() - now()) / 1000));

  function refreshRender() {
    if (!session) return;
    setStatus(session.renderStatus());
    setHint(session.romajiHint());
    setMyIndex(session.index);
  }

  function computeStats(): { wpm: number; acc: number } {
    if (!session) return { wpm: 0, acc: 0 };
    const { hits, misses } = session.stats;
    const minutes = Math.max((Date.now() - startAt()) / 60000, 1 / 60000);
    const wpm = Math.round(hits / 5 / minutes);
    const acc = hits + misses === 0 ? 100 : Math.round((hits / (hits + misses)) * 100);
    return { wpm, acc };
  }

  function handleMessage(msg: ServerMsg) {
    switch (msg.type) {
      case "waiting":
        setScreen("waiting");
        break;
      case "match_start":
        setQuote(msg.quote);
        setOpponent(msg.opponent);
        setTotal(msg.total);
        setStartAt(msg.startAt);
        setNow(Date.now());
        setMyIndex(0);
        setOppIndex(0);
        setResult(null);
        setFinalStats(null);
        session = new TypingSession(msg.quote.reading);
        refreshRender();
        setScreen("playing");
        clock = setInterval(() => setNow(Date.now()), 100);
        break;
      case "opponent_progress":
        setOppIndex(msg.index);
        break;
      case "match_end":
        stopClock();
        setFinalStats(computeStats());
        setResult(msg.result);
        setScreen("finished");
        break;
      case "opponent_left":
        stopClock();
        setResult("left");
        setScreen("finished");
        break;
      case "error":
        setConnError(msg.message);
        break;
    }
  }

  function stopClock() {
    if (clock) {
      clearInterval(clock);
      clock = undefined;
    }
  }

  function join(e: Event) {
    e.preventDefault();
    setConnError("");
    socket = new GameSocket({
      onOpen: () => socket?.send({ type: "join", name: name().trim() }),
      onMessage: handleMessage,
      onClose: () => {
        if (screen() !== "finished") setConnError("接続が切れました");
      },
    });
  }

  function onKeyDown(e: KeyboardEvent) {
    if (screen() !== "playing" || !started() || !session || session.done) return;
    if (e.key.length !== 1 || e.ctrlKey || e.metaKey || e.altKey) return;

    const res = session.input(e.key);
    if (res === "ignore") return;
    e.preventDefault();
    if (res === "ok") {
      refreshRender();
      socket?.send({ type: "progress", index: session.index });
      if (session.done) socket?.send({ type: "finish" });
    } else {
      // ミス時も再描画する（UI でフィードバックを点滅させたい場合に備えて）
      refreshRender();
    }
  }

  function playAgain() {
    setMyIndex(0);
    setOppIndex(0);
    setResult(null);
    socket?.send({ type: "join", name: name().trim() });
    setScreen("waiting");
  }

  onMount(() => window.addEventListener("keydown", onKeyDown));
  onCleanup(() => {
    window.removeEventListener("keydown", onKeyDown);
    stopClock();
    socket?.close();
  });

  const pct = (i: number) => (total() === 0 ? 0 : (i / total()) * 100);

  return (
    <div class="app">
      <h1 class="title">文豪タイピング対戦</h1>

      <Show when={screen() === "lobby"}>
        <form class="panel" onSubmit={join}>
          <p class="lead">名前を入れて対戦相手を待ちます。</p>
          <input
            class="name-input"
            placeholder="名前"
            maxlength="24"
            value={name()}
            onInput={(e) => setName(e.currentTarget.value)}
            autofocus
          />
          <button class="btn" type="submit">対戦する</button>
          <Show when={connError()}>
            <p class="error">{connError()}</p>
          </Show>
        </form>
      </Show>

      <Show when={screen() === "waiting"}>
        <div class="panel">
          <div class="spinner" />
          <p class="lead">対戦相手を待っています…</p>
        </div>
      </Show>

      <Show when={screen() === "playing" && quote()}>
        <div class="game">
          <div class="meta">
            <span>{quote()!.author}『{quote()!.title}』</span>
          </div>

          <div class="bars">
            <div class="bar-row">
              <span class="bar-label">あなた</span>
              <div class="bar"><div class="fill me" style={{ width: `${pct(myIndex())}%` }} /></div>
            </div>
            <div class="bar-row">
              <span class="bar-label">{opponent() || "相手"}</span>
              <div class="bar"><div class="fill opp" style={{ width: `${pct(oppIndex())}%` }} /></div>
            </div>
          </div>

          <div class="quote-text">{quote()!.text}</div>

          <div class="reading">
            <For each={[...quote()!.reading]}>
              {(ch, i) => <span class={`k ${status()[i()] ?? "todo"}`}>{ch}</span>}
            </For>
          </div>

          <div class="hint">{hint()}</div>

          <Show when={!started()}>
            <div class="countdown">
              <Show when={countdownLeft() > 0} fallback={<span>GO!</span>}>
                {countdownLeft()}
              </Show>
            </div>
          </Show>
        </div>
      </Show>

      <Show when={screen() === "finished"}>
        <div class="panel">
          <h2 class={`result ${result()}`}>
            {result() === "win" && "勝ち！"}
            {result() === "lose" && "負け…"}
            {result() === "draw" && "引き分け"}
            {result() === "left" && "相手が退出しました"}
          </h2>
          <Show when={finalStats()}>
            <p class="stats">
              {finalStats()!.wpm} WPM ・ 正確率 {finalStats()!.acc}%
            </p>
          </Show>
          <button class="btn" onClick={playAgain}>もう一度</button>
        </div>
      </Show>
    </div>
  );
};

export default App;
