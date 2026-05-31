// ローマ字打鍵エンジン。
//
// 純粋なひらがなの読みを打鍵「トークン」の列に変換し、各トークンに許容される
// ローマ字綴りをすべて持たせて、打鍵を照合する。文豪の文章で実際に現れる以下の
// ケースに対応する。
//   - ヘボン式・訓令式の両方（し: shi/si, つ: tsu/tu, じ: ji/zi …）
//   - 拗音（きゃ → kya, しゃ → sha/sya, ちょ → cho/tyo/cyo …）
//   - 促音（っ → 子音重ね、および xtu/ltu 形。っち → cchi/tchi）
//   - ん（nn / n' / xn、または次の音が子音のときは単独の "n"）
//
// これは100%ローカルで動く。自分の打鍵はネットワーク往復を待たずに即描画され、
// 結果の進捗 index だけをサーバーへ送る。

type Token = {
  kanaLen: number; // このトークンが消費する読みの文字数（コードポイント）
  startRune: number; // 読みの中で最初に消費する rune の位置
  cands: string[]; // 許容されるローマ字綴り
  isN: boolean; // ん の特別扱い
  singleN: boolean; // ここで ん を単独の "n" で確定してよいか
};

const VOWELS = "aeiou";
const isConsonant = (c: string) => c.length === 1 && !VOWELS.includes(c);

// 単一かなの基本表（最も一般的な綴りを先頭に。先頭要素がヒント表示に使われる）。
const BASE: Record<string, string[]> = {
  あ: ["a"], い: ["i"], う: ["u"], え: ["e"], お: ["o"],
  か: ["ka"], き: ["ki"], く: ["ku"], け: ["ke"], こ: ["ko"],
  が: ["ga"], ぎ: ["gi"], ぐ: ["gu"], げ: ["ge"], ご: ["go"],
  さ: ["sa"], し: ["shi", "si"], す: ["su"], せ: ["se"], そ: ["so"],
  ざ: ["za"], じ: ["ji", "zi"], ず: ["zu"], ぜ: ["ze"], ぞ: ["zo"],
  た: ["ta"], ち: ["chi", "ti"], つ: ["tsu", "tu"], て: ["te"], と: ["to"],
  だ: ["da"], ぢ: ["di", "ji"], づ: ["du", "zu"], で: ["de"], ど: ["do"],
  な: ["na"], に: ["ni"], ぬ: ["nu"], ね: ["ne"], の: ["no"],
  は: ["ha"], ひ: ["hi"], ふ: ["fu", "hu"], へ: ["he"], ほ: ["ho"],
  ば: ["ba"], び: ["bi"], ぶ: ["bu"], べ: ["be"], ぼ: ["bo"],
  ぱ: ["pa"], ぴ: ["pi"], ぷ: ["pu"], ぺ: ["pe"], ぽ: ["po"],
  ま: ["ma"], み: ["mi"], む: ["mu"], め: ["me"], も: ["mo"],
  や: ["ya"], ゆ: ["yu"], よ: ["yo"],
  ら: ["ra"], り: ["ri"], る: ["ru"], れ: ["re"], ろ: ["ro"],
  わ: ["wa"], を: ["wo"],
  ー: ["-"],
  // 単独で打つ小書きかな（今の名文ではまれだが、対応しておいても害はない）
  ぁ: ["xa", "la"], ぃ: ["xi", "li"], ぅ: ["xu", "lu"], ぇ: ["xe", "le"], ぉ: ["xo", "lo"],
  っ: ["xtu", "ltu", "xtsu", "ltsu"],
};

// 拗音用の子音プレフィックス（「い段」のかな + 小書きの や/ゆ/よ）。
const YOUON: Record<string, string[]> = {
  き: ["ky"], ぎ: ["gy"], し: ["sh", "sy"], じ: ["j", "zy", "jy"],
  ち: ["ch", "ty", "cy"], ぢ: ["dy"], に: ["ny"], ひ: ["hy"],
  び: ["by"], ぴ: ["py"], み: ["my"], り: ["ry"],
};
const SMALL_VOWEL: Record<string, string> = { ゃ: "a", ゅ: "u", ょ: "o" };

// 促音（っ）を次の音の候補に適用する。
function sokuonize(cands: string[]): string[] {
  const out = new Set<string>();
  for (const c of cands) {
    if (c.length > 0 && isConsonant(c[0])) {
      out.add(c[0] + c); // 先頭子音を重ねる: ka -> kka
      if (c.startsWith("ch")) out.add("t" + c); // っち -> tchi
    }
    for (const p of ["xtu", "ltu", "xtsu", "ltsu"]) out.add(p + c);
  }
  return [...out];
}

function tokenize(reading: string): Token[] {
  const runes = [...reading];
  const tokens: Token[] = [];
  let i = 0;
  let pendingSokuon = 0; // 次の音を待っている、積み上がった っ の数

  while (i < runes.length) {
    const ch = runes[i];

    if (ch === "っ") {
      pendingSokuon++;
      i++;
      continue;
    }

    let kanaLen = 1;
    let cands: string[];
    let isN = false;

    if (ch === "ん") {
      isN = true;
      cands = ["nn", "n'", "xn"];
      i++;
    } else if (i + 1 < runes.length && SMALL_VOWEL[runes[i + 1]] && YOUON[ch]) {
      const v = SMALL_VOWEL[runes[i + 1]];
      cands = YOUON[ch].map((p) => p + v);
      kanaLen = 2;
      i += 2;
    } else {
      cands = BASE[ch] ? [...BASE[ch]] : [ch]; // 想定外の文字はそのまま打つフォールバック
      i++;
    }

    if (pendingSokuon > 0 && !isN) {
      cands = sokuonize(cands);
      kanaLen += pendingSokuon;
    } else if (pendingSokuon > 0) {
      // っん は実在しない並びなので、っ の rune をこのトークンに吸収するだけにする。
      kanaLen += pendingSokuon;
    }
    pendingSokuon = 0;

    tokens.push({ kanaLen, startRune: 0, cands, isN, singleN: false });
  }

  // 後ろに何も続かない末尾の っ は、最後のトークンに畳み込む（無ければ作る）。
  if (pendingSokuon > 0) {
    if (tokens.length > 0) {
      tokens[tokens.length - 1].kanaLen += pendingSokuon;
    } else {
      tokens.push({ kanaLen: pendingSokuon, startRune: 0, cands: ["xtu"], isN: false, singleN: false });
    }
  }

  // rune オフセットと、ん を単独 "n" で確定してよいか（先読みが必要）を解決する。
  let r = 0;
  for (const tok of tokens) {
    tok.startRune = r;
    r += tok.kanaLen;
  }
  for (let k = 0; k < tokens.length; k++) {
    if (!tokens[k].isN) continue;
    const next = tokens[k + 1];
    // ん を単独 "n" にできるのは、次の音が "n" でも "y" でもない子音で始まるときだけ
    // （でないと な行 / や行 / 母音 と曖昧になる）。
    tokens[k].singleN =
      !!next &&
      next.cands.every((c) => c.length > 0 && isConsonant(c[0]) && c[0] !== "n" && c[0] !== "y");
  }

  return tokens;
}

export type InputResult = "ok" | "miss" | "ignore";

export class TypingSession {
  readonly reading: string;
  readonly total: number;
  private tokens: Token[];
  private ti = 0;
  private buffer = "";
  private completed = 0;
  private hits = 0;
  private misses = 0;

  constructor(reading: string) {
    this.reading = reading;
    this.tokens = tokenize(reading);
    this.total = this.tokens.reduce((a, t) => a + t.kanaLen, 0);
  }

  get index(): number {
    return this.completed;
  }

  get done(): boolean {
    return this.ti >= this.tokens.length;
  }

  get stats(): { hits: number; misses: number } {
    return { hits: this.hits, misses: this.misses };
  }

  private finalizeToken(): void {
    this.completed += this.tokens[this.ti].kanaLen;
    this.ti++;
    this.buffer = "";
  }

  /** 1文字を入力する。前進したか("ok")、誤りか("miss")、完全に無視すべきか
   *  （打鍵対象外のキー、"ignore"）を返す。 */
  input(raw: string): InputResult {
    if (this.done) return "ignore";
    const ch = raw.toLowerCase();
    if (!/^[a-z\-']$/.test(ch)) return "ignore";

    const tok = this.tokens[this.ti];
    const next = this.buffer + ch;

    if (tok.isN) {
      if (tok.cands.includes(next)) {
        this.hits++;
        this.finalizeToken();
        return "ok";
      }
      if (tok.cands.some((c) => c.startsWith(next))) {
        this.hits++;
        this.buffer = next;
        return "ok";
      }
      // 単独 "n" での確定。buffer が "n"、ここで許容され、かつ `ch` が次トークンの
      // 先頭になる場合。ん を確定し、`ch` を次トークンに対して再処理する。
      if (this.buffer === "n" && tok.singleN) {
        const nx = this.tokens[this.ti + 1];
        if (nx && nx.cands.some((c) => c.startsWith(ch))) {
          this.finalizeToken();
          return this.input(ch);
        }
      }
      this.misses++;
      return "miss";
    }

    const matches = tok.cands.filter((c) => c.startsWith(next));
    if (matches.length === 0) {
      this.misses++;
      return "miss";
    }
    this.hits++;
    if (tok.cands.includes(next)) {
      this.finalizeToken();
    } else {
      this.buffer = next;
    }
    return "ok";
  }

  /** 読み文字列の rune ごとの表示ステータス。 */
  renderStatus(): ("done" | "active" | "todo")[] {
    const runes = [...this.reading];
    const tok = this.tokens[this.ti];
    return runes.map((_, i) => {
      if (i < this.completed) return "done";
      if (tok && i >= tok.startRune && i < tok.startRune + tok.kanaLen) return "active";
      return "todo";
    });
  }

  /** これから打つべき残りのローマ字をヒントとして返す。 */
  romajiHint(): string {
    const tok = this.tokens[this.ti];
    if (!tok) return "";
    const cand = tok.cands.find((c) => c.startsWith(this.buffer)) ?? tok.cands[0];
    let hint = cand.slice(this.buffer.length);
    for (let k = this.ti + 1; k < this.tokens.length; k++) {
      hint += this.tokens[k].cands[0];
    }
    return hint;
  }
}
