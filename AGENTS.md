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
- `garooptv-cli` は GaroopTV の認証URL生成と GraphQL 操作、番組表
- 使えるコマンドは `garoop-cli agent manifest` でJSON取得できる（`mutates: true` は書き込み系）

## Garoopサービス操作の対応表
| ユーザーの依頼例 | コマンド |
|---|---|
| 「ログインして」 | `garoop-cli me` → 未ログインなら `garoop-cli login --email ...`（本人がパスワードを入力）/ Google・LINE 登録なら `garoop-cli login --code` |
| 「今GaroopTVで何やってる？」「明日の番組表」 | `garooptv-cli tv schedule --now` / `tv schedule --date YYYY-MM-DD` |
| 「GaroopTVでこんな番組をやってほしい」 | `garooptv-cli tv propose --title ... --description ...` |
| 「子どもにお手伝いのミッションを出したい」 | `garoop-cli kids family unlock` → `kids family create --title ... --reward-garu ...` |
| 「子どもがミッションできたって」「承認して」 | `kids family report ID` / `kids family review ID --approve` |
| 「ミッション一覧」「このミッションに応募」 | `garoop-cli kids mission list` / `kids mission accept ID` |
| 「この作品をミッションに提出」 | `garoop-cli kids mission submit ID --file ...` |
| 「作ったゲームを Garoop Land に載せたい」 | `garoop-cli land game submit DIR --title ... --description ...` |
| 「スクールの課題を出して」 | `garoop-cli school assignment submit --course-id ... --course-title ... --file ...` |
| 「書いた小説を載せて（音声付きで）」 | `garoop-cli novel template` で下書き → `novel submit --file ... [--audio-dir ...]` |
| 「出したものはどうなった？」 | `garoop-cli submission mine` |
| 「ガルちゃんの画像／動画を作って」 | `garuchan-cli studio image --prompt ...` / `studio video --text ...` |
| 「うちの赤ちゃんと話したい」 | `garuchan-cli baby list` → `baby chat ID "..."` |
| （スタッフ）「届いた提出物を確認して」 | `submission list` → `submission review ID --approve/--reject` |
| （スタッフ）「承認したゲーム/小説を公開して」 | `land game publish` / `novel publish`（garoop-data へPR） |

## Garoopサービス操作の注意
- ログインが必要な操作の前に `garoop-cli me` でセッションを確認する。未ログイン・期限切れ（約24時間で切れる）なら、メール登録のユーザーには `garoop-cli login --email <メール>` を実行する。パスワードは端末の伏せ字入力か macOS のダイアログでユーザー本人が入れる
- Google / LINE で登録したユーザーは `garoop-cli login --code`。ブラウザで「CLIにログイン」ページが開くので、本人が「コードを出す」を押してダイアログに貼る
- パスワード・Cookie・合言葉をチャットで聞かない・受け取らない。コマンド引数や標準入力に渡さない。ユーザーがチャットに貼ってしまったら使わずに、ダイアログで入れ直してもらう
- 閲覧系（`tv schedule`、`kids mission list`、`novel list` など）はログイン不要。まずこれで疎通を確かめてよい
- 提出系はまず dry-run の出力（送る内容・ファイル）をユーザーに見せてから `--execute`
- 保護者の合言葉はコマンド引数に書かない。`kids family unlock` を実行すると保護者本人が入力する（35分有効）。エージェントが合言葉を聞き出す・標準入力に流し込む・保存・表示することはしない
- `kids family review --approve` はガル（おこづかい相当）が動く。ユーザーの明示的な指示がある場合だけ実行する
- スタッフ用コマンド（`submission list/review`、`* publish`、`tv schedule-add`）は `GAROOP_ADMIN_SECRET` を持つスタッフの依頼でのみ使う
- garoop-data は**公開リポジトリ**。PR の内容も公開される。子どもの実名・学校名・住所・顔写真など個人情報を入れない
- 動画・音声生成はローカルの garuchan_creator / VOICEVOX が起動していることが前提
