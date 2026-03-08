package cmd

import (
	"fmt"
	"os"

	"github.com/yamashitadaiki/garoop-cli/internal/social"
	"github.com/spf13/cobra"
)

var (
	noteCookieJSON string
	noteImage      string
	notePublish    bool
)

var noteCmd = &cobra.Command{
	Use:     "note",
	Short:   "Noteへの投稿",
	GroupID: "garoop_cli",
}

var notePostCmd = &cobra.Command{
	Use:   "post [タイトル] [本文ファイル]",
	Short: "Noteへ投稿（または下書き保存）します",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := os.ReadFile(args[1])
		if err != nil {
			return fmt.Errorf("本文ファイル読み込み失敗: %w", err)
		}

		client, err := social.NewNoteClient(executeMode, noteCookieJSON)
		if err != nil {
			return err
		}

		url, err := client.Post(args[0], string(content), noteImage, notePublish)
		if err != nil {
			return err
		}
		fmt.Printf("Note投稿完了: %s\n", url)
		return nil
	},
}

func init() {
	notePostCmd.Flags().StringVar(&noteCookieJSON, "cookie-json", "tokens/note_cookie.json", "note.comのcookie JSONファイル")
	notePostCmd.Flags().StringVar(&noteImage, "image", "", "アイキャッチ画像パスまたはURL")
	notePostCmd.Flags().BoolVar(&notePublish, "publish", false, "公開状態で投稿する（未指定時は下書き）")

	noteCmd.AddCommand(notePostCmd)
	rootCmd.AddCommand(noteCmd)
}
