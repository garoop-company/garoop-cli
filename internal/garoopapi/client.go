package garoopapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yamashitadaiki/garoop-cli/internal/authutil"
)

const defaultEndpoint = "https://api.garoop.jp/query"

var sessionPath = authutil.TokenPath("garoop_session.json")

type Client struct {
	Endpoint string
	Cookie   string
	// AdminToken はスタッフ用の操作（提出物の確認など）でだけ使う。NewAdminClient 以外では空。
	AdminToken string
	client     *http.Client
}

type Response struct {
	Data   map[string]any `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func NewClient() *Client {
	endpoint := strings.TrimSpace(os.Getenv("GAROOP_GRAPHQL_ENDPOINT"))
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	cookie := strings.TrimSpace(os.Getenv("GAROOP_COOKIE"))
	if cookie == "" {
		var saved struct {
			Cookie string `json:"cookie"`
		}
		if err := authutil.LoadJSON(sessionPath, &saved); err == nil {
			cookie = strings.TrimSpace(saved.Cookie)
		}
	}

	return &Client{
		Endpoint: endpoint,
		Cookie:   cookie,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// NewAdminClient は GAROOP_ADMIN_SECRET（kids_api の GRAPHQL_ADMIN_SECRET）を付けたクライアントを返す。
// スタッフ用コマンドだけが使う。通常のコマンドに秘密鍵を載せないため、NewClient とは分けている。
func NewAdminClient() (*Client, error) {
	token := strings.TrimSpace(os.Getenv("GAROOP_ADMIN_SECRET"))
	if token == "" {
		return nil, fmt.Errorf("スタッフ用の操作です。GAROOP_ADMIN_SECRET を設定してください")
	}
	c := NewClient()
	c.AdminToken = token
	return c, nil
}

// SessionPath はログインCookieの保存先。
func SessionPath() string { return sessionPath }

func SaveCookie(cookie string) error {
	payload := map[string]string{
		"cookie":   strings.TrimSpace(cookie),
		"saved_at": time.Now().Format(time.RFC3339),
	}
	return authutil.SaveJSON(sessionPath, payload)
}

func (c *Client) Query(query string, variables map[string]any) (*Response, error) {
	out, _, err := c.do(query, variables)
	return out, err
}

// ErrNoSession はログイン要求にサーバーがセッションを返さなかったこと（認証情報の誤りなど）を表す。
var ErrNoSession = errors.New("no session returned")

// Login はログイン系の mutation を送り、サーバーが返した sessionId Cookie を保存する。
// Origin ヘッダーを付けないので、api.garoop.jp はブラウザ外のクライアントとして受け付ける。
func (c *Client) Login(query string, variables map[string]any) (*Response, error) {
	out, cookies, err := c.do(query, variables)
	if err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return out, fmt.Errorf("%s", out.Errors[0].Message)
	}
	for _, ck := range cookies {
		if ck.Name == "sessionId" && ck.Value != "" {
			return out, SaveCookie("sessionId=" + ck.Value)
		}
	}
	return out, ErrNoSession
}

// ClearSession は保存済みのログインCookieを消す。
func ClearSession() error {
	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (c *Client) do(query string, variables map[string]any) (*Response, []*http.Cookie, error) {
	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.Endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.Cookie) != "" {
		req.Header.Set("Cookie", c.Cookie)
	}
	if c.AdminToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AdminToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("graphql request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out Response
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, nil, err
	}
	return &out, resp.Cookies(), nil
}

func AuthURLQuery(provider string) (string, string, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "google":
		return "getGoogleAuthUrl", `query GetGoogleAuthUrl($redirectUrl: String!) { getGoogleAuthUrl(redirectUrl: $redirectUrl) }`, nil
	case "line":
		return "getLineAuthUrl", `query GetLineAuthUrl($redirectUrl: String!) { getLineAuthUrl(redirectUrl: $redirectUrl) }`, nil
	case "facebook":
		return "getFacebookAuthUrl", `query GetFacebookAuthUrl($redirectUrl: String!) { getFacebookAuthUrl(redirectUrl: $redirectUrl) }`, nil
	case "tiktok":
		return "getTikTokAuthUrl", `query GetTikTokAuthUrl($redirectUrl: String!) { getTikTokAuthUrl(redirectUrl: $redirectUrl) }`, nil
	case "x", "twitter":
		return "getXAuthUrl", `query GetXAuthUrl($redirectUrl: String!) { getXAuthUrl(redirectUrl: $redirectUrl) }`, nil
	default:
		return "", "", fmt.Errorf("unsupported provider: %s", provider)
	}
}
