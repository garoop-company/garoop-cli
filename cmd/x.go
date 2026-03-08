package cmd

import (
	"fmt"
	"strconv"

	"github.com/yamashitadaiki/garoop-cli/internal/content"
	"github.com/yamashitadaiki/garoop-cli/internal/social"
	"github.com/spf13/cobra"
)

var (
	xImage   string
	xNoImage bool
)

var xCmd = &cobra.Command{
	Use:     "x",
	Short:   "X(Twitter)への投稿・返信",
	GroupID: "garoop_cli",
}

var xPostCmd = &cobra.Command{
	Use:   "post [メッセージ]",
	Short: "Xへ投稿します",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewXClient(executeMode)
		if err != nil {
			return err
		}
		text := content.AppendHashtags(args[0], hashtags)
		postID, err := client.Post(text, selectedImagePath())
		if err != nil {
			return err
		}
		fmt.Printf("X投稿完了: %s\n", postID)
		return nil
	},
}

var xReplyCmd = &cobra.Command{
	Use:   "reply [tweet-id] [メッセージ]",
	Short: "Xの投稿にリプライします",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := strconv.ParseInt(args[0], 10, 64); err != nil {
			return fmt.Errorf("tweet-idは数値で指定してください")
		}

		client, err := social.NewXClient(executeMode)
		if err != nil {
			return err
		}

		text := content.AppendHashtags(args[1], hashtags)
		replyID, err := client.Reply(args[0], text, selectedImagePath())
		if err != nil {
			return err
		}
		fmt.Printf("Xリプライ完了: %s\n", replyID)
		return nil
	},
}

var xReplyGaroopCmd = &cobra.Command{
	Use:   "reply-garoop [tweet-id] [メッセージ]",
	Short: "既定Xアカウント（初期値: garoop_company）向けにリプライします",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := strconv.ParseInt(args[0], 10, 64); err != nil {
			return fmt.Errorf("tweet-idは数値で指定してください")
		}

		client, err := social.NewXClient(executeMode)
		if err != nil {
			return err
		}

		prefix := "@" + xDefaultAccount + " "
		text := content.AppendHashtags(prefix+args[1], hashtags)
		replyID, err := client.Reply(args[0], text, selectedImagePath())
		if err != nil {
			return err
		}
		fmt.Printf("Xリプライ完了(宛先:@%s): %s\n", xDefaultAccount, replyID)
		return nil
	},
}

var tweetCmd = &cobra.Command{
	Use:     "tweet [メッセージ]",
	Short:   "Xへ投稿します（x post のエイリアス）",
	GroupID: "garoop_cli",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return xPostCmd.RunE(cmd, args)
	},
}

func selectedImagePath() string {
	if xNoImage {
		return ""
	}
	if xImage != "" {
		return xImage
	}
	return garuchanImage
}

func init() {
	xCmd.PersistentFlags().StringVar(&xImage, "image", "", "投稿に添付する画像パス")
	xCmd.PersistentFlags().BoolVar(&xNoImage, "no-image", false, "画像を添付しない")

	xCmd.AddCommand(xPostCmd)
	xCmd.AddCommand(xReplyCmd)
	xCmd.AddCommand(xReplyGaroopCmd)

	rootCmd.AddCommand(xCmd)
	rootCmd.AddCommand(tweetCmd)
}
