package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"garoop-cli/internal/content"
	"garoop-cli/internal/social"
	"github.com/spf13/cobra"
)

var (
	instagramImageURL  string
	instagramFeedLimit int
	instagramFeedOut   string
	instagramPushLimit int
	garuchanEndpoint   string
	garuchanAPIKey     string
)

var instagramCmd = &cobra.Command{
	Use:     "instagram",
	Short:   "Instagramへの投稿",
	GroupID: "garoop_cli",
}

var instagramPostCmd = &cobra.Command{
	Use:   "post [キャプション]",
	Short: "Instagramへ画像投稿します",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewInstagramClient(executeMode)
		if err != nil {
			return err
		}

		caption := content.AppendHashtags(args[0], hashtags)
		imageURL := instagramImageURL
		if imageURL == "" {
			imageURL = garuchanImageURL
		}
		if executeMode && imageURL == "" {
			return fmt.Errorf("Instagram実行には公開画像URLが必要です。--image-url か --garuchan-image-url を指定してください")
		}

		postID, err := client.PostImage(caption, imageURL)
		if err != nil {
			return err
		}
		fmt.Printf("Instagram投稿完了: %s\n", postID)
		return nil
	},
}

var instagramFeedGaruchanCmd = &cobra.Command{
	Use:   "feed-garuchan",
	Short: "Instagram投稿をガルちゃん向けテキストに変換して保存します",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewInstagramClient(executeMode)
		if err != nil {
			return err
		}

		media, err := client.RecentMedia(instagramFeedLimit)
		if err != nil {
			return err
		}
		if len(media) == 0 {
			return fmt.Errorf("取得できるInstagram投稿がありません")
		}

		var b strings.Builder
		b.WriteString("ガルちゃん向けInstagram投稿要約\n")
		b.WriteString("出力日時: ")
		b.WriteString(time.Now().Format(time.RFC3339))
		b.WriteString("\n\n")
		for i, m := range media {
			b.WriteString(fmt.Sprintf("[%d] media_id=%s type=%s\n", i+1, m.ID, m.MediaType))
			b.WriteString(fmt.Sprintf("投稿日: %s\n", m.Timestamp))
			b.WriteString(fmt.Sprintf("URL: %s\n", m.Permalink))
			if strings.TrimSpace(m.Caption) != "" {
				b.WriteString("本文:\n")
				b.WriteString(strings.TrimSpace(m.Caption))
				b.WriteString("\n")
			}
			if strings.TrimSpace(m.MediaURL) != "" {
				b.WriteString(fmt.Sprintf("media_url: %s\n", m.MediaURL))
			}
			b.WriteString("\n")
		}

		if err := os.MkdirAll(filepath.Dir(instagramFeedOut), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(instagramFeedOut, []byte(b.String()), 0o644); err != nil {
			return err
		}
		fmt.Printf("ガルちゃん向けフィードを生成しました: %s\n", instagramFeedOut)
		return nil
	},
}

var instagramPushGaruchanCmd = &cobra.Command{
	Use:   "push-garuchan",
	Short: "実行ユーザーのInstagram写真/動画をガルちゃんへ送信します",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("ガルちゃんのごはん準備を開始...")
		client, err := social.NewInstagramClient(executeMode)
		if err != nil {
			return err
		}

		endpoint := strings.TrimSpace(garuchanEndpoint)
		if endpoint == "" {
			endpoint = strings.TrimSpace(os.Getenv("GARUCHAN_UPLOAD_URL"))
		}
		apiKey := strings.TrimSpace(garuchanAPIKey)
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("GARUCHAN_API_KEY"))
		}

		results, err := client.PushRecentMediaToGaruchan(instagramPushLimit, endpoint, apiKey)
		if err != nil {
			return err
		}
		for _, r := range results {
			fmt.Printf("もぐもぐログ: media_id=%s type=%s status=%s\n", r.MediaID, r.MediaType, r.Status)
		}
		fmt.Printf("食事完了: Instagramから%d件を食べました\n", len(results))
		return nil
	},
}

var instagramCommentCmd = &cobra.Command{
	Use:   "comment [media-id] [コメント]",
	Short: "Instagramメディアへコメントします",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewInstagramClient(executeMode)
		if err != nil {
			return err
		}
		commentID, err := client.Comment(args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Instagramコメント完了: %s\n", commentID)
		return nil
	},
}

var instagramLikeCmd = &cobra.Command{
	Use:   "like [media-id]",
	Short: "Instagramメディアへいいねします",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewInstagramClient(executeMode)
		if err != nil {
			return err
		}
		result, err := client.Like(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Instagramいいね完了: %s\n", result)
		return nil
	},
}

func init() {
	instagramPostCmd.Flags().StringVar(&instagramImageURL, "image-url", "", "投稿する画像の公開URL")
	instagramFeedGaruchanCmd.Flags().IntVar(&instagramFeedLimit, "limit", 10, "取得する投稿数")
	instagramFeedGaruchanCmd.Flags().StringVar(&instagramFeedOut, "out", "data/garuchan_instagram_feed.txt", "出力先ファイル")
	instagramPushGaruchanCmd.Flags().IntVar(&instagramPushLimit, "limit", 10, "送信する投稿数")
	instagramPushGaruchanCmd.Flags().StringVar(&garuchanEndpoint, "endpoint", "", "ガルちゃんアップロードAPIエンドポイント（未指定時はGARUCHAN_UPLOAD_URL）")
	instagramPushGaruchanCmd.Flags().StringVar(&garuchanAPIKey, "api-key", "", "ガルちゃんAPIキー（未指定時はGARUCHAN_API_KEY）")
	instagramCmd.AddCommand(instagramPostCmd)
	instagramCmd.AddCommand(instagramFeedGaruchanCmd)
	instagramCmd.AddCommand(instagramPushGaruchanCmd)
	instagramCmd.AddCommand(instagramCommentCmd)
	instagramCmd.AddCommand(instagramLikeCmd)
	rootCmd.AddCommand(instagramCmd)
}
