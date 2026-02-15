# fumi (文) — Design Document

> Local-first, terminal-native なメモアプリ。ブラウザでもターミナルでもNeovimでも、同じメモにさっとアクセス。

---

## 設計思想

**Local-first**: すべてのデータはローカルのSQLiteに保存。オフラインでも完全動作し、オンライン時にバックグラウンドで同期。git操作のような明示的な操作は不要。

**Daemon中心アーキテクチャ**: ローカルで常駐するRust製デーモン(`fumid`)がすべてのクライアント(CLI/TUI/Web/Neovim)に統一APIを提供。インデックス更新・同期・LLM連携もデーモンが担う。

**Plain textとの親和性**: 内部フォーマットはMarkdown。どのクライアントからも自然に読み書きでき、他ツールへのコピペも容易。

---

## アーキテクチャ概要

```
┌───────────────────────────────────────────────────────────┐
│                      Clients                              │
│                                                           │
│  ┌──────────┐   ┌──────────┐  ┌───────────┐   ┌─────────┐ │
│  │ fumi CLI │   │ fumi TUI │  │  Neovim   │   │   Web   │ │
│  │ (Rust)   │   │ (Rust/   │  │  Plugin   │   │ (React) │ │
│  │          │   │  Ratatui)│  │  (Lua)    │   │         │ │
│  └────┬─────┘   └────┬─────┘  └─────┬─────┘   └────┬────┘ │
│       │              │              │              │      │
│       └──────────────┴──────┬───────┴──────────────┘      │
│                             │                             │
│                    Unix Socket / HTTP                     │
│                             │                             │
│                             ▼                             │
│                    ┌─────────────────┐                    │
│                    │     fumid       │                    │
│                    │  (Rust Daemon)  │                    │
│                    └────────┬────────┘                    │
│                             │                             │
│          ┌──────────────────┼──────────────────┐         │
│          │                  │                  │         │
│   ┌──────▼──────┐  ┌───────▼───────┐  ┌──────▼──────┐  │
│   │   SQLite    │  │  FTS5 Index   │  │   Vector    │  │
│   │  (メモ本体) │  │ (全文検索)    │  │  Index      │  │
│   │             │  │               │  │ (意味検索)  │  │
│   └─────────────┘  └───────────────┘  └─────────────┘  │
│                                                           │
│                        Local Machine                      │
└──────────────────────────┬────────────────────────────────┘
                           │
                    Background Sync
                    (オンライン時のみ)
                           │
                ┌──────────▼──────────┐
                │    Sync Server      │
                │  (Self-hosted /     │
                │   Cloud)            │
                │                     │
                │  ┌───────────────┐  │
                │  │  PostgreSQL   │  │
                │  │  + pgvector   │  │
                │  └───────────────┘  │
                │  ┌───────────────┐  │
                │  │ Object Store  │  │
                │  │ (S3 / MinIO)  │  │
                │  └───────────────┘  │
                └─────────────────────┘
```

---

## データモデル

### Memo (メモ本体)

```sql
CREATE TABLE memos (
    id          TEXT PRIMARY KEY,  -- ULID (時系列ソート可能)
    title       TEXT,
    body        TEXT NOT NULL,     -- Markdown
    memo_type   TEXT NOT NULL DEFAULT 'note',
                -- 'note' | 'checklist' | 'meeting' | 'task_list'
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TEXT NOT NULL,     -- ISO 8601
    created_by  TEXT NOT NULL,     -- user_id
    updated_at  TEXT NOT NULL,
    updated_by  TEXT NOT NULL,
    deleted_at  TEXT,              -- soft delete
    version     INTEGER NOT NULL DEFAULT 1,
    -- 同期用メタデータ
    sync_id     TEXT,              -- サーバー側ID
    local_dirty BOOLEAN NOT NULL DEFAULT FALSE
);
```

### Tag (タグ)

```sql
CREATE TABLE tags (
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE memo_tags (
    memo_id TEXT NOT NULL REFERENCES memos(id),
    tag_id  TEXT NOT NULL REFERENCES tags(id),
    PRIMARY KEY (memo_id, tag_id)
);
```

### Link (Webページ紐付け)

```sql
CREATE TABLE links (
    id          TEXT PRIMARY KEY,
    memo_id     TEXT NOT NULL REFERENCES memos(id),
    url         TEXT NOT NULL,
    title       TEXT,           -- OGP等から自動取得
    description TEXT,
    favicon_url TEXT,
    created_at  TEXT NOT NULL
);
```

### Attachment (画像・スクショ)

```sql
CREATE TABLE attachments (
    id           TEXT PRIMARY KEY,
    memo_id      TEXT NOT NULL REFERENCES memos(id),
    filename     TEXT NOT NULL,
    mime_type    TEXT NOT NULL,
    size_bytes   INTEGER NOT NULL,
    storage_path TEXT NOT NULL,   -- ローカルファイルパス
    hash         TEXT NOT NULL,   -- SHA-256 (重複排除)
    created_at   TEXT NOT NULL
);
```

### Task (簡易タスク管理)

```sql
CREATE TABLE tasks (
    id          TEXT PRIMARY KEY,
    memo_id     TEXT REFERENCES memos(id),  -- メモから派生したタスク
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'todo',
                -- 'todo' | 'in_progress' | 'done'
    priority    INTEGER DEFAULT 0,
    due_date    TEXT,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
```

### Checklist Item (チェックリスト)

```sql
-- Markdownの `- [ ]` から自動パース＆同期
-- body内のチェックリストとDBの状態を双方向同期
CREATE TABLE checklist_items (
    id          TEXT PRIMARY KEY,
    memo_id     TEXT NOT NULL REFERENCES memos(id),
    text        TEXT NOT NULL,
    is_checked  BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order  INTEGER NOT NULL DEFAULT 0
);
```

### Embedding (意味検索用)

```sql
-- SQLiteではsqlite-vssまたはsqlite-vec拡張を利用
CREATE VIRTUAL TABLE memo_embeddings USING vec0(
    memo_id TEXT PRIMARY KEY,
    embedding FLOAT[384]  -- all-MiniLM-L6-v2等
);
```

---

## fumid — デーモン設計

ローカルで常駐し、すべてのクライアントにサービスを提供する中核コンポーネント。

### 責務

| 機能 | 説明 |
|------|------|
| **API提供** | Unix Domain Socket + HTTP (localhost) でJSON-RPC/REST API |
| **インデックス管理** | FTS5・ベクトルインデックスをバックグラウンド更新 |
| **同期** | オンライン時にSync Serverとバックグラウンド同期 |
| **LLM連携** | Embedding生成、要約、タグ提案 etc. |
| **ファイル監視** | 添付ファイルのハッシュ管理・重複排除 |
| **オフラインバッファ** | オフライン時の変更をキューに蓄積、復帰時に同期 |

### API設計 (JSON-RPC over Unix Socket)

```
── Memo CRUD ──
memo.create    { title?, body, type?, tags?, links? }     → Memo
memo.get       { id }                                     → Memo
memo.update    { id, title?, body?, tags?, is_archived? }  → Memo
memo.delete    { id }                                     → void
memo.archive   { id }                                     → Memo
memo.unarchive { id }                                     → Memo

── Quick Capture (さっと書く) ──
memo.quick     { body, tags? }                            → Memo
memo.clip      { url, comment? }                          → Memo  (Webクリップ)

── 検索 ──
search.fulltext  { query, archived? }                     → Memo[]
search.fuzzy     { query, fields? }                       → Memo[]
search.semantic  { query, limit? }                        → ScoredMemo[]
search.tags      { tags[], op: 'and'|'or' }               → Memo[]

── タグ ──
tag.list                                                  → Tag[]
tag.add        { memo_id, tag }                           → void
tag.remove     { memo_id, tag }                           → void
tag.rename     { old, new }                               → void

── タスク ──
task.list      { status?, memo_id? }                      → Task[]
task.update    { id, status?, priority?, due_date? }       → Task
task.export    { ids[], format: 'markdown'|'json'|'csv' } → string

── 添付ファイル ──
attachment.add    { memo_id, file_path | base64 }         → Attachment
attachment.get    { id }                                  → binary
attachment.delete { id }                                  → void

── LLM ──
llm.summarize     { memo_id }                             → string
llm.suggest_tags  { memo_id }                             → string[]
llm.ask           { query, context_memo_ids? }            → string
llm.embeddings    { memo_id }                             → void  (再生成)

── 同期 ──
sync.status                                               → SyncStatus
sync.force                                                → void
sync.share       { memo_id, user_email }                  → ShareLink

── メタ ──
health                                                    → Status
config.get       { key }                                  → value
config.set       { key, value }                           → void
```

### プロセス管理

```bash
# systemd / launchd で自動起動
# もしくは手動管理
fumid start          # デーモン起動
fumid stop           # 停止
fumid status         # ステータス確認
fumid logs           # ログ表示
```

---

## クライアント設計

### 1. CLI (`fumi`)

ターミナルからのさっとした操作用。パイプ・リダイレクトとも親和。

```bash
# Quick capture — さっと書く
fumi "買い物: 牛乳、卵、パン"
fumi -t shopping "牛乳、卵、パン"          # タグ付き
echo "アイデアメモ" | fumi                  # パイプ入力
fumi --clip https://example.com "良い記事"  # Webクリップ

# 新規作成（エディタ起動）
fumi new                                    # $EDITOR で起動
fumi new --type meeting "週次定例"          # 議事録テンプレート
fumi new --type checklist "引越しTODO"      # チェックリスト

# 検索
fumi search "Rust コンパイラ"               # 全文検索
fumi search --fuzzy "compler"               # fuzzy find
fumi search --semantic "型システムの設計"    # 意味検索
fumi search --tag work --tag urgent         # タグ検索
fumi search --interactive                   # fzf風インタラクティブ

# タグ操作
fumi tag <memo_id> +work +important         # タグ追加
fumi tag <memo_id> -draft                   # タグ削除
fumi tags                                   # タグ一覧

# タスク
fumi tasks                                  # タスク一覧
fumi tasks --status todo                    # フィルタ
fumi task done <task_id>                    # 完了
fumi task export --format markdown          # エクスポート

# アーカイブ
fumi archive <memo_id>
fumi archived                               # アーカイブ一覧

# 添付
fumi attach <memo_id> ./screenshot.png
cat screenshot.png | fumi attach <memo_id>  # パイプでも
# macOS: screencapture → 自動添付
fumi screenshot <memo_id>

# LLM連携
fumi ai summarize <memo_id>
fumi ai tags <memo_id>                      # タグ提案
fumi ai ask "先週の定例で決まったこと"       # メモをコンテキストに質問

# エクスポート（他ツール連携）
fumi export <memo_id> --format markdown     # Markdown
fumi export <memo_id> --format json         # JSON
fumi export <memo_id> --format github-issue # GitHub Issue形式
fumi export <memo_id> --format jira         # Jira形式
# クリップボードに直接
fumi export <memo_id> --format markdown | pbcopy

# 共有
fumi share <memo_id> user@example.com
```

### 2. TUI (`fumi tui`)

Ratatui製のリッチなターミナルUI。

```
┌─ fumi ───────────────────────────────────────────────────────┐
│ 🔍 search: compiler design_                                  │
│ Filter: [all] [notes] [tasks] [meetings] [archived]          │
├──────────────────────┬───────────────────────────────────────┤
│ ▸ Lua型チェッカー設計  │ # Lua型チェッカー設計                 │
│   #typua #compiler    │                                       │
│   2026-02-14 15:30    │ ## アーキテクチャ                      │
│                       │                                       │
│   salsa統合メモ       │ salsa crateを使って                    │
│   #typua #salsa       │ incremental computation を実現...     │
│   2026-02-13 10:00    │                                       │
│                       │ ## データ構造                          │
│   週次定例 2/10       │                                       │
│   #meeting #work      │ ```rust                               │
│   2026-02-10 14:00    │ #[salsa::tracked]                     │
│                       │ struct SourceFile {                    │
│   買い物リスト        │     #[id]                              │
│   #shopping           │     path: PathBuf,                    │
│   2026-02-09 18:00    │     contents: String,                 │
│   ☑ 牛乳 ☐ 卵       │ }                                     │
│                       │ ```                                   │
│                       │                                       │
│                       │ Tags: #typua #compiler #salsa         │
│                       │ Links: https://salsa-rs.github.io/... │
├──────────────────────┴───────────────────────────────────────┤
│ [n]ew [e]dit [t]ag [a]rchive [d]elete [/]search [?]help     │
└──────────────────────────────────────────────────────────────┘
```

**キーバインド設計**:

```toml
# ~/.config/fumi/keymap.toml

[keymap]
preset = "vim"  # "vim" | "emacs" | "custom"

[keymap.normal]
j     = "move_down"
k     = "move_up"
"/"   = "search"
n     = "new_memo"
e     = "edit_memo"
t     = "tag_memo"
a     = "archive_memo"
dd    = "delete_memo"
"C-d" = "scroll_half_down"
"C-u" = "scroll_half_up"
gg    = "go_top"
G     = "go_bottom"
"C-p" = "fuzzy_find"
q     = "quit"

[keymap.insert]
"C-c"   = "cancel"
"C-s"   = "save"
"Esc"   = "normal_mode"

# Emacs preset
[keymap.presets.emacs.normal]
"C-n" = "move_down"
"C-p" = "move_up"
"C-s" = "search"
"C-x C-f" = "new_memo"
```

### 3. Web Client

React + TailwindCSS。fumidのHTTP APIに接続。

**主要画面**:

| 画面 | 説明 |
|------|------|
| **Dashboard** | 最近のメモ、ピン留め、タスクサマリ |
| **Editor** | Markdownエディタ (CodeMirror 6) + リアルタイムプレビュー |
| **Search** | 統合検索 (全文/fuzzy/semantic/tag) |
| **Tasks** | かんばんボード風タスクビュー |
| **Checklist** | チェックリスト特化ビュー |
| **Share** | 共有メモ一覧・権限管理 |

**特徴**:
- PWA対応 → オフラインでも閲覧・編集可能
- ドラッグ&ドロップで画像添付
- Webページ共有API (`navigator.share`) でワンタップクリップ
- CodeMirror 6 ベースのエディタ (vim/emacsモードプラグイン内蔵)

### 4. Neovim Plugin (`fumi.nvim`)

Luaで実装。fumidのUnix Socketに接続。

```lua
-- lazy.nvim
{
  "tkcd/fumi.nvim",
  dependencies = {
    "nvim-telescope/telescope.nvim",  -- 検索UI
    "nvim-lua/plenary.nvim",
  },
  config = function()
    require("fumi").setup({
      -- fumid への接続
      socket_path = vim.fn.expand("~/.local/share/fumi/fumi.sock"),
      -- Telescope統合
      telescope = true,
      -- 自動保存間隔 (ms)
      auto_save_interval = 3000,
      -- デフォルトのメモタイプ
      default_type = "note",
    })
  end,
}
```

**コマンド・機能**:

```vim
" Quick capture — カーソル行 or visual selectionをメモに
:FumiCapture              " 現在行をメモに
:'<,'>FumiCapture         " 選択範囲をメモに
:FumiCapture #tag1 #tag2  " タグ付きキャプチャ

" メモ操作
:FumiNew                  " 新規メモ (バッファで編集)
:FumiNew meeting          " 議事録テンプレート
:FumiEdit <id>            " 既存メモを開く
:FumiList                 " Telescope picker でメモ一覧

" 検索 (Telescope統合)
:FumiSearch               " Telescopeでインタラクティブ検索
:FumiSearch --semantic     " 意味検索モード
:FumiTags                 " タグ一覧 → 選択でフィルタ

" タスク
:FumiTasks                " タスク一覧バッファ
:FumiToggle               " カーソル行のチェックボックスをtoggle

" Webクリップ
:FumiClip https://...     " URLをクリップ

" LLM
:FumiAI summarize         " 現在のメモを要約
:FumiAI suggest-tags      " タグ提案
```

**バッファ統合**: メモをNeovimバッファとして開き、保存(`:w`)でfumidに自動送信。treesitterでMarkdownハイライト。チェックリストアイテムは`<CR>`でtoggle。

---

## 検索エンジン設計

4種類の検索を統合的に提供。

### 全文検索 (Full-text Search)

SQLite FTS5を利用。日本語対応にはICUトークナイザまたはMeCab/Linderaベースのカスタムトークナイザ。

```sql
CREATE VIRTUAL TABLE memos_fts USING fts5(
    title, body, tags,
    content='memos',
    content_rowid='rowid',
    tokenize='unicode61 remove_diacritics 2'
    -- 日本語: カスタムトークナイザ or trigram
);

-- トリガーで自動同期
CREATE TRIGGER memos_ai AFTER INSERT ON memos BEGIN
    INSERT INTO memos_fts(rowid, title, body, tags)
    VALUES (new.rowid, new.title, new.body, ...);
END;
```

### Fuzzy Search

nucleo crate (Neovimのtelescope等でも使われている) を利用。

```rust
use nucleo_matcher::{Matcher, Config};

fn fuzzy_search(query: &str, memos: &[Memo]) -> Vec<ScoredMemo> {
    let mut matcher = Matcher::new(Config::DEFAULT.match_paths());
    memos
        .iter()
        .filter_map(|memo| {
            let score = matcher.fuzzy_match(&memo.title, query)?;
            Some(ScoredMemo { memo, score })
        })
        .sorted_by(|a, b| b.score.cmp(&a.score))
        .collect()
}
```

### 意味検索 (Semantic Search)

ローカルembeddingモデルでベクトル化、sqlite-vecで近傍検索。

```rust
// Embedding生成 (onnxruntime + all-MiniLM-L6-v2)
async fn embed(text: &str) -> Vec<f32> {
    let session = ort::Session::builder()?
        .with_model("models/all-MiniLM-L6-v2.onnx")?
        .build()?;
    // tokenize & infer...
}

// メモ保存時にバックグラウンドでembedding生成
async fn on_memo_saved(memo: &Memo) {
    let embedding = embed(&format!("{} {}", memo.title, memo.body)).await;
    db.execute(
        "INSERT OR REPLACE INTO memo_embeddings(memo_id, embedding) VALUES (?, ?)",
        (memo.id, embedding),
    )?;
}

// 検索
async fn semantic_search(query: &str, limit: usize) -> Vec<ScoredMemo> {
    let query_vec = embed(query).await;
    db.query(
        "SELECT memo_id, distance FROM memo_embeddings
         WHERE embedding MATCH ? ORDER BY distance LIMIT ?",
        (query_vec, limit),
    )
}
```

### タグ検索

```sql
-- AND検索: すべてのタグを持つメモ
SELECT m.* FROM memos m
JOIN memo_tags mt ON m.id = mt.memo_id
JOIN tags t ON mt.tag_id = t.id
WHERE t.name IN ('work', 'urgent')
GROUP BY m.id
HAVING COUNT(DISTINCT t.name) = 2;

-- OR検索: いずれかのタグを持つメモ
SELECT DISTINCT m.* FROM memos m
JOIN memo_tags mt ON m.id = mt.memo_id
JOIN tags t ON mt.tag_id = t.id
WHERE t.name IN ('work', 'urgent');
```

---

## 同期設計

### ローカル → サーバー 同期

CRDT (Conflict-free Replicated Data Types) の簡易版。メモ単位のLast-Write-Wins + 操作ログ。

```rust
// 変更キュー (オフライン時もここに蓄積)
struct ChangeLog {
    id: Ulid,
    memo_id: String,
    operation: Operation,  // Create | Update | Delete | Archive
    payload: serde_json::Value,
    timestamp: DateTime<Utc>,
    device_id: String,
    synced: bool,
}

// 同期フロー
async fn sync_loop(server: &SyncClient, db: &Database) {
    loop {
        if is_online() {
            // 1. ローカルの未同期変更をpush
            let pending = db.get_unsynced_changes().await?;
            for change in pending {
                server.push_change(&change).await?;
                db.mark_synced(change.id).await?;
            }

            // 2. サーバーの新規変更をpull
            let remote = server.pull_changes(db.last_sync_cursor()).await?;
            for change in remote {
                db.apply_remote_change(&change).await?;
            }
        }
        tokio::time::sleep(Duration::from_secs(30)).await;
    }
}
```

### コンフリクト解決

```
基本戦略: Last-Write-Wins (タイムスタンプベース)
追加安全策:
  - 同一メモの同時編集検知 → 両方保持してユーザーに提示
  - 添付ファイルは append-only → コンフリクトなし
  - タグ操作は CRDT set (add-wins) → 自動マージ
```

---

## 共有機能

```
┌───────────┐                    ┌───────────┐
│  User A   │                    │  User B   │
│  (fumid)  │◄──── Sync ────►   │  (fumid)  │
│           │     Server         │           │
└───────────┘                    └───────────┘
                    │
             ┌──────▼──────┐
             │ Share Table  │
             │ memo_id      │
             │ owner_id     │
             │ shared_with  │
             │ permission   │
             │ (read/write) │
             └─────────────┘
```

共有されたメモはSync Server経由で相手のfumidに配信。Webのみの閲覧用共有リンクも生成可能。

---

## LLM連携設計

### プロバイダ抽象化

```toml
# ~/.config/fumi/config.toml

[llm]
# ローカルモデル (Ollama)
provider = "ollama"
model = "llama3.2"
endpoint = "http://localhost:11434"

# または API
# provider = "anthropic"
# model = "claude-sonnet-4-5-20250514"
# api_key_env = "ANTHROPIC_API_KEY"

[llm.embedding]
# Embeddingはローカル推奨 (高速・オフライン対応)
provider = "local"
model = "all-MiniLM-L6-v2"
# model_path = "~/.local/share/fumi/models/"
```

### LLM機能

| 機能 | 説明 | トリガー |
|------|------|----------|
| **自動タグ提案** | メモ保存時にタグを推測 | バックグラウンド |
| **要約生成** | 長いメモのサマリ | 手動 or 自動 |
| **意味検索** | Embeddingベースの類似メモ検索 | 検索時 |
| **質問応答** | メモをコンテキストにした質問 | `fumi ai ask` |
| **タイトル生成** | 無題メモにタイトル提案 | バックグラウンド |
| **テンプレート補完** | 議事録テンプレートの自動補完 | 編集中 |

---

## エクスポート / 外部ツール連携

### フォーマット変換

```rust
trait ExportFormat {
    fn export(&self, memo: &Memo) -> String;
}

struct MarkdownExport;    // そのまま
struct JsonExport;        // 構造化JSON
struct GithubIssue;       // title + body + labels
struct JiraTicket;        // summary + description + labels
struct AsanaTask;         // name + notes + tags
struct LinearIssue;       // title + description + labels
```

### クリップボード統合

```bash
# GitHub Issue形式でクリップボードに
fumi export <id> --format github-issue | pbcopy

# または直接API連携
fumi push <id> --to github --repo owner/repo
fumi push <id> --to jira --project PROJ
```

### Webhook / API連携

```toml
# ~/.config/fumi/integrations.toml

[[webhook]]
name = "github"
url = "https://api.github.com/repos/{owner}/{repo}/issues"
auth = "token"
token_env = "GITHUB_TOKEN"
format = "github-issue"

[[webhook]]
name = "slack"
url = "https://hooks.slack.com/services/..."
format = "slack-message"
```

---

## 設定ファイル

```toml
# ~/.config/fumi/config.toml

[general]
data_dir = "~/.local/share/fumi"
editor = "$EDITOR"          # メモ編集に使うエディタ
default_type = "note"       # デフォルトのメモタイプ

[daemon]
socket_path = "~/.local/share/fumi/fumi.sock"
http_port = 18080           # Web UI用
auto_start = true

[sync]
enabled = true
server_url = "https://fumi.example.com"
interval_secs = 30

[search]
fuzzy_threshold = 0.3       # fuzzy matchの閾値
semantic_enabled = true
fts_language = "japanese"   # 全文検索の言語設定

[ui.tui]
keymap = "vim"              # "vim" | "emacs" | "custom"
theme = "tokyonight"        # TUIのカラーテーマ
preview_width = 60          # プレビューペインの幅 (%)

[ui.web]
theme = "auto"              # "light" | "dark" | "auto"
vim_mode = false            # エディタのvimモード

[llm]
provider = "ollama"
model = "llama3.2"
endpoint = "http://localhost:11434"
auto_tags = true            # 自動タグ提案
auto_title = true           # 自動タイトル生成
auto_summarize = false      # 自動要約 (長いメモのみ)

[llm.embedding]
provider = "local"
model = "all-MiniLM-L6-v2"
```

---

## テンプレートシステム

```markdown
<!-- ~/.config/fumi/templates/meeting.md -->
# {{title}}

**日時**: {{date}}
**参加者**: 
**場所/URL**: 

## アジェンダ
1. 

## 議事内容

## 決定事項
- [ ] 

## アクションアイテム
- [ ] @誰 : 何をいつまでに

---

<!-- ~/.config/fumi/templates/checklist.md -->
# {{title}}

- [ ] 
- [ ] 
- [ ] 
```

---

## Crate / ライブラリ構成

```
fumi/
├── Cargo.toml              (workspace)
├── crates/
│   ├── fumi-core/          # データモデル、ビジネスロジック
│   │   ├── src/
│   │   │   ├── memo.rs
│   │   │   ├── tag.rs
│   │   │   ├── task.rs
│   │   │   ├── search.rs
│   │   │   └── export.rs
│   │   └── Cargo.toml
│   │
│   ├── fumi-db/            # SQLite操作、マイグレーション
│   │   ├── src/
│   │   │   ├── sqlite.rs
│   │   │   ├── fts.rs       # FTS5
│   │   │   ├── vec.rs       # sqlite-vec
│   │   │   └── migrations/
│   │   └── Cargo.toml
│   │
│   ├── fumi-sync/          # 同期プロトコル、コンフリクト解決
│   │   └── Cargo.toml
│   │
│   ├── fumi-llm/           # LLMプロバイダ抽象化
│   │   ├── src/
│   │   │   ├── provider.rs  # trait LlmProvider
│   │   │   ├── ollama.rs
│   │   │   ├── anthropic.rs
│   │   │   ├── openai.rs
│   │   │   └── embedding.rs # ローカルembedding (ort)
│   │   └── Cargo.toml
│   │
│   ├── fumid/              # デーモン本体
│   │   ├── src/
│   │   │   ├── main.rs
│   │   │   ├── api.rs       # JSON-RPC handler
│   │   │   ├── http.rs      # HTTP server (axum)
│   │   │   └── scheduler.rs # バックグラウンドジョブ
│   │   └── Cargo.toml
│   │
│   ├── fumi-cli/           # CLIクライアント
│   │   ├── src/
│   │   │   ├── main.rs
│   │   │   └── commands/
│   │   └── Cargo.toml
│   │
│   └── fumi-tui/           # TUIクライアント
│       ├── src/
│       │   ├── main.rs
│       │   ├── app.rs
│       │   ├── views/
│       │   └── keybind.rs
│       └── Cargo.toml
│
├── fumi-web/               # Web client (React)
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
│
├── fumi-nvim/              # Neovim plugin (Lua)
│   ├── lua/
│   │   └── fumi/
│   │       ├── init.lua
│   │       ├── client.lua   # fumid通信
│   │       ├── commands.lua
│   │       ├── telescope.lua
│   │       └── buffer.lua
│   └── plugin/
│       └── fumi.vim
│
├── fumi-server/            # Sync server (optional)
│   ├── src/
│   │   ├── main.rs
│   │   ├── api.rs
│   │   ├── auth.rs
│   │   └── sync.rs
│   └── Cargo.toml
│
├── nix/                    # Nix flake
│   ├── flake.nix
│   └── flake.lock
│
└── docs/
```

### 主要依存クレート

```toml
# fumid
[dependencies]
axum = "0.8"                  # HTTP server
tokio = { version = "1", features = ["full"] }
rusqlite = { version = "0.32", features = ["bundled", "fts5"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
ulid = "1"
nucleo-matcher = "0.3"        # Fuzzy search
ort = "2"                     # ONNX Runtime (embedding)
notify = "7"                  # File watcher
tracing = "0.1"
toml = "0.8"
```

---

## 要件 ↔ 設計の対応表

| #   | 要件                  | 対応する設計要素                                          |
| --- | ------                | ------------------                                        |
| 1   | ブラウザから          | Web Client (React + PWA)                                  |
| 2   | ターミナルから        | CLI + TUI (Rust)                                          |
| 3   | さっと書きたい        | `fumi "text"` / Quick capture API / Neovim `:FumiCapture` |
| 4   | Webページ紐付け       | Links テーブル + `fumi --clip` + OGP自動取得              |
| 5   | チェックリスト        | checklist_items テーブル + Markdown `- [ ]` 双方向同期    |
| 6   | 議事録                | テンプレートシステム + `--type meeting`                   |
| 7   | 画像・スクショ        | Attachments + D&D (Web) + `fumi attach` (CLI)             |
| 8   | どこでも同じメモ共有  | fumid + SQLite + Sync Server                              |
| 9   | git管理は面倒         | 自動バックグラウンド同期 (明示操作不要)                   |
| 10  | あとから検索          | FTS5 全文検索                                             |
| 11  | あとからタグ付け      | `fumi tag` / Web UI / Neovim コマンド                     |
| 12  | タグで検索            | `search.tags` API + CLIフラグ                             |
| 13  | 意味で検索            | sqlite-vec + ローカルembedding                            |
| 14  | fuzzy find            | nucleo crate + `search.fuzzy` API                         |
| 15  | アーカイブ            | `is_archived` フラグ + archive/unarchive API              |
| 16  | 簡易タスク管理        | Tasks テーブル + TUI/Web かんばんビュー                   |
| 17  | 他タスクツールへ貼付  | ExportFormat trait + `fumi export` + Webhook              |
| 18  | 共有                  | Sync Server + Share API + 閲覧用リンク                    |
| 19  | LLM連携               | fumi-llm crate + Ollama/API プロバイダ                    |
| 20  | 作成者・更新者        | created_at/by, updated_at/by メタデータ                   |
| 21  | オフラインバッファ    | ChangeLog + local_dirty フラグ + キュー                   |
| 22  | vim/emacsキーバインド | keymap.toml + プリセット切替                              |
| 23  | Neovimプラグイン      | fumi.nvim (Lua) + Telescope統合                           |

---

## 実装ロードマップ

### Phase 1 — Core (MVP)
- [ ] fumi-core: データモデル、Memoの CRUD
- [ ] fumi-db: SQLite + マイグレーション + FTS5
- [ ] fumid: デーモン基盤 + Unix Socket API
- [ ] fumi-cli: `fumi "text"`, `fumi new`, `fumi search`, `fumi tag`

### Phase 2 — Rich Clients
- [ ] fumi-tui: Ratatui ベースの TUI (vim/emacs keybind)
- [ ] fumi-web: React Web UI (エディタ、検索、チェックリスト)
- [ ] fumi.nvim: Neovim plugin (capture, edit, search)

### Phase 3 — Search & Intelligence
- [ ] Fuzzy search (nucleo)
- [ ] Semantic search (ort + sqlite-vec)
- [ ] fumi-llm: タグ提案、要約、質問応答

### Phase 4 — Sync & Share
- [ ] fumi-sync: オフラインキュー + 同期プロトコル
- [ ] fumi-server: Sync server + 認証
- [ ] 共有機能 + 閲覧リンク

### Phase 5 — Integrations
- [ ] エクスポート (GitHub, Jira, Asana, Linear)
- [ ] Webhook連携
- [ ] 画像添付 (Object Storage)
- [ ] Nix flake パッケージング
