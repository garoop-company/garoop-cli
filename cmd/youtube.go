package cmd

import (
	"fmt"
	"os"

	"garoop-cli/internal/content"
	"garoop-cli/internal/social"
	"github.com/spf13/cobra"
)

var (
	youtubeDescription string
	youtubeThumbnail   string
	youtubeChannelID   string
	youtubeOwnChannel  string
	youtubeReplyText   string
	youtubeReplyLimit  int
	youtubeMaxReplies  int
)

var youtubeCmd = &cobra.Command{
	Use:     "youtube",
	Short:   "YouTube動画アップロード・コメント",
	GroupID: "garoop_cli",
}

var youtubeUploadCmd = &cobra.Command{
	Use:   "upload [動画ファイル] [タイトル]",
	Short: "YouTubeへ動画をアップロードします",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewYouTubeClient(executeMode)
		if err != nil {
			return err
		}

		description := content.AppendHashtags(youtubeDescription, hashtags)
		thumbnail := youtubeThumbnail
		if thumbnail == "" {
			thumbnail = garuchanImage
		}

		videoID, err := client.UploadVideo(args[0], args[1], description, thumbnail)
		if err != nil {
			return err
		}
		fmt.Printf("YouTubeアップロード完了: %s\n", videoID)
		return nil
	},
}

var youtubeCommentCmd = &cobra.Command{
	Use:   "comment [video-id] [コメント]",
	Short: "YouTube動画にコメントします",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewYouTubeClient(executeMode)
		if err != nil {
			return err
		}

		comment := content.AppendHashtags(args[1], hashtags)
		commentID, err := client.Comment(args[0], comment)
		if err != nil {
			return err
		}
		fmt.Printf("YouTubeコメント完了: %s\n", commentID)
		return nil
	},
}

var youtubeAutoReplyCmd = &cobra.Command{
	Use:   "auto-reply",
	Short: "指定チャンネルの未返信コメントへ自動返信します",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewYouTubeClient(executeMode)
		if err != nil {
			return err
		}

		ownChannelID := youtubeOwnChannel
		if ownChannelID == "" {
			ownChannelID = os.Getenv("YOUTUBE_OWN_CHANNEL_ID")
		}

		replyText := content.AppendHashtags(youtubeReplyText, hashtags)
		results, err := client.AutoReplyChannelComments(youtubeChannelID, ownChannelID, replyText, youtubeReplyLimit, youtubeMaxReplies)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Println("返信対象コメントはありませんでした")
			return nil
		}
		for _, r := range results {
			fmt.Printf("返信完了: video=%s parent=%s reply=%s\n", r.VideoID, r.TargetCommentID, r.ReplyCommentID)
		}
		return nil
	},
}

func init() {
	youtubeUploadCmd.Flags().StringVar(&youtubeDescription, "description", "", "動画説明文")
	youtubeUploadCmd.Flags().StringVar(&youtubeThumbnail, "thumbnail", "", "サムネ画像パス（未指定時はガルちゃん画像）")
	youtubeAutoReplyCmd.Flags().StringVar(&youtubeChannelID, "channel-id", "UCVXDkfy7aD08L7y7JK1AtmA", "返信対象チャンネルID")
	youtubeAutoReplyCmd.Flags().StringVar(&youtubeOwnChannel, "own-channel-id", "", "自チャンネルID（未指定時はYOUTUBE_OWN_CHANNEL_ID環境変数）")
	youtubeAutoReplyCmd.Flags().StringVar(&youtubeReplyText, "reply", "コメントありがとうございます！", "返信文")
	youtubeAutoReplyCmd.Flags().IntVar(&youtubeReplyLimit, "limit", 20, "取得する最新コメント数")
	youtubeAutoReplyCmd.Flags().IntVar(&youtubeMaxReplies, "max-replies", 5, "1回で返信する最大件数")

	youtubeCmd.AddCommand(youtubeUploadCmd)
	youtubeCmd.AddCommand(youtubeCommentCmd)
	youtubeCmd.AddCommand(youtubeAutoReplyCmd)
	rootCmd.AddCommand(youtubeCmd)
}
