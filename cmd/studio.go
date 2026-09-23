package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopapi"
	"github.com/yamashitadaiki/garoop-cli/internal/voice"
)

// ガルちゃんキャラクターを使ったメディア生成。
// - 画像/文章: kids_api の generate* ミューテーション（ログインセッション必須）
// - 動画: garuchan_creator（VOICEVOX + Wav2Lip）の /api/generate-video（ローカル起動）
// - 音声: VOICEVOX エンジン

const defaultCreatorURL = "http://localhost:3000"

var (
	studioPrompt      string
	studioMode        string
	studioInputImage  string
	studioRequestType string
	studioSystem      string
	studioOut         string
	studioEngine      string
	studioSpeaker     int
	studioVoiceSpk    int
	studioPrefix      string
	studioSourceURL   string
	studioTitle       string
	studioDescription string
	studioPrivacy     string
	studioTags        []string
)

var studioRequestTypes = []string{"EDUCATION", "VIDEO", "QUESTION", "GAME"}

var studioCmd = serviceCommand("studio", "ガルちゃんスタジオ: キャラ画像・動画・音声・文章の生成とアップロード", ProfileGaroop, ProfileGaruchan)

var studioImageCmd = &cobra.Command{
	Use:   "image",
	Short: "ガルちゃん画像を生成して保存（既定はdry-run。ログインセッション必須）",
	Long: `画像を生成してファイルに保存します。
--mode garuchan  ガルちゃんの参照画像をもとに生成（既定）
--mode text      テキストのみから生成
--mode edit      --input-image の画像を編集
--mode openai    OpenAI で生成`,
	Example: `  garuchan-cli studio image --prompt "ガルちゃんが宇宙服で月面ジャンプ" --out moon.png --execute`,
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, err := readTextOrFile(studioPrompt)
		if err != nil {
			return err
		}
		if prompt == "" {
			return fmt.Errorf("--prompt が必要です（@file 可）")
		}
		requestType := strings.ToUpper(strings.TrimSpace(studioRequestType))
		if !containsString(studioRequestTypes, requestType) {
			return fmt.Errorf("--request-type は %s のいずれかです", strings.Join(studioRequestTypes, ", "))
		}
		vars := map[string]any{"text": prompt, "requestType": requestType}
		if s := strings.TrimSpace(studioSystem); s != "" {
			vars["system"] = s
		}
		var field, query string
		switch studioMode {
		case "garuchan":
			field = "generateImageByGeminiGaruchan"
			query = `mutation Gen($text: String!, $requestType: LLMRequestType!, $system: String) {
				generateImageByGeminiGaruchan(text: $text, requestType: $requestType, system: $system)
			}`
		case "text":
			field = "generateImageByGeminiTextOnly"
			query = `mutation Gen($text: String!, $requestType: LLMRequestType!, $system: String) {
				generateImageByGeminiTextOnly(text: $text, requestType: $requestType, system: $system)
			}`
		case "openai":
			field = "generateImageByOpenAITextOnly"
			query = `mutation Gen($text: String!, $requestType: LLMRequestType!, $system: String) {
				generateImageByOpenAITextOnly(text: $text, requestType: $requestType, system: $system)
			}`
		case "edit":
			if err := requireValue("input-image", studioInputImage); err != nil {
				return err
			}
			b, err := os.ReadFile(studioInputImage)
			if err != nil {
				return err
			}
			vars["imageData"] = base64.StdEncoding.EncodeToString(b)
			field = "generateImageByGemini"
			query = `mutation Gen($text: String!, $imageData: String, $requestType: LLMRequestType!, $system: String) {
				generateImageByGemini(text: $text, imageData: $imageData, requestType: $requestType, system: $system)
			}`
		default:
			return fmt.Errorf("--mode は garuchan|text|edit|openai のいずれかです")
		}

		out := studioOutPath(studioOut, "garuchan", "png")
		if !executeMode {
			if _, ok := vars["imageData"]; ok {
				vars["imageData"] = fmt.Sprintf("<base64 %d bytes>", len(vars["imageData"].(string)))
			}
			return printJSON(map[string]any{"mode": "dry-run", "field": field, "variables": vars, "out": out})
		}

		resp, err := garoopapi.NewClient().WithTimeout(3*time.Minute).Query(query, vars)
		if err != nil {
			return err
		}
		if len(resp.Errors) > 0 {
			return fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
		}
		result, _ := resp.Data[field].(string)
		img, err := studioDecodeImage(result)
		if err != nil {
			return err
		}
		if err := os.WriteFile(out, img, 0o644); err != nil {
			return err
		}
		return printJSON(map[string]any{"mode": "execute", "out": out, "bytes": len(img), "contentType": http.DetectContentType(img)})
	},
}

var studioTextCmd = &cobra.Command{
	Use:     "text",
	Short:   "ガルちゃんのプロンプト設定で文章を生成（既定はdry-run。ログインセッション必須）",
	Example: `  garuchan-cli studio text --prompt "ガルちゃんが紹介する今日のミッション告知文を3案" --execute`,
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, err := readTextOrFile(studioPrompt)
		if err != nil {
			return err
		}
		if prompt == "" {
			return fmt.Errorf("--prompt が必要です（@file 可）")
		}
		requestType := strings.ToUpper(strings.TrimSpace(studioRequestType))
		if !containsString(studioRequestTypes, requestType) {
			return fmt.Errorf("--request-type は %s のいずれかです", strings.Join(studioRequestTypes, ", "))
		}
		field := map[string]string{"gemini": "generateTextByGemini", "groq": "generateTextByGroq"}[studioEngine]
		if field == "" {
			return fmt.Errorf("--engine は gemini|groq のいずれかです")
		}
		vars := map[string]any{"text": prompt, "requestType": requestType}
		if s := strings.TrimSpace(studioSystem); s != "" {
			vars["system"] = s
		}
		query := fmt.Sprintf(`mutation Gen($text: String!, $requestType: LLMRequestType!, $system: String) {
			%s(text: $text, requestType: $requestType, system: $system)
		}`, field)
		if !executeMode {
			return printJSON(map[string]any{"mode": "dry-run", "field": field, "variables": vars})
		}
		resp, err := garoopapi.NewClient().WithTimeout(2*time.Minute).Query(query, vars)
		if err != nil {
			return err
		}
		if len(resp.Errors) > 0 {
			return fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
		}
		fmt.Println(resp.Data[field])
		return nil
	},
}

var studioVideoCmd = &cobra.Command{
	Use:   "video",
	Short: "ガルちゃんが話す口パク動画を生成（ローカルの garuchan_creator を使用）",
	Long: `garuchan_creator（VOICEVOX + Wav2Lip）で、ガルちゃんがテキストを話す動画(mp4)を生成します。
事前に garuchan_creator で ` + "`make up`" + ` を実行しておいてください。
接続先は GARUCHAN_CREATOR_URL（既定: http://localhost:3000）。テキストは300文字まで。`,
	Example: `  garuchan-cli studio video --text "こんにちは！ガルちゃんだよ" --speaker 3 --out hello.mp4`,
	RunE: func(cmd *cobra.Command, args []string) error {
		text, err := readTextOrFile(studioPrompt)
		if err != nil {
			return err
		}
		if text == "" {
			return fmt.Errorf("--text が必要です（@file 可）")
		}
		if n := len([]rune(text)); n > 300 {
			return fmt.Errorf("テキストは300文字までです（現在%d文字）", n)
		}
		payload := map[string]any{"text": text}
		if studioSpeaker > 0 {
			payload["speaker"] = studioSpeaker
		}
		body, _ := json.Marshal(payload)
		endpoint := creatorURL() + "/api/generate-video"
		fmt.Fprintf(os.Stderr, "動画生成中（%s）...\n", endpoint)
		resp, err := (&http.Client{Timeout: 15 * time.Minute}).Post(endpoint, "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("garuchan_creator に接続できません（%s）。`make up` 済みか確認してください: %w", endpoint, err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 300 {
			return fmt.Errorf("動画生成失敗: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
		out := studioOutPath(studioOut, "garuchan", "mp4")
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return err
		}
		return printJSON(map[string]any{"out": out, "bytes": len(data), "credit": resp.Header.Get("X-Credit-Text")})
	},
}

var studioVoiceCmd = &cobra.Command{
	Use:     "voice",
	Short:   "VOICEVOXでテキストを読み上げた音声を生成（ローカル）",
	Example: `  garuchan-cli studio voice --text "今日もミッションがんばろう！" --speaker 3 --out voice.mp3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		text, err := readTextOrFile(studioPrompt)
		if err != nil {
			return err
		}
		if text == "" {
			return fmt.Errorf("--text が必要です（@file 可）")
		}
		wav, err := voice.Synthesize(voice.EngineURL(), text, studioVoiceSpk)
		if err != nil {
			return err
		}
		out := studioOutPath(studioOut, "garuchan-voice", "wav")
		data := wav
		if strings.HasSuffix(strings.ToLower(out), ".mp3") {
			if data, err = voice.WAVToMP3(wav, "96k"); err != nil {
				return err
			}
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return err
		}
		return printJSON(map[string]any{"out": out, "bytes": len(data)})
	},
}

var studioUploadCmd = &cobra.Command{
	Use:   "upload [file]",
	Short: "生成物をGaroopのメディアストレージ(S3)へアップロード（既定はdry-run）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := os.Stat(args[0])
		if err != nil {
			return err
		}
		if !executeMode {
			return printJSON(map[string]any{
				"mode": "dry-run", "file": args[0], "bytes": info.Size(),
				"contentType": garoopapi.DetectContentType(args[0]), "prefix": studioPrefix,
			})
		}
		uploaded, err := garoopapi.NewClient().UploadFile(args[0], studioPrefix)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"mode": "execute", "object": uploaded})
	},
}

var studioYouTubeUploadCmd = &cobra.Command{
	Use:   "youtube-upload",
	Short: "公開URLの動画をGaroopのYouTubeチャンネルへアップロード（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireValue("source-url", studioSourceURL); err != nil {
			return err
		}
		if err := requireValue("title", studioTitle); err != nil {
			return err
		}
		input := map[string]any{
			"sourceUrl":     strings.TrimSpace(studioSourceURL),
			"title":         strings.TrimSpace(studioTitle),
			"privacyStatus": studioPrivacy,
		}
		if d := strings.TrimSpace(studioDescription); d != "" {
			input["description"] = d
		}
		if len(studioTags) > 0 {
			input["tags"] = studioTags
		}
		return runMutationOrDryRun("uploadYoutubeVideo", `mutation UploadYoutubeVideo($input: UploadYoutubeVideoInput!) {
			uploadYoutubeVideo(input: $input) { videoId videoUrl }
		}`, map[string]any{"input": input})
	},
}

// studioDecodeImage は生成結果（base64 / data URL / 画像URL）を画像バイト列にする。
func studioDecodeImage(result string) ([]byte, error) {
	result = strings.TrimSpace(result)
	if result == "" {
		return nil, fmt.Errorf("画像が返りませんでした")
	}
	if strings.HasPrefix(result, "http://") || strings.HasPrefix(result, "https://") {
		resp, err := http.Get(result)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}
	if i := strings.Index(result, ","); strings.HasPrefix(result, "data:") && i > 0 {
		result = result[i+1:]
	}
	img, err := base64.StdEncoding.DecodeString(result)
	if err != nil || !strings.HasPrefix(http.DetectContentType(img), "image/") {
		return nil, fmt.Errorf("画像ではなくテキストが返りました: %s", truncateRunes(result, 200))
	}
	return img, nil
}

func studioOutPath(out, prefix, ext string) string {
	if strings.TrimSpace(out) != "" {
		return out
	}
	return fmt.Sprintf("%s-%s.%s", prefix, time.Now().Format("20060102-150405"), ext)
}

func creatorURL() string {
	if v := strings.TrimSpace(os.Getenv("GARUCHAN_CREATOR_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultCreatorURL
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func init() {
	studioImageCmd.Flags().StringVar(&studioPrompt, "prompt", "", "生成指示（必須。@file 可）")
	studioImageCmd.Flags().StringVar(&studioMode, "mode", "garuchan", "garuchan|text|edit|openai")
	studioImageCmd.Flags().StringVar(&studioInputImage, "input-image", "", "編集元画像（--mode edit）")
	studioImageCmd.Flags().StringVar(&studioRequestType, "request-type", "VIDEO", "プロンプト設定の種別 EDUCATION|VIDEO|QUESTION|GAME")
	studioImageCmd.Flags().StringVar(&studioSystem, "system", "", "システムプロンプト（省略時はアカウント設定）")
	studioImageCmd.Flags().StringVar(&studioOut, "out", "", "保存先（既定: garuchan-日時.png）")

	studioTextCmd.Flags().StringVar(&studioPrompt, "prompt", "", "生成指示（必須。@file 可）")
	studioTextCmd.Flags().StringVar(&studioEngine, "engine", "gemini", "gemini|groq")
	studioTextCmd.Flags().StringVar(&studioRequestType, "request-type", "VIDEO", "EDUCATION|VIDEO|QUESTION|GAME")
	studioTextCmd.Flags().StringVar(&studioSystem, "system", "", "システムプロンプト")

	studioVideoCmd.Flags().StringVar(&studioPrompt, "text", "", "ガルちゃんのセリフ（必須、300文字まで。@file 可）")
	studioVideoCmd.Flags().IntVar(&studioSpeaker, "speaker", 0, "VOICEVOX 話者ID（省略時はサーバー既定）")
	studioVideoCmd.Flags().StringVar(&studioOut, "out", "", "保存先（既定: garuchan-日時.mp4）")

	studioVoiceCmd.Flags().StringVar(&studioPrompt, "text", "", "読み上げテキスト（必須。@file 可）")
	studioVoiceCmd.Flags().IntVar(&studioVoiceSpk, "speaker", 1, "VOICEVOX 話者ID")
	studioVoiceCmd.Flags().StringVar(&studioOut, "out", "", "保存先 .wav / .mp3（mp3はffmpeg必要）")

	studioUploadCmd.Flags().StringVar(&studioPrefix, "prefix", "garuchan-studio", "S3キーのプレフィックス")

	studioYouTubeUploadCmd.Flags().StringVar(&studioSourceURL, "source-url", "", "動画の公開URL（必須。studio upload 後のURLなど）")
	studioYouTubeUploadCmd.Flags().StringVar(&studioTitle, "title", "", "タイトル（必須）")
	studioYouTubeUploadCmd.Flags().StringVar(&studioDescription, "description", "", "説明")
	studioYouTubeUploadCmd.Flags().StringVar(&studioPrivacy, "privacy", "private", "private|unlisted|public")
	studioYouTubeUploadCmd.Flags().StringSliceVar(&studioTags, "tags", nil, "タグ（カンマ区切り）")

	markMutates(studioImageCmd, studioTextCmd, studioUploadCmd, studioYouTubeUploadCmd)
	studioCmd.AddCommand(studioImageCmd, studioTextCmd, studioVideoCmd, studioVoiceCmd, studioUploadCmd, studioYouTubeUploadCmd)
	rootCmd.AddCommand(studioCmd)
}
