// Package voice は VOICEVOX エンジン（garuchan_creator の docker compose で :50021 に起動）で音声合成する。
package voice

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultEngineURL = "http://127.0.0.1:50021"

// EngineURL は VOICEVOX_ENGINE_URL か既定値を返す。
func EngineURL() string {
	if v := strings.TrimSpace(os.Getenv("VOICEVOX_ENGINE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultEngineURL
}

var client = &http.Client{Timeout: 5 * time.Minute}

// Synthesize はテキストをWAVに変換する。
func Synthesize(engineURL, text string, speaker int) ([]byte, error) {
	q := url.Values{"text": {text}, "speaker": {fmt.Sprint(speaker)}}
	resp, err := client.Post(engineURL+"/audio_query?"+q.Encode(), "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("VOICEVOX に接続できません（%s）。garuchan_creator で `make up` するか VOICEVOX_ENGINE_URL を設定してください: %w", engineURL, err)
	}
	query, err := readOK(resp)
	if err != nil {
		return nil, fmt.Errorf("audio_query 失敗: %w", err)
	}
	resp, err = client.Post(engineURL+"/synthesis?speaker="+fmt.Sprint(speaker), "application/json", bytes.NewReader(query))
	if err != nil {
		return nil, err
	}
	wav, err := readOK(resp)
	if err != nil {
		return nil, fmt.Errorf("synthesis 失敗: %w", err)
	}
	return wav, nil
}

// WAVToMP3 は ffmpeg でWAVをMP3に変換する（リポジトリに置くサイズを抑えるため）。
func WAVToMP3(wav []byte, bitrate string) ([]byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg が見つかりません（例: brew install ffmpeg）")
	}
	dir, err := os.MkdirTemp("", "garoop-voice-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in, out := filepath.Join(dir, "in.wav"), filepath.Join(dir, "out.mp3")
	if err := os.WriteFile(in, wav, 0o600); err != nil {
		return nil, err
	}
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-i", in, "-codec:a", "libmp3lame", "-b:a", bitrate, out)
	if b, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg 変換失敗: %v %s", err, strings.TrimSpace(string(b)))
	}
	return os.ReadFile(out)
}

func readOK(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}
