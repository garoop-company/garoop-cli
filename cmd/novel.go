package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopdata"
	"github.com/yamashitadaiki/garoop-cli/internal/voice"
)

// Garoop Novel（novel.garoop.jp）。本文は garoop-data/public/novel 配下の静的JSON。
// 規約は garoop-data/.claude/skills/garoop-novel-content/SKILL.md に準拠する。
//   - submit:  だれでも。kids_api の提出物（NOVEL）として出す。スタッフが確認する
//   - publish: スタッフ。確認済みの小説を garoop-data に載せるPRを作る

const (
	novelIndexPath   = "novel/novels.json"
	novelChapterDir  = "novel/chapters"
	novelAudioDir    = "novel/audio"
	novelMaxAudioMiB = 8
)

var (
	novelLang      string
	novelSeries    string
	novelFile      string
	novelAudioSrc  string
	novelVoicevox  bool
	novelSpeaker   int
	novelAudioRate string
)

var novelSeriesPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,47}$`)

var novelAudioExt = map[string]bool{".mp3": true, ".m4a": true, ".ogg": true, ".wav": true}

// novelDraft は `novel publish --file` の入力形式。
type novelDraft struct {
	SeriesKey       string   `json:"seriesKey"`
	EpisodeNumber   int      `json:"episodeNumber,omitempty"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Category        string   `json:"category"`
	Keywords        any      `json:"keywords"`
	Lang            string   `json:"lang,omitempty"`
	AnimationPreset string   `json:"animationPreset,omitempty"`
	Pages           []string `json:"pages"`
	CompanionLines  []string `json:"companionLines"`
}

// novelEntry は novels.json の1要素（既存と同じキー順）。
type novelEntry struct {
	ID              string   `json:"id"`
	SeriesKey       string   `json:"seriesKey"`
	EpisodeNumber   int      `json:"episodeNumber"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Category        string   `json:"category"`
	ChapterFile     string   `json:"chapterFile"`
	PageCount       int      `json:"pageCount"`
	AnimationPreset string   `json:"animationPreset"`
	Keywords        string   `json:"keywords"`
	Lang            string   `json:"lang"`
	CreatedAt       string   `json:"createdAt"`
	AudioUrls       []string `json:"audioUrls,omitempty"`
}

type novelChapter struct {
	ID             string   `json:"id"`
	Pages          []string `json:"pages"`
	CompanionLines []string `json:"companionLines"`
	AudioUrls      []string `json:"audioUrls,omitempty"`
}

var novelCmd = serviceCommand("novel", "Garoop Novel: 小説の閲覧・投稿（音声付きも可）", ProfileGaroop)

var novelListCmd = &cobra.Command{
	Use:   "list",
	Short: "小説一覧",
	RunE: func(cmd *cobra.Command, args []string) error {
		var entries []map[string]any
		if _, err := garoopdata.FetchJSON(novelIndexPath, &entries); err != nil {
			return err
		}
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			if novelLang != "" && fmt.Sprint(e["lang"]) != novelLang {
				continue
			}
			if novelSeries != "" && fmt.Sprint(e["seriesKey"]) != novelSeries {
				continue
			}
			out = append(out, e)
		}
		return printJSON(out)
	},
}

var novelGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "小説の本文を取得",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var entries []map[string]any
		if _, err := garoopdata.FetchJSON(novelIndexPath, &entries); err != nil {
			return err
		}
		for _, e := range entries {
			if fmt.Sprint(e["id"]) != args[0] {
				continue
			}
			chapter := map[string]any{}
			if file, _ := e["chapterFile"].(string); file != "" {
				if _, err := garoopdata.FetchJSON(novelChapterDir+"/"+file, &chapter); err != nil {
					return err
				}
			}
			return printJSON(map[string]any{"novel": e, "chapter": chapter})
		}
		return fmt.Errorf("小説 %s が見つかりません", args[0])
	},
}

var novelTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "novel publish --file 用の入力テンプレートを表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		return printJSON(novelDraft{
			SeriesKey:      "yamashita-garoop",
			Title:          "○○ ― サブタイトル",
			Description:    "1〜2文の引き（140字以内目安）",
			Category:       "人物列伝",
			Keywords:       "キーワード1, キーワード2",
			Lang:           "ja",
			Pages:          []string{"1ページ目の本文（400〜600字）", "…（基本8ページ）"},
			CompanionLines: []string{"1ページ目へのガルちゃんのひとこと（15〜30字）", "…（pagesと同数）"},
		})
	},
}

var novelSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "自作小説を投稿（スタッフが確認して公開。音声はオプション。既定はdry-run。ログイン必須）",
	Long: `小説を Garoop Novel に投稿します。入力は JSON ファイルです（形式は ` + "`novel template`" + ` で確認）。
音声を付けるときは --audio-dir にページ順の音声ファイル（mp3/m4a/ogg/wav、ファイル名順、ページ数と同数・19個まで）を置きます。
VOICEVOX で読み上げを作るなら、先に ` + "`studio voice`" + ` で音声ファイルを作ってください。
確認状況は ` + "`submission mine --kind NOVEL`" + ` で見られます。`,
	Example: `  garoop-cli novel template > draft.json
  garoop-cli novel submit --file draft.json --audio-dir ./voices --execute`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireValue("file", novelFile); err != nil {
			return err
		}
		raw, err := os.ReadFile(novelFile)
		if err != nil {
			return err
		}
		var draft novelDraft
		if err := json.Unmarshal(raw, &draft); err != nil {
			return fmt.Errorf("入力JSONを解釈できません: %w", err)
		}
		if err := validateNovelDraft(&draft); err != nil {
			return err
		}
		audio := []string{}
		if novelAudioSrc != "" {
			if audio, err = novelAudioFiles(novelAudioSrc, len(draft.Pages)); err != nil {
				return err
			}
			if len(audio) > 19 {
				return fmt.Errorf("音声ファイルは19個までです")
			}
		}
		body, err := json.Marshal(draft)
		if err != nil {
			return err
		}
		return submitContent("submitContent", map[string]any{
			"kind":        "NOVEL",
			"title":       draft.Title,
			"description": draft.Description,
			"targetId":    draft.SeriesKey,
			"targetTitle": draft.Category,
			"body":        string(body),
		}, audio, "novels", novelMaxAudioMiB*1024*1024)
	},
}

var novelPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "[スタッフ] 確認済みの小説を garoop-data へ公開するPRを作成（音声はオプション。既定はdry-run）",
	Long: `小説を Garoop Novel に投稿します。入力は JSON ファイルです（形式は ` + "`novel template`" + ` で確認）。

音声（オプション）:
  --audio-dir DIR   ページ順に並ぶ音声ファイル（mp3/m4a/ogg/wav、ファイル名順）をページ数と同数用意
  --voicevox        各ページを VOICEVOX で読み上げ生成し MP3 化（VOICEVOX エンジンと ffmpeg が必要）

音声URLは chapters/<id>.json と novels.json の audioUrls に入ります。`,
	Example: `  garoop-cli novel template > draft.json
  garoop-cli novel publish --file draft.json
  garoop-cli novel publish --file draft.json --voicevox --speaker 3 --execute`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireValue("file", novelFile); err != nil {
			return err
		}
		if novelAudioSrc != "" && novelVoicevox {
			return fmt.Errorf("--audio-dir と --voicevox は同時に指定できません")
		}
		raw, err := os.ReadFile(novelFile)
		if err != nil {
			return err
		}
		var draft novelDraft
		if err := json.Unmarshal(raw, &draft); err != nil {
			return fmt.Errorf("入力JSONを解釈できません: %w", err)
		}
		if err := validateNovelDraft(&draft); err != nil {
			return err
		}

		p := garoopdata.NewPublisher()
		existing, found, err := p.ReadPublic(novelIndexPath)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("%s を取得できません", novelIndexPath)
		}
		var entries []novelEntry
		if err := json.Unmarshal(existing, &entries); err != nil {
			return fmt.Errorf("novels.json を解釈できません: %w", err)
		}
		maxEpisode := 0
		for _, e := range entries {
			if e.SeriesKey == draft.SeriesKey && e.EpisodeNumber > maxEpisode {
				maxEpisode = e.EpisodeNumber
			}
		}
		episode := draft.EpisodeNumber
		if episode <= 0 {
			episode = maxEpisode + 1
		}
		id := fmt.Sprintf("%s-%03d", draft.SeriesKey, episode)
		for _, e := range entries {
			if e.ID == id {
				return fmt.Errorf("小説ID %s は既に存在します（episodeNumber を変えてください）", id)
			}
		}

		entry := novelEntry{
			ID:              id,
			SeriesKey:       draft.SeriesKey,
			EpisodeNumber:   episode,
			Title:           draft.Title,
			Description:     draft.Description,
			Category:        draft.Category,
			ChapterFile:     id + ".json",
			PageCount:       len(draft.Pages),
			AnimationPreset: draft.AnimationPreset,
			Keywords:        novelKeywords(draft.Keywords),
			Lang:            draft.Lang,
			CreatedAt:       time.Now().Format("2006-01-02"),
		}
		chapter := novelChapter{ID: id, Pages: draft.Pages, CompanionLines: draft.CompanionLines}

		changes := []garoopdata.FileChange{}
		audioPlan, audioFiles, err := novelAudio(id, draft.Pages)
		if err != nil {
			return err
		}
		if len(audioPlan) > 0 {
			entry.AudioUrls = audioPlan
			chapter.AudioUrls = audioPlan
			changes = append(changes, audioFiles...)
		}

		chapterJSON, err := garoopdata.MarshalPretty(chapter)
		if err != nil {
			return err
		}
		index, err := appendToJSONArray(existing, entry)
		if err != nil {
			return err
		}
		changes = append([]garoopdata.FileChange{
			{Path: "public/" + novelChapterDir + "/" + entry.ChapterFile, Content: chapterJSON},
			{Path: "public/" + novelIndexPath, Content: index},
		}, changes...)

		title := fmt.Sprintf("novel: add %s \"%s\"", id, draft.Title)
		body := fmt.Sprintf("garoop-cli `novel publish` で公開する、確認済みの小説です。\n\n- シリーズ: %s（第%d話）\n- カテゴリ: %s\n- ページ数: %d\n- 音声: %s\n",
			draft.SeriesKey, episode, draft.Category, len(draft.Pages), novelAudioLabel(len(audioPlan)))
		extra := map[string]any{"novel": entry}
		if novelVoicevox && !executeMode {
			extra["audioNote"] = "dry-run では音声合成を行いません。--execute 時に VOICEVOX で生成します"
		}
		return publishOrDryRun(p, garoopdata.PullRequest{
			Branch:        garoopdata.BranchName("novel", id),
			Title:         title,
			Body:          body,
			CommitMessage: title,
			Files:         changes,
		}, extra)
	},
}

func validateNovelDraft(d *novelDraft) error {
	d.SeriesKey = strings.TrimSpace(d.SeriesKey)
	d.Title = strings.TrimSpace(d.Title)
	d.Description = strings.TrimSpace(d.Description)
	d.Category = strings.TrimSpace(d.Category)
	if !novelSeriesPattern.MatchString(d.SeriesKey) {
		return fmt.Errorf("seriesKey は英小文字・数字・ハイフンで指定してください（例: yamashita-garoop）")
	}
	if d.Title == "" || d.Description == "" || d.Category == "" {
		return fmt.Errorf("title / description / category は必須です")
	}
	if d.Lang == "" {
		d.Lang = "ja"
	}
	if d.AnimationPreset == "" {
		d.AnimationPreset = "book-classic"
	}
	if len(d.Pages) == 0 {
		return fmt.Errorf("pages が空です")
	}
	if len(d.Pages) != len(d.CompanionLines) {
		return fmt.Errorf("pages（%d）と companionLines（%d）は同数にしてください", len(d.Pages), len(d.CompanionLines))
	}
	for i := range d.Pages {
		d.Pages[i] = strings.TrimSpace(d.Pages[i])
		d.CompanionLines[i] = strings.TrimSpace(d.CompanionLines[i])
		if d.Pages[i] == "" || d.CompanionLines[i] == "" {
			return fmt.Errorf("%dページ目の本文または companionLine が空です", i+1)
		}
	}
	return nil
}

func novelKeywords(v any) string {
	switch k := v.(type) {
	case string:
		return strings.TrimSpace(k)
	case []any:
		parts := make([]string, 0, len(k))
		for _, s := range k {
			parts = append(parts, strings.TrimSpace(fmt.Sprint(s)))
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}

// novelAudio は音声ファイルの公開URL一覧と、PRに含めるファイルを返す。
func novelAudio(id string, pages []string) ([]string, []garoopdata.FileChange, error) {
	if novelAudioSrc == "" && !novelVoicevox {
		return nil, nil, nil
	}
	urls := []string{}
	files := []garoopdata.FileChange{}
	add := func(i int, ext string, content []byte) error {
		if len(content) > novelMaxAudioMiB*1024*1024 {
			return fmt.Errorf("%dページ目の音声が大きすぎます（%dMBまで。mp3推奨）", i+1, novelMaxAudioMiB)
		}
		rel := fmt.Sprintf("%s/%s/%02d%s", novelAudioDir, id, i+1, ext)
		urls = append(urls, garoopdata.PublicURL(rel))
		if content != nil {
			files = append(files, garoopdata.FileChange{Path: "public/" + rel, Content: content})
		}
		return nil
	}

	if novelAudioSrc != "" {
		paths, err := novelAudioFiles(novelAudioSrc, len(pages))
		if err != nil {
			return nil, nil, err
		}
		for i, path := range paths {
			n := filepath.Base(path)
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, err
			}
			if err := add(i, strings.ToLower(filepath.Ext(n)), b); err != nil {
				return nil, nil, err
			}
		}
		return urls, files, nil
	}

	engine := voice.EngineURL()
	for i, text := range pages {
		if !executeMode {
			if err := add(i, ".mp3", nil); err != nil {
				return nil, nil, err
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "VOICEVOX で %d/%d ページ目を生成中...\n", i+1, len(pages))
		wav, err := voice.Synthesize(engine, text, novelSpeaker)
		if err != nil {
			return nil, nil, err
		}
		mp3, err := voice.WAVToMP3(wav, novelAudioRate)
		if err != nil {
			return nil, nil, err
		}
		if err := add(i, ".mp3", mp3); err != nil {
			return nil, nil, err
		}
	}
	return urls, files, nil
}

// novelAudioFiles はフォルダ内の音声ファイルをファイル名順に返す。ページ数と同数でなければエラー。
func novelAudioFiles(dir string, pages int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && novelAudioExt[strings.ToLower(filepath.Ext(e.Name()))] {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) != pages {
		return nil, fmt.Errorf("音声ファイル数（%d）をページ数（%d）と揃えてください", len(names), pages)
	}
	paths := make([]string, len(names))
	for i, n := range names {
		paths[i] = filepath.Join(dir, n)
	}
	return paths, nil
}

func novelAudioLabel(n int) string {
	if n == 0 {
		return "なし"
	}
	return fmt.Sprintf("%dファイル", n)
}

func init() {
	novelListCmd.Flags().StringVar(&novelLang, "lang", "", "言語で絞り込み（ja など）")
	novelListCmd.Flags().StringVar(&novelSeries, "series", "", "seriesKey で絞り込み")

	novelSubmitCmd.Flags().StringVar(&novelFile, "file", "", "小説JSON（必須。形式は novel template で確認）")
	novelSubmitCmd.Flags().StringVar(&novelAudioSrc, "audio-dir", "", "ページごとの音声ファイルのフォルダ")
	novelPublishCmd.Flags().StringVar(&novelFile, "file", "", "小説JSON（必須。形式は novel template で確認）")
	novelPublishCmd.Flags().StringVar(&novelAudioSrc, "audio-dir", "", "ページごとの音声ファイルのフォルダ")
	novelPublishCmd.Flags().BoolVar(&novelVoicevox, "voicevox", false, "VOICEVOXで読み上げ音声を生成")
	novelPublishCmd.Flags().IntVar(&novelSpeaker, "speaker", 1, "VOICEVOX 話者ID")
	novelPublishCmd.Flags().StringVar(&novelAudioRate, "audio-bitrate", "64k", "生成MP3のビットレート")

	markMutates(novelSubmitCmd, novelPublishCmd)
	novelCmd.AddCommand(novelListCmd, novelGetCmd, novelTemplateCmd, novelSubmitCmd, novelPublishCmd)
	rootCmd.AddCommand(novelCmd)
}
