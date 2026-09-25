package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopapi"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopdata"
)

const (
	groupServices = "garoop_services"

	// annotationProfiles はサービス系コマンドをどのバイナリで表示するか（カンマ区切り）。
	annotationProfiles = "garoop/profiles"
	// annotationMutates は外部に書き込む（--execute が必要な）コマンドの印。agent manifest で使う。
	annotationMutates = "garoop/mutates"
)

// serviceCommand はサービス操作グループのトップレベルコマンドを作る。
func serviceCommand(use, short string, profiles ...string) *cobra.Command {
	return &cobra.Command{
		Use:         use,
		Short:       short,
		GroupID:     groupServices,
		Annotations: map[string]string{annotationProfiles: strings.Join(profiles, ",")},
	}
}

// markMutates は --execute 指定時のみ外部へ書き込むコマンドであることを示す。
func markMutates(cmds ...*cobra.Command) {
	for _, c := range cmds {
		if c.Annotations == nil {
			c.Annotations = map[string]string{}
		}
		c.Annotations[annotationMutates] = "true"
	}
}

func requireValue(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("--%s が必要です", name)
	}
	return nil
}

// readTextOrFile は "@path" 形式ならファイル内容を、それ以外は値そのものを返す。
func readTextOrFile(v string) (string, error) {
	if strings.HasPrefix(v, "@") {
		b, err := os.ReadFile(strings.TrimPrefix(v, "@"))
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	return strings.TrimSpace(v), nil
}

// loginUser は getLoginUser を呼び、未ログインならエラーにする。
func loginUser() (map[string]any, error) {
	resp, err := garoopQueryAPI(`query GetLoginUser { getLoginUser { id name point planType planExpiresAt } }`, nil)
	if err != nil {
		return nil, err
	}
	u, _ := resp.Data["getLoginUser"].(map[string]any)
	if u == nil {
		if garoopapi.NewClient().Cookie == "" {
			return nil, fmt.Errorf("ログインしていません。`garoop-cli login --email <メール>` でログインしてください（Google / LINE 登録の場合は `garoop-cli session-set-cookie`）")
		}
		return nil, fmt.Errorf("ログインが切れています（約24時間で切れます）。`garoop-cli login --email <メール>` でログインし直してください（Google / LINE 登録の場合は `garoop-cli session-set-cookie`）")
	}
	return u, nil
}

// publishOrDryRun は garoop-data への変更を dry-run 表示するか、--execute 時にPRを作成する。
func publishOrDryRun(p *garoopdata.Publisher, pr garoopdata.PullRequest, extra map[string]any) error {
	files := make([]map[string]any, 0, len(pr.Files))
	for _, f := range pr.Files {
		entry := map[string]any{"path": f.Path, "bytes": len(f.Content)}
		if !executeMode && isPreviewable(f) {
			entry["content"] = string(f.Content)
		}
		files = append(files, entry)
	}
	out := map[string]any{
		"repo":   p.Repo,
		"base":   p.BaseBranch,
		"branch": pr.Branch,
		"title":  pr.Title,
		"files":  files,
	}
	for k, v := range extra {
		out[k] = v
	}
	if !executeMode {
		out["mode"] = "dry-run"
		if p.HasToken() {
			out["note"] = "--execute を付けると garoop-data にPRを作成します（マージ後に data.garoop.jp へ反映）"
		} else {
			out["note"] = "GitHubトークンが無いため公開CDNの内容を基準にしています。--execute には `gh auth login` か GITHUB_TOKEN が必要です"
		}
		return printJSON(out)
	}
	url, err := p.OpenPullRequest(pr)
	if err != nil {
		return err
	}
	out["mode"] = "execute"
	out["pullRequest"] = url
	return printJSON(out)
}

func isPreviewable(f garoopdata.FileChange) bool {
	if len(f.Content) > 16*1024 || !utf8.Valid(f.Content) {
		return false
	}
	lower := strings.ToLower(f.Path)
	for _, ext := range []string{".json", ".md", ".txt"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// appendToJSONArray は既存のJSON配列（キー順を保持）に要素を追加して整形済みJSONを返す。
// existing が nil なら新規配列を作る。
func appendToJSONArray(existing []byte, item any) ([]byte, error) {
	var arr []json.RawMessage
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &arr); err != nil {
			return nil, fmt.Errorf("既存JSONが配列ではありません: %w", err)
		}
	}
	raw, err := garoopdata.MarshalPretty(item)
	if err != nil {
		return nil, err
	}
	arr = append(arr, json.RawMessage(raw))
	return garoopdata.MarshalPretty(arr)
}

// graphQLField は GraphQL レスポンスの特定フィールドを任意の型へデコードする。
func graphQLField(resp *garoopapi.Response, field string, out any) error {
	raw, err := json.Marshal(resp.Data[field])
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

var meCmd = &cobra.Command{
	Use:     "me",
	Short:   "ログイン中のGaroopユーザー情報を表示（セッション確認）",
	GroupID: groupServices,
	Annotations: map[string]string{
		annotationProfiles: strings.Join([]string{ProfileGaroop, ProfileGaruchan, ProfileGaroopTV}, ","),
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		u, err := loginUser()
		if err != nil {
			return err
		}
		return printJSON(u)
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
