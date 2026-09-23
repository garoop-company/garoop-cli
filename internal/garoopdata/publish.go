package garoopdata

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultRepo       = "garoop-company/garoop-data"
	defaultBaseBranch = "main"
	githubAPI         = "https://api.github.com"
)

// FileChange は PR に含める1ファイル分の変更。Path はリポジトリルートからの相対パス（例: public/novel/novels.json）。
type FileChange struct {
	Path    string
	Content []byte
}

// PullRequest は garoop-data へ出すPRの内容。
type PullRequest struct {
	Branch        string
	Title         string
	Body          string
	CommitMessage string
	Files         []FileChange
}

// Publisher は garoop-data リポジトリへの読み書きを GitHub API で行う。
// ローカルclone不要で、Homebrew 導入のみの環境からでも使える。
type Publisher struct {
	Repo       string
	BaseBranch string
	token      string
	client     *http.Client
}

// NewPublisher は GAROOP_DATA_REPO / GAROOP_DATA_BRANCH と GitHub トークン
// （GITHUB_TOKEN, GH_TOKEN, `gh auth token` の順）から Publisher を作る。
func NewPublisher() *Publisher {
	repo := strings.TrimSpace(os.Getenv("GAROOP_DATA_REPO"))
	if repo == "" {
		repo = defaultRepo
	}
	branch := strings.TrimSpace(os.Getenv("GAROOP_DATA_BRANCH"))
	if branch == "" {
		branch = defaultBaseBranch
	}
	return &Publisher{
		Repo:       repo,
		BaseBranch: branch,
		token:      discoverToken(),
		client:     &http.Client{Timeout: 120 * time.Second},
	}
}

func discoverToken() string {
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	if _, err := exec.LookPath("gh"); err == nil {
		out, err := exec.Command("gh", "auth", "token").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// HasToken は GitHub への書き込みに使えるトークンがあるかを返す。
func (p *Publisher) HasToken() bool { return p.token != "" }

// ReadPublic は public/ 配下のファイルを読む。トークンがあれば GitHub の最新（ベースブランチ）を、
// 無ければ公開CDNを参照する（dry-run をトークン無しでも動かすため）。
func (p *Publisher) ReadPublic(rel string) ([]byte, bool, error) {
	rel = strings.TrimLeft(rel, "/")
	if !p.HasToken() {
		return Fetch(rel)
	}
	path := "public/" + rel
	req, err := p.newRequest(http.MethodGet, fmt.Sprintf("/repos/%s/contents/%s?ref=%s", p.Repo, escapePath(path), url.QueryEscape(p.BaseBranch)), nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "application/vnd.github.raw+json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("GitHub から %s を取得できません: status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, true, nil
}

// OpenPullRequest はベースブランチから新規ブランチを切り、Files を1コミットで追加してPRを作成する。
// 戻り値はPRのURL。
func (p *Publisher) OpenPullRequest(pr PullRequest) (string, error) {
	if !p.HasToken() {
		return "", fmt.Errorf("GitHub トークンがありません。`gh auth login` するか GITHUB_TOKEN を設定してください")
	}
	if len(pr.Files) == 0 {
		return "", fmt.Errorf("変更ファイルがありません")
	}

	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if _, err := p.doJSON(http.MethodGet, fmt.Sprintf("/repos/%s/git/ref/heads/%s", p.Repo, escapePath(p.BaseBranch)), nil, &ref); err != nil {
		return "", fmt.Errorf("ベースブランチ取得失敗: %w", err)
	}
	var baseCommit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if _, err := p.doJSON(http.MethodGet, fmt.Sprintf("/repos/%s/git/commits/%s", p.Repo, ref.Object.SHA), nil, &baseCommit); err != nil {
		return "", fmt.Errorf("ベースコミット取得失敗: %w", err)
	}

	tree := make([]map[string]string, 0, len(pr.Files))
	for _, f := range pr.Files {
		var blob struct {
			SHA string `json:"sha"`
		}
		payload := map[string]string{
			"content":  base64.StdEncoding.EncodeToString(f.Content),
			"encoding": "base64",
		}
		if _, err := p.doJSON(http.MethodPost, fmt.Sprintf("/repos/%s/git/blobs", p.Repo), payload, &blob); err != nil {
			return "", fmt.Errorf("%s のアップロード失敗: %w", f.Path, err)
		}
		tree = append(tree, map[string]string{"path": f.Path, "mode": "100644", "type": "blob", "sha": blob.SHA})
	}

	var newTree struct {
		SHA string `json:"sha"`
	}
	if _, err := p.doJSON(http.MethodPost, fmt.Sprintf("/repos/%s/git/trees", p.Repo), map[string]any{
		"base_tree": baseCommit.Tree.SHA,
		"tree":      tree,
	}, &newTree); err != nil {
		return "", fmt.Errorf("tree 作成失敗: %w", err)
	}

	var commit struct {
		SHA string `json:"sha"`
	}
	if _, err := p.doJSON(http.MethodPost, fmt.Sprintf("/repos/%s/git/commits", p.Repo), map[string]any{
		"message": pr.CommitMessage,
		"tree":    newTree.SHA,
		"parents": []string{ref.Object.SHA},
	}, &commit); err != nil {
		return "", fmt.Errorf("commit 作成失敗: %w", err)
	}

	if _, err := p.doJSON(http.MethodPost, fmt.Sprintf("/repos/%s/git/refs", p.Repo), map[string]string{
		"ref": "refs/heads/" + pr.Branch,
		"sha": commit.SHA,
	}, nil); err != nil {
		return "", fmt.Errorf("ブランチ作成失敗: %w", err)
	}

	var created struct {
		HTMLURL string `json:"html_url"`
	}
	if _, err := p.doJSON(http.MethodPost, fmt.Sprintf("/repos/%s/pulls", p.Repo), map[string]string{
		"title": pr.Title,
		"head":  pr.Branch,
		"base":  p.BaseBranch,
		"body":  pr.Body,
	}, &created); err != nil {
		return "", fmt.Errorf("PR 作成失敗（ブランチ %s は作成済み）: %w", pr.Branch, err)
	}
	return created.HTMLURL, nil
}

// BranchName は PR 用のブランチ名を作る（例: cli/novel-shimizu-tax-002-20260923-101500）。
func BranchName(kind, slug string) string {
	return fmt.Sprintf("cli/%s-%s-%s", kind, Slugify(slug), time.Now().Format("20060102-150405"))
}

// Slugify は英数字とハイフンのみのスラッグにする。空になった場合は "item" を返す。
func Slugify(s string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 48 {
		out = strings.Trim(out[:48], "-")
	}
	if out == "" {
		return "item"
	}
	return out
}

func (p *Publisher) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, githubAPI+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	return req, nil
}

func (p *Publisher) doJSON(method, path string, payload any, out any) (int, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := p.newRequest(method, path, body)
	if err != nil {
		return 0, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("GitHub API %s %s: status=%d body=%s", method, path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}
