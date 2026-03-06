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
	garoopProvider      string
	garoopRedirectURL   string
	garoopCookie        string
	garoopCookieFile    string
	garoopQuery         string
	garoopQueryFile     string
	garoopVariables     string
	garoopVariablesFile string
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

func init() {
	garoopAuthURLCmd.Flags().StringVar(&garoopProvider, "provider", "", "google|line|facebook|tiktok|x")
	garoopAuthURLCmd.Flags().StringVar(&garoopRedirectURL, "redirect-url", "https://create.garoop.jp", "OAuth redirect URL")

	garoopSessionSetCookieCmd.Flags().StringVar(&garoopCookie, "cookie", "", "Cookieヘッダ文字列")
	garoopSessionSetCookieCmd.Flags().StringVar(&garoopCookieFile, "cookie-file", "", "Cookie文字列ファイル")

	garoopGQLCmd.Flags().StringVar(&garoopQuery, "query", "", "GraphQLクエリ文字列")
	garoopGQLCmd.Flags().StringVar(&garoopQueryFile, "query-file", "", "GraphQLクエリファイル")
	garoopGQLCmd.Flags().StringVar(&garoopVariables, "variables", "", "JSON variables")
	garoopGQLCmd.Flags().StringVar(&garoopVariablesFile, "variables-file", "", "JSON variables file")

	rootCmd.AddCommand(garoopAuthURLCmd, garoopSessionSetCookieCmd, garoopGQLCmd)
}
