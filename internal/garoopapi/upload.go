package garoopapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultCreateURL = "https://create.garoop.jp"

// CreateBaseURL は create.garoop.jp（kids_web）のベースURL。GAROOP_CREATE_URL で上書きできる。
func CreateBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("GAROOP_CREATE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultCreateURL
}

// WithTimeout は画像生成など時間のかかる呼び出し用にタイムアウトを変える。
func (c *Client) WithTimeout(d time.Duration) *Client {
	c.client = &http.Client{Timeout: d}
	return c
}

// HasSession はセッションCookieが設定済みかを返す。
func (c *Client) HasSession() bool { return strings.TrimSpace(c.Cookie) != "" }

// DetectContentType は拡張子からContent-Typeを推定する。
func DetectContentType(path string) string {
	if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// UploadedObject は S3 へのアップロード結果。
type UploadedObject struct {
	ObjectKey   string `json:"objectKey"`
	ContentType string `json:"contentType"`
	Bytes       int    `json:"bytes"`
}

// UploadFile は createS3UploadUrl で署名URLを取得し、ファイルをPUTする。
func (c *Client) UploadFile(path, prefix string) (*UploadedObject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	contentType := DetectContentType(path)
	input := map[string]any{
		"fileName":    filepath.Base(path),
		"contentType": contentType,
	}
	if strings.TrimSpace(prefix) != "" {
		input["prefix"] = strings.TrimSpace(prefix)
	}
	resp, err := c.Query(`mutation CreateS3UploadUrl($input: CreateS3UploadURLInput!) {
		createS3UploadUrl(input: $input) { objectKey method url expiresInSec }
	}`, map[string]any{"input": input})
	if err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
	}
	raw, _ := json.Marshal(resp.Data["createS3UploadUrl"])
	var presigned struct {
		ObjectKey string `json:"objectKey"`
		Method    string `json:"method"`
		URL       string `json:"url"`
	}
	if err := json.Unmarshal(raw, &presigned); err != nil || presigned.URL == "" {
		return nil, fmt.Errorf("署名URLを取得できませんでした")
	}

	req, err := http.NewRequest(http.MethodPut, presigned.URL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	putResp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	defer putResp.Body.Close()
	if putResp.StatusCode >= 300 {
		body, _ := io.ReadAll(putResp.Body)
		return nil, fmt.Errorf("S3 upload failed: status=%d body=%s", putResp.StatusCode, strings.TrimSpace(string(body)))
	}
	return &UploadedObject{ObjectKey: presigned.ObjectKey, ContentType: contentType, Bytes: len(data)}, nil
}

// PostForm はセッションCookie付きで multipart/form-data を送信し、JSONレスポンスを返す。
// filePath が空ならファイル無しで送る。
func (c *Client) PostForm(endpoint string, fields map[string]string, fileField, filePath string) (map[string]any, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		part, err := w.CreateFormFile(fileField, filepath.Base(filePath))
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(part, f); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return c.doWebJSON(req)
}

// PostJSON はセッションCookie付きでJSONを送信し、JSONレスポンスを返す。
func (c *Client) PostJSON(endpoint string, payload any) (map[string]any, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doWebJSON(req)
}

func (c *Client) doWebJSON(req *http.Request) (map[string]any, error) {
	if c.HasSession() {
		req.Header.Set("Cookie", c.Cookie)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode >= 300 {
		return out, fmt.Errorf("request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return out, nil
}
