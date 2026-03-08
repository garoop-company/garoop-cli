package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"garoop-cli/internal/garoopapi"
	"github.com/spf13/cobra"
)

var (
	garoopProvider        string
	garoopRedirectURL     string
	garoopCookie          string
	garoopCookieFile      string
	garoopQuery           string
	garoopQueryFile       string
	garoopVariables       string
	garoopVariablesFile   string
	garoopSocialPlatform  string
	garoopCode            string
	garoopState           string
	garoopLimit           int
	garoopInput           string
	garoopTweetID         string
	garoopMediaID         string
	garoopThreadID        string
	garoopVideoID         string
	garoopText            string
	garoopParentCommentID string
)

var garoopAuthURLCmd = &cobra.Command{
	Use:     "auth-url",
	Short:   "Google/LINE/Facebook/TikTok/X のログインURLを取得",
	GroupID: "garooptv_cli",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopProvider) == "" {
			return fmt.Errorf("--provider が必要です")
		}
		if strings.TrimSpace(garoopRedirectURL) == "" {
			return fmt.Errorf("--redirect-url が必要です")
		}

		field, query, err := garoopapi.AuthURLQuery(garoopProvider)
		if err != nil {
			return err
		}

		client := garoopapi.NewClient()
		resp, err := client.Query(query, map[string]any{
			"redirectUrl": garoopRedirectURL,
		})
		if err != nil {
			return err
		}
		if len(resp.Errors) > 0 {
			return fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
		}
		u, _ := resp.Data[field].(string)
		if strings.TrimSpace(u) == "" {
			return fmt.Errorf("auth url が取得できませんでした")
		}
		fmt.Println(u)
		return nil
	},
}

var garoopSessionSetCookieCmd = &cobra.Command{
	Use:     "session-set-cookie",
	Short:   "GaroopのセッションCookieを保存 (tokens/garoop_session.json)",
	GroupID: "garooptv_cli",
	RunE: func(cmd *cobra.Command, args []string) error {
		cookie := strings.TrimSpace(garoopCookie)
		if cookie == "" && strings.TrimSpace(garoopCookieFile) != "" {
			b, err := os.ReadFile(garoopCookieFile)
			if err != nil {
				return err
			}
			cookie = strings.TrimSpace(string(b))
		}
		if cookie == "" {
			return fmt.Errorf("--cookie または --cookie-file が必要です")
		}
		if err := garoopapi.SaveCookie(cookie); err != nil {
			return err
		}
		fmt.Println("保存しました: tokens/garoop_session.json")
		return nil
	},
}

var garoopGQLCmd = &cobra.Command{
	Use:     "gql",
	Short:   "任意GraphQLリクエストを送信",
	GroupID: "garooptv_cli",
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.TrimSpace(garoopQuery)
		if query == "" && strings.TrimSpace(garoopQueryFile) != "" {
			b, err := os.ReadFile(garoopQueryFile)
			if err != nil {
				return err
			}
			query = strings.TrimSpace(string(b))
		}
		if query == "" {
			return fmt.Errorf("--query か --query-file を指定してください")
		}

		variables := map[string]any{}
		if strings.TrimSpace(garoopVariables) != "" {
			if err := json.Unmarshal([]byte(garoopVariables), &variables); err != nil {
				return fmt.Errorf("--variables はJSON形式で指定してください: %w", err)
			}
		}
		if strings.TrimSpace(garoopVariablesFile) != "" {
			b, err := os.ReadFile(garoopVariablesFile)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, &variables); err != nil {
				return fmt.Errorf("--variables-file はJSON形式で指定してください: %w", err)
			}
		}

		client := garoopapi.NewClient()
		resp, err := client.Query(query, variables)
		if err != nil {
			return err
		}
		pretty, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(pretty))
		return nil
	},
}

var garoopSocialCmd = &cobra.Command{
	Use:     "social",
	Short:   "SNS連携・SNSアクションを扱う",
	GroupID: "garooptv_cli",
}

var garoopSocialAuthURLCmd = &cobra.Command{
	Use:   "auth-url",
	Short: "SNS連携用のOAuth開始URLを取得",
	RunE: func(cmd *cobra.Command, args []string) error {
		platform, err := normalizeSocialPlatform(garoopSocialPlatform)
		if err != nil {
			return err
		}
		query := `query GetSocialConnectAuthUrl($platform: SocialPlatform!, $redirectUrl: String!) {
			getSocialConnectAuthUrl(platform: $platform, redirectUrl: $redirectUrl) {
				url
				platform
			}
		}`
		resp, err := garoopQueryAPI(query, map[string]any{
			"platform":    platform,
			"redirectUrl": strings.TrimSpace(garoopRedirectURL),
		})
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getSocialConnectAuthUrl"])
	},
}

var garoopSocialConnectionsCmd = &cobra.Command{
	Use:   "connections",
	Short: "現在のSNS接続状態を取得",
	RunE: func(cmd *cobra.Command, args []string) error {
		query := `query GetMySocialConnections {
			getMySocialConnections {
				platform
				connected
				providerUserId
				username
				expiresAt
				scopes
				updatedAt
			}
		}`
		resp, err := garoopQueryAPI(query, nil)
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getMySocialConnections"])
	},
}

var garoopSocialConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "SNSアカウントを接続する（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		platform, err := normalizeSocialPlatform(garoopSocialPlatform)
		if err != nil {
			return err
		}
		if strings.TrimSpace(garoopCode) == "" {
			return fmt.Errorf("--code が必要です")
		}
		if strings.TrimSpace(garoopRedirectURL) == "" {
			return fmt.Errorf("--redirect-url が必要です")
		}
		mutation := `mutation ConnectSocialAccount($input: ConnectSocialAccountInput!) {
			connectSocialAccount(input: $input) {
				success
			}
		}`
		variables := map[string]any{
			"input": map[string]any{
				"platform":    platform,
				"code":        strings.TrimSpace(garoopCode),
				"redirectUrl": strings.TrimSpace(garoopRedirectURL),
			},
		}
		if state := strings.TrimSpace(garoopState); state != "" {
			variables["input"].(map[string]any)["state"] = state
		}
		return runMutationOrDryRun("connectSocialAccount", mutation, variables)
	},
}

var garoopSocialDisconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "SNSアカウント接続を解除する（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		platform, err := normalizeSocialPlatform(garoopSocialPlatform)
		if err != nil {
			return err
		}
		mutation := `mutation DisconnectSocialAccount($platform: SocialPlatform!) {
			disconnectSocialAccount(platform: $platform) {
				success
			}
		}`
		return runMutationOrDryRun("disconnectSocialAccount", mutation, map[string]any{
			"platform": platform,
		})
	},
}

var garoopSocialReplyTargetsCmd = &cobra.Command{
	Use:   "reply-targets",
	Short: "既定の返信対象アカウント一覧を取得",
	RunE: func(cmd *cobra.Command, args []string) error {
		query := `query GetDefaultSocialReplyTargets {
			getDefaultSocialReplyTargets {
				platform
				accountId
				accountUrl
				username
			}
		}`
		resp, err := garoopQueryAPI(query, nil)
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getDefaultSocialReplyTargets"])
	},
}

var garoopSocialInstagramMediaCmd = &cobra.Command{
	Use:   "instagram-media",
	Short: "接続中Instagramのメディア一覧を取得",
	RunE: func(cmd *cobra.Command, args []string) error {
		query := `query GetMyInstagramMedia($limit: Int) {
			getMyInstagramMedia(limit: $limit) {
				id
				caption
				mediaType
				mediaProductType
				permalink
				timestamp
			}
		}`
		variables := map[string]any{}
		if garoopLimit > 0 {
			variables["limit"] = garoopLimit
		}
		resp, err := garoopQueryAPI(query, variables)
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getMyInstagramMedia"])
	},
}

var garoopSocialInstagramResolveCmd = &cobra.Command{
	Use:   "instagram-resolve",
	Short: "Instagram URL/shortcode から mediaId を解決",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopInput) == "" {
			return fmt.Errorf("--input が必要です")
		}
		query := `query ResolveInstagramMediaId($input: String!) {
			resolveInstagramMediaId(input: $input) {
				input
				found
				mediaId
				permalink
				matchedBy
			}
		}`
		resp, err := garoopQueryAPI(query, map[string]any{"input": strings.TrimSpace(garoopInput)})
		if err != nil {
			return err
		}
		return printJSON(resp.Data["resolveInstagramMediaId"])
	},
}

var garoopSocialXDebugCmd = &cobra.Command{
	Use:   "x-debug",
	Short: "X投稿のデバッグ情報を取得",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopTweetID) == "" {
			return fmt.Errorf("--tweet-id が必要です")
		}
		query := `query GetXTweetDebug($tweetId: String!) {
			getXTweetDebug(tweetId: $tweetId) {
				id
				authorId
				conversationId
				replySettings
				text
			}
		}`
		resp, err := garoopQueryAPI(query, map[string]any{"tweetId": strings.TrimSpace(garoopTweetID)})
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getXTweetDebug"])
	},
}

var garoopSocialXLikeCmd = &cobra.Command{
	Use:   "x-like",
	Short: "X投稿にいいねする（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopTweetID) == "" {
			return fmt.Errorf("--tweet-id が必要です")
		}
		mutation := `mutation LikeOnX($input: LikeOnXInput!) {
			likeOnX(input: $input) {
				success
				platform
				targetId
				responseId
				message
			}
		}`
		return runMutationOrDryRun("likeOnX", mutation, map[string]any{
			"input": map[string]any{"tweetId": strings.TrimSpace(garoopTweetID)},
		})
	},
}

var garoopSocialXReplyCmd = &cobra.Command{
	Use:   "x-reply",
	Short: "X投稿へ返信する（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopTweetID) == "" {
			return fmt.Errorf("--tweet-id が必要です")
		}
		if strings.TrimSpace(garoopText) == "" {
			return fmt.Errorf("--text が必要です")
		}
		mutation := `mutation ReplyOnX($input: ReplyOnXInput!) {
			replyOnX(input: $input) {
				success
				platform
				targetId
				responseId
				message
			}
		}`
		return runMutationOrDryRun("replyOnX", mutation, map[string]any{
			"input": map[string]any{"tweetId": strings.TrimSpace(garoopTweetID), "text": strings.TrimSpace(garoopText)},
		})
	},
}

var garoopSocialXRetweetCmd = &cobra.Command{
	Use:   "x-retweet",
	Short: "X投稿をリポストする（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopTweetID) == "" {
			return fmt.Errorf("--tweet-id が必要です")
		}
		mutation := `mutation RetweetOnX($input: RetweetOnXInput!) {
			retweetOnX(input: $input) {
				success
				platform
				targetId
				responseId
				message
			}
		}`
		return runMutationOrDryRun("retweetOnX", mutation, map[string]any{
			"input": map[string]any{"tweetId": strings.TrimSpace(garoopTweetID)},
		})
	},
}

var garoopSocialXQuoteCmd = &cobra.Command{
	Use:   "x-quote",
	Short: "X投稿を引用する（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopTweetID) == "" {
			return fmt.Errorf("--tweet-id が必要です")
		}
		if strings.TrimSpace(garoopText) == "" {
			return fmt.Errorf("--text が必要です")
		}
		mutation := `mutation QuoteOnX($input: QuoteOnXInput!) {
			quoteOnX(input: $input) {
				success
				platform
				targetId
				responseId
				message
			}
		}`
		return runMutationOrDryRun("quoteOnX", mutation, map[string]any{
			"input": map[string]any{"tweetId": strings.TrimSpace(garoopTweetID), "text": strings.TrimSpace(garoopText)},
		})
	},
}

var garoopSocialInstagramCommentCmd = &cobra.Command{
	Use:   "instagram-comment",
	Short: "Instagram投稿へコメントする（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopMediaID) == "" {
			return fmt.Errorf("--media-id が必要です")
		}
		if strings.TrimSpace(garoopText) == "" {
			return fmt.Errorf("--text が必要です")
		}
		input := map[string]any{"mediaId": strings.TrimSpace(garoopMediaID), "text": strings.TrimSpace(garoopText)}
		if parentCommentID := strings.TrimSpace(garoopParentCommentID); parentCommentID != "" {
			input["parentCommentId"] = parentCommentID
		}
		mutation := `mutation CommentOnInstagram($input: CommentOnInstagramInput!) {
			commentOnInstagram(input: $input) {
				success
				platform
				targetId
				responseId
				message
			}
		}`
		return runMutationOrDryRun("commentOnInstagram", mutation, map[string]any{"input": input})
	},
}

var garoopSocialThreadsReplyCmd = &cobra.Command{
	Use:   "threads-reply",
	Short: "Threads投稿へ返信する（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopThreadID) == "" {
			return fmt.Errorf("--thread-id が必要です")
		}
		if strings.TrimSpace(garoopText) == "" {
			return fmt.Errorf("--text が必要です")
		}
		mutation := `mutation ReplyOnThreads($input: ReplyOnThreadsInput!) {
			replyOnThreads(input: $input) {
				success
				platform
				targetId
				responseId
				message
			}
		}`
		return runMutationOrDryRun("replyOnThreads", mutation, map[string]any{
			"input": map[string]any{"threadId": strings.TrimSpace(garoopThreadID), "text": strings.TrimSpace(garoopText)},
		})
	},
}

var garoopSocialYouTubeCommentCmd = &cobra.Command{
	Use:   "youtube-comment",
	Short: "YouTube動画へコメントする（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopVideoID) == "" {
			return fmt.Errorf("--video-id が必要です")
		}
		if strings.TrimSpace(garoopText) == "" {
			return fmt.Errorf("--text が必要です")
		}
		input := map[string]any{"videoId": strings.TrimSpace(garoopVideoID), "text": strings.TrimSpace(garoopText)}
		if parentCommentID := strings.TrimSpace(garoopParentCommentID); parentCommentID != "" {
			input["parentCommentId"] = parentCommentID
		}
		mutation := `mutation CommentOnYoutube($input: CommentOnYoutubeInput!) {
			commentOnYoutube(input: $input) {
				success
				platform
				targetId
				responseId
				message
			}
		}`
		return runMutationOrDryRun("commentOnYoutube", mutation, map[string]any{"input": input})
	},
}

var garoopSocialYouTubeLikeCmd = &cobra.Command{
	Use:   "youtube-like",
	Short: "YouTube動画に高評価する（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(garoopVideoID) == "" {
			return fmt.Errorf("--video-id が必要です")
		}
		mutation := `mutation LikeOnYoutube($input: LikeOnYoutubeInput!) {
			likeOnYoutube(input: $input) {
				success
				platform
				targetId
				responseId
				message
			}
		}`
		return runMutationOrDryRun("likeOnYoutube", mutation, map[string]any{
			"input": map[string]any{"videoId": strings.TrimSpace(garoopVideoID)},
		})
	},
}

func garoopQueryAPI(query string, variables map[string]any) (*garoopapi.Response, error) {
	client := garoopapi.NewClient()
	resp, err := client.Query(query, variables)
	if err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
	}
	return resp, nil
}

func runMutationOrDryRun(field string, query string, variables map[string]any) error {
	if !executeMode {
		payload := map[string]any{
			"mode":      "dry-run",
			"field":     field,
			"query":     query,
			"variables": variables,
		}
		return printJSON(payload)
	}
	resp, err := garoopQueryAPI(query, variables)
	if err != nil {
		return err
	}
	return printJSON(resp.Data[field])
}

func printJSON(v any) error {
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(pretty))
	return nil
}

func normalizeSocialPlatform(platform string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "x", "twitter":
		return "X", nil
	case "instagram", "ig":
		return "INSTAGRAM", nil
	case "threads", "thread":
		return "THREADS", nil
	case "youtube", "yt":
		return "YOUTUBE", nil
	default:
		return "", fmt.Errorf("unsupported social platform: %s", platform)
	}
}

func init() {
	garoopAuthURLCmd.Flags().StringVar(&garoopProvider, "provider", "", "google|line|facebook|tiktok|x")
	garoopAuthURLCmd.Flags().StringVar(&garoopRedirectURL, "redirect-url", "https://create.garoop.jp", "OAuth redirect URL")

	garoopSessionSetCookieCmd.Flags().StringVar(&garoopCookie, "cookie", "", "Cookieヘッダ文字列")
	garoopSessionSetCookieCmd.Flags().StringVar(&garoopCookieFile, "cookie-file", "", "Cookie文字列ファイル")

	garoopGQLCmd.Flags().StringVar(&garoopQuery, "query", "", "GraphQLクエリ文字列")
	garoopGQLCmd.Flags().StringVar(&garoopQueryFile, "query-file", "", "GraphQLクエリファイル")
	garoopGQLCmd.Flags().StringVar(&garoopVariables, "variables", "", "JSON variables")
	garoopGQLCmd.Flags().StringVar(&garoopVariablesFile, "variables-file", "", "JSON variables file")

	garoopSocialAuthURLCmd.Flags().StringVar(&garoopSocialPlatform, "platform", "", "x|instagram|threads|youtube")
	garoopSocialAuthURLCmd.Flags().StringVar(&garoopRedirectURL, "redirect-url", "https://create.garoop.jp", "OAuth redirect URL")
	garoopSocialConnectCmd.Flags().StringVar(&garoopSocialPlatform, "platform", "", "x|instagram|threads|youtube")
	garoopSocialConnectCmd.Flags().StringVar(&garoopCode, "code", "", "OAuth callback code")
	garoopSocialConnectCmd.Flags().StringVar(&garoopState, "state", "", "OAuth state (Xなどで必要)")
	garoopSocialConnectCmd.Flags().StringVar(&garoopRedirectURL, "redirect-url", "https://create.garoop.jp", "OAuth redirect URL")
	garoopSocialDisconnectCmd.Flags().StringVar(&garoopSocialPlatform, "platform", "", "x|instagram|threads|youtube")
	garoopSocialInstagramMediaCmd.Flags().IntVar(&garoopLimit, "limit", 10, "取得件数")
	garoopSocialInstagramResolveCmd.Flags().StringVar(&garoopInput, "input", "", "Instagram URL / shortcode / mediaId")
	garoopSocialXDebugCmd.Flags().StringVar(&garoopTweetID, "tweet-id", "", "X tweet ID")
	garoopSocialXLikeCmd.Flags().StringVar(&garoopTweetID, "tweet-id", "", "X tweet ID")
	garoopSocialXReplyCmd.Flags().StringVar(&garoopTweetID, "tweet-id", "", "X tweet ID")
	garoopSocialXReplyCmd.Flags().StringVar(&garoopText, "text", "", "返信本文")
	garoopSocialXRetweetCmd.Flags().StringVar(&garoopTweetID, "tweet-id", "", "X tweet ID")
	garoopSocialXQuoteCmd.Flags().StringVar(&garoopTweetID, "tweet-id", "", "X tweet ID")
	garoopSocialXQuoteCmd.Flags().StringVar(&garoopText, "text", "", "引用本文")
	garoopSocialInstagramCommentCmd.Flags().StringVar(&garoopMediaID, "media-id", "", "Instagram media ID")
	garoopSocialInstagramCommentCmd.Flags().StringVar(&garoopText, "text", "", "コメント本文")
	garoopSocialInstagramCommentCmd.Flags().StringVar(&garoopParentCommentID, "parent-comment-id", "", "親コメントID（返信時）")
	garoopSocialThreadsReplyCmd.Flags().StringVar(&garoopThreadID, "thread-id", "", "Threads post ID")
	garoopSocialThreadsReplyCmd.Flags().StringVar(&garoopText, "text", "", "返信本文")
	garoopSocialYouTubeCommentCmd.Flags().StringVar(&garoopVideoID, "video-id", "", "YouTube video ID")
	garoopSocialYouTubeCommentCmd.Flags().StringVar(&garoopText, "text", "", "コメント本文")
	garoopSocialYouTubeCommentCmd.Flags().StringVar(&garoopParentCommentID, "parent-comment-id", "", "親コメントID（返信時）")
	garoopSocialYouTubeLikeCmd.Flags().StringVar(&garoopVideoID, "video-id", "", "YouTube video ID")

	garoopSocialCmd.AddCommand(
		garoopSocialAuthURLCmd,
		garoopSocialConnectionsCmd,
		garoopSocialConnectCmd,
		garoopSocialDisconnectCmd,
		garoopSocialReplyTargetsCmd,
		garoopSocialInstagramMediaCmd,
		garoopSocialInstagramResolveCmd,
		garoopSocialXDebugCmd,
		garoopSocialXLikeCmd,
		garoopSocialXReplyCmd,
		garoopSocialXRetweetCmd,
		garoopSocialXQuoteCmd,
		garoopSocialInstagramCommentCmd,
		garoopSocialThreadsReplyCmd,
		garoopSocialYouTubeCommentCmd,
		garoopSocialYouTubeLikeCmd,
	)

	rootCmd.AddCommand(garoopAuthURLCmd, garoopSessionSetCookieCmd, garoopGQLCmd, garoopSocialCmd)
}
