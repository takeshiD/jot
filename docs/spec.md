# Jot 仕様

## 1. Taskwarrior との連携

### 対応バージョン
- **Taskwarrior 3.x**(3.5.0 で検証)。2.6 系でも使うコマンドは同じだが未検証。
- 同期(taskchampion sync)は対象外。Jot は同期設定に触れない。

### 使うコマンド
Jot は `task` コマンドだけを使い、Taskwarrior の DB ファイル(`taskchampion.sqlite3` 等)を直接読み書きしない。独自のタスクDBも持たない。
引数はすべて `exec` で個別の argv として渡し、シェルは介さない。

| 用途 | コマンド |
|---|---|
| 読み込み | `task <rc上書き> status.not:deleted export` |
| 1件取得 | `task <rc上書き> uuid:<UUID> export` |
| 追加 | `task <rc上書き> rc.verbose=new-uuid add [jottoday:… jotorder:…] -- <タイトル>` (`--` 以降は解析されずそのまま説明になる) |
| 変更 | `task <rc上書き> <UUID> modify <attr>:<値> / +tag / -tag / project:<名>` |
| 完了 / 取消 | `task <UUID> done` / `task <UUID> modify status:pending` |
| 削除 | `task <UUID> delete` |
| 自由入力の正確な保存(フォールバック) | `task <UUID> export` → 値を差し替えた JSON を `task import -`|

`<rc上書き>` は `rc.confirmation=off rc.recurrence.confirmation=no rc.color=off rc.json.array=on rc.verbose=nothing`(hooks は利用者の設定のまま) と UDA 定義。
対象は常に UUID で指定するので、ID の振り直しの影響を受けない。

### UDA の定義方法
Jot は次の UDA を毎回コマンドライン上書き(`rc.uda.<名>.type=…`)で定義する。**利用者の taskrc は変更しない。**

| UDA | 型 | 内容 |
|---|---|---|
| `jottoday` | string | Today に入れた日(論理日付 `YYYY-MM-DD`)。空なら Today 外 |
| `jotorder` | numeric | Today 内の手動順(昇順) |
| `jotnote` | string | メモ(複数行、改行を含む) |

taskrc に定義が無くても、値は `task export` / `task info` にそのまま出る(未定義UDAとして保持される)。
`task jottoday:… list` のようにフィルタやレポートで使いたい場合は `jot --print-taskrc` の出力を taskrc に追記する(任意)。

### 並行操作時の扱い
- 書き込みはフィールド単位の `task <UUID> modify` なので、Jot が起動中に `task` で別のフィールドを変えても上書きしない。
- 各操作の直後に `task export` で全件を読み直す。`r`(再読込)でいつでも `task` 側の変更を取り込める。起動中の自動再読込はしない。
- Taskwarrior 3 のDBロック中で `task` が失敗した場合、最大3回まで待って再試行する。それでも失敗すればエラーを表示する。
- `import` フォールバックは「export 直後に import」の短い区間で行う。この間に他から同じタスクを変更すると、その変更は失われうる(未解決事項参照)。
- 対象タスクが他所で削除済みなどで `task` が失敗した場合、メッセージを表示して再読込する。

## 2. フィールド対応

| Jot | Taskwarrior |
|---|---|
| タイトル | `description`(1行。改行・タブは空白に置換) |
| メモ | UDA `jotnote`(複数行) |
| 注釈 | `annotations`(表示と URL 抽出のみ。Jot からは編集しない) |
| タグ | 標準 `tags`(`+tag` / `-tag`) |
| グループ | 標準 `project` |
| Today | UDA `jottoday` |
| Today 内の順序 | UDA `jotorder` |
| 完了 / 削除 | 標準 `status`(`completed` + `end` / `deleted`) |
| 期限・予定 | `due` / `scheduled`(表示のみ。Jot は変更しない) |

メモを annotations ではなく UDA にした理由: annotations は1行ずつ・タイムスタンプがキーで、複数行メモの編集(行の並び・空行・同一秒の重複)を正確に往復できないため。
`task` で付けた注釈の URL も `o` で開ける。

タグ名に使えないもの: 空白、`:` `(` `)` 引用符 `\`、先頭の `+` `-`。project 名に空白・引用符・括弧は使えない(入力時にエラー表示)。

## 3. Today

- `space` で pending のタスクを Today に追加: `jottoday` = 今日の論理日付、`jotorder` = 現在の Today の最大値 + 1(末尾)。
- もう一度 `space` で解除: `jottoday` と `jotorder` を消す。
- `due` / `scheduled` / `status` / `wait` は一切変更しない。
- 完了済みのタスクは Today に追加できない(`x` で完了取消してから)。
- Today 画面で `a` で追加したタスクは Today の末尾にも入る。
- `task` コマンドで `jottoday:YYYY-MM-DD` を付けたタスクも Today に出る(`jotorder` が無ければ順序付きの項目の後ろ、作成順)。

### 日付が変わったときの扱い
「今日」は **論理日付**: ローカル時刻から `day_start_hour` 時間引いた日付(既定 0 = 0:00 で日付が変わる。4 にすると 3:59 までは前日)。

| タスクの状態 | 表示 |
|---|---|
| pending かつ `jottoday` = 今日 | Today に表示 |
| pending かつ `jottoday` < 今日 | **持ち越し**: Today に表示し続け、`↻MM/DD`(入れた日)を付ける。`carry_over = false` の場合は Today から外れる(`jottoday` は残るので、再度 `space` で今日に入れ直せる) |
| completed かつ 完了時刻(`end`)の論理日付 = 今日 | その日の終わりまで Today に `✓` 付きで残る(順序もそのまま。`x` で取消すると未完了に戻る) |
| completed かつ 完了が昨日以前 | Today から外れる |
| deleted | 表示しない |

- 日付境界をまたいでも Jot はデータを書き換えない(表示の判定だけ)。持ち越し項目の `jottoday` は「最初に入れた日」のまま。
- 起動中に日付が変わると(1秒ごとに確認)表示を更新し「日付が変わりました」と表示する。
- 検証用に `--now` / `JOT_NOW` で時刻を差し替えられる(その時刻から実時間で進む)。差し替え中は完了時に `end` も差し替えた時刻で記録する(`task <UUID> modify status:completed end:<時刻>`)。通常時は `task done`。

## 4. 順序

- Today 画面で `J` / `K`(`shift+↓` / `shift+↑`)で選択項目を1つ下 / 上へ動かす。
- 動かした結果の並びが 1..n になるよう、値が変わる項目だけ `jotorder` を書き換える(通常2件)。
- 順序は UDA に保存されるため、Jot の終了・再起動、`task` での他タスクの変更・追加・ID振り直しの後も保たれる。
- 並べ替えは Today 画面のみ。全タスク画面は「pending を先、urgency 降順」(完了済み表示時は完了日の新しい順で末尾)。

## 5. メモと URL 起動の安全性

- `o` でメモ(`jotnote`)と注釈から URL らしい文字列を抽出し一覧表示。番号キーか `enter` で開く。
- 開けるのは **`http` / `https` で、ホストがあり、空白・制御文字を含まない** URL のみ。
  `javascript:` `data:` `file:` `vbscript:` `ftp:` などは一覧に「[ブロック] 理由」と表示され、選んでも開かない。
- 末尾の句読点・閉じ括弧(`.,;:!?)]}>、。」）`)は URL に含めない。
- 起動コマンド: 設定 `browser.command`(argv 配列)。`"{url}"` の位置に URL、無ければ末尾に追加。未設定なら OS 既定(Linux `xdg-open`、macOS `open`)。
- **シェルを経由しない**(`exec` に argv を直接渡す)。URL は常に1つの引数で、`$(...)` や `;` などはそのまま文字として渡る。
- 起動したプロセスの入出力は端末につながず、終了を待たない。起動失敗はステータス行にエラー表示。

## 6. 表示

- 1タスク = 1行。タイトルが長いときは表示幅(全角=2)で切り詰めて `~` を付ける。タグ等のメタ情報はタイトルの最低幅を確保した上で入る分だけ表示。
- 端末サイズの変更に追従。幅 10 未満・高さ 4 未満では「端末が小さすぎます」のみ表示。
- 入力欄・メモ欄では、1回の読み込みでまとめて届いた文字列(SSH/tmux 越しの "home" "esc" "enter" 等)を常に文字として挿入し、キー操作とは解釈しない。Esc・Enter・ctrl+s は実際の特殊キーのときだけ働く。
- 描画はキー処理の直後に同期的に行う(変化した行だけを書き換える独自の描画。Bubble Tea は入力・イベント処理に使う)。
- 東アジアの「曖昧幅」文字は記号に使わない(CJK ロケールでも桁がずれないように)。
- テーマ: `dark`(既定) / `light` / `dusk`。ANSI 256 色で指定。選択行は背景色 + `›`、Today は `☀`(全タスク画面)、タグは `#` + 色、グループは見出し行 `▸ 名前 (件数)` + 色。

## 7. エラー表示
- `task` が無い: 「エラー: task コマンドが見つかりません (…)」をステータス行に表示し、アプリは続行(空の一覧)。
- `task` がエラー終了: 「エラー: task: <taskの出力の1行目> (task <短縮した引数>)」を表示(原因を先頭に置き、UUID は8桁に短縮)。
- 設定ファイルが不正: 起動前に `jot: 設定エラー: <ファイル>: <理由>` を標準エラーに出して終了コード2。

## 8. 未解決事項・既知の制約
- `import` フォールバック中(ミリ秒単位)に同じタスクが他から変更されると、その変更が失われうる。通常の操作(Today/順序/タグ/完了/削除と、普通の文字列のタイトル・メモ)は `modify` のみで行うため影響しない。
- Taskwarrior 2.6 系、macOS / Windows での動作は未検証。
- 起動中の自動再読込(ファイル監視)は行わない。`task` 側の変更は `r` か再起動で反映。
- 繰り返しタスク・依存関係・期限の編集 UI は無い(表示のみ)。
- タスク数が非常に多い(数万件)場合、操作ごとの全件 export が遅くなる可能性がある。
- ブラウザ起動の成否は起動コマンドを開始できたかまでしか分からない(`xdg-open` 内部の失敗は検出しない)。
- 一覧の折り返し表示(タイトル全体を複数行で見る)は無い。全文は詳細表示(`enter`)で確認する。
