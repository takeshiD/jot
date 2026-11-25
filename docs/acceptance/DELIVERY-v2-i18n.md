# 納品・受入記録: Jot — 多言語化(英語/日本語)の追加

- 受入日時: 2026-11-25T07:34:18.302Z
- 要求版: f3ffcdcc6e9824ae
- 成果物 fingerprint: 217bbfe07170bf6e75fad0f5
- 基準版: 0c85ff24f729057c
- 検収回数: 1

## 使い始め方

準備:

    make build

- **jot** (tui): `./bin/jot` — Jot itself. Reads and writes your Taskwarrior data (TASKRC/TASKDATA; defaults ~/.taskrc and ~/.task) through the task command. Display language: the config's language (en/ja), otherwise the first non-empty of LC_ALL > LC_MESSAGES > LANG (ja* gives Japanese, anything else English), otherwise English. Options: --config / --theme / --now / --check-config / --print-taskrc / --version
- **sandbox** (tui): `sh scripts/sandbox.sh --seed` — Starts Jot on isolated Taskwarrior data with sample tasks under $HOME/jot-sandbox (change with JOT_SANDBOX); your real data is not touched. Try `LANG=ja_JP.UTF-8 sh scripts/sandbox.sh --seed` or `LC_ALL=C sh scripts/sandbox.sh --seed` to compare languages
- **e2e** (tui): `sh scripts/e2e-session.sh` — Runs Jot on isolated data. After you quit, prints the Today items as the plain task CLI sees them (ORD/DESC/STATUS/TAGS/TODAY), then changes another task with the CLI; press Enter to finish
- **check-config** (cli): `./bin/jot --check-config` — Validates the config file. If valid, prints 'Config OK: <path> (theme=…, preset=…, language=<chosen language> (config file / locale VAR=value / default))'. If invalid (including a language other than en/ja), prints the reason and exits with code 2

## 検収判定

The orchestrator's re-run passed all 31 machine checks, C1–C33 except the two review criteria C17 and C33. I reviewed those two by reading the code and the spec and found no problems. In my own user trial (S1–S10), language selection, the switch between English and Japanese, and data compatibility across languages all behaved as specified. All must requirements R1–R23 are met, so the deliverable is accepted. The remaining items are minor display issues that do not affect use, recorded as follow_ups.

- R1: satisfied — C1 passed on re-run: make build-linux-amd64 gave the same hash on two builds, --version printed jot 0.1.0, and bubbletea v1.3.10 and lipgloss v1.1.0 are pinned in go.mod. Setup (make build) also succeeded.
- R2: satisfied — C14: grep finds no references to the Taskwarrior DB files, and the spec documents version 3.5.0, rc.uda and concurrent use. Unchanged from the previous accepted version.
- R3: satisfied — C3 and C19 passed on re-run: the CLI round trip works in both directions in a real terminal. In my trial, task export also matched what Jot showed.
- R4: satisfied — C3, C11 and C27: Today add/remove, switching views, and persistence after restart all work.
- R5: satisfied — C4: in a real terminal across the date boundary, unfinished items carry over and completed items drop out. C2 TestDateRollover and related tests pass. My trial with --now confirmed ↻ is shown in both languages.
- R6: satisfied — C3: the order survives a restart and a CLI change to another task. My trial also kept an order set in English when viewed in Japanese.
- R7: satisfied — C3 and C19: tags are visible to the CLI and grouping is displayed.
- R8: satisfied — C5 and C19: multi-line notes are saved and displayed. The field mapping is in spec §2.
- R9: satisfied — C9: the recording script received each URL as a single argument, and javascript: and file: were blocked. My trial showed blocking in both languages.
- R10: satisfied — C6, C7, C8 and C27: basic operations, help, and error display without crashing work in both languages. In my trial the missing-task error was translated in both languages.
- R11: satisfied — C10, C12 and C13: custom keys, invalid-config errors, and the XDG/JOT_CONFIG/--config paths all work.
- R12: satisfied — C10 and C11: the light and dark theme colours are confirmed from actual screen styles.
- R13: satisfied — C5, C27 and C28: layout follows resizes to 60x20, 40x12 and 100x30 without breaking, and Japanese width is handled correctly.
- R14: satisfied — C2 (all tests PASS, no SKIP) and C16 (HOME is not touched).
- R15: satisfied — C15: the required sections are present. I read spec §8 and §9: the language section and unresolved items are written honestly.
- R16: satisfied — C1: reproducible linux/amd64 build.
- R17: satisfied — C26, C30 and C31: language in the config wins over the locale (CLI and real terminal). Only en/ja are accepted (C22; JA and jp rejected in my trial).
- R18: satisfied — C25 and C29: all 9 patterns, including the order's example, match expectations. I also checked the code: FromLocale evaluates only the first non-empty of LC_ALL > LC_MESSAGES > LANG, and IsJapaneseLocale checks whether the language part is ja.
- R19: satisfied — C22, C23 and C24: jp and a wrong type exit with code 2 and show the file name and reason, both with --check-config and at startup.
- R20: satisfied — C20: catalog key match, format verb match, no unused keys, and no hard-coded Japanese in the source are all checked by tests and pass. I read catalog.go (central en/ja tables) and the weekday code (via ui.weekdays), and grep found no hard-coded English display strings in the UI. My trial confirmed translation of the main screens, help, URL picker, confirmation, errors and --help.
- R21: satisfied — Lang.Err shows non-*i18n.Error values (such as task output) unchanged. jottoday uses a fixed DateLayout. C20 TestDataIndependentOfLanguage passes. In my trial, export was identical across languages and switching languages did not break anything.
- R22: satisfied — The existing scenarios (C3–C11, C19) pass unchanged under LC_ALL=ja. C27 and C28 show no breakage in either language at 80x24, 60x20 and 40x12.
- R23: satisfied — C21: go test passes under 6 locale settings. C20 adds the language tests. C32: the sample config comments out language and the README and spec contain the required sections.

## 後続事項（要求外・受入を妨げない）

- English empty-Today text breaks mid-word at 60 columns: In English at 60x20, 'Today is empty... to add o / ne to Today' is split mid-word. It does not break the layout, but wrapping at word boundaries would read better.
- Long errors and confirmations are cut off at the end: The hint at the end of the missing-task error is cut off even at 80 columns, and the end of the delete confirmation is cut off at 60 columns. The cause and the available actions are still visible, so this does not block use.
- Go's standard flag error is English only: 'flag provided but not defined' from an unknown option stays in English even under a Japanese locale. The help text after it is translated.

