package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopapi"
)

var (
	loginEmail string
	loginCode  bool
)

// cliLoginPageURL は Google / LINE で登録した人がコードを出すページ（kids_web の /cli-login）。
func cliLoginPageURL() string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("GAROOP_WEB_URL")), "/")
	if base == "" {
		base = "https://create.garoop.jp"
	}
	return base + "/ja/cli-login"
}

var loginCmd = &cobra.Command{
	Use:     "login",
	Short:   "Garoopにログイン（メールはパスワードで、Google / LINE は --code で）",
	GroupID: groupServices,
	Annotations: map[string]string{
		annotationProfiles: strings.Join([]string{ProfileGaroop, ProfileGaruchan, ProfileGaroopTV}, ","),
	},
	Long: `メールアドレスで登録した Garoop アカウントにログインし、セッションを保存します。
パスワードは端末なら伏せ字で、AIエージェントから実行された場合は macOS の入力ダイアログで
本人が入力します。パスワードは保存しません（保存するのはセッションだけ。約24時間有効）。

Google / LINE などで登録した人は --code を使います。ブラウザで
create.garoop.jp の「CLIにログイン」ページが開くので、「コードを出す」を押して、
出てきたコードを入力欄に貼り付けます（5分間・1回限り）。この場合はブラウザと同じ
ログインを使うので、logout してもブラウザのログインはそのままです。`,
	Example: `  garoop-cli login --email you@example.com   # メールで登録した人
  garoop-cli login --code                    # Google / LINE などで登録した人
  garoop-cli me`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if loginCode {
			return loginWithTransferCode()
		}
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
		}`, map[string]any{"input": map[string]any{"mailAddress": email, "password": password}}, "mail"); err != nil {
			if errors.Is(err, garoopapi.ErrNoSession) {
				return fmt.Errorf("ログインできませんでした。メールアドレスかパスワードが違います（Google / LINE で登録したアカウントは `login --code` を使ってください）")
			}
			return fmt.Errorf("ログインできませんでした: %v", err)
		}
		return reportLogin()
	},
}

// loginWithTransferCode はブラウザで発行したワンタイムコードでログインする。
func loginWithTransferCode() error {
	page := cliLoginPageURL()
	fmt.Fprintf(os.Stderr, "ブラウザで %s を開き、「コードを出す」を押してコードを貼り付けてください\n", page)
	_ = openBrowser(page) // 開けなくても URL は表示済み
	code, err := readSecret("ログイン用コード")
	if err != nil {
		return err
	}
	if code == "" {
		return fmt.Errorf("コードが空です")
	}
	client := garoopapi.NewClient()
	client.Cookie = ""
	if _, err := client.Login(`mutation ExchangeSessionTransferToken($token: String!) {
		exchangeSessionTransferToken(token: $token) { id success }
	}`, map[string]any{"token": code}, "transfer"); err != nil {
		if errors.Is(err, garoopapi.ErrNoSession) {
			return fmt.Errorf("ログインできませんでした。コードの期限（5分）が切れたか、使用済みです。%s でコードを出し直してください", page)
		}
		return fmt.Errorf("ログインできませんでした: %v", err)
	}
	return reportLogin()
}

func reportLogin() error {
	u, err := loginUser()
	if err != nil {
		return err
	}
	fmt.Printf("ログインしました: %v（保存先 %s）\n", u["name"], garoopapi.SessionPath())
	return nil
}
