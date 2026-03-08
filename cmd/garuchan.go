package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yamashitadaiki/garoop-cli/internal/garuchan"
	"github.com/yamashitadaiki/garoop-cli/internal/social"
	"github.com/spf13/cobra"
)

var (
	garuchanStatePath   = "data/garuchan_state.json"
	garuchanName        string
	garuchanModel       string
	garuchanForce       bool
	garuchanFeedLimit   int
	garuchanChannelID   string
	garuchanNoteUser    string
	garuchanManualText  string
	garuchanManualTitle string
)

var garuchanBirthCmd = &cobra.Command{
	Use:     "birth",
	Short:   "ガルちゃんを誕生させます",
	GroupID: "garuchan_cli",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("ガルちゃん誕生フェーズを開始...")
		fmt.Printf("LLMミルク準備中: model=%s\n", garuchanModel)
		pullModelProgress(garuchanModel)

		state, err := garuchan.Birth(garuchanStatePath, garuchanName, garuchanModel, garuchanForce)
		if err != nil {
			return err
		}
		fmt.Printf("誕生しました: name=%s stage=%s model=%s\n", state.Name, state.Stage, state.Model)
		return nil
	},
}

var garuchanPullLLMCmd = &cobra.Command{
	Use:     "pull-llm [model]",
	Short:   "LLMミルクを補充します（演出）",
	GroupID: "garuchan_cli",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pullModelProgress(args[0])
		fmt.Printf("LLMミルク補充完了: %s\n", args[0])
		return nil
	},
}

var garuchanStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "ガルちゃんの育成状態を表示",
	GroupID: "garuchan_cli",
	RunE: func(cmd *cobra.Command, args []string) error {
		state, err := garuchan.Load(garuchanStatePath)
		if err != nil {
			return err
		}
		fmt.Printf("name=%s model=%s stage=%s feeds=%d calories=%d born_at=%s last_fed=%s\n",
			state.Name, state.Model, state.Stage, state.TotalFeeds, state.TotalCalories,
			state.BornAt.Format(time.RFC3339), state.LastFedAt.Format(time.RFC3339))
		return nil
	},
}

var garuchanFeedCmd = &cobra.Command{
	Use:     "feed",
	Short:   "SNS情報をガルちゃんに食べさせる",
	GroupID: "garuchan_cli",
}

var garuchanFeedInstagramCmd = &cobra.Command{
	Use:   "instagram",
	Short: "Instagram投稿を食べさせる",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewInstagramClient(executeMode)
		if err != nil {
			return err
		}
		media, err := client.RecentMedia(garuchanFeedLimit)
		if err != nil {
			return err
		}
		return feedItems("instagram", len(media), summarizeInstagram(media))
	},
}

var garuchanFeedXCmd = &cobra.Command{
	Use:   "x",
	Short: "X投稿を食べさせる",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewXClient(executeMode)
		if err != nil {
			return err
		}
		posts, err := client.RecentOwnPosts(garuchanFeedLimit)
		if err != nil {
			return err
		}
		return feedItems("x", len(posts), summarizeX(posts))
	},
}

var garuchanFeedYouTubeCmd = &cobra.Command{
	Use:   "youtube",
	Short: "YouTube動画情報を食べさせる",
	RunE: func(cmd *cobra.Command, args []string) error {
		channelID := strings.TrimSpace(garuchanChannelID)
		if channelID == "" {
			channelID = strings.TrimSpace(os.Getenv("YOUTUBE_OWN_CHANNEL_ID"))
		}
		if channelID == "" && executeMode {
			return fmt.Errorf("--channel-id または YOUTUBE_OWN_CHANNEL_ID が必要です")
		}

		client, err := social.NewYouTubeClient(executeMode)
		if err != nil {
			return err
		}
		videos, err := client.RecentChannelVideos(channelID, garuchanFeedLimit)
		if err != nil {
			return err
		}
		return feedItems("youtube", len(videos), summarizeYouTube(videos))
	},
}

var garuchanFeedNoteCmd = &cobra.Command{
	Use:   "note",
	Short: "Note記事情報を食べさせる",
	RunE: func(cmd *cobra.Command, args []string) error {
		user := strings.TrimSpace(garuchanNoteUser)
		if user == "" {
			user = strings.TrimSpace(os.Getenv("NOTE_USERNAME"))
		}
		if user == "" {
			return fmt.Errorf("--username または NOTE_USERNAME が必要です")
		}
		client, err := social.NewNoteClient(executeMode, "tokens/note_cookie.json")
		if err != nil {
			return err
		}
		articles, err := client.RecentArticles(user, garuchanFeedLimit)
		if err != nil {
			return err
		}
		return feedItems("note", len(articles), summarizeNote(articles))
	},
}

var garuchanFeedTextCmd = &cobra.Command{
	Use:   "text",
	Short: "任意テキストを食べさせる",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garuchanManualText) == "" && strings.TrimSpace(garuchanManualTitle) == "" {
			return fmt.Errorf("--title か --text を指定してください")
		}
		payload := strings.TrimSpace(garuchanManualTitle + "\n" + garuchanManualText)
		return feedItems("manual", 1, payload)
	},
}

func init() {
	garuchanBirthCmd.Flags().StringVar(&garuchanStatePath, "state", garuchanStatePath, "ガルちゃん状態ファイル")
	garuchanStatusCmd.Flags().StringVar(&garuchanStatePath, "state", garuchanStatePath, "ガルちゃん状態ファイル")
	garuchanFeedCmd.PersistentFlags().StringVar(&garuchanStatePath, "state", garuchanStatePath, "ガルちゃん状態ファイル")

	garuchanBirthCmd.Flags().StringVar(&garuchanName, "name", "ガルちゃん", "赤ちゃんの名前")
	garuchanBirthCmd.Flags().StringVar(&garuchanModel, "model", "llama3.2", "誕生時に使うLLMモデル名")
	garuchanBirthCmd.Flags().BoolVar(&garuchanForce, "force", false, "既存状態を上書きして再誕生させる")

	garuchanFeedInstagramCmd.Flags().IntVar(&garuchanFeedLimit, "limit", 5, "取得件数")
	garuchanFeedXCmd.Flags().IntVar(&garuchanFeedLimit, "limit", 5, "取得件数")
	garuchanFeedYouTubeCmd.Flags().IntVar(&garuchanFeedLimit, "limit", 5, "取得件数")
	garuchanFeedYouTubeCmd.Flags().StringVar(&garuchanChannelID, "channel-id", "", "YouTubeチャンネルID")
	garuchanFeedNoteCmd.Flags().IntVar(&garuchanFeedLimit, "limit", 5, "取得件数")
	garuchanFeedNoteCmd.Flags().StringVar(&garuchanNoteUser, "username", "", "Noteユーザー名")
	garuchanFeedTextCmd.Flags().StringVar(&garuchanManualTitle, "title", "", "テキストタイトル")
	garuchanFeedTextCmd.Flags().StringVar(&garuchanManualText, "text", "", "テキスト本文")

	garuchanFeedCmd.AddCommand(garuchanFeedInstagramCmd, garuchanFeedXCmd, garuchanFeedYouTubeCmd, garuchanFeedNoteCmd, garuchanFeedTextCmd)
	rootCmd.AddCommand(garuchanBirthCmd, garuchanPullLLMCmd, garuchanStatusCmd, garuchanFeedCmd)
}

func pullModelProgress(model string) {
	steps := []string{
		"レイヤーを準備中",
		"重みをダウンロード中",
		"知能ミルクを充填中",
		"初期化中",
	}
	for _, s := range steps {
		fmt.Printf("[model:%s] %s...\n", model, s)
		time.Sleep(120 * time.Millisecond)
	}
}

func feedItems(source string, count int, preview string) error {
	state, err := garuchan.Load(garuchanStatePath)
	if err != nil {
		return fmt.Errorf("先に `birth` してください: %w", err)
	}
	if count <= 0 {
		return fmt.Errorf("%s から食べさせる情報がありません", source)
	}

	fmt.Printf("ガルちゃんに%s情報を食べさせます...\n", strings.ToUpper(source))
	fmt.Println("もぐもぐ中...")
	time.Sleep(80 * time.Millisecond)

	calories := count * 30
	garuchan.Feed(state, calories)
	if err := garuchan.Save(garuchanStatePath, state); err != nil {
		return err
	}

	fmt.Printf("食事完了: source=%s items=%d calories=%d stage=%s\n", source, count, calories, state.Stage)
	if strings.TrimSpace(preview) != "" {
		fmt.Printf("ごはんメモ:\n%s\n", preview)
	}
	return nil
}

func summarizeInstagram(media []social.InstagramMedia) string {
	lines := make([]string, 0, len(media))
	for i, m := range media {
		if i >= 3 {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s (%s)", truncate(m.Caption, 48), m.MediaType))
	}
	return strings.Join(lines, "\n")
}

func summarizeX(posts []social.XPost) string {
	lines := make([]string, 0, len(posts))
	for i, p := range posts {
		if i >= 3 {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s", truncate(p.Text, 64)))
	}
	return strings.Join(lines, "\n")
}

func summarizeYouTube(videos []social.YouTubeVideo) string {
	lines := make([]string, 0, len(videos))
	for i, v := range videos {
		if i >= 3 {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s", truncate(v.Title, 64)))
	}
	return strings.Join(lines, "\n")
}

func summarizeNote(articles []social.NoteArticle) string {
	lines := make([]string, 0, len(articles))
	for i, a := range articles {
		if i >= 3 {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s", truncate(a.Title, 64)))
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "..."
}
