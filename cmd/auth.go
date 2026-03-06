package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"garoop-cli/internal/authutil"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/dghubble/oauth1"
	"github.com/spf13/cobra"
)

const (
	xTokenPath         = "tokens/x.json"
	youtubeTokenPath   = "tokens/youtube.json"
	instagramTokenPath = "tokens/instagram.json"
	noteCookiePath     = "tokens/note_cookie.json"
)

type noteCookieEntry struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	HttpOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
}

var (
	authCode         string
	authRedirectURI  string
	authVerifier     string
	noteCookieInput  string
	authVerifyOnline bool
	authOpenBrowser  bool
	authLoginTimeout time.Duration
	noteEmail        string
	notePassword     string
)

var authCmd = &cobra.Command{
	Use:     "auth",
	Short:   "各サービスの認証設定",
	GroupID: "garoop_cli",
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "保存済み認証情報の状態を表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		printTokenStatus("X", xTokenPath)
		printTokenStatus("YouTube", youtubeTokenPath)
		printTokenStatus("Instagram", instagramTokenPath)
		printTokenStatus("Note", noteCookiePath)
		return nil
	},
}

var authVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "認証設定を検証します（--onlineでAPI疎通確認）",
	RunE: func(cmd *cobra.Command, args []string) error {
		failed := 0
		checks := []struct {
			name string
			fn   func(bool) error
		}{
			{name: "X", fn: verifyXAuth},
			{name: "YouTube", fn: verifyYouTubeAuth},
			{name: "Instagram", fn: verifyInstagramAuth},
			{name: "Note", fn: verifyNoteAuth},
		}

		for _, c := range checks {
			if err := c.fn(authVerifyOnline); err != nil {
				failed++
				fmt.Printf("%s: NG (%v)\n", c.name, err)
			} else {
				if authVerifyOnline {
					fmt.Printf("%s: OK (online)\n", c.name)
				} else {
					fmt.Printf("%s: OK (local)\n", c.name)
				}
			}
		}

		if failed > 0 {
			return fmt.Errorf("認証検証に失敗: %dサービス", failed)
		}
		return nil
	},
}

var authXCmd = &cobra.Command{
	Use:   "x",
	Short: "X認証",
}

var authXLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "X OAuth1.0aの認証を自動実行してaccess tokenを保存します",
	RunE: func(cmd *cobra.Command, args []string) error {
		consumerKey := strings.TrimSpace(os.Getenv("X_CONSUMER_KEY"))
		consumerSecret := strings.TrimSpace(os.Getenv("X_CONSUMER_SECRET"))
		if consumerKey == "" || consumerSecret == "" {
			return fmt.Errorf("X_CONSUMER_KEY / X_CONSUMER_SECRET を先に設定してください")
		}
		if strings.TrimSpace(authRedirectURI) == "" {
			return fmt.Errorf("--redirect-uri を指定してください")
		}

		config := oauth1.NewConfig(consumerKey, consumerSecret)
		config.CallbackURL = authRedirectURI

		requestToken, requestSecret, err := config.RequestToken()
		if err != nil {
			return err
		}
		authURL, err := config.AuthorizationURL(requestToken)
		if err != nil {
			return err
		}

		if authOpenBrowser {
			if err := openBrowser(authURL.String()); err != nil {
				return err
			}
		} else {
			fmt.Printf("このURLをブラウザで開いてください:\n%s\n", authURL.String())
		}
		values, err := waitForOAuthCallback(authRedirectURI, authLoginTimeout)
		if err != nil {
			return err
		}
		if token := strings.TrimSpace(values.Get("oauth_token")); token != "" && token != requestToken {
			return fmt.Errorf("oauth_token不一致: expected=%s got=%s", requestToken, token)
		}
		verifier := strings.TrimSpace(values.Get("oauth_verifier"))
		if verifier == "" {
			verifier = strings.TrimSpace(authVerifier)
		}
		if verifier == "" {
			return fmt.Errorf("oauth_verifierが空です")
		}

		accessToken, accessSecret, err := config.AccessToken(requestToken, requestSecret, verifier)
		if err != nil {
			return err
		}

		payload := map[string]string{
			"consumer_key":        consumerKey,
			"consumer_secret":     consumerSecret,
			"access_token":        accessToken,
			"access_token_secret": accessSecret,
		}
		if err := authutil.SaveJSON(xTokenPath, payload); err != nil {
			return err
		}
		fmt.Printf("保存しました: %s\n", xTokenPath)
		return nil
	},
}

var authYouTubeCmd = &cobra.Command{
	Use:   "youtube",
	Short: "YouTube OAuth認証",
}

var authYouTubeLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "YouTube OAuth認証を自動実行してtoken保存",
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := strings.TrimSpace(os.Getenv("YOUTUBE_CLIENT_ID"))
		clientSecret := strings.TrimSpace(os.Getenv("YOUTUBE_CLIENT_SECRET"))
		if clientID == "" || clientSecret == "" {
			return fmt.Errorf("YOUTUBE_CLIENT_ID / YOUTUBE_CLIENT_SECRET を設定してください")
		}
		if strings.TrimSpace(authRedirectURI) == "" {
			return fmt.Errorf("--redirect-uri を指定してください")
		}

		state, err := newOAuthState()
		if err != nil {
			return err
		}
		scope := url.QueryEscape("https://www.googleapis.com/auth/youtube https://www.googleapis.com/auth/youtube.upload")
		authURL := fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&access_type=offline&prompt=consent&state=%s",
			url.QueryEscape(clientID),
			url.QueryEscape(authRedirectURI),
			scope,
			url.QueryEscape(state),
		)
		if authOpenBrowser {
			if err := openBrowser(authURL); err != nil {
				return err
			}
		} else {
			fmt.Printf("このURLをブラウザで開いてください:\n%s\n", authURL)
		}

		values, err := waitForOAuthCallback(authRedirectURI, authLoginTimeout)
		if err != nil {
			return err
		}
		if strings.TrimSpace(values.Get("state")) != state {
			return fmt.Errorf("stateが一致しません")
		}
		code := strings.TrimSpace(values.Get("code"))
		if code == "" {
			return fmt.Errorf("codeが取得できませんでした")
		}
		return exchangeYouTubeToken(clientID, clientSecret, authRedirectURI, code)
	},
}

var authYouTubeURLCmd = &cobra.Command{
	Use:   "url",
	Short: "YouTube OAuth認証URLを表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := strings.TrimSpace(os.Getenv("YOUTUBE_CLIENT_ID"))
		if clientID == "" {
			return fmt.Errorf("YOUTUBE_CLIENT_ID を設定してください")
		}
		if strings.TrimSpace(authRedirectURI) == "" {
			return fmt.Errorf("--redirect-uri を指定してください")
		}
		scope := url.QueryEscape("https://www.googleapis.com/auth/youtube https://www.googleapis.com/auth/youtube.upload")
		u := fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&access_type=offline&prompt=consent",
			url.QueryEscape(clientID),
			url.QueryEscape(authRedirectURI),
			scope,
		)
		fmt.Println(u)
		return nil
	},
}

var authYouTubeExchangeCmd = &cobra.Command{
	Use:   "exchange",
	Short: "YouTube認証コードをtokenへ交換して保存",
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := strings.TrimSpace(os.Getenv("YOUTUBE_CLIENT_ID"))
		clientSecret := strings.TrimSpace(os.Getenv("YOUTUBE_CLIENT_SECRET"))
		if clientID == "" || clientSecret == "" {
			return fmt.Errorf("YOUTUBE_CLIENT_ID / YOUTUBE_CLIENT_SECRET を設定してください")
		}
		if strings.TrimSpace(authCode) == "" || strings.TrimSpace(authRedirectURI) == "" {
			return fmt.Errorf("--code と --redirect-uri が必要です")
		}

		return exchangeYouTubeToken(clientID, clientSecret, authRedirectURI, authCode)
	},
}

var authYouTubeRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "YouTube access tokenを更新",
	RunE: func(cmd *cobra.Command, args []string) error {
		var token map[string]any
		if err := authutil.LoadJSON(youtubeTokenPath, &token); err != nil {
			return err
		}
		refreshToken := asString(token["refresh_token"])
		clientID := asString(token["client_id"])
		clientSecret := asString(token["client_secret"])
		if refreshToken == "" || clientID == "" || clientSecret == "" {
			return fmt.Errorf("%s に refresh_token/client_id/client_secret が必要です", youtubeTokenPath)
		}

		form := url.Values{}
		form.Set("client_id", clientID)
		form.Set("client_secret", clientSecret)
		form.Set("refresh_token", refreshToken)
		form.Set("grant_type", "refresh_token")
		resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 300 {
			return fmt.Errorf("token更新失敗: %s", string(body))
		}

		var refreshed map[string]any
		if err := json.Unmarshal(body, &refreshed); err != nil {
			return err
		}
		for k, v := range refreshed {
			token[k] = v
		}
		token["saved_at"] = time.Now().Format(time.RFC3339)
		if err := authutil.SaveJSON(youtubeTokenPath, token); err != nil {
			return err
		}
		fmt.Printf("更新しました: %s\n", youtubeTokenPath)
		return nil
	},
}

var authInstagramCmd = &cobra.Command{
	Use:   "instagram",
	Short: "Instagram(Facebook Graph)認証",
}

var authInstagramLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Instagram(Facebook) OAuth認証を自動実行してtoken保存",
	RunE: func(cmd *cobra.Command, args []string) error {
		appID := strings.TrimSpace(os.Getenv("INSTAGRAM_APP_ID"))
		appSecret := strings.TrimSpace(os.Getenv("INSTAGRAM_APP_SECRET"))
		if appID == "" || appSecret == "" {
			return fmt.Errorf("INSTAGRAM_APP_ID / INSTAGRAM_APP_SECRET を設定してください")
		}
		if strings.TrimSpace(authRedirectURI) == "" {
			return fmt.Errorf("--redirect-uri を指定してください")
		}
		state, err := newOAuthState()
		if err != nil {
			return err
		}
		scope := "instagram_basic,instagram_content_publish,pages_show_list,pages_read_engagement"
		authURL := fmt.Sprintf(
			"https://www.facebook.com/v22.0/dialog/oauth?client_id=%s&redirect_uri=%s&scope=%s&response_type=code&state=%s",
			url.QueryEscape(appID),
			url.QueryEscape(authRedirectURI),
			url.QueryEscape(scope),
			url.QueryEscape(state),
		)
		if authOpenBrowser {
			if err := openBrowser(authURL); err != nil {
				return err
			}
		} else {
			fmt.Printf("このURLをブラウザで開いてください:\n%s\n", authURL)
		}
		values, err := waitForOAuthCallback(authRedirectURI, authLoginTimeout)
		if err != nil {
			return err
		}
		if strings.TrimSpace(values.Get("state")) != state {
			return fmt.Errorf("stateが一致しません")
		}
		code := strings.TrimSpace(values.Get("code"))
		if code == "" {
			return fmt.Errorf("codeが取得できませんでした")
		}
		return exchangeInstagramToken(appID, appSecret, authRedirectURI, code)
	},
}

var authInstagramURLCmd = &cobra.Command{
	Use:   "url",
	Short: "Instagram連携のFacebook認証URLを表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		appID := strings.TrimSpace(os.Getenv("INSTAGRAM_APP_ID"))
		if appID == "" {
			return fmt.Errorf("INSTAGRAM_APP_ID を設定してください")
		}
		if strings.TrimSpace(authRedirectURI) == "" {
			return fmt.Errorf("--redirect-uri を指定してください")
		}
		scope := "instagram_basic,instagram_content_publish,pages_show_list,pages_read_engagement"
		u := fmt.Sprintf(
			"https://www.facebook.com/v22.0/dialog/oauth?client_id=%s&redirect_uri=%s&scope=%s&response_type=code",
			url.QueryEscape(appID),
			url.QueryEscape(authRedirectURI),
			url.QueryEscape(scope),
		)
		fmt.Println(u)
		return nil
	},
}

var authInstagramExchangeCmd = &cobra.Command{
	Use:   "exchange",
	Short: "Instagram(Facebook)認証コードを交換してtoken保存",
	RunE: func(cmd *cobra.Command, args []string) error {
		appID := strings.TrimSpace(os.Getenv("INSTAGRAM_APP_ID"))
		appSecret := strings.TrimSpace(os.Getenv("INSTAGRAM_APP_SECRET"))
		if appID == "" || appSecret == "" {
			return fmt.Errorf("INSTAGRAM_APP_ID / INSTAGRAM_APP_SECRET を設定してください")
		}
		if strings.TrimSpace(authCode) == "" || strings.TrimSpace(authRedirectURI) == "" {
			return fmt.Errorf("--code と --redirect-uri が必要です")
		}

		return exchangeInstagramToken(appID, appSecret, authRedirectURI, authCode)
	},
}

var authNoteCmd = &cobra.Command{
	Use:   "note",
	Short: "Note認証情報",
}

var authNoteSetCookieCmd = &cobra.Command{
	Use:   "set-cookie",
	Short: "note.comのcookie JSONを tokens/note_cookie.json に保存",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(noteCookieInput) == "" {
			return fmt.Errorf("--cookie-json を指定してください")
		}
		b, err := os.ReadFile(noteCookieInput)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(noteCookiePath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(noteCookiePath, b, 0o600); err != nil {
			return err
		}
		fmt.Printf("保存しました: %s\n", noteCookiePath)
		return nil
	},
}

var authNoteLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Noteにログインしてcookieを自動保存",
	RunE: func(cmd *cobra.Command, args []string) error {
		return loginNoteAndSaveCookie(noteEmail, notePassword, authLoginTimeout)
	},
}

func init() {
	authCmd.PersistentFlags().BoolVar(&authOpenBrowser, "open-browser", true, "認証URLを自動でブラウザ起動する")
	authCmd.PersistentFlags().DurationVar(&authLoginTimeout, "timeout", 3*time.Minute, "認証待機タイムアウト")

	authVerifyCmd.Flags().BoolVar(&authVerifyOnline, "online", false, "APIへ接続して実際に検証する")
	authXLoginCmd.Flags().StringVar(&authVerifier, "verifier", "", "フォールバック用 verifier")
	authXLoginCmd.Flags().StringVar(&authRedirectURI, "redirect-uri", "http://127.0.0.1:18766/auth/x/callback", "OAuth Redirect URI")

	authYouTubeLoginCmd.Flags().StringVar(&authRedirectURI, "redirect-uri", "http://127.0.0.1:18767/auth/youtube/callback", "OAuth Redirect URI")
	authYouTubeURLCmd.Flags().StringVar(&authRedirectURI, "redirect-uri", "", "OAuth Redirect URI")
	authYouTubeExchangeCmd.Flags().StringVar(&authCode, "code", "", "OAuth認証コード")
	authYouTubeExchangeCmd.Flags().StringVar(&authRedirectURI, "redirect-uri", "", "OAuth Redirect URI")

	authInstagramLoginCmd.Flags().StringVar(&authRedirectURI, "redirect-uri", "http://127.0.0.1:18768/auth/instagram/callback", "OAuth Redirect URI")
	authInstagramURLCmd.Flags().StringVar(&authRedirectURI, "redirect-uri", "", "OAuth Redirect URI")
	authInstagramExchangeCmd.Flags().StringVar(&authCode, "code", "", "OAuth認証コード")
	authInstagramExchangeCmd.Flags().StringVar(&authRedirectURI, "redirect-uri", "", "OAuth Redirect URI")

	authNoteSetCookieCmd.Flags().StringVar(&noteCookieInput, "cookie-json", "", "エクスポート済みcookie JSON")
	authNoteLoginCmd.Flags().StringVar(&noteEmail, "email", "", "Noteログインメール（任意）")
	authNoteLoginCmd.Flags().StringVar(&notePassword, "password", "", "Noteログインパスワード（任意）")

	authXCmd.AddCommand(authXLoginCmd)
	authYouTubeCmd.AddCommand(authYouTubeLoginCmd, authYouTubeURLCmd, authYouTubeExchangeCmd, authYouTubeRefreshCmd)
	authInstagramCmd.AddCommand(authInstagramLoginCmd, authInstagramURLCmd, authInstagramExchangeCmd)
	authNoteCmd.AddCommand(authNoteLoginCmd, authNoteSetCookieCmd)

	authCmd.AddCommand(authStatusCmd, authVerifyCmd, authXCmd, authYouTubeCmd, authInstagramCmd, authNoteCmd)
	rootCmd.AddCommand(authCmd)
}

func printTokenStatus(name, path string) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("%s: configured (%s)\n", name, path)
		return
	}
	fmt.Printf("%s: not configured\n", name)
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func fetchInstagramBusinessID(userToken string) (string, string, error) {
	u := "https://graph.facebook.com/v22.0/me/accounts?fields=id,name,instagram_business_account&access_token=" + url.QueryEscape(userToken)
	resp, err := http.Get(u)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("Instagram Business ID取得失敗: %s", string(body))
	}

	var out struct {
		Data []struct {
			ID                       string `json:"id"`
			InstagramBusinessAccount struct {
				ID string `json:"id"`
			} `json:"instagram_business_account"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", "", err
	}
	for _, page := range out.Data {
		if page.InstagramBusinessAccount.ID != "" {
			return page.InstagramBusinessAccount.ID, page.ID, nil
		}
	}
	return "", "", fmt.Errorf("instagram_business_account を持つページが見つかりません")
}

func verifyXAuth(online bool) error {
	consumerKey := strings.TrimSpace(os.Getenv("X_CONSUMER_KEY"))
	consumerSecret := strings.TrimSpace(os.Getenv("X_CONSUMER_SECRET"))
	accessToken := strings.TrimSpace(os.Getenv("X_ACCESS_TOKEN"))
	accessSecret := strings.TrimSpace(os.Getenv("X_ACCESS_TOKEN_SECRET"))
	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		var token struct {
			ConsumerKey       string `json:"consumer_key"`
			ConsumerSecret    string `json:"consumer_secret"`
			AccessToken       string `json:"access_token"`
			AccessTokenSecret string `json:"access_token_secret"`
		}
		if err := authutil.LoadJSON(xTokenPath, &token); err == nil {
			if consumerKey == "" {
				consumerKey = strings.TrimSpace(token.ConsumerKey)
			}
			if consumerSecret == "" {
				consumerSecret = strings.TrimSpace(token.ConsumerSecret)
			}
			if accessToken == "" {
				accessToken = strings.TrimSpace(token.AccessToken)
			}
			if accessSecret == "" {
				accessSecret = strings.TrimSpace(token.AccessTokenSecret)
			}
		}
	}
	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		return fmt.Errorf("credentials不足（env または %s）", xTokenPath)
	}
	if !online {
		return nil
	}

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	client := config.Client(context.Background(), token)
	req, err := http.NewRequest(http.MethodGet, "https://api.twitter.com/2/users/me", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API応答エラー: %s", strings.TrimSpace(string(b)))
	}
	return nil
}

func verifyYouTubeAuth(online bool) error {
	accessToken := strings.TrimSpace(os.Getenv("YOUTUBE_ACCESS_TOKEN"))
	if accessToken == "" {
		var token struct {
			AccessToken string `json:"access_token"`
		}
		if err := authutil.LoadJSON(youtubeTokenPath, &token); err == nil {
			accessToken = strings.TrimSpace(token.AccessToken)
		}
	}
	if accessToken == "" {
		return fmt.Errorf("access_token不足（env または %s）", youtubeTokenPath)
	}
	if !online {
		return nil
	}

	u := "https://oauth2.googleapis.com/tokeninfo?access_token=" + url.QueryEscape(accessToken)
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API応答エラー: %s", strings.TrimSpace(string(b)))
	}
	return nil
}

func verifyInstagramAuth(online bool) error {
	accessToken := strings.TrimSpace(os.Getenv("INSTAGRAM_ACCESS_TOKEN"))
	igUserID := strings.TrimSpace(os.Getenv("INSTAGRAM_IG_USER_ID"))
	if accessToken == "" || igUserID == "" {
		var token struct {
			AccessToken string `json:"access_token"`
			IGUserID    string `json:"ig_user_id"`
		}
		if err := authutil.LoadJSON(instagramTokenPath, &token); err == nil {
			if accessToken == "" {
				accessToken = strings.TrimSpace(token.AccessToken)
			}
			if igUserID == "" {
				igUserID = strings.TrimSpace(token.IGUserID)
			}
		}
	}
	if accessToken == "" || igUserID == "" {
		return fmt.Errorf("credentials不足（env または %s）", instagramTokenPath)
	}
	if !online {
		return nil
	}

	u := fmt.Sprintf("https://graph.facebook.com/v22.0/%s?fields=id,username&access_token=%s",
		url.QueryEscape(igUserID), url.QueryEscape(accessToken))
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API応答エラー: %s", strings.TrimSpace(string(b)))
	}
	return nil
}

func verifyNoteAuth(online bool) error {
	raw, err := os.ReadFile(noteCookiePath)
	if err != nil {
		return fmt.Errorf("cookieファイル読み込み失敗: %w", err)
	}
	var cookies []noteCookieEntry
	if err := json.Unmarshal(raw, &cookies); err != nil {
		return fmt.Errorf("cookie JSON解析失敗: %w", err)
	}
	if len(cookies) == 0 {
		return fmt.Errorf("cookieが空です")
	}
	if !online {
		return nil
	}

	jar, _ := cookiejar.New(nil)
	noteURL, _ := url.Parse("https://note.com")
	httpCookies := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		httpCookies = append(httpCookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HttpOnly: c.HttpOnly,
			Secure:   c.Secure,
		})
	}
	jar.SetCookies(noteURL, httpCookies)
	client := &http.Client{Jar: jar, Timeout: 20 * time.Second}

	req, err := http.NewRequest(http.MethodGet, "https://note.com/settings/profile", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API応答エラー: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func exchangeYouTubeToken(clientID, clientSecret, redirectURI, code string) error {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("token交換失敗: %s", string(body))
	}

	var token map[string]any
	if err := json.Unmarshal(body, &token); err != nil {
		return err
	}
	token["client_id"] = clientID
	token["client_secret"] = clientSecret
	token["saved_at"] = time.Now().Format(time.RFC3339)
	if err := authutil.SaveJSON(youtubeTokenPath, token); err != nil {
		return err
	}
	fmt.Printf("保存しました: %s\n", youtubeTokenPath)
	return nil
}

func exchangeInstagramToken(appID, appSecret, redirectURI, code string) error {
	tokenURL := fmt.Sprintf(
		"https://graph.facebook.com/v22.0/oauth/access_token?client_id=%s&redirect_uri=%s&client_secret=%s&code=%s",
		url.QueryEscape(appID), url.QueryEscape(redirectURI), url.QueryEscape(appSecret), url.QueryEscape(code),
	)
	resp, err := http.Get(tokenURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("token交換失敗: %s", string(body))
	}

	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil {
		return err
	}
	if token.AccessToken == "" {
		return fmt.Errorf("access_tokenが取得できませんでした")
	}

	igUserID, pageID, err := fetchInstagramBusinessID(token.AccessToken)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"access_token": token.AccessToken,
		"ig_user_id":   igUserID,
		"page_id":      pageID,
		"saved_at":     time.Now().Format(time.RFC3339),
	}
	if err := authutil.SaveJSON(instagramTokenPath, payload); err != nil {
		return err
	}
	fmt.Printf("保存しました: %s\n", instagramTokenPath)
	return nil
}

func waitForOAuthCallback(redirectURI string, timeout time.Duration) (url.Values, error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, fmt.Errorf("redirect-uriが不正です")
	}
	listener, err := net.Listen("tcp", u.Host)
	if err != nil {
		return nil, fmt.Errorf("callbackサーバ起動失敗: %w", err)
	}
	defer listener.Close()

	queryCh := make(chan url.Values, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(u.Path, func(w http.ResponseWriter, r *http.Request) {
		if errMsg := strings.TrimSpace(r.URL.Query().Get("error")); errMsg != "" {
			errCh <- fmt.Errorf("認証エラー: %s", errMsg)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Authentication failed. You can close this tab."))
			return
		}
		select {
		case queryCh <- r.URL.Query():
		default:
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Authentication successful. You can close this tab."))
	})
	srv := &http.Server{Handler: mux}
	go func() {
		if serveErr := srv.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case v := <-queryCh:
		return v, nil
	case e := <-errCh:
		return nil, e
	case <-timer.C:
		return nil, fmt.Errorf("認証タイムアウト: %s", timeout)
	}
}

func newOAuthState() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func openBrowser(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ブラウザ起動失敗: %w", err)
	}
	return nil
}

func loginNoteAndSaveCookie(email, password string, timeout time.Duration) error {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.Flag("disable-gpu", false),
		)...,
	)
	defer cancelAlloc()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	deadlineCtx, cancelDeadline := context.WithTimeout(ctx, timeout)
	defer cancelDeadline()

	if err := chromedp.Run(deadlineCtx,
		network.Enable(),
		chromedp.Navigate("https://note.com/login"),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("noteログインページ起動失敗: %w", err)
	}

	if strings.TrimSpace(email) != "" && strings.TrimSpace(password) != "" {
		_ = chromedp.Run(deadlineCtx,
			chromedp.SetValue(`input[type="email"], input[name="email"]`, email, chromedp.ByQuery),
			chromedp.SetValue(`input[type="password"], input[name="password"]`, password, chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		)
	}

	start := time.Now()
	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("noteログイン待機タイムアウト: %s", timeout)
		}
		var cookies []*network.Cookie
		err := chromedp.Run(deadlineCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			c, err := network.GetCookies().WithUrls([]string{"https://note.com"}).Do(ctx)
			if err != nil {
				return err
			}
			cookies = c
			return nil
		}))
		if err == nil && hasNoteSessionCookie(cookies) {
			entries := make([]noteCookieEntry, 0, len(cookies))
			for _, c := range cookies {
				entries = append(entries, noteCookieEntry{
					Name:     c.Name,
					Value:    c.Value,
					Domain:   c.Domain,
					Path:     c.Path,
					HttpOnly: c.HTTPOnly,
					Secure:   c.Secure,
				})
			}
			if err := authutil.SaveJSON(noteCookiePath, entries); err != nil {
				return err
			}
			fmt.Printf("保存しました: %s\n", noteCookiePath)
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}

func hasNoteSessionCookie(cookies []*network.Cookie) bool {
	for _, c := range cookies {
		n := strings.ToLower(strings.TrimSpace(c.Name))
		if strings.Contains(n, "session") || strings.Contains(n, "auth") {
			return true
		}
	}
	return false
}
