package garoopapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"garoop-cli/internal/authutil"
)

const defaultEndpoint = "https://api.garoop.jp/query"
const sessionPath = "tokens/garoop_session.json"

type Client struct {
	Endpoint string
	Cookie   string
	client   *http.Client
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

func SaveCookie(cookie string) error {
	payload := map[string]string{
		"cookie":   strings.TrimSpace(cookie),
		"saved_at": time.Now().Format(time.RFC3339),
	}
	return authutil.SaveJSON(sessionPath, payload)
}

func (c *Client) Query(query string, variables map[string]any) (*Response, error) {
	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.Endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.Cookie) != "" {
		req.Header.Set("Cookie", c.Cookie)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("graphql request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out Response
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
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
