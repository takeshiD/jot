# Jot 検収結果

Jot は [kenshu-loop](https://github.com/takeshiD/kenshu-loop)（発注・納品・検収型エージェントループ）で開発・検収した。

- 発注書: [order.json](order.json)（要求16件、R1〜R16）
- 受入記録（kenshu-loop が生成）: [DELIVERY.md](DELIVERY.md)
- kenshu-loop 0.1.0、ハーネスは Claude Code。受注AIと検収AIは別セッションで動き、検収AIは編集ツールを使えない権限で実行した。

## 検収サイクル

| ラウンド | 内容 | 結果 |
|---|---|---|
| 1 | 初回納品（基準 C1〜C17）。オーケストレータが新しい環境で再実行（失敗0）。検収AIの利用者試行14シナリオ（失敗1・未確認5）と技術審査 | **差し戻し（rework）**: R10。入力欄とメモ欄で、1回の読み込みで届いた文字列（`esc` `enter` `home` 等）がキー操作として処理され、メモの下書きが消えたり空の値が保存されたりする。回帰基準 REG1 を追加（現成果物で再現を確認） |
| 2 | 修正納品。1回目の提出は、基準の内容を変えたことと存在しない基準IDを参照したことで受理されなかった。2回目の提出（C1〜C19）を受理。再実行（REG1 を含む20件、失敗0）、利用者試行11シナリオ（失敗0・未確認4）、技術審査 | **合格（accepted）**。要求16件すべて satisfied |

- 受入: 2026-11-24T08:55:24Z。成果物 fingerprint `2c44476a15c978aa1c78a26c`、基準版 `a69033db23f107ed`
- 総費用 約 $35.87、所要 約46分

## 発注者側の追加確認（受入後）

テストはすべて、`HOME`・`TASKRC`・`TASKDATA` を一時ディレクトリに向けて実行した。

- `CGO_ENABLED=0 go test -count=1 ./...`: 全パッケージ ok（config / today / tw / ui / urls）
- `CGO_ENABLED=0 go vet ./...`: 指摘なし
- `make build-linux-amd64` を2回実行し、どちらも sha256 `91b9930c35425ba0409fd4409d75b9197ba0c339c518315a21c86e288048949a` で一致（Go 1.27.3、`-mod=vendor`）
- 実際の PTY（80x24、その後 50x14 に縮小）で隔離データを操作した。全タスク画面で3件を Today に追加し、Today 画面で並べ替えた。素の `task export` に `jottoday`／`jotorder` が反映され、`due` は変わらないことを確認。狭い幅でも各行が1行に収まった
- 利用者のホームにある `~/.task` と `~/.taskrc` は作成も変更もされていない

## 後続事項（受入を妨げない）

- 設定の `[keys]` でプリセットのキーを別の操作に割り当てると、元の操作からそのキーが警告なしに外れる（`--check-config` は OK を返す）
- `task` が見つからないとき、本文に「Todayは空です」と出る（エラーはステータス行のみに表示）
- Go のモジュールパスが `github.com/tkcd/jot` のままで、GitHub の `takeshiD/jot` と一致しない。clone して `make build` すればビルドできるが、`go install github.com/takeshiD/jot/cmd/jot@latest` は使えない
- 既定ブラウザ（`xdg-open`）で実際に開く動作は、検収条件（実ブラウザを開かない）のため試していない
- 仕様上の未解決事項は [../spec.md](../spec.md) §8 を参照

---

# v2: 多言語化(英語 / 日本語)の検収結果

- 発注書: [order-v2-i18n.json](order-v2-i18n.json)（要求23件。R1〜R16 は v1 のまま回帰確認の対象として残し、R17〜R23 を追加）
- 受入記録: [DELIVERY-v2-i18n.md](DELIVERY-v2-i18n.md)
- 発注の訂正: 最初の発注では日本語コードを `jp` としていたが、利用者の指示で標準の `ja` に変更した。`jp` のまま走っていた実行は、受注AIの作業中(約2.5分、費用 $0)に SIGINT で中断した。途中の成果物は破棄し、`ja` に直した発注書で `init --force` して再発注した。

| ラウンド | 内容 | 結果 |
|---|---|---|
| 1 | 納品(基準 C1〜C33)。オーケストレータが新しい環境で機械検査31件を再実行(失敗0)。利用者試行10シナリオ(失敗0・未確認5)。C17・C33 はコードを読んで審査 | **合格(accepted)**。R1〜R23 をすべて満たす |

- 受入: 2026-11-25T07:34:18Z。成果物 fingerprint `217bbfe07170bf6e75fad0f5`、基準版 `0c85ff24f729057c`
- 費用 約 $10.02、所要 約22.5分

## 発注者側の追加確認

- `go test -count=1 ./...` を、ロケール環境変数なし・`C`・`ja_JP.UTF-8`・`en_US.UTF-8` の4通りで実行し、すべて ok（cmd/jot, config, i18n, today, tw, ui, urls）。`go vet` も指摘なし
- `make build-linux-amd64` を2回実行し、どちらも sha256 `334b7ec0c81f75866aa9ab1ce6913120ba032204992eefaf5de7e4843e91009c` で一致
- `--check-config` の結果
  - `language = "jp"` / `1` → 終了コード 2
  - `"ja"` → OK
  - `LANG=ja_JP.UTF-8` のときはエラー文が日本語、`LC_MESSAGES=C` + `LANG=ja_JP.UTF-8` のときは英語
- 隔離データで実際に操作し、次を確認した
  - `LANG=ja_JP.UTF-8` で起動すると日本語UI（`全タスク`、`(水)`、`Todayに追加しました`）
  - 同じロケールでも `language = "en"` の設定で起動すると英語UI（`All`、`Wed`、`Added to Today`）
  - タイトル・タグ・project 名は翻訳されず、`task export` の `jottoday`/`jotorder` は言語によらず同じ値

## v2 の後続事項（受入を妨げない）

- 60桁で英語の「Today is empty…」が単語の途中で折り返される
- 長いエラーや確認文の末尾が狭い幅で切れる（原因と操作は見える）
- 未知のコマンドラインオプションに対する Go 標準の `flag` のエラーは英語のまま（直後のヘルプは翻訳される）
