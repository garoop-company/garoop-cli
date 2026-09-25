package authutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// TokenPath は認証情報ファイル name の保存先を返す。
// 以前はカレントディレクトリの tokens/ に置いていたため、そこにあればそのまま使う。
// なければ GAROOP_CLI_TOKEN_DIR、既定は ~/.config/garoop-cli/tokens。
// どのディレクトリからエージェントが実行しても同じログイン状態を使え、
// 作業中のリポジトリへ Cookie やトークンを置いてしまうこともない。
func TokenPath(name string) string {
	if dir := strings.TrimSpace(os.Getenv("GAROOP_CLI_TOKEN_DIR")); dir != "" {
		return filepath.Join(dir, name)
	}
	legacy := filepath.Join("tokens", name)
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return legacy
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "garoop-cli", "tokens", name)
}

func SaveJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func LoadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
