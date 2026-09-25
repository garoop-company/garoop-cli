package cmd

import (
	"fmt"
	"strings"

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
	Short: "Garoop業務をAIエージェント経由で自動化するCLIツールです",
	// エラー時に usage 全文を出さない（AIエージェントが読むログを短く保つ）。エラー本文は main で表示する
	SilenceUsage:  true,
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s へようこそ！ --help でコマンドを確認してください。\n", cmd.Root().Name())
	},
}

// commandGroups は help の見出し。表示するコマンドがあるものだけ applyProfile で登録する。
var commandGroups = []*cobra.Group{
	{ID: "garoop_cli", Title: "garoop-cli"},
	{ID: "garuchan_cli", Title: "garuchan-cli"},
	{ID: "garooptv_cli", Title: "garooptv-cli"},
	{ID: groupServices, Title: "Garoopサービス操作"},
}

func init() {
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
		rootCmd.Short = "ガルちゃん育成と子育てをAIエージェント経由で支援するCLIツールです"
		allowedGroupIDs["garuchan_cli"] = true
	case ProfileGaroopTV:
		rootCmd.Use = "garooptv-cli"
		rootCmd.Short = "GaroopTV連携をAIエージェント経由で扱うCLIツールです"
		allowedGroupIDs["garooptv_cli"] = true
	default:
		rootCmd.Use = "garoop-cli"
		rootCmd.Short = "Garoop業務をAIエージェント経由で自動化するCLIツールです"
		allowedGroupIDs["garoop_cli"] = true
	}

	for _, c := range rootCmd.Commands() {
		if c.GroupID == "" {
			continue
		}
		if profiles, ok := c.Annotations[annotationProfiles]; ok {
			c.Hidden = !containsProfile(profiles, profile)
			continue
		}
		c.Hidden = !allowedGroupIDs[c.GroupID]
	}

	// 他バイナリ向けのコマンドしかないグループは見出しだけ残るので登録しない。
	// 未登録のグループを参照すると cobra が失敗するため、隠したコマンドのグループは外す。
	visibleGroups := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		if c.GroupID == "" {
			continue
		}
		if c.Hidden {
			c.GroupID = ""
			continue
		}
		visibleGroups[c.GroupID] = true
	}
	for _, g := range commandGroups {
		if visibleGroups[g.ID] {
			rootCmd.AddGroup(g)
		}
	}
}

func containsProfile(profiles, profile string) bool {
	if profile != ProfileGaruchan && profile != ProfileGaroopTV {
		profile = ProfileGaroop
	}
	for _, p := range strings.Split(profiles, ",") {
		if strings.TrimSpace(p) == profile {
			return true
		}
	}
	return false
}
