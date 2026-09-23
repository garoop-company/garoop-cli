// Package garoopdata は data.garoop.jp（garoop-data リポジトリの public/ 配下）の
// 読み取りと、GitHub Pull Request 経由でのコンテンツ公開を扱う。
package garoopdata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultBaseURL = "https://data.garoop.jp"

// BaseURL は公開CDNのベースURL。GAROOP_DATA_BASE_URL で上書きできる。
func BaseURL() string {
	if v := strings.TrimSpace(os.Getenv("GAROOP_DATA_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultBaseURL
}

// PublicURL は public/ 配下の相対パスを公開URLに変換する。
func PublicURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return BaseURL() + "/" + strings.TrimLeft(path, "/")
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Fetch は公開CDNからファイルを取得する。404 のときは found=false を返す。
func Fetch(path string) ([]byte, bool, error) {
	resp, err := httpClient.Get(PublicURL(path))
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("fetch %s failed: status=%d", PublicURL(path), resp.StatusCode)
	}
	return body, true, nil
}

// FetchJSON は公開CDNからJSONを取得してデコードする。
func FetchJSON(path string, v any) (bool, error) {
	b, found, err := Fetch(path)
	if err != nil || !found {
		return found, err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return true, fmt.Errorf("%s のJSONを解釈できません: %w", path, err)
	}
	return true, nil
}

// MarshalPretty は garoop-data の既存ファイルと同じ体裁（2スペース・HTMLエスケープなし・末尾改行）でJSONを書き出す。
// json.RawMessage を含む値はキー順を保ったまま出力される。
func MarshalPretty(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
