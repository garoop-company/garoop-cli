# garoop-cli
Garoop向けの公開CLIです。  
用途別に 3 つのバイナリがあります。通常の手打ちCLIとしても使えますが、このリポジトリでは AI エージェント経由での利用を基本運用にします。

- `garoop-cli`: SNS投稿・認証・業務自動化
- `garuchan-cli`: ガルちゃん育成・子育てログ
- `garooptv-cli`: GaroopTVログインURL取得・GraphQL操作・番組表

加えて、GaroopTV / Kids / Land / School / Novel / Baby / ガルちゃんスタジオを操作する「Garoopサービス操作」コマンドがあります（後述）。

## 推奨利用形態
- まずは `ChatGPT Plus` にログインして `Codex` からこのリポジトリを開き、AI エージェント経由で `garoop-cli` を実行する形を推奨
- ローカルで完結させたい場合は、代替として `Qwen 3.5` または `Gemma 4` 系をローカルにインストールして使う
- スマホから使いたい場合は、`Android + Termux` で CLI を直接動かすか、`Codex web` に作業を依頼する形が現実的
- 直接コマンドを手で打つより、エージェントに「何をしたいか」を渡して実行させる運用を優先する

## Agent-Ready 方針
このリポジトリは、`Codex` などの AI エージェントがこの CLI をインストールして実行することを前提にしています。

- エージェントはまず `README.md` と `AGENTS.md` を読む
- インストール後は `--help`、`auth status`、`auth verify` などの安全な確認コマンドから始める
- 投稿や外部API操作は、明示されない限り `dry-run` を優先する
- 実行時に必要なバイナリは `garoop-cli`、`garuchan-cli`、`garooptv-cli` の3つ

### エージェント向けインストール契約
エージェントがセットアップするときは、次のどちらかを優先します。

1. Homebrew でインストールする
2. `scripts/install.sh` でインストールする

最小例:
```bash
brew tap garoop-company/homebrew-tap
brew install garoop-cli
brew install garuchan-cli
brew install garooptv-cli
```

補足:
- 上の `brew install ...` は `garoop-company/homebrew-tap` を `brew tap` 済みであることが前提です
- 未 tap の環境では、先に `brew tap garoop-company/homebrew-tap` を実行してください

または:
```bash
curl -fsSL https://raw.githubusercontent.com/garoop-company/garoop-cli/main/scripts/install.sh | bash -s -- --binary all
```

### エージェント向け実行契約
エージェントは次の順で扱う想定です。

1. `garoop-cli --help` などで利用可能コマンドを確認する
2. `garoop-cli auth status` などで現在状態を把握する
3. 投稿や更新はまず `dry-run` で確認する
4. ユーザーの明示があるときだけ `--execute` を付ける

エージェントに依頼する例:
- 「必要ならインストールしてから `garoop-cli auth status` を見て」
- 「まず dry-run で X 投稿文を作って」
- 「`garooptv-cli` で GraphQL の疎通確認をして」

## Agent Quick Start
### 1. CLI本体のインストール（Homebrew）
```bash
brew tap garoop-company/homebrew-tap
brew install garoop-cli
brew install garuchan-cli
brew install garooptv-cli
```

すでに `garoop-company/homebrew-tap` を追加済みなら、次だけでも構いません。

```bash
brew install garoop-cli
brew install garuchan-cli
brew install garooptv-cli
```

Homebrew 配布は GoReleaser 経由で行います。タグ付きリリース後に `garoop-company/homebrew-tap` の Formula が更新される前提です。

### 1.1 CLI本体のインストール（install.sh / macOS・Linux・Android）
```bash
curl -fsSL https://raw.githubusercontent.com/garoop-company/garoop-cli/main/scripts/install.sh | bash

# 3つすべて入れる
curl -fsSL https://raw.githubusercontent.com/garoop-company/garoop-cli/main/scripts/install.sh | bash -s -- --binary all

# 既存バイナリを上書き
curl -fsSL https://raw.githubusercontent.com/garoop-company/garoop-cli/main/scripts/install.sh | bash -s -- --binary garoop-cli --force

# アンインストール
curl -fsSL https://raw.githubusercontent.com/garoop-company/garoop-cli/main/scripts/install.sh | bash -s -- --uninstall --binary all
```

### 2. まず推奨する使い方: ChatGPT Plus + Codex
`ChatGPT Plus` を使っているユーザーが増えているため、最初の選択肢としては `Codex` を推奨します。はじめてでも順番どおりに進めれば使えます。

#### 2.1 Codex のインストール
```bash
npm i -g @openai/codex
codex --version
```

#### 2.2 Codex にログインするまでの手順
1. `ChatGPT Plus` が有効な OpenAI アカウントを用意する
2. ターミナルで次を実行する

```bash
codex --login
```

3. ブラウザが開いたら、`ChatGPT Plus` のアカウントでサインインする
4. 画面の案内に従って `Sign in with ChatGPT` を完了する
5. ターミナルに戻り、必要なら次でログイン状態を確認する

```bash
codex
```

補足:
- メールアドレスやパスワードを CLI 引数で渡す必要はありません
- 認証はブラウザで行われるので、難しい設定を手で書かなくて大丈夫です
- うまくいかない場合は、いったん `codex logout` を実行してから `codex --login` をやり直してください

#### 2.3 このリポジトリで実行する方法
このリポジトリのディレクトリに移動してから `codex` を起動します。

```bash
cd /Users/yamashitadaiki/git_work/garoop-cli
codex
```

起動後は、自然文でそのまま依頼して構いません。専門用語を知らなくても問題ありません。

依頼例:
- 「`garoop-cli auth status` を確認して」
- 「安全な dry-run で X 投稿文を3案作って」
- 「`garooptv-cli gql --query 'query { __typename }'` を実行して」

### 3. ローカルAIエージェントで使う場合（Qwen 3.5 / Gemma 4）
例: `Ollama` に `Qwen 3.5` または `Gemma 4` 系を入れて、ローカルのエージェントからこのリポジトリを操作する

```bash
ollama pull qwen3.5-coder:14b
# 例: Gemma 4 系を使う場合
ollama pull gemma4:e4b
ollama list
garoop-cli --help
```

モデル選択の目安:
- コード生成寄りなら `qwen3.5-coder` 系
- 軽めにローカル実行したいなら `gemma4:e2b` や `gemma4:e4b` が候補
- より重いローカル環境なら `gemma4:26b` や `gemma4:31b` も候補
- 実際に使うタグ名は `ollama pull gemma4:e4b` のように明示指定する運用を推奨します

エージェントへの依頼例:
- 「`garoop-cli` で X の下書きを作って dry-run して」
- 「`garuchan-cli` で今日の育児ログを確認して」
- 「`garooptv-cli` で GraphQL の疎通確認をして」

### 4. 利用可能コマンド確認
```bash
garoop-cli --help
garuchan-cli --help
garooptv-cli --help
```

### 4.1 スマホで使う方法
スマホからでも使えます。難しい操作を最初から全部覚える必要はありません。まずは次のどちらかを選べば十分です。

- `Android + Termux`: スマホ上で `garoop-cli` を直接動かしたい人向け
- `Codex web`: スマホのブラウザから AI に作業を依頼したい人向け

#### Android / Termux で直接実行する
```bash
pkg update -y
pkg install -y curl tar

# install.shで自動インストール
curl -fsSL https://raw.githubusercontent.com/garoop-company/garoop-cli/main/scripts/install.sh | bash

garoop-cli --help
```

注:
- スマホ実行は現状 `Android (Termux)` を推奨。
- `auth note login` はブラウザ自動操作のため、スマホよりPC実行を推奨です。
- OAuth認証時に自動でブラウザが開けない場合は `--open-browser=false` を付けてURLを手動で開いてください。

#### スマホのブラウザから Codex を使う
`ChatGPT Plus` を使っている場合は、`Codex web` に作業を依頼する運用もできます。スマホではこちらのほうが入りやすい場合があります。

基本の流れ:
1. `ChatGPT Plus` でサインインする
2. `Codex web` を開く
3. 必要に応じて GitHub を接続する
4. このリポジトリに対して自然文で作業を依頼する

依頼例:
- 「`garoop-cli` の認証状態を確認したい」
- 「X投稿の下書きを dry-run 前提で作って」
- 「README のスマホ向け説明を改善して」

### 5. 最初の動作確認（安全な dry-run）
```bash
garoop-cli auth status
garoop-cli auth verify
garoop-cli x post "本日の業務報告です"
garoop-cli x reply-garoop 1234567890123456789 "いつもありがとうございます！"
garuchan-cli birth --name ガルちゃん --model llama3.2
garooptv-cli gql --query 'query { __typename }'
```

AI エージェントに依頼する場合の基本方針:
- まず `--help` や `auth status` で状況確認をさせる
- 投稿・認証・API操作は原則 `dry-run` から始める
- 実APIを叩く必要があるときだけ `--execute` を付ける

## デフォルト連携アカウント
- X: `@garoop_company` (`https://x.com/garoop_company`)
- Instagram: `@garuchan_wakuwaku` (`https://www.instagram.com/garuchan_wakuwaku/`)

上記はデフォルト値としてCLIに設定済みです。必要なら上書きできます。
```bash
garoop-cli --x-account your_x_account --instagram-account your_instagram_account x post "テスト投稿"
```

## よく使うコマンド
### `garoop-cli`
```bash
garoop-cli x post "本日の業務報告です"
garoop-cli x reply 1234567890 "返信します"
garoop-cli x reply-garoop 1234567890 "Garoop宛ての返信です"
garoop-cli instagram post "イベント情報です"
garoop-cli instagram comment MEDIA_ID "コメントします"
garoop-cli instagram like MEDIA_ID
garoop-cli youtube upload ./movie.mp4 "動画タイトル" --description "動画説明"
garoop-cli youtube comment VIDEO_ID "コメントします"
garoop-cli youtube auto-reply --channel-id UCVXDkfy7aD08L7y7JK1AtmA --max-replies 5
garoop-cli note post "記事タイトル" ./article.html
garoop-cli stocks order AAPL 1 --side buy
garoop-cli gemini login
garoop-cli gemini prompt "ガルちゃん育成の投稿文を3案作って"
garoop-cli gemini exec -- --help
```

### `garuchan-cli`
```bash
garuchan-cli birth --name ガルちゃん --model llama3.2
garuchan-cli status
garuchan-cli feed instagram --limit 5
garuchan-cli feed x --limit 5
garuchan-cli feed youtube --channel-id UCVXDkfy7aD08L7y7JK1AtmA --limit 5
garuchan-cli feed note --username garoop_company --limit 5
garuchan-cli context build --source-root /Users/yamashitadaiki/git_work/garoop_top
garuchan-cli parenting log sleep "昼寝" --child 花子 --minutes 90
garuchan-cli parenting today
```

### `garooptv-cli`
```bash
garooptv-cli auth-url --provider google --redirect-url https://create.garoop.jp
garooptv-cli auth-url --provider line --redirect-url https://create.garoop.jp
garooptv-cli auth-url --provider facebook --redirect-url https://create.garoop.jp
garooptv-cli auth-url --provider tiktok --redirect-url https://create.garoop.jp
garooptv-cli auth-url --provider x --redirect-url https://create.garoop.jp
garooptv-cli session-set-cookie --cookie "_session=...; ..."
garooptv-cli gql --query-file ./query.graphql --variables-file ./vars.json
garooptv-cli social auth-url --platform instagram --redirect-url https://create.garoop.jp
garooptv-cli social connections
garooptv-cli social instagram-media --limit 10
garooptv-cli social instagram-resolve --input https://www.instagram.com/p/ABC123/
garooptv-cli social x-debug --tweet-id 1234567890
garooptv-cli social x-reply --tweet-id 1234567890 --text "ありがとうございます"
garooptv-cli social youtube-comment --video-id abc123 --text "ナイス動画です"

# 実行系は既定で dry-run。実行するときだけ --execute を付ける
garooptv-cli social x-like --tweet-id 1234567890 --execute
```

## Garoopサービス操作（AIエージェント向け）
Garoop の各サービスを AI エージェントから操作するコマンド群です。`garoop-cli --help` の「Garoopサービス操作」に表示されます。

| サービス | コマンド | できること |
|---|---|---|
| 共通 | `me` / `agent manifest` | ログイン確認 / 全コマンドのJSONマニフェスト |
| GaroopTV | `tv channels` `tv schedule` `tv propose` | 番組表の閲覧（今放送中の番組も）、扱ってほしいコンテンツの提案 |
| おうちミッション | `kids family children/unlock/create/list/report/review/delete` | 保護者が子どもにやってほしいことを登録 → 子どもが報告 → 保護者が承認してごほうびのガル |
| Garoop Kids | `kids mission list/get/accept/accepted/submit` `kids lesson list/branches/clear/comment` | ミッション閲覧・応募・成果物提出、レッスン |
| Garoop Land | `land game list` `land game submit` | 自作HTML5ゲームの投稿 |
| Garoop School | `school assignment submit/list` `school contact` | 課題提出と確認状況、問い合わせ |
| Garoop Novel | `novel list/get/template/submit` | 自作小説の投稿（音声付きも可：`--audio-dir`） |
| 提出物 | `submission mine` | 課題・作品・小説・番組提案の確認状況 |
| Garoop Baby | `baby list/public/get/chat/grow` | オンラインの赤ちゃんとの会話・育成 |
| ガルちゃんスタジオ | `studio image/text/video/voice/upload/youtube-upload` | ガルちゃん画像・口パク動画・音声・文章の生成 |
| スタッフ用 | `submission list/review` `land game publish` `novel publish` `tv schedule-add` `studio youtube-upload` | 提出物の確認・承認、承認済み作品の公開（garoop-data へPR）、番組表の編集 |

表示されるバイナリ: `tv` は `garoop-cli` と `garooptv-cli`、`baby` / `studio` は `garoop-cli` と `garuchan-cli`、それ以外は `garoop-cli`。

### はじめての準備（ユーザー本人がやること）
番組表・ミッション一覧・小説一覧などの閲覧は、インストールだけで使えます。ログインが要るのは提出・おうちミッション・赤ちゃんとの会話などです。

1. CLI を入れる（上の Homebrew か install.sh。エージェントに頼んでもよい）
2. ブラウザで garoop.jp にログインし、開発者ツール → Application（Safari はストレージ）→ Cookie → `sessionId` の値をコピー
3. **自分の端末で** 次を実行して貼り付ける
   ```bash
   garoop-cli session-set-cookie   # 値を貼り付けて Enter
   garoop-cli me                   # ログイン確認
   ```
4. おうちミッションを使う保護者は、使うたびに **自分の端末で** `garoop-cli kids family unlock` を実行して合言葉を入れる（35分有効）

Cookie と合言葉はパスワードと同じ扱いです。AI エージェントのチャットに貼ったり、コマンド引数に書いたりしないでください。
エージェントの中から本人が入力するには、Claude Code ならプロンプトで `! garoop-cli session-set-cookie`、Codex なら別のターミナルで実行します。
ログイン情報は `~/.config/garoop-cli/tokens/` に保存されるので、どのディレクトリからエージェントを動かしても同じログインを使えます（`GAROOP_CLI_TOKEN_DIR` で変更可。以前の `./tokens/` があればそちらを優先）。
ログインが切れたら `me` がそう伝えるので、2〜3 をやり直します。

### 書き込みの仕組み
- すべての書き込み系は既定で dry-run。`--execute` を付けたときだけ実行します
- **ログイン**: ブラウザでログイン後の `sessionId` Cookie を保存して使います（後述「はじめての準備」）
- **提出物（課題・ミッション成果物・ゲーム・小説・番組提案）**: api.garoop.jp の提出受付に送られ、スタッフが確認します。ファイルは自分専用の領域（`uploads/users/<userId>/`）にアップロードされます
- **おうちミッション**: Garoop Pay で子どもを登録済みであることが前提です。登録・承認・削除には保護者の合言葉が必要です（`kids family unlock`。35分有効。`GAROOP_PARENT_PASSCODE` か標準入力で渡し、引数では渡しません）
- **スタッフ用**: `GAROOP_ADMIN_SECRET`（kids_api の `GRAPHQL_ADMIN_SECRET`）が必要です。公開（garoop-data へのPR）には `gh auth login` か `GITHUB_TOKEN` も必要です。garoop-data は公開リポジトリなので、子どもの個人情報を含めないでください
- **動画・音声生成**: ローカルの `garuchan_creator`（`make up`）と VOICEVOX を使います。MP3 変換には `ffmpeg` が必要です

### 例
```bash
# 番組表: 今なにやってる？
garooptv-cli tv schedule --now
# 番組を提案
garooptv-cli tv propose --title "宇宙飛行士の1日" --description "ISSでの生活を子ども向けに" --channel 1 --execute

# おうちミッション（保護者）
garoop-cli kids family unlock                    # 合言葉を入力
garoop-cli kids family create --title "くつをそろえよう" --detail "げんかんのくつを全部そろえる" \
  --reward-garu 30 --reward-note "シール" --due 2026-10-31 --execute
# 子どもが報告 → 保護者が承認（ガルが渡る）
garoop-cli kids family report 6 --comment "そろえたよ" --photo ./kutsu.jpg --execute
garoop-cli kids family review 6 --approve --comment "えらい！" --execute

# ミッションの成果物を提出（承認されるとポイント）
garoop-cli kids mission submit 20 --file ./work.png --execute

# 自作ゲームを投稿（フォルダ直下に index.html。zip にまとめて送信）
garoop-cli land game submit ./my-game --title "スペースジャンプ" \
  --description "ガルちゃんが星を集めるゲーム" --category action --author たろう --execute

# 課題提出（複数ファイル可）
garoop-cli school assignment submit --course-id ai-startup-01 --course-title "AI起業 第1回" \
  --file ./pitch.pdf --file ./demo.mp4 --execute

# 小説を投稿（音声付き）
garoop-cli novel template > draft.json
garoop-cli novel submit --file draft.json --audio-dir ./voices --execute

# 確認状況
garoop-cli submission mine

# ガルちゃん画像・動画
garuchan-cli studio image --prompt "ガルちゃんが宇宙服で月面ジャンプ" --out moon.png --execute
garuchan-cli studio video --text "こんにちは！ガルちゃんだよ" --out hello.mp4

# スタッフ: 確認して承認
GAROOP_ADMIN_SECRET=... garoop-cli submission list --kind GAME
GAROOP_ADMIN_SECRET=... garoop-cli submission review 12 --approve --comment "公開します" --execute
garoop-cli land game publish ./my-game --id space-jump --title "スペースジャンプ" --description "..." --execute
```

### 追加の環境変数
```bash
export GAROOP_GRAPHQL_ENDPOINT=https://api.garoop.jp/query  # 既定値
export GAROOP_COOKIE="sessionId=..."                      # session-set-cookie の代わり（CI など）
export GAROOP_CLI_TOKEN_DIR=~/.config/garoop-cli/tokens    # ログイン情報の保存先（既定値）
export GAROOP_PARENT_PASSCODE=...                         # kids family unlock 用（任意）
export GAROOP_ADMIN_SECRET=...                            # スタッフ用
export GAROOP_DATA_BASE_URL=https://data.garoop.jp
export GAROOP_DATA_REPO=garoop-company/garoop-data         # スタッフの公開用
export GITHUB_TOKEN=...                                   # gh auth login 済みなら不要
export GARUCHAN_CREATOR_URL=http://localhost:3000
export VOICEVOX_ENGINE_URL=http://127.0.0.1:50021
```

## 認証セットアップ
### X OAuth1.0a
```bash
export X_CONSUMER_KEY=...
export X_CONSUMER_SECRET=...
garoop-cli auth x login --redirect-uri http://127.0.0.1:18766/auth/x/callback
garoop-cli auth verify --online
```

### YouTube OAuth2
```bash
export YOUTUBE_CLIENT_ID=...
export YOUTUBE_CLIENT_SECRET=...
garoop-cli auth youtube login --redirect-uri http://127.0.0.1:18767/auth/youtube/callback
garoop-cli auth youtube refresh

## OSS公開・配布
- GitHub Releases、Homebrew、install.sh で配布しています
- リリース手順は `docs/OSS_RELEASE.md:1` にまとめています
- ローカル確認は `make build`、タグ付き配布前の確認は `make release-check` を使えます
```

### Instagram OAuth
```bash
export INSTAGRAM_APP_ID=...
export INSTAGRAM_APP_SECRET=...
garoop-cli auth instagram login --redirect-uri http://127.0.0.1:18768/auth/instagram/callback
```

`auth instagram login` 後に `--execute` で実行すれば、認証アカウント（Business/Creator + Facebookページ連携）の投稿取得や投稿処理が可能です。

### Note
```bash
garoop-cli auth note login
# メール/パスワード指定も可能
garoop-cli auth note login --email "you@example.com" --password "your-password"
```

## 実行時のポイント
- デフォルトは `dry-run` です。実API実行時のみ `--execute` を付けてください。
- デフォルト画像は `assets/garuchan.webp` です（`--garuchan-image` で変更可）。
- X/YouTube投稿時はデフォルトで `#ガルちゃん #子供起業 #Garoop` を付与します。
- 株式取引は最初に `ALPACA_BASE_URL=https://paper-api.alpaca.markets` を推奨します。

## 環境変数
```bash
# X
export X_CONSUMER_KEY=...
export X_CONSUMER_SECRET=...
export X_ACCESS_TOKEN=...
export X_ACCESS_TOKEN_SECRET=...

# Instagram
export INSTAGRAM_ACCESS_TOKEN=...
export INSTAGRAM_IG_USER_ID=...

# YouTube
export YOUTUBE_ACCESS_TOKEN=...
export YOUTUBE_OWN_CHANNEL_ID=...

# Stocks (Alpaca)
export ALPACA_API_KEY=...
export ALPACA_API_SECRET=...
export ALPACA_BASE_URL=https://paper-api.alpaca.markets
```

## Open Source
このリポジトリはオープンソースとして公開できます。ライセンスは `MIT` です。

公開利用時の注意:
- APIキー、アクセストークン、Cookie、パスワードはリポジトリに含めないでください
- 認証情報は環境変数やローカルの安全な保存先で管理してください
- 外部サービス連携に必要な `X`、`YouTube`、`Instagram`、`Alpaca` などの認証情報は各利用者が用意してください
- 各サービスのAPI利用規約とレート制限を確認してから実運用してください

## 公開リリース（メンテナ向け）
```bash
git tag v0.1.0
git push origin v0.1.0
```

上記で GitHub Actions 経由の GoReleaser が走り、GitHub Releases と Homebrew tap が更新されます。
