package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	geminiModel string
)

var geminiCmd = &cobra.Command{
	Use:     "gemini",
	Short:   "Gemini CLI連携（ローカルの gemini コマンドを呼び出す）",
	GroupID: "garoop_cli",
}

var geminiLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "gemini を対話起動してGoogleログイン/認証を行う",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGeminiWithStdio(nil)
	},
}

var geminiPromptCmd = &cobra.Command{
	Use:   "prompt [text]",
	Short: "1回のプロンプトをgemini CLIに投げる",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt := strings.TrimSpace(args[0])
		if prompt == "" {
			return fmt.Errorf("promptが空です")
		}

		gArgs := []string{"-p", prompt}
		if strings.TrimSpace(geminiModel) != "" {
			gArgs = append(gArgs, "-m", strings.TrimSpace(geminiModel))
		}
		return runGeminiWithStdio(gArgs)
	},
}

var geminiExecCmd = &cobra.Command{
	Use:                "exec -- [gemini args...]",
	Short:              "gemini コマンドへ任意引数をそのまま渡す",
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("使い方: garoop-cli gemini exec -- <gemini args...>")
		}
		return runGeminiWithStdio(args)
	},
}

func init() {
	geminiPromptCmd.Flags().StringVar(&geminiModel, "model", "", "Geminiモデル名（例: gemini-2.5-pro）")

	geminiCmd.AddCommand(geminiLoginCmd, geminiPromptCmd, geminiExecCmd)
	rootCmd.AddCommand(geminiCmd)
}

func runGeminiWithStdio(args []string) error {
	if _, err := exec.LookPath("gemini"); err != nil {
		return fmt.Errorf("gemini コマンドが見つかりません。例: npm i -g @google/gemini-cli")
	}

	c := exec.Command("gemini", args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("gemini 実行失敗: %w", err)
	}
	return nil
}
