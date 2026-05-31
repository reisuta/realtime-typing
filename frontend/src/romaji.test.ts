import { describe, it, expect } from "vitest";
import { TypingSession } from "./romaji";

// 文字列全体をセッションに打ち込み、全文字が受理され("ok")、ちょうど完了で
// 終わったら true を返す。
function typeAll(reading: string, romaji: string): boolean {
  const s = new TypingSession(reading);
  for (const ch of romaji) {
    if (s.input(ch) !== "ok") return false;
  }
  return s.done && s.index === s.total;
}

describe("TypingSession", () => {
  it("accepts basic hiragana", () => {
    expect(typeAll("あいうえお", "aiueo")).toBe(true);
    expect(typeAll("ねこ", "neko")).toBe(true);
  });

  it("accepts Hepburn and kunrei variants", () => {
    expect(typeAll("し", "shi")).toBe(true);
    expect(typeAll("し", "si")).toBe(true);
    expect(typeAll("つ", "tsu")).toBe(true);
    expect(typeAll("つ", "tu")).toBe(true);
    expect(typeAll("じ", "ji")).toBe(true);
    expect(typeAll("じ", "zi")).toBe(true);
    expect(typeAll("ふ", "fu")).toBe(true);
    expect(typeAll("ふ", "hu")).toBe(true);
  });

  it("accepts youon", () => {
    expect(typeAll("きゃ", "kya")).toBe(true);
    expect(typeAll("しゃ", "sha")).toBe(true);
    expect(typeAll("しゃ", "sya")).toBe(true);
    expect(typeAll("ちょ", "cho")).toBe(true);
    expect(typeAll("ちょ", "tyo")).toBe(true);
    expect(typeAll("じゃ", "ja")).toBe(true);
  });

  it("accepts sokuon", () => {
    expect(typeAll("いった", "itta")).toBe(true);
    expect(typeAll("きっぷ", "kippu")).toBe(true);
    expect(typeAll("いっしゃ", "issha")).toBe(true);
    expect(typeAll("まっち", "macchi")).toBe(true);
    expect(typeAll("まっち", "matchi")).toBe(true);
  });

  it("handles ん in all the usual ways", () => {
    // 子音の前: 単独 n でも nn でもよい
    expect(typeAll("こんど", "kondo")).toBe(true);
    expect(typeAll("こんど", "konndo")).toBe(true);
    // 母音の前: nn / n' が必須（単独 n は曖昧になる）
    expect(typeAll("げんあん", "gennann")).toBe(true);
    expect(typeAll("げんあん", "gen'ann")).toBe(true);
    // 末尾の ん: 単独 n では足りない
    expect(typeAll("ほん", "hon")).toBe(false);
    expect(typeAll("ほん", "honn")).toBe(true);
  });

  it("rejects wrong characters as a miss without advancing", () => {
    const s = new TypingSession("ねこ");
    expect(s.input("n")).toBe("ok");
    expect(s.input("x")).toBe("miss");
    expect(s.index).toBe(0); // ね はまだ未完成
    expect(s.input("e")).toBe("ok");
    expect(s.index).toBe(1); // ね 完成
  });

  it("tracks progress index in reading runes and reports done", () => {
    const s = new TypingSession("ねこである");
    expect(s.total).toBe(5);
    for (const ch of "nekodearu") s.input(ch);
    expect(s.done).toBe(true);
    expect(s.index).toBe(5);
  });

  it("types a full quote reading with youon and sokuon", () => {
    const reading = "おしゃかさまはごくらくのはすいけのふちをひとりであるいていらっしゃいました";
    const romaji =
      "oshakasamahagokurakunohasuikenofuchiwohitoridearuiteirasshaimashita";
    expect(typeAll(reading, romaji)).toBe(true);
  });
});
