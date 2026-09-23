package cmd

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopdata"
)

// Garoop Land のユーザー投稿ゲーム。
//   - game submit:  だれでも。ゲーム一式を zip にして kids_api の提出物（GAME）として出す。スタッフが確認する
//   - game publish: スタッフ。確認済みのゲームを garoop-data/public/land/user-games/<id>/ に置き index.json に登録するPRを作る
// 公開URLは https://data.garoop.jp/land/user-games/<id>/index.html。

const (
	landUserGamesDir   = "land/user-games"
	landUserGamesIndex = "land/user-games/index.json"
	landMaxTotalBytes  = 30 * 1024 * 1024
	landMaxFileBytes   = 10 * 1024 * 1024
	landMaxFiles       = 300
)

var (
	landGameID      string
	landTitle       string
	landDescription string
	landCategory    string
	landAuthor      string
	landMinutes     int
	landThumbnail   string
)

var landIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,47}$`)

var landCategories = []string{"board", "puzzle", "story", "strategy", "action", "quiz", "other"}

var landAllowedExt = map[string]bool{
	".html": true, ".htm": true, ".js": true, ".mjs": true, ".css": true, ".json": true, ".txt": true, ".md": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true, ".ico": true,
	".mp3": true, ".ogg": true, ".wav": true, ".m4a": true, ".mp4": true, ".webm": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true,
	".wasm": true, ".glb": true, ".gltf": true, ".bin": true, ".map": true,
}

type landGameEntry struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Author      string `json:"author,omitempty"`
	Minutes     int    `json:"minutes,omitempty"`
	PlayURL     string `json:"playUrl"`
	Thumbnail   string `json:"thumbnail,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

var landCmd = serviceCommand("land", "Garoop Land: 自作ゲームの一覧・投稿", ProfileGaroop)

var landGameCmd = &cobra.Command{
	Use:   "game",
	Short: "ユーザー投稿ゲーム",
}

var landGameListCmd = &cobra.Command{
	Use:   "list",
	Short: "投稿済みゲーム一覧",
	RunE: func(cmd *cobra.Command, args []string) error {
		var games []json.RawMessage
		found, err := garoopdata.FetchJSON(landUserGamesIndex, &games)
		if err != nil {
			return err
		}
		if !found {
			games = []json.RawMessage{}
		}
		return printJSON(games)
	},
}

var landGameSubmitCmd = &cobra.Command{
	Use:   "submit [dir|index.html]",
	Short: "自作HTML5ゲームを投稿（スタッフが確認して公開。既定はdry-run。ログイン必須）",
	Long: `自作ゲームを Garoop Land に投稿します。

- フォルダ（index.html を含む）または単体のHTMLファイルを指定します
- 使えるファイル: HTML/JS/CSS/JSON/画像/音声/フォント/wasm など（合計30MB・300ファイル・1ファイル10MBまで）
- zip にまとめてアップロードし、スタッフが確認して公開します。確認状況は ` + "`submission mine --kind GAME`" + ``,
	Example: `  garoop-cli land game submit ./my-game --title "スペースジャンプ" \
    --description "ガルちゃんが星をジャンプで集めるゲーム" --category action --author "たろう" --minutes 5 \
    --thumbnail ./thumb.webp --execute`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireValue("title", landTitle); err != nil {
			return err
		}
		description, err := readTextOrFile(landDescription)
		if err != nil {
			return err
		}
		if description == "" {
			return fmt.Errorf("--description が必要です（@file 可）")
		}
		if !containsString(landCategories, landCategory) {
			return fmt.Errorf("--category は %s のいずれかで指定してください", strings.Join(landCategories, ", "))
		}
		files, err := landCollectFiles(args[0])
		if err != nil {
			return err
		}
		zipPath, err := landZip(files)
		if err != nil {
			return err
		}
		defer os.Remove(zipPath)
		uploads := []string{zipPath}
		if t := strings.TrimSpace(landThumbnail); t != "" {
			uploads = append(uploads, t)
		}
		meta, _ := json.Marshal(map[string]any{
			"category": landCategory, "author": strings.TrimSpace(landAuthor), "minutes": landMinutes,
			"files": sortedFileKeys(files), "entry": "index.html",
		})
		return submitContent("submitContent", map[string]any{
			"kind":        "GAME",
			"title":       strings.TrimSpace(landTitle),
			"description": description,
			"body":        string(meta),
		}, uploads, "games", landMaxTotalBytes)
	},
}

var landGamePublishCmd = &cobra.Command{
	Use:   "publish [dir|index.html]",
	Short: "[スタッフ] 確認済みのゲームを garoop-data へ公開するPRを作成（既定はdry-run）",
	Long: `確認済み（submission review --approve 済み）のゲームを garoop-data に置くPRを作ります。
マージ後に https://data.garoop.jp/land/user-games/<id>/ で公開されます。
garoop-data は公開リポジトリです。作者の本名など個人情報を入れないでください。`,
	Example: `  garoop-cli land game publish ./my-game --id space-jump --title "スペースジャンプ" \
    --description "ガルちゃんが星をジャンプで集めるゲーム" --category action --author "たろう" --minutes 5 \
    --thumbnail ./thumb.webp`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := strings.TrimSpace(landGameID)
		if !landIDPattern.MatchString(id) {
			return fmt.Errorf("--id は英小文字・数字・ハイフンの3〜48文字で指定してください（例: space-jump）")
		}
		if err := requireValue("title", landTitle); err != nil {
			return err
		}
		description, err := readTextOrFile(landDescription)
		if err != nil {
			return err
		}
		if description == "" {
			return fmt.Errorf("--description が必要です（@file 可）")
		}
		if !containsString(landCategories, landCategory) {
			return fmt.Errorf("--category は %s のいずれかで指定してください", strings.Join(landCategories, ", "))
		}

		files, err := landCollectFiles(args[0])
		if err != nil {
			return err
		}
		base := "public/" + landUserGamesDir + "/" + id + "/"
		changes := make([]garoopdata.FileChange, 0, len(files)+2)
		for _, rel := range sortedFileKeys(files) {
			changes = append(changes, garoopdata.FileChange{Path: base + rel, Content: files[rel]})
		}

		entry := landGameEntry{
			ID:          id,
			Title:       strings.TrimSpace(landTitle),
			Description: description,
			Category:    landCategory,
			Author:      strings.TrimSpace(landAuthor),
			Minutes:     landMinutes,
			PlayURL:     garoopdata.PublicURL(landUserGamesDir + "/" + id + "/index.html"),
			CreatedAt:   time.Now().Format("2006-01-02"),
		}
		if strings.TrimSpace(landThumbnail) != "" {
			ext := strings.ToLower(filepath.Ext(landThumbnail))
			if !map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true}[ext] {
				return fmt.Errorf("--thumbnail は png/jpg/webp/gif を指定してください")
			}
			b, err := os.ReadFile(landThumbnail)
			if err != nil {
				return err
			}
			if len(b) > landMaxFileBytes {
				return fmt.Errorf("サムネイルが大きすぎます")
			}
			changes = append(changes, garoopdata.FileChange{Path: base + "thumbnail" + ext, Content: b})
			entry.Thumbnail = garoopdata.PublicURL(landUserGamesDir + "/" + id + "/thumbnail" + ext)
		}

		p := garoopdata.NewPublisher()
		existing, _, err := p.ReadPublic(landUserGamesIndex)
		if err != nil {
			return err
		}
		var current []landGameEntry
		if len(existing) > 0 {
			if err := json.Unmarshal(existing, &current); err != nil {
				return fmt.Errorf("%s を解釈できません: %w", landUserGamesIndex, err)
			}
		}
		for _, g := range current {
			if g.ID == id {
				return fmt.Errorf("ゲームID %s は既に使われています", id)
			}
		}
		index, err := appendToJSONArray(existing, entry)
		if err != nil {
			return err
		}
		changes = append(changes, garoopdata.FileChange{Path: "public/" + landUserGamesIndex, Content: index})

		title := fmt.Sprintf("land: add user game %s \"%s\"", id, entry.Title)
		body := fmt.Sprintf("garoop-cli `land game publish` で公開する、確認済みのゲームです。\n\n- タイトル: %s\n- 作者: %s\n- カテゴリ: %s\n- ファイル数: %d\n- 公開URL（マージ後）: %s\n\n## レビュー観点\n- 外部サイトへの不審な通信・リダイレクトが無いこと\n- 子ども向けとして不適切な表現が無いこと\n",
			entry.Title, entry.Author, entry.Category, len(files), entry.PlayURL)
		return publishOrDryRun(p, garoopdata.PullRequest{
			Branch:        garoopdata.BranchName("land-game", id),
			Title:         title,
			Body:          body,
			CommitMessage: title,
			Files:         changes,
		}, map[string]any{"game": entry})
	},
}

// landCollectFiles はアップロード対象ファイルを相対パス→内容で返す。
func landCollectFiles(src string) (map[string][]byte, error) {
	info, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	if !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(src))
		if ext != ".html" && ext != ".htm" {
			return nil, fmt.Errorf("単体ファイルの場合は .html を指定してください")
		}
		b, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		if len(b) > landMaxFileBytes {
			return nil, fmt.Errorf("%s が大きすぎます（10MBまで）", src)
		}
		files["index.html"] = b
		return files, nil
	}

	total := 0
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("シンボリックリンクは使えません: %s", rel)
		}
		if d.IsDir() {
			return nil
		}
		if !landAllowedExt[strings.ToLower(filepath.Ext(name))] {
			return fmt.Errorf("この種類のファイルはアップロードできません: %s", rel)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(b) > landMaxFileBytes {
			return fmt.Errorf("%s が大きすぎます（10MBまで）", rel)
		}
		total += len(b)
		files[filepath.ToSlash(rel)] = b
		if len(files) > landMaxFiles {
			return fmt.Errorf("ファイル数が多すぎます（%dまで）", landMaxFiles)
		}
		if total > landMaxTotalBytes {
			return fmt.Errorf("合計サイズが大きすぎます（%dMBまで）", landMaxTotalBytes/1024/1024)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, ok := files["index.html"]; !ok {
		return nil, fmt.Errorf("フォルダ直下に index.html が必要です")
	}
	return files, nil
}

// landZip はゲーム一式を一時ファイルの zip にまとめる。
func landZip(files map[string][]byte) (string, error) {
	f, err := os.CreateTemp("", "garoop-game-*.zip")
	if err != nil {
		return "", err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for _, name := range sortedFileKeys(files) {
		entry, err := w.Create(name)
		if err != nil {
			return "", err
		}
		if _, err := entry.Write(files[name]); err != nil {
			return "", err
		}
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func sortedFileKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func init() {
	for _, c := range []*cobra.Command{landGameSubmitCmd, landGamePublishCmd} {
		c.Flags().StringVar(&landTitle, "title", "", "ゲーム名（必須）")
		c.Flags().StringVar(&landDescription, "description", "", "説明（必須。@file 可）")
		c.Flags().StringVar(&landCategory, "category", "other", strings.Join(landCategories, "|"))
		c.Flags().StringVar(&landAuthor, "author", "", "作者名（ニックネーム推奨）")
		c.Flags().IntVar(&landMinutes, "minutes", 0, "目安プレイ時間（分）")
		c.Flags().StringVar(&landThumbnail, "thumbnail", "", "サムネイル画像（png/jpg/webp/gif）")
	}
	landGamePublishCmd.Flags().StringVar(&landGameID, "id", "", "ゲームID（英小文字・数字・ハイフン。必須）")

	markMutates(landGameSubmitCmd, landGamePublishCmd)
	landGameCmd.AddCommand(landGameListCmd, landGameSubmitCmd, landGamePublishCmd)
	landCmd.AddCommand(landGameCmd)
	rootCmd.AddCommand(landCmd)
}
