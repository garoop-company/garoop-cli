# AGENTS.md

このリポジトリでは `garoop-cli` / `garuchan-cli` / `garooptv-cli` を AI エージェント経由で扱う。

## 基本方針
- 直接CLIを手打ちするより、AI エージェントに自然文で依頼して実行させる
- 推奨エージェントは次のどちらか
- 第一推奨は `ChatGPT Plus` にログインして `Codex` で実行するクラウドエージェント
- ローカル完結が必要な場合は `Qwen 3.5` または `Gemma 4` 系をインストールして動かすローカルエージェント

## 実行ルール
- まず `--help`、`auth status`、`auth verify` などの読み取り系コマンドから始める
- 投稿・認証・外部API操作は、明示されない限り `dry-run` を優先する
- 実行系は本当に必要なときだけ `--execute` を使う
- 既定画像は `assets/garuchan.webp` を使う

## Codex向け
- このリポジトリは Codex がインストールと実行の両方を担当する前提で扱ってよい
- バイナリ未導入なら Homebrew、または `scripts/install.sh` による導入を優先する
- 実行前に `README.md` を参照し、推奨手順に沿って進める
- ユーザーが自然文で依頼したら、対応するCLIコマンドに落として実行する
- 最初の確認は `--help`、`auth status`、`auth verify` を優先する
- 書き込み系や外部API実行は、ユーザーの明示がない限り `dry-run` を優先する
- `garoop-cli` は SNS投稿・認証・業務自動化
- `garuchan-cli` は ガルちゃん育成・子育てログ
- `garooptv-cli` は GaroopTV の認証URL生成と GraphQL 操作

## ローカルLLM向け
- 例として `Ollama` 上の `qwen3.5-coder` 系または `gemma4` 系モデルを想定してよい
- リポジトリを開いた状態で、このファイルと `README.md` を参照してコマンドを生成する
