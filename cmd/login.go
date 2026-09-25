package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopapi"
)

var loginEmail string

var loginCmd = &cobra.Command{
	Use:     "login",
	Short:   "Garoopにメールアドレスとパスワードでログイン（ブラウザ不要）",
	GroupID: groupServices,
	Annotations: map[string]string{
		annotationProfiles: strings.Join([]string{ProfileGaroop, ProfileGaruchan, ProfileGaroopTV}, ","),
	},
	Long: `メールアドレスで登録した Garoop アカウントにログインし、セッションを保存します。
パスワードは端末なら伏せ字で、AIエージェントから実行された場合は macOS の入力ダイアログで
本人が入力します。パスワードは保存しません（保存するのはセッションだけ。約24時間有効）。

Google / LINE などでログインしている場合は session-set-cookie を使ってください。`,
	Example: `  garoop-cli login --email you@example.com
  garoop-cli me`,
	RunE: func(cmd *cobra.Command, args []string) error {
		email := strings.TrimSpace(loginEmail)
		if email == "" {
			v, err := readPlain("メールアドレス")
			if err != nil {
				return err
			}
			email = v
		}
		if email == "" {
			return fmt.Errorf("メールアドレスが空です")
		}
		password, err := readSecret("Garoopのパスワード")
		if err != nil {
			return err
		}
		if password == "" {
			return fmt.Errorf("パスワードが空です")
		}
		client := garoopapi.NewClient()
		client.Cookie = ""
		if _, err := client.Login(`mutation MailLogin($input: MailLoginInput!) {
			mailLogin(input: $input) { id success }
		}`, map[string]any{"input": map[string]any{"mailAddress": email, "password": password}}); err != nil {
			if errors.Is(err, garoopapi.ErrNoSession) {
				return fmt.Errorf("ログインできませんでした。メールアドレスかパスワードが違います（Google / LINE で登録したアカウントは session-set-cookie を使ってください）")
			}
			return fmt.Errorf("ログインできませんでした: %v", err)
		}
		u, err := loginUser()
		if err != nil {
			return err
		}
		fmt.Printf("ログインしました: %v（保存先 %s）\n", u["name"], garoopapi.SessionPath())
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:     "logout",
	Short:   "Garoopからログアウトし、保存したセッションを消す",
	GroupID: groupServices,
	Annotations: map[string]string{
		annotationProfiles: strings.Join([]string{ProfileGaroop, ProfileGaruchan, ProfileGaroopTV}, ","),
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if garoopapi.NewClient().Cookie != "" {
			// サーバー側のセッションも消す。失敗しても手元のセッションは消す
			_, _ = garoopapi.NewClient().Query(`mutation Logout { logout { success } }`, nil)
		}
		if err := garoopapi.ClearSession(); err != nil {
			return err
		}
		fmt.Println("ログアウトしました")
		return nil
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginEmail, "email", "", "登録したメールアドレス（省略時は入力を求める）")
	rootCmd.AddCommand(loginCmd, logoutCmd)
}
