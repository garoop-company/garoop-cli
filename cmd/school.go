package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopapi"
)

// Garoop School（school.garoop.jp）。
// 課題は kids_api の submitAssignment で提出する（ファイルは自分の領域にアップロード。講師・スタッフが確認）。

const defaultSchoolURL = "https://school.garoop.jp"

var (
	schoolCourseID    string
	schoolCourseTitle string
	schoolFiles       []string
	schoolComment     string
	schoolName        string
	schoolEmail       string
	schoolContent     string
)

var schoolCmd = serviceCommand("school", "Garoop School: 課題提出・問い合わせ", ProfileGaroop)

var schoolAssignmentCmd = &cobra.Command{
	Use:   "assignment",
	Short: "課題",
}

var schoolAssignmentSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "課題を提出（既定はdry-run。ログインセッション必須）",
	Long: `課題の成果物ファイルを提出します。ファイルは自分の領域にアップロードされ、講師・スタッフが確認します。
ファイルは --file を繰り返して複数（20個・各50MBまで）指定できます。
確認状況は ` + "`school assignment list`" + ` で見られます。`,
	Example: `  garoop-cli school assignment submit --course-id ai-startup-01 --course-title "AI起業 第1回" \
    --file ./pitch.pdf --file ./demo.mp4 --comment "デモ動画もつけました" --execute`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireValue("course-id", schoolCourseID); err != nil {
			return err
		}
		if err := requireValue("course-title", schoolCourseTitle); err != nil {
			return err
		}
		if len(schoolFiles) == 0 || len(schoolFiles) > 20 {
			return fmt.Errorf("--file を1〜20個指定してください")
		}
		input := map[string]any{
			"courseId":    strings.TrimSpace(schoolCourseID),
			"courseTitle": strings.TrimSpace(schoolCourseTitle),
			"objectKeys":  []string{},
		}
		if c := strings.TrimSpace(schoolComment); c != "" {
			input["comment"] = c
		}
		return submitContent("submitAssignment", input, schoolFiles, "assignments", kidsSubmitMaxBytes)
	},
}

var schoolAssignmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "提出した課題と確認状況",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := garoopQueryAPI(`query { getMyContentSubmissions(kind: ASSIGNMENT) { `+submissionFields+` } }`, nil)
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getMyContentSubmissions"])
	},
}

var schoolContactCmd = &cobra.Command{
	Use:   "contact",
	Short: "Garoop School へ問い合わせ（既定はdry-run）",
	RunE: func(cmd *cobra.Command, args []string) error {
		for name, v := range map[string]string{"name": schoolName, "email": schoolEmail, "content": schoolContent} {
			if err := requireValue(name, v); err != nil {
				return err
			}
		}
		content, err := readTextOrFile(schoolContent)
		if err != nil {
			return err
		}
		endpoint := schoolBaseURL() + "/api/contact"
		payload := map[string]string{"name": strings.TrimSpace(schoolName), "email": strings.TrimSpace(schoolEmail), "content": content}
		if !executeMode {
			return printJSON(map[string]any{"mode": "dry-run", "endpoint": endpoint, "payload": payload})
		}
		resp, err := garoopapi.NewClient().PostJSON(endpoint, payload)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"mode": "execute", "result": resp})
	},
}

func schoolBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("GAROOP_SCHOOL_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultSchoolURL
}

func init() {
	schoolAssignmentSubmitCmd.Flags().StringVar(&schoolCourseID, "course-id", "", "コース/課題ID（必須）")
	schoolAssignmentSubmitCmd.Flags().StringVar(&schoolCourseTitle, "course-title", "", "コース/課題名（必須）")
	schoolAssignmentSubmitCmd.Flags().StringSliceVar(&schoolFiles, "file", nil, "提出ファイル（必須。複数可、各50MBまで）")
	schoolAssignmentSubmitCmd.Flags().StringVar(&schoolComment, "comment", "", "講師へのひとこと")

	schoolContactCmd.Flags().StringVar(&schoolName, "name", "", "お名前（必須）")
	schoolContactCmd.Flags().StringVar(&schoolEmail, "email", "", "返信先メール（必須）")
	schoolContactCmd.Flags().StringVar(&schoolContent, "content", "", "問い合わせ内容（必須。@file 可）")

	markMutates(schoolAssignmentSubmitCmd, schoolContactCmd)
	schoolAssignmentCmd.AddCommand(schoolAssignmentSubmitCmd, schoolAssignmentListCmd)
	schoolCmd.AddCommand(schoolAssignmentCmd, schoolContactCmd)
	rootCmd.AddCommand(schoolCmd)
}
