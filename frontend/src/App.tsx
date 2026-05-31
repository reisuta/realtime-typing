import { batch, createSignal, onCleanup, onMount, Show, For, type Component } from "solid-js";
import type { Quote, ServerMsg, JoinMsg, Difficulty } from "./protocol";
import { GameSocket } from "./ws";
import { TypingSession } from "./romaji";

type Screen = "lobby" | "waiting" | "playing" | "finished";
type Result = "win" | "lose" | "draw" | "left";

// 弱い順 → 強い順。サーバーの難易度キーと一致させる。
const DIFFICULTIES: { key: Difficulty; label: string }[] = [
  { key: "nyuumon", label: "入門" },
  { key: "shoshinsha", label: "初心者" },
  { key: "yasashii", label: "やさしい" },
  { key: "futsuu", label: "ふつう" },
  { key: "joukyuu", label: "上級" },
  { key: "tatsujin", label: "達人" },
  { key: "oni", label: "鬼" },
];

const App: Component = () => {
  const [screen, setScreen] = createSignal<Screen>("lobby");
  const [name, setName] = createSignal("");
  const [connError, setConnError] = createSignal("");
  const [mode, setMode] = createSignal<"human" | "cpu">("human");
  const [difficulty, setDifficulty] = createSignal<Difficulty>("futsuu");

  const [quote, setQuote] = createSignal<Quote | null>(null);
  const [opponent, setOpponent] = createSignal("");
  const [total, setTotal] = createSignal(0);
  const [startAt, setStartAt] = createSignal(0);
  const [now, setNow] = createSignal(0);

  const [myIndex, setMyIndex] = createSignal(0);
  const [oppIndex, setOppIndex] = createSignal(0);
  const [status, setStatus] = createSignal<("done" | "typing" | "active" | "todo")[]>([]);
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
    // 1打鍵あたりの signal 更新を1回の再描画にまとめる。
    batch(() => {
      setStatus(session!.renderStatus());
      setHint(session!.romajiHint());
      setMyIndex(session!.index);
    });
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

  function joinPayload(): JoinMsg {
    return mode() === "cpu"
      ? { type: "join", name: name().trim(), mode: "cpu", difficulty: difficulty() }
      : { type: "join", name: name().trim() };
  }

  function connect() {
    setConnError("");
    socket = new GameSocket({
      onOpen: () => socket?.send(joinPayload()),
      onMessage: handleMessage,
      onClose: () => {
        if (screen() !== "finished") setConnError("接続が切れました");
      },
    });
  }

  function startHuman(e: Event) {
    e.preventDefault();
    setMode("human");
    connect();
  }

  function startCPU(d: Difficulty) {
    setMode("cpu");
    setDifficulty(d);
    connect();
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
    socket?.send(joinPayload());
    // 画面遷移は server からの match_start / waiting に任せる。
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
        <form class="panel" onSubmit={startHuman}>
          <p class="lead">名前を入れて対戦しよう。</p>
          <input
            class="name-input"
            placeholder="名前"
            maxlength="24"
            value={name()}
            onInput={(e) => setName(e.currentTarget.value)}
            autofocus
          />
          <button class="btn" type="submit">人と対戦する</button>

          <div class="divider">または</div>

          <p class="lead cpu-label">CPUと対戦（左ほど弱い）</p>
          <div class="cpu-buttons">
            <For each={DIFFICULTIES}>
              {(d) => (
                <button type="button" class="btn btn-ghost" onClick={() => startCPU(d.key)}>
                  {d.label}
                </button>
              )}
            </For>
          </div>

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
