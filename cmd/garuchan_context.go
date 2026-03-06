package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	llmSourceRoot  string
	llmOutputPath  string
	llmMaxArticles int
)

var garuchanContextCmd = &cobra.Command{
	Use:     "context",
	Short:   "LLM投入用のGaroopコンテキストを生成",
	GroupID: "garuchan_cli",
}

var garuchanContextBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "garoop_topを参照してLLM用Markdownを生成",
	RunE: func(cmd *cobra.Command, args []string) error {
		return buildLLMContextMarkdown(llmSourceRoot, llmOutputPath, llmMaxArticles)
	},
}

func init() {
	garuchanContextBuildCmd.Flags().StringVar(&llmSourceRoot, "source-root", "/Users/yamashitadaiki/git_work/garoop_top", "garoop_topプロジェクトのルート")
	garuchanContextBuildCmd.Flags().StringVar(&llmOutputPath, "out", "data/llm/garoop_context.md", "生成先Markdownパス")
	garuchanContextBuildCmd.Flags().IntVar(&llmMaxArticles, "max-articles", 10, "取り込む記事数")

	garuchanContextCmd.AddCommand(garuchanContextBuildCmd)
	rootCmd.AddCommand(garuchanContextCmd)
}

func buildLLMContextMarkdown(sourceRoot, outPath string, maxArticles int) error {
	companyCandidates := []string{
		"amplify-studio/public/data/rag/company-info.md",
		"amplify-studio/data/rag/company-info.md",
	}
	companyPath, companyText := readFirstExisting(sourceRoot, companyCandidates, 12000)
	if companyPath == "" {
		return fmt.Errorf("company-info.md が見つかりません: %s", sourceRoot)
	}

	memberPath := filepath.Join(sourceRoot, "amplify-studio/public/data/member-profiles.json")
	memberSummary := readMemberProfilesSummary(memberPath)

	newsPath := filepath.Join(sourceRoot, "amplify-studio/src/app/data/news.ts")
	newsText := readOptionalFile(newsPath, 5000)

	articleDir := filepath.Join(sourceRoot, "amplify-studio/public/articles")
	articleLines := collectArticleSummaries(articleDir, maxArticles)

	builder := &strings.Builder{}
	fmt.Fprintf(builder, "# Garoop LLM Context\n\n")
	fmt.Fprintf(builder, "GeneratedAt: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(builder, "SourceRoot: %s\n\n", sourceRoot)

	fmt.Fprintf(builder, "## Company Info\n")
	fmt.Fprintf(builder, "Source: %s\n\n", companyPath)
	builder.WriteString(companyText)
	if !strings.HasSuffix(companyText, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString("\n")

	if strings.TrimSpace(memberSummary) != "" {
		fmt.Fprintf(builder, "## Member Profiles Summary\n")
		fmt.Fprintf(builder, "Source: %s\n\n", memberPath)
		builder.WriteString(memberSummary)
		builder.WriteString("\n\n")
	}

	if strings.TrimSpace(newsText) != "" {
		fmt.Fprintf(builder, "## News Data Snippet\n")
		fmt.Fprintf(builder, "Source: %s\n\n", newsPath)
		builder.WriteString("```ts\n")
		builder.WriteString(newsText)
		if !strings.HasSuffix(newsText, "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString("```\n\n")
	}

	if len(articleLines) > 0 {
		fmt.Fprintf(builder, "## Recent Articles\n")
		fmt.Fprintf(builder, "SourceDir: %s\n\n", articleDir)
		for _, l := range articleLines {
			builder.WriteString("- ")
			builder.WriteString(l)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, []byte(builder.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("生成しました: %s\n", outPath)
	return nil
}

func readFirstExisting(root string, candidates []string, limit int) (string, string) {
	for _, rel := range candidates {
		p := filepath.Join(root, rel)
		if _, err := os.Stat(p); err == nil {
			return p, readOptionalFile(p, limit)
		}
	}
	return "", ""
}

func readOptionalFile(path string, limit int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	out := strings.TrimSpace(string(b))
	r := []rune(out)
	if limit > 0 && len(r) > limit {
		out = string(r[:limit]) + "\n\n...(truncated)"
	}
	return out
}

func readMemberProfilesSummary(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return ""
	}
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return ""
	}
	lines := make([]string, 0, len(list))
	for i, item := range list {
		if i >= 10 {
			break
		}
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(m["name"]))
		role := strings.TrimSpace(fmt.Sprint(m["role"]))
		profile := strings.TrimSpace(fmt.Sprint(m["profile"]))
		if name == "" || name == "<nil>" {
			continue
		}
		line := name
		if role != "" && role != "<nil>" {
			line += " / " + role
		}
		if profile != "" && profile != "<nil>" {
			line += " / " + truncate(profile, 80)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func collectArticleSummaries(dir string, max int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type article struct {
		path string
		mod  time.Time
	}
	items := make([]article, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, article{
			path: filepath.Join(dir, e.Name()),
			mod:  info.ModTime(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].mod.After(items[j].mod)
	})
	if max <= 0 {
		max = 10
	}
	if len(items) > max {
		items = items[:max]
	}

	out := make([]string, 0, len(items))
	for _, it := range items {
		title := extractTitleFromMarkdown(it.path)
		if title == "" {
			title = filepath.Base(it.path)
		}
		out = append(out, fmt.Sprintf("%s (%s)", title, it.path))
	}
	return out
}

func extractTitleFromMarkdown(path string) string {
	text := readOptionalFile(path, 2000)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "title:") {
			return strings.Trim(strings.TrimPrefix(line, "title:"), ` "'`)
		}
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}
