package social

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type NoteClient struct {
	httpClient *http.Client
	headers    map[string]string
	execute    bool
}

type NoteCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	HttpOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
}

type NoteArticle struct {
	Title   string
	Link    string
	PubDate string
}

func NewNoteClient(execute bool, cookiePath string) (*NoteClient, error) {
	if !execute {
		return &NoteClient{execute: false}, nil
	}
	if strings.TrimSpace(cookiePath) == "" {
		return nil, fmt.Errorf("Note実行には --cookie-json が必要です")
	}

	raw, err := os.ReadFile(cookiePath)
	if err != nil {
		return nil, fmt.Errorf("cookie読み込み失敗: %w", err)
	}

	var noteCookies []NoteCookie
	if err := json.Unmarshal(raw, &noteCookies); err != nil {
		return nil, fmt.Errorf("cookie JSON解析失敗: %w", err)
	}
	if len(noteCookies) == 0 {
		return nil, fmt.Errorf("cookie JSONが空です")
	}

	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse("https://note.com")
	cookies := make([]*http.Cookie, 0, len(noteCookies))
	for _, c := range noteCookies {
		cookies = append(cookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HttpOnly: c.HttpOnly,
			Secure:   c.Secure,
		})
	}
	jar.SetCookies(u, cookies)

	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "Mozilla/5.0",
		"Referer":      "https://editor.note.com/",
	}
	for _, c := range cookies {
		if c.Name == "XSRF-TOKEN" {
			val, _ := url.QueryUnescape(c.Value)
			headers["X-XSRF-TOKEN"] = val
			break
		}
	}

	return &NoteClient{
		httpClient: &http.Client{Jar: jar, Timeout: 30 * time.Second},
		headers:    headers,
		execute:    true,
	}, nil
}

func (c *NoteClient) Post(title, bodyHTML, imagePathOrURL string, publish bool) (string, error) {
	if !c.execute {
		return fmt.Sprintf("[dry-run] Note post: title=%q publish=%t image=%q", title, publish, imagePathOrURL), nil
	}

	eyecatchKey := ""
	if strings.TrimSpace(imagePathOrURL) != "" {
		localPath, cleanup, err := prepareImagePath(imagePathOrURL)
		if err != nil {
			return "", err
		}
		defer cleanup()

		key, err := c.uploadImage(localPath)
		if err != nil {
			return "", err
		}
		eyecatchKey = key
	}

	id, key, err := c.createArticle(title, bodyHTML)
	if err != nil {
		return "", err
	}
	if err := c.updateDraft(id, title, bodyHTML, eyecatchKey); err != nil {
		return "", err
	}
	if publish {
		if err := c.publishArticle(id, key, title, bodyHTML); err != nil {
			return "", err
		}
	}
	return "https://note.com/n/" + key, nil
}

func (c *NoteClient) doJSON(method, endpoint string, payload any) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, err
}

func (c *NoteClient) createArticle(title, htmlContent string) (string, string, error) {
	payload := map[string]any{
		"name": title,
		"body": htmlContent,
	}
	respBody, code, err := c.doJSON(http.MethodPost, "https://note.com/api/v1/text_notes", payload)
	if err != nil {
		return "", "", err
	}
	if code >= 300 {
		return "", "", fmt.Errorf("note記事作成失敗: status=%d body=%s", code, string(respBody))
	}

	var out struct {
		Data struct {
			ID  json.Number `json:"id"`
			Key string      `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", "", err
	}
	return out.Data.ID.String(), out.Data.Key, nil
}

func (c *NoteClient) updateDraft(id, title, htmlContent, eyecatchKey string) error {
	payload := map[string]any{
		"name":        title,
		"body":        htmlContent,
		"body_length": len([]rune(htmlContent)),
	}
	if eyecatchKey != "" {
		payload["eyecatch_image_key"] = eyecatchKey
	}

	endpoint := fmt.Sprintf("https://note.com/api/v1/text_notes/draft_save?id=%s&is_temp_saved=true", id)
	respBody, code, err := c.doJSON(http.MethodPost, endpoint, payload)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("note draft保存失敗: status=%d body=%s", code, string(respBody))
	}
	return nil
}

func (c *NoteClient) publishArticle(id, key, title, htmlContent string) error {
	payload := map[string]any{
		"name":                    title,
		"free_body":               htmlContent,
		"body_length":             len([]rune(htmlContent)),
		"status":                  "published",
		"send_notifications_flag": true,
		"slug":                    "slug-" + key,
	}

	endpoint := fmt.Sprintf("https://note.com/api/v1/text_notes/%s", id)
	respBody, code, err := c.doJSON(http.MethodPut, endpoint, payload)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("note publish失敗: status=%d body=%s", code, string(respBody))
	}
	return nil
}

func (c *NoteClient) uploadImage(imagePath string) (string, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(imagePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://note.com/api/v1/upload_image", &body)
	if err != nil {
		return "", err
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("note画像アップロード失敗: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	return out.Data.Key, nil
}

func prepareImagePath(pathOrURL string) (string, func(), error) {
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		resp, err := http.Get(pathOrURL)
		if err != nil {
			return "", func() {}, err
		}
		defer resp.Body.Close()

		tmp, err := os.CreateTemp("", "note-image-*")
		if err != nil {
			return "", func() {}, err
		}
		if _, err := io.Copy(tmp, resp.Body); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			return "", func() {}, err
		}
		_ = tmp.Close()
		return tmp.Name(), func() { _ = os.Remove(tmp.Name()) }, nil
	}
	return pathOrURL, func() {}, nil
}

func (c *NoteClient) RecentArticles(username string, limit int) ([]NoteArticle, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("usernameが空です")
	}
	if !c.execute {
		return []NoteArticle{
			{
				Title:   "ガルちゃん向けNoteサンプル記事",
				Link:    "https://note.com/example/n/dry-run",
				PubDate: "Thu, 05 Mar 2026 00:00:00 +0000",
			},
		}, nil
	}

	u := fmt.Sprintf("https://note.com/%s/rss", username)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Note RSS取得失敗: %s", string(body))
	}

	var rss struct {
		Channel struct {
			Items []struct {
				Title   string `xml:"title"`
				Link    string `xml:"link"`
				PubDate string `xml:"pubDate"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&rss); err != nil {
		return nil, err
	}

	articles := make([]NoteArticle, 0, limit)
	for i, it := range rss.Channel.Items {
		if i >= limit {
			break
		}
		articles = append(articles, NoteArticle{
			Title:   strings.TrimSpace(it.Title),
			Link:    strings.TrimSpace(it.Link),
			PubDate: strings.TrimSpace(it.PubDate),
		})
	}
	return articles, nil
}
