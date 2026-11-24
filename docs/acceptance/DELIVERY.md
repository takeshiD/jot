# 納品・受入記録: Jot — Today中心のTaskwarrior用キーボードTUI

- 受入日時: 2026-11-24T08:55:24.525Z
- 要求版: df685cab7856bb54
- 成果物 fingerprint: 2c44476a15c978aa1c78a26c
- 基準版: a69033db23f107ed
- 検収回数: 2

## 使い始め方

準備:

    make build

- **jot** (tui): `./bin/jot` — Jot 本体。利用者の TASKRC/TASKDATA(既定は ~/.taskrc, ~/.task)を task コマンド経由で使う。--config/--theme/--now/--check-config/--print-taskrc/--version
- **sandbox** (tui): `sh scripts/sandbox.sh --seed` — $HOME/jot-sandbox(JOT_SANDBOX で変更可)に隔離した Taskwarrior データでサンプルを入れて Jot を起動する。本番データには触れない
- **e2e** (tui): `sh scripts/e2e-session.sh` — 隔離データで Jot を起動し、終了後に素の task CLI から見た Today 項目(ORD/DESC/STATUS/TAGS/TODAY)を表示する。続けて CLI で別タスク(週報)を変更し、Enter で終了
- **check-config** (cli): `./bin/jot --check-config` — 設定ファイルを検証する(正しければ『設定OK: パス』、不正なら理由を表示して終了コード2)

## 検収判定

合格。要求16件すべて満たしている。前回 R10 で差し戻した、入力欄とメモ欄で文字列 'esc' 'enter' 'home' などがキー操作として処理される問題は直っており、REG1・C19・C18 が新しい環境での再実行で pass した。C1〜C16 と C18・C19 もすべて pass。利用者試行の11シナリオもすべて pass(CLI 往復、Today、並べ替えの保持、日付の持ち越し、URL の安全性、設定、テーマ、狭い端末、task の失敗時)。C17 のレビューでは、spec の §1・§2・§3・§5・§8 が client.go・today.go・urls.go と一致していることを確認した。軽微な改善点は follow_ups に挙げた。プリセットのキーを設定で別の操作に割り当てると元の操作のキーが警告なしに外れる件と、task が無いときの本文表示の件。

- R1: satisfied — C1: go.mod で bubbletea v1.3.10 と lipgloss v1.1.0 に固定され、make build と make build-linux-amd64 で jot 0.1.0 がビルドできた。表示名は Jot。
- R2: satisfied — C14 で DB ファイルへの参照が無いことを確認した。client.go は exec.Command で task を呼び、rc.uda.* を上書きで渡し、locked のときは再試行し、import はフォールバックだけに使う。いずれも spec §1 の記述と一致する。対応バージョン(3.5.0)と並行操作の扱いも書かれている。
- R3: satisfied — C3、C19、go test の RoundTrip 系が pass した。利用者試行 S3/S4 でも、Jot での変更が task export / task all に、CLI での変更が r の後の Jot に同じ内容で見えた。
- R4: satisfied — Space(設定で変えたキーも)で Today に追加・解除でき、Tab で画面を切り替えられる。Today は UDA jottoday/jotorder に入り、due は変わらなかった(利用者試行 S2 の task export、C11 の再起動後の保持)。
- R5: satisfied — spec §3 の論理日付・持ち越し・完了済みは当日だけ残る、という仕様は自然で、today.go と一致する。C4 と利用者試行 S5(--now での起動と、起動中の日付変更)で、仕様どおりに動くことを画面で確認した。
- R6: satisfied — K/J で並べ替えられ、jotorder UDA に保存される。C3 と利用者試行 S3 で、CLI での別タスクの変更や再起動の後も順序が残った。
- R7: satisfied — project ごとにグループ表示され、v で切り替えられる。t で付けたり外したりしたタグは、task export の tags に標準のタグとして出た(C3、利用者試行 S4)。
- R8: satisfied — タイトルは description、複数行のメモは UDA jotnote に入る。spec §2 に対応が書かれ、annotations を使わなかった理由も妥当。日本語の複数行メモが保存され、詳細画面に表示された(C5、利用者試行 S4/S9)。
- R9: satisfied — urls.go は http/https だけを許可し、{url} を置き換えて exec.Command で起動する(シェルを使わない)。C9 と利用者試行 S6 で、記録用スクリプトに URL が1引数のまま渡り、javascript:/file: がブロックされることを確認した。xdg-open による既定ブラウザ起動は、実際のブラウザを開かない条件のため試していない。
- R10: satisfied — 前回差し戻した点(文字列 'esc' 'enter' などがキー操作として処理される)は直っていた。REG1、C19、C18 がすべて pass。基本操作、ヘルプ、task が無いときと失敗したときのエラー表示も、C6〜C8 と利用者試行 S4/S10 で確認した。
- R11: satisfied — XDG 既定のパス、JOT_CONFIG、--config で設定ファイルを変えられ(C13)、キーを変更でき(C10、利用者試行 S7)、vim 風の操作が既定、arrows プリセットもある。不正な設定は理由を表示して終了コード2になる(C12、利用者試行 S7)。設定例も同梱されている。
- R12: satisfied — dark/light/dusk の3テーマがある。C10 と C11 で、選択行・☀・グループ・タグがそれぞれ別の色になり、テーマ間でも配色が違うことを確認した。
- R13: satisfied — C5 と利用者試行 S9 で、60x20 と 40x12 でも各行が1行に収まり、全角文字の幅も正しく、サイズ変更に追従することを確認した。
- R14: satisfied — C2 で go test が30件すべて PASS(SKIP なし)。一時データを使う結合テストで要求された項目を確認している。C16 でテスト後の HOME に .task が作られないことも確認した。
- R15: satisfied — C15 で必要な節がそろっていることを確認した。spec の内容(§1/§2/§3/§5/§8)も読んで、実装と一致し、未解決事項が正直に書かれていると判断した(C17)。
- R16: satisfied — C1: make build-linux-amd64 を2回実行してハッシュが一致した。

## 後続事項（要求外・受入を妨げない）

- 設定でプリセットのキーを奪ったときの扱いを明記・警告する: config.go は、[keys] に書いた操作がプリセットのキーと重なると、元の操作からそのキーを黙って外す。たとえば quit=["a"] にするとタスク追加のキーが無くなるが、--check-config は OK を返す。エラーになるのは、利用者の書いた設定どうしが重なったときだけ。README の『キーの重複はエラー』と食い違って見えるので、README に書くか、--check-config で警告を出すとよい。
- task が無いときの本文の表示: task コマンドが見つからないときも、本文に『Todayは空です…』と出る。エラーの状態が分かる本文にすると誤解が減る。

