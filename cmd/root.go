package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	ProfileGaroop   = "garoop"
	ProfileGaruchan = "garuchan"
	ProfileGaroopTV = "garooptv"
)

var (
	executeMode      bool
	hashtags         []string
	garuchanImage    string
	garuchanImageURL string
	xDefaultAccount  string
	igDefaultAccount string
)

var rootCmd = &cobra.Command{
	Use:   "garoop-cli",
	Short: "Garoopに関する業務を自動化するCLIツールです",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s へようこそ！ --help でコマンドを確認してください。\n", cmd.Root().Name())
	},
}

func init() {
	rootCmd.AddGroup(
		&cobra.Group{
			ID:    "garoop_cli",
			Title: "garoop-cli",
		},
		&cobra.Group{
			ID:    "garuchan_cli",
			Title: "garuchan-cli",
		},
		&cobra.Group{
			ID:    "garooptv_cli",
			Title: "garooptv-cli",
		},
	)

	rootCmd.PersistentFlags().BoolVar(&executeMode, "execute", false, "実際のAPIを実行する（未指定時はdry-run）")
	rootCmd.PersistentFlags().StringSliceVar(&hashtags, "hashtags", []string{"ガルちゃん", "子供起業", "Garoop"}, "投稿に付与するハッシュタグ")
	rootCmd.PersistentFlags().StringVar(&garuchanImage, "garuchan-image", "assets/garuchan.webp", "ガルちゃん画像のローカルパス")
	rootCmd.PersistentFlags().StringVar(&garuchanImageURL, "garuchan-image-url", "", "ガルちゃん画像の公開URL（Instagram向け）")
	rootCmd.PersistentFlags().StringVar(&xDefaultAccount, "x-account", "garoop_company", "既定のXアカウント名（@なし）")
	rootCmd.PersistentFlags().StringVar(&igDefaultAccount, "instagram-account", "garuchan_wakuwaku", "既定のInstagramアカウント名（@なし）")
}

func Execute() error {
	return ExecuteWithProfile(ProfileGaroop)
}

func ExecuteWithProfile(profile string) error {
	applyProfile(profile)
	return rootCmd.Execute()
}

func applyProfile(profile string) {
	allowedGroupIDs := map[string]bool{}

	switch profile {
	case ProfileGaruchan:
		rootCmd.Use = "garuchan-cli"
		rootCmd.Short = "ガルちゃん育成と子育てを支援するCLIツールです"
		allowedGroupIDs["garuchan_cli"] = true
	case ProfileGaroopTV:
		rootCmd.Use = "garooptv-cli"
		rootCmd.Short = "GaroopTV連携のためのCLIツールです"
		allowedGroupIDs["garooptv_cli"] = true
	default:
		rootCmd.Use = "garoop-cli"
		rootCmd.Short = "Garoopに関する業務を自動化するCLIツールです"
		allowedGroupIDs["garoop_cli"] = true
	}

	for _, c := range rootCmd.Commands() {
		if c.GroupID == "" {
			continue
		}
		c.Hidden = !allowedGroupIDs[c.GroupID]
	}
}
