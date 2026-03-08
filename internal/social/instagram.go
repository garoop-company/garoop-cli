package social

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yamashitadaiki/garoop-cli/internal/authutil"
)

type InstagramClient struct {
	accessToken string
	igUserID    string
	execute     bool
	httpClient  *http.Client
}

type InstagramMedia struct {
	ID        string `json:"id"`
	Caption   string `json:"caption"`
	MediaType string `json:"media_type"`
	MediaURL  string `json:"media_url"`
	Permalink string `json:"permalink"`
	Timestamp string `json:"timestamp"`
	Children  struct {
		Data []InstagramMedia `json:"data"`
	} `json:"children"`
}

type GaruchanPushResult struct {
	MediaID   string
	MediaType string
	Status    string
}

type garuchanUploadRequest struct {
	Endpoint string
	APIKey   string
}

func NewInstagramClient(execute bool) (*InstagramClient, error) {
	if !execute {
		return &InstagramClient{
			execute:    false,
			httpClient: http.DefaultClient,
		}, nil
	}

	token := strings.TrimSpace(os.Getenv("INSTAGRAM_ACCESS_TOKEN"))
	userID := strings.TrimSpace(os.Getenv("INSTAGRAM_IG_USER_ID"))
	if token == "" || userID == "" {
		var saved struct {
			AccessToken string `json:"access_token"`
			IGUserID    string `json:"ig_user_id"`
		}
		if err := authutil.LoadJSON("tokens/instagram.json", &saved); err == nil {
			if token == "" {
				token = strings.TrimSpace(saved.AccessToken)
			}
			if userID == "" {
				userID = strings.TrimSpace(saved.IGUserID)
			}
		}
	}
	if token == "" || userID == "" {
		return nil, fmt.Errorf("Instagramの実行には環境変数か tokens/instagram.json の認証情報が必要です")
	}

	return &InstagramClient{
		accessToken: token,
		igUserID:    userID,
		execute:     true,
		httpClient:  http.DefaultClient,
	}, nil
}

func (c *InstagramClient) PostImage(caption string, imageURL string) (string, error) {
	if !c.execute {
		return fmt.Sprintf("[dry-run] Instagram post: caption=%q image_url=%q", caption, imageURL), nil
	}
	if imageURL == "" {
		return "", fmt.Errorf("Instagram投稿には公開URLの画像が必要です（--image-url）")
	}

	createURL := fmt.Sprintf("https://graph.facebook.com/v22.0/%s/media", c.igUserID)
	form := url.Values{}
	form.Set("image_url", imageURL)
	form.Set("caption", caption)
	form.Set("access_token", c.accessToken)

	resp, err := c.httpClient.PostForm(createURL, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Instagram media作成失敗: %s", string(body))
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return "", err
	}

	publishURL := fmt.Sprintf("https://graph.facebook.com/v22.0/%s/media_publish", c.igUserID)
	publishForm := url.Values{}
	publishForm.Set("creation_id", created.ID)
	publishForm.Set("access_token", c.accessToken)
	pubResp, err := c.httpClient.PostForm(publishURL, publishForm)
	if err != nil {
		return "", err
	}
	defer pubResp.Body.Close()
	if pubResp.StatusCode >= 300 {
		body, _ := io.ReadAll(pubResp.Body)
		return "", fmt.Errorf("Instagram publish失敗: %s", string(body))
	}

	var published struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(pubResp.Body).Decode(&published); err != nil {
		return "", err
	}
	return published.ID, nil
}

func (c *InstagramClient) Comment(mediaID string, message string) (string, error) {
	if !c.execute {
		return fmt.Sprintf("[dry-run] Instagram comment: media_id=%q message=%q", mediaID, message), nil
	}
	if strings.TrimSpace(mediaID) == "" {
		return "", fmt.Errorf("mediaIDが空です")
	}
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("messageが空です")
	}

	endpoint := fmt.Sprintf("https://graph.facebook.com/v22.0/%s/comments", mediaID)
	form := url.Values{}
	form.Set("message", message)
	form.Set("access_token", c.accessToken)

	resp, err := c.httpClient.PostForm(endpoint, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Instagramコメント失敗: %s", strings.TrimSpace(string(body)))
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *InstagramClient) Like(mediaID string) (string, error) {
	if !c.execute {
		return fmt.Sprintf("[dry-run] Instagram like: media_id=%q", mediaID), nil
	}
	if strings.TrimSpace(mediaID) == "" {
		return "", fmt.Errorf("mediaIDが空です")
	}

	endpoint := fmt.Sprintf("https://graph.facebook.com/v22.0/%s/likes", mediaID)
	form := url.Values{}
	form.Set("access_token", c.accessToken)

	resp, err := c.httpClient.PostForm(endpoint, form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Instagramいいね失敗: %s", strings.TrimSpace(string(body)))
	}

	var out struct {
		Success any `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		// 成功時にbodyが空でも失敗させない
		return "ok", nil
	}
	return fmt.Sprintf("%v", out.Success), nil
}

func (c *InstagramClient) UploadPhotoByContainer(caption string, imageURL string) (string, error) {
	return c.PostImage(caption, imageURL)
}

func (c *InstagramClient) RecentMedia(limit int) ([]InstagramMedia, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	if !c.execute {
		return []InstagramMedia{
			{
				ID:        "dry-run-media-id",
				Caption:   "ガルちゃん向けサンプル投稿",
				MediaType: "IMAGE",
				MediaURL:  "https://example.com/garuchan.png",
				Permalink: "https://www.instagram.com/p/dry-run/",
				Timestamp: "2026-03-05T00:00:00+0000",
			},
		}, nil
	}

	u := fmt.Sprintf("https://graph.facebook.com/v22.0/%s/media", c.igUserID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("fields", "id,caption,media_type,media_url,permalink,timestamp,children{media_type,media_url,id}")
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("access_token", c.accessToken)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Instagram media一覧取得失敗: %s", strings.TrimSpace(string(body)))
	}

	var out struct {
		Data []InstagramMedia `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

func (c *InstagramClient) PushRecentMediaToGaruchan(limit int, endpoint string, apiKey string) ([]GaruchanPushResult, error) {
	mediaList, err := c.RecentMedia(limit)
	if err != nil {
		return nil, err
	}

	request := garuchanUploadRequest{
		Endpoint: strings.TrimSpace(endpoint),
		APIKey:   strings.TrimSpace(apiKey),
	}
	if c.execute && request.Endpoint == "" {
		return nil, fmt.Errorf("GARUCHAN_UPLOAD_URL もしくは --endpoint が必要です")
	}

	targets := flattenMedia(mediaList)
	if len(targets) == 0 {
		return nil, fmt.Errorf("送信対象のメディアが見つかりません")
	}

	results := make([]GaruchanPushResult, 0, len(targets))
	for _, m := range targets {
		if !c.execute {
			results = append(results, GaruchanPushResult{
				MediaID:   m.ID,
				MediaType: m.MediaType,
				Status:    "dry-run",
			})
			continue
		}

		if strings.TrimSpace(m.MediaURL) == "" {
			results = append(results, GaruchanPushResult{
				MediaID:   m.ID,
				MediaType: m.MediaType,
				Status:    "skipped(no media_url)",
			})
			continue
		}

		if err := c.uploadToGaruchan(m, request); err != nil {
			return results, fmt.Errorf("media_id=%s upload失敗: %w", m.ID, err)
		}

		results = append(results, GaruchanPushResult{
			MediaID:   m.ID,
			MediaType: m.MediaType,
			Status:    "uploaded",
		})
	}

	return results, nil
}

func flattenMedia(input []InstagramMedia) []InstagramMedia {
	out := make([]InstagramMedia, 0, len(input))
	for _, m := range input {
		if strings.EqualFold(m.MediaType, "CAROUSEL_ALBUM") && len(m.Children.Data) > 0 {
			for _, child := range m.Children.Data {
				child.Caption = m.Caption
				child.Permalink = m.Permalink
				child.Timestamp = m.Timestamp
				out = append(out, child)
			}
			continue
		}
		out = append(out, m)
	}
	return out
}

func (c *InstagramClient) uploadToGaruchan(m InstagramMedia, request garuchanUploadRequest) error {
	resp, err := c.httpClient.Get(m.MediaURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Instagram mediaダウンロード失敗: %s", strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	ext := ".bin"
	if strings.EqualFold(m.MediaType, "IMAGE") {
		ext = ".jpg"
	}
	if strings.EqualFold(m.MediaType, "VIDEO") {
		ext = ".mp4"
	}
	filename := "instagram_" + m.ID + ext

	filePart, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return err
	}
	if _, err := filePart.Write(data); err != nil {
		return err
	}
	_ = writer.WriteField("source", "instagram")
	_ = writer.WriteField("instagram_media_id", m.ID)
	_ = writer.WriteField("media_type", m.MediaType)
	_ = writer.WriteField("caption", m.Caption)
	_ = writer.WriteField("permalink", m.Permalink)
	_ = writer.WriteField("timestamp", m.Timestamp)
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, request.Endpoint, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if request.APIKey != "" {
		req.Header.Set("X-API-Key", request.APIKey)
	}

	uploadResp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(uploadResp.Body)
		return fmt.Errorf("ガルちゃんAPI送信失敗: status=%d body=%s", uploadResp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
