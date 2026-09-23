package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopapi"
)

// Garoop Baby（baby.garoop.jp）のオンライン赤ちゃん。kids_api の Baby 系 GraphQL を使う。
// ローカルの `garuchan-cli birth/feed` とは別に、サービス上の赤ちゃんを育てる。

const babySummaryFields = `id name animalType personality description avatarUrl growthLevel growthScore gentleness challenge vocabulary summary updatedAt`

var (
	babyLimit     int
	babyMediaType string
	babyText      string
	babyMediaURL  string
	babyScore     int
)

var babyCmd = serviceCommand("baby", "Garoop Baby: オンラインの赤ちゃん（ガルちゃん）の一覧・会話・育成", ProfileGaroop, ProfileGaruchan)

var babyListCmd = &cobra.Command{
	Use:   "list",
	Short: "自分の赤ちゃん一覧（ログイン必須）",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := garoopQueryAPI(`query { getMyBabies { `+babySummaryFields+` } }`, nil)
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getMyBabies"])
	},
}

var babyPublicCmd = &cobra.Command{
	Use:   "public",
	Short: "公開されている赤ちゃん一覧（ログイン不要）",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := garoopQueryAPI(`query GetPublicBabies($limit: Int) { getPublicBabies(limit: $limit) { id name animalType personality description avatarUrl growthLevel } }`,
			map[string]any{"limit": babyLimit})
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getPublicBabies"])
	},
}

var babyGetCmd = &cobra.Command{
	Use:   "get [babyId]",
	Short: "赤ちゃんの詳細と育成イベント",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("babyId は数値で指定してください")
		}
		resp, err := garoopQueryAPI(`query GetBaby($babyId: Int!) {
			getBaby(babyId: $babyId) { `+babySummaryFields+` personalityTraits abilities }
			getBabyGrowthEvents(babyId: $babyId) { id mediaType contentText mediaUrl growthScoreDelta createdAt }
		}`, map[string]any{"babyId": id})
		if err != nil {
			return err
		}
		baby, _ := resp.Data["getBaby"].(map[string]any)
		if baby == nil {
			return fmt.Errorf("赤ちゃん %d が見つかりません（本人の赤ちゃんのみ取得できます）", id)
		}
		return printJSON(map[string]any{"baby": baby, "growthEvents": resp.Data["getBabyGrowthEvents"]})
	},
}

var babyChatCmd = &cobra.Command{
	Use:   "chat [babyId] [message]",
	Short: "赤ちゃんと会話（会話ログと記憶に残るため既定はdry-run）",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("babyId は数値で指定してください")
		}
		query := `mutation ChatWithBaby($input: ChatWithBabyInput!) { chatWithBaby(input: $input) { reply } }`
		vars := map[string]any{"input": map[string]any{"babyId": id, "message": strings.TrimSpace(args[1])}}
		if !executeMode {
			return runMutationOrDryRun("chatWithBaby", query, vars)
		}
		resp, err := garoopapi.NewClient().WithTimeout(2*time.Minute).Query(query, vars)
		if err != nil {
			return err
		}
		if len(resp.Errors) > 0 {
			return fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
		}
		return printJSON(resp.Data["chatWithBaby"])
	},
}

var babyGrowCmd = &cobra.Command{
	Use:   "grow [babyId]",
	Short: "テキスト・画像・動画を与えて育てる（既定はdry-run）",
	Example: `  garuchan-cli baby grow 12 --type TEXT --text "今日は公園でどんぐりを拾ったよ"
  garuchan-cli baby grow 12 --type IMAGE --media-url https://example.com/park.jpg --text "公園の写真"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("babyId は数値で指定してください")
		}
		mediaType := strings.ToUpper(strings.TrimSpace(babyMediaType))
		if !containsString([]string{"TEXT", "IMAGE", "VIDEO"}, mediaType) {
			return fmt.Errorf("--type は TEXT|IMAGE|VIDEO のいずれかです")
		}
		input := map[string]any{"babyId": id, "mediaType": mediaType}
		if t := strings.TrimSpace(babyText); t != "" {
			input["contentText"] = t
		}
		if u := strings.TrimSpace(babyMediaURL); u != "" {
			input["mediaUrl"] = u
		}
		if mediaType == "TEXT" && input["contentText"] == nil {
			return fmt.Errorf("--type TEXT には --text が必要です")
		}
		if mediaType != "TEXT" && input["mediaUrl"] == nil {
			return fmt.Errorf("--type %s には --media-url が必要です", mediaType)
		}
		if babyScore > 0 {
			input["growthScoreDelta"] = babyScore
		}
		return runMutationOrDryRun("addBabyGrowthEvent", `mutation AddBabyGrowthEvent($input: AddBabyGrowthEventInput!) {
			addBabyGrowthEvent(input: $input) { id mediaType contentText mediaUrl growthScoreDelta createdAt }
		}`, map[string]any{"input": input})
	},
}

func init() {
	babyPublicCmd.Flags().IntVar(&babyLimit, "limit", 20, "取得件数（最大50）")
	babyGrowCmd.Flags().StringVar(&babyMediaType, "type", "TEXT", "TEXT|IMAGE|VIDEO")
	babyGrowCmd.Flags().StringVar(&babyText, "text", "", "与える内容・説明")
	babyGrowCmd.Flags().StringVar(&babyMediaURL, "media-url", "", "画像・動画のURL")
	babyGrowCmd.Flags().IntVar(&babyScore, "score", 0, "成長スコア加算（省略時はサーバー既定）")

	markMutates(babyChatCmd, babyGrowCmd)
	babyCmd.AddCommand(babyListCmd, babyPublicCmd, babyGetCmd, babyChatCmd, babyGrowCmd)
	rootCmd.AddCommand(babyCmd)
}
