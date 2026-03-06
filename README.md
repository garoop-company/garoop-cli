# garoop-cli
Garoop向けの公開CLIです。  
用途別に 3 つのバイナリがあります。

- `garoop-cli`: SNS投稿・認証・業務自動化
- `garuchan-cli`: ガルちゃん育成・子育てログ
- `garooptv-cli`: GaroopTVログインURL取得・GraphQL操作

## Quick Start
### 1. インストール（Homebrew）
```bash
brew tap yamashitadaiki/homebrew-tap
brew install garoop-cli
brew install garuchan-cli
brew install garooptv-cli
```

### 1.1 インストール（install.sh / macOS・Linux・Android）
```bash
curl -fsSL https://raw.githubusercontent.com/yamashitadaiki/garoop-cli/main/scripts/install.sh | bash

# 3つすべて入れる
curl -fsSL https://raw.githubusercontent.com/yamashitadaiki/garoop-cli/main/scripts/install.sh | bash -s -- --binary all

# 既存バイナリを上書き
curl -fsSL https://raw.githubusercontent.com/yamashitadaiki/garoop-cli/main/scripts/install.sh | bash -s -- --binary garoop-cli --force

# アンインストール
curl -fsSL https://raw.githubusercontent.com/yamashitadaiki/garoop-cli/main/scripts/install.sh | bash -s -- --uninstall --binary all
```

### 2. 利用可能コマンド確認
```bash
garoop-cli --help
garuchan-cli --help
garooptv-cli --help
```

### 2.1 スマホ（Android/Termux）
```bash
pkg update -y
pkg install -y curl tar

# install.shで自動インストール
curl -fsSL https://raw.githubusercontent.com/yamashitadaiki/garoop-cli/main/scripts/install.sh | bash

garoop-cli --help
```

注:
- スマホ実行は現状 `Android (Termux)` を推奨。
- `auth note login` はブラウザ自動操作のため、スマホよりPC実行を推奨です。
- OAuth認証時に自動でブラウザが開けない場合は `--open-browser=false` を付けてURLを手動で開いてください。

### 3. 最初の動作確認（安全な dry-run）
```bash
garoop-cli auth status
garoop-cli auth verify
garoop-cli x post "本日の業務報告です"
garoop-cli x reply-garoop 1234567890123456789 "いつもありがとうございます！"
garuchan-cli birth --name ガルちゃん --model llama3.2
garooptv-cli gql --query 'query { __typename }'
```

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
garoop-cli note post "記事タイトル" ./article.html --cookie-json ./tokens/note_cookie.json
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

## 公開リリース（メンテナ向け）
```bash
git tag v0.1.0
git push origin v0.1.0
```

上記で GitHub Actions 経由の GoReleaser が走り、GitHub Releases と Homebrew tap が更新されます。
