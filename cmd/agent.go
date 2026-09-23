package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// AIエージェント向けの自己記述。--help を解析しなくても使えるコマンドとフラグをJSONで返す。

var agentIncludeHidden bool

var agentCmd = &cobra.Command{
	Use:     "agent",
	Short:   "AIエージェント向けの情報（コマンド一覧をJSONで出力）",
	GroupID: groupServices,
	Annotations: map[string]string{
		annotationProfiles: strings.Join([]string{ProfileGaroop, ProfileGaruchan, ProfileGaroopTV}, ","),
	},
}

var agentManifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "全コマンド・フラグ・書き込み有無をJSONで出力",
	Long: `AIエージェントがこのCLIを使うためのマニフェストを出力します。
mutates=true のコマンドは既定で dry-run になり、--execute を付けたときだけ外部へ書き込みます。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root := cmd.Root()
		commands := []map[string]any{}
		for _, c := range root.Commands() {
			if (c.Hidden && !agentIncludeHidden) || c.Name() == "help" || c.Name() == "completion" {
				continue
			}
			commands = append(commands, agentDescribe(c, false)...)
		}
		return printJSON(map[string]any{
			"binary": root.Name(),
			"about":  root.Short,
			"conventions": []string{
				"出力はJSON（一部のSNS系コマンドはテキスト）",
				"mutates=true のコマンドは既定でdry-run。実行は --execute を付けたときだけ",
				"Garoopサービス（api.garoop.jp）のログインが必要なコマンドは `garooptv-cli session-set-cookie --cookie \"sessionId=...\"` でセッションを保存してから使う",
				"garoop-data（data.garoop.jp）へのコンテンツ公開はPR作成。`gh auth login` または GITHUB_TOKEN が必要",
				"--description / --detail / --prompt などは @path でファイルから読み込める",
			},
			"globalFlags": agentFlags(root.PersistentFlags()),
			"commands":    commands,
		})
	},
}

func agentDescribe(c *cobra.Command, parentMutates bool) []map[string]any {
	mutates := parentMutates || c.Annotations[annotationMutates] == "true"
	if !c.HasSubCommands() {
		entry := map[string]any{
			"command": c.CommandPath(),
			"usage":   c.UseLine(),
			"summary": c.Short,
			"mutates": mutates,
			"flags":   agentFlags(c.LocalNonPersistentFlags()),
		}
		if c.Example != "" {
			entry["example"] = strings.TrimSpace(c.Example)
		}
		return []map[string]any{entry}
	}
	out := []map[string]any{}
	for _, sub := range c.Commands() {
		if sub.Hidden || sub.Name() == "help" {
			continue
		}
		out = append(out, agentDescribe(sub, mutates)...)
	}
	return out
}

func agentFlags(fs *pflag.FlagSet) []map[string]any {
	flags := []map[string]any{}
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		flags = append(flags, map[string]any{
			"name":    "--" + f.Name,
			"type":    f.Value.Type(),
			"default": f.DefValue,
			"usage":   f.Usage,
		})
	})
	return flags
}

func init() {
	agentManifestCmd.Flags().BoolVar(&agentIncludeHidden, "all", false, "他のバイナリ向けのコマンドも含める")

	// 既存の --execute 前提コマンドにも印を付ける
	markMutates(
		xPostCmd, xReplyCmd, xReplyGaroopCmd,
		instagramPostCmd, instagramPushGaruchanCmd, instagramCommentCmd, instagramLikeCmd,
		youtubeUploadCmd, youtubeCommentCmd, youtubeAutoReplyCmd,
		notePostCmd, stocksOrderCmd,
		garoopSocialConnectCmd, garoopSocialDisconnectCmd, garoopSocialXLikeCmd, garoopSocialXReplyCmd,
		garoopSocialXRetweetCmd, garoopSocialXQuoteCmd, garoopSocialInstagramCommentCmd,
		garoopSocialThreadsReplyCmd, garoopSocialYouTubeCommentCmd, garoopSocialYouTubeLikeCmd,
	)

	agentCmd.AddCommand(agentManifestCmd)
	rootCmd.AddCommand(agentCmd)
}
