<img src="assets/icon.svg" width="64" height="64" alt="">

# Jot — Today中心のTaskwarrior TUI

Jot は、Taskwarrior の既存タスクから「今日やること(Today)」を選んで手動で並べ、
タグ・グループ(project)・メモ(URL付き)を扱うキーボード中心の端末アプリです
(Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea) / Lip Gloss / Bubbles)。

- データはすべて標準の `task` コマンド経由で読み書きします。独自DBは持たず、Taskwarrior のDBファイルにも直接触れません。
  Jot での変更は `task export` / `task list` にそのまま見え、`task` での変更は Jot の再読込(`r`)や再起動で反映されます。
- Today・並び順・メモは UDA(`jottoday` / `jotorder` / `jotnote`)として Taskwarrior に保存されます。期限(due)・予定(scheduled)・状態は変更しません。

- 表示言語は英語(`en`)と日本語(`ja`)。設定 `language` か、未指定ならロケール(`LC_ALL` > `LC_MESSAGES` > `LANG`)で決まり、既定は英語です([表示言語](#表示言語-language))。

仕様の詳細(Today/日付/順序/フィールド対応/URL起動の安全性/表示言語/未解決事項)は [docs/spec.md](docs/spec.md) を参照してください。

## 必要環境

- Linux (他のUnixでも動く見込み。検証は Linux のみ)
- Taskwarrior **3.x** (3.5.0 で検証) が `PATH` にあること
- ビルドに Go 1.27 以上 (依存は `go.mod` / `go.sum` で固定し `vendor/` に同梱。ネットワーク不要・cgo 不要)

## ビルドと実行

```sh
make build            # = CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=0.1.0" -o bin/jot ./cmd/jot
./bin/jot
```

linux/amd64 の配布用バイナリ(同じソースからは同一ハッシュになります):

```sh
make build-linux-amd64   # -> dist/jot-linux-amd64
```

テスト(単体 + 一時Taskwarriorデータを使う結合テスト。ホームの ~/.taskrc や ~/.task には触れません):

```sh
make test             # = CGO_ENABLED=0 go test ./...
```

### 本番データに触れずに試す

```sh
scripts/sandbox.sh --seed            # $HOME/jot-sandbox に隔離したデータでサンプル付き起動
JOT_SANDBOX=/tmp/jt scripts/sandbox.sh --seed   # 置き場所を変える
```

`sandbox.sh` は `TASKRC` / `TASKDATA` をサンドボックスに向けてから `bin/jot` を起動します。

### オプションと環境変数

| | |
|---|---|
| `--config PATH` / `$JOT_CONFIG` | 設定ファイル (既定 `$XDG_CONFIG_HOME/jot/config.toml`、未設定なら `~/.config/jot/config.toml`) |
| `--theme NAME` | テーマを一時的に上書き (`dark` / `light` / `dusk`) |
| `--check-config` | 設定を検証して終了 (不正なら理由を表示し終了コード2) |
| `--print-taskrc` | `task` 側でUDAを認識させたい場合に taskrc に追記できる行を表示 (任意) |
| `--now TIME` / `$JOT_NOW` | 現在時刻の差し替え(テスト・検証用。例 `2026-11-24T23:59:55`。指定時刻から実時間で進む) |
| `$JOT_TASK` | 使う `task` コマンドのパス (設定 `taskwarrior.command` より優先) |
| `TASKRC` / `TASKDATA` | Taskwarrior 標準の環境変数。そのまま `task` に渡ります |
| `LC_ALL` / `LC_MESSAGES` / `LANG` | 設定 `language` が無いときの表示言語の判定に使用(下記) |

## 画面

(日本語表示の例。英語表示では `All 12`、`2026-11-24 Tue`、`? help  q quit  space Today  a add …` のようになります)

```
 Jot  Today 3  全タスク 12                          2026-11-24 (火)
   1 ☐ 週報を書く                                   #mtg @work
›  2 ☐ 牛乳を買う                                   ✎ @home
   3 ✓ Bubble Tea のドキュメントを読む
 ...
 (ステータス行)
 ? ヘルプ  q 終了  space Today  a 追加  x 完了  tab 切替 ...
```

- **Today** 画面: Today の項目を手動の順番で表示。`↻11/23` は前日以前から持ち越した項目、`✓` は今日完了した項目。
- **全タスク** 画面: pending のタスクを project ごとにグループ表示(`v` で切替)。`☀` は Today に入っているタスク。`c` で完了済みも表示。
- `✎` はメモあり、`#tag` はタグ、`@project` はグループ。

## キー一覧 (既定 = vim プリセット)

| キー | 操作 |
|---|---|
| `j` / `k` / `↓` / `↑` | 下 / 上へ移動 |
| `g` / `G` / `Home` / `End` | 先頭 / 末尾 |
| `space` | Today に追加 / 解除 |
| `tab` (`1` / `2`) | Today ⇄ 全タスク 切替 (Today / 全タスク を直接) |
| `J` / `K` (`shift+↓` / `shift+↑`) | Today 内で項目を下 / 上へ移動(並べ替え) |
| `a` | タスク追加 (Today 画面で追加すると Today にも入る) |
| `e` | タイトル編集 |
| `n` | メモ編集(複数行。`ctrl+s` 保存 / `esc` 取消 / `enter` 改行) |
| `t` | タグ編集(空白区切りで全タグを編集。追加/削除が反映) |
| `p` | グループ(project)編集 |
| `x` | 完了 / 完了取消 |
| `D` | 削除(`y` で確定) |
| `o` | メモ・注釈内の URL 一覧 → `enter` か番号で開く |
| `enter` / `l` | 詳細表示(メモ・注釈・期限など。`esc` で戻る) |
| `v` | グループ表示 切替 |
| `c` | 完了済みの表示 切替 |
| `r` / `ctrl+r` | 再読込(`task` コマンド側の変更を反映) |
| `?` | ヘルプ(現在のキー割り当てを表示) |
| `q` / `ctrl+c` | 終了 |

入力欄では `enter` で確定、`esc` で取消。`arrows` プリセット(矢印・`ctrl+n/p`・`F1`/`F2`/`F5` 等)もあります。

## 設定

設定例: [examples/config.toml](examples/config.toml)

```sh
mkdir -p ~/.config/jot && cp examples/config.toml ~/.config/jot/config.toml
jot --check-config
```

```toml
language = "ja"            # en | ja (書かなければロケールで判定)
theme = "light"            # dark | light | dusk
preset = "vim"             # vim | arrows
day_start_hour = 4         # 午前4時までは前日扱い
carry_over = true          # 未完了のTodayを翌日へ持ち越す
group_by = "project"       # project | none

[browser]
command = ["firefox", "--new-tab", "{url}"]   # 未指定なら xdg-open / open

[keys]                     # プリセットからの差分だけ書く
toggle_today = ["space", "T"]
move_up = ["K", "ctrl+k"]
```

操作名: `up down top bottom toggle_today switch_view view_today view_all move_up move_down add edit_title edit_note edit_tags edit_project toggle_done delete open_url detail toggle_group toggle_completed refresh help quit`

不正な設定(不明な項目・操作名・キー名、キーの重複、未知のテーマ、`language` が en/ja 以外、TOMLの構文・型エラーなど)は起動時と `--check-config` で
`jot: 設定エラー: <ファイル>: <理由>`(英語表示では `jot: config error: <file>: <reason>`)を表示して終了コード 2 で終了します。

## 表示言語 (language)

Jot の画面・メッセージは英語(`en`)と日本語(`ja`)に対応しています。決め方:

1. 設定ファイルの top-level キー `language` があればそれを使う。受け付ける値は `"en"` と `"ja"` **のみ**(`"jp"`・`"JA"`・`"ja_JP"`・空文字・数値などはエラー)。ロケールより優先されます。
2. `language` が無いとき、`LC_ALL` → `LC_MESSAGES` → `LANG` の順で **最初の空でない** 変数だけを見て、
   日本語ロケール(`ja_JP.UTF-8`、`ja_JP`、`ja` など先頭が `ja`)なら日本語、それ以外(`C`、`POSIX`、`en_US.UTF-8` など)なら英語。
   例: `LC_ALL` が空・`LC_MESSAGES=C`・`LANG=ja_JP.UTF-8` → 英語。(`LANGUAGE` 変数は見ません)
3. どれも未設定なら英語(既定・フォールバック)。

```sh
LANG=ja_JP.UTF-8 jot          # 日本語
LC_ALL=C jot                  # 英語
jot --check-config            # 例: Config OK: … language=en (locale LANG=en_US.UTF-8)
```

`--check-config` は採用された言語とその根拠(設定ファイル / ロケール変数 / 既定)を表示します。
`language` が不正なら、他の不正な設定と同じく `--check-config` でも起動時でも
`jot: config error: <ファイル>: language must be "en" or "ja": "jp"`(日本語環境では `jot: 設定エラー: <ファイル>: language は "en" か "ja": "jp"`)を表示して終了コード 2 になります。
このエラー自体の言語はロケールで決まります。

**翻訳されないもの:** タイトル・メモ・タグ・project 名・URL などのユーザーデータ、`pending` などの Taskwarrior の生の値、
`task` コマンドの出力(エラー時に表示する1行目を含む)。保存される UDA 名・値(`jottoday` の日付形式など)も言語に関係なく同じなので、
言語を切り替えても同じ Taskwarrior データをそのまま使えます。起動中に言語を切り替える機能はありません(再起動で反映)。

翻訳文字列は [internal/i18n/catalog.go](internal/i18n/catalog.go) に英日まとめて置き、両言語のキー集合と書式指定子が一致することをテストで確認しています。

## Taskwarrior との連携(概要)

- 読み込み: `task status.not:deleted export`
- 書き込み: `task <uuid> modify …` / `task add … -- <タイトル>` / `task <uuid> done` / `task <uuid> delete`
  (自由入力のテキストが `task` の引数解析で変わってしまう場合のみ `task export` → `task import` で正確に保存)
- UDA は毎回 `rc.uda.*` の上書きとして渡すため、`~/.taskrc` を変更する必要はありません。
  `task` のレポート・フィルタでも使いたい場合は `jot --print-taskrc >> ~/.taskrc`。

詳細は [docs/spec.md](docs/spec.md)。

検収結果は [docs/acceptance/ACCEPTANCE.md](docs/acceptance/ACCEPTANCE.md)。
