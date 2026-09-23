package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopapi"
)

// 提出物（kids_api の content_submission）。
// 課題・ミッション成果物・ゲーム・小説・番組提案は、どれもここを通して出す。
// ファイルは先に createS3UploadUrl で自分の領域へ上げ、そのキーを添えて提出する。

const submissionFields = `id kind targetId targetTitle title description objectKeys status reviewComment reviewedAt createdAt`

var submissionKinds = []string{"ASSIGNMENT", "MISSION", "GAME", "NOVEL", "TV_PROPOSAL"}

var (
	submissionKind    string
	submissionStatus  string
	submissionLimit   int
	submissionApprove bool
	submissionReject  bool
	submissionComment string
)

var submissionCmd = serviceCommand("submission", "提出物（課題・作品・小説・番組提案）の確認。スタッフ用の確認操作もここ", ProfileGaroop)

var submissionMineCmd = &cobra.Command{
	Use:   "mine",
	Short: "自分の提出物と確認状況（ログイン必須）",
	RunE: func(cmd *cobra.Command, args []string) error {
		vars := map[string]any{}
		if k, err := optionalKind(submissionKind); err != nil {
			return err
		} else if k != "" {
			vars["kind"] = k
		}
		resp, err := garoopQueryAPI(`query Mine($kind: ContentSubmissionKind) {
			getMyContentSubmissions(kind: $kind) { `+submissionFields+` }
		}`, vars)
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getMyContentSubmissions"])
	},
}

var submissionListCmd = &cobra.Command{
	Use:   "list",
	Short: "[スタッフ] 提出物の一覧（GAROOP_ADMIN_SECRET が必要）",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := garoopapi.NewAdminClient()
		if err != nil {
			return err
		}
		vars := map[string]any{"limit": submissionLimit}
		if k, err := optionalKind(submissionKind); err != nil {
			return err
		} else if k != "" {
			vars["kind"] = k
		}
		if s := strings.ToUpper(strings.TrimSpace(submissionStatus)); s != "" {
			vars["status"] = s
		}
		resp, err := client.Query(`query List($kind: ContentSubmissionKind, $status: ContentSubmissionStatus, $limit: Int) {
			getContentSubmissions(kind: $kind, status: $status, limit: $limit) { userId body `+submissionFields+` }
		}`, vars)
		if err != nil {
			return err
		}
		if len(resp.Errors) > 0 {
			return fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
		}
		return printJSON(resp.Data["getContentSubmissions"])
	},
}

var submissionReviewCmd = &cobra.Command{
	Use:   "review [id]",
	Short: "[スタッフ] 提出物を承認/差し戻し（既定はdry-run。MISSION の承認でポイントが付く）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if submissionApprove == submissionReject {
			return fmt.Errorf("--approve か --reject のどちらかを指定してください")
		}
		vars := map[string]any{"id": args[0], "approve": submissionApprove}
		if c := strings.TrimSpace(submissionComment); c != "" {
			vars["comment"] = c
		}
		query := `mutation Review($id: ID!, $approve: Boolean!, $comment: String) {
			reviewContentSubmission(id: $id, approve: $approve, comment: $comment) { ` + submissionFields + ` }
		}`
		if !executeMode {
			return printJSON(map[string]any{"mode": "dry-run", "field": "reviewContentSubmission", "variables": vars})
		}
		client, err := garoopapi.NewAdminClient()
		if err != nil {
			return err
		}
		resp, err := client.Query(query, vars)
		if err != nil {
			return err
		}
		if len(resp.Errors) > 0 {
			return fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
		}
		return printJSON(resp.Data["reviewContentSubmission"])
	},
}

// uploadFiles はファイルを自分の領域（uploads/users/<id>/<prefix>/）へ上げ、S3 キーを返す。
func uploadFiles(paths []string, prefix string) ([]string, error) {
	client := garoopapi.NewClient()
	keys := make([]string, 0, len(paths))
	for _, p := range paths {
		fmt.Fprintf(os.Stderr, "アップロード中: %s\n", p)
		obj, err := client.UploadFile(p, prefix)
		if err != nil {
			return nil, fmt.Errorf("%s のアップロードに失敗しました: %w", p, err)
		}
		keys = append(keys, obj.ObjectKey)
	}
	return keys, nil
}

// checkFiles は提出前にファイルの有無と大きさを確かめ、dry-run 用の一覧を返す。
func checkFiles(paths []string, maxBytes int64) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			return nil, fmt.Errorf("%s はフォルダです。ファイルを指定してください（まとめる場合はzip）", p)
		}
		if info.Size() > maxBytes {
			return nil, fmt.Errorf("%s が大きすぎます（%dMBまで）", p, maxBytes/1024/1024)
		}
		out = append(out, map[string]any{"path": p, "bytes": info.Size(), "contentType": garoopapi.DetectContentType(p)})
	}
	return out, nil
}

// submitContent は submitContent / submitAssignment を呼ぶ。dry-run では送る内容だけを表示する。
func submitContent(field string, input map[string]any, files []string, prefix string, maxBytes int64) error {
	checked, err := checkFiles(files, maxBytes)
	if err != nil {
		return err
	}
	inputType := map[string]string{"submitContent": "SubmitContentInput!", "submitAssignment": "SubmitAssignmentInput!"}[field]
	query := fmt.Sprintf(`mutation Submit($input: %s) { %s(input: $input) { %s } }`, inputType, field, submissionFields)
	if !executeMode {
		if _, err := loginUser(); err != nil {
			return err
		}
		return printJSON(map[string]any{
			"mode":  "dry-run",
			"field": field,
			"input": input,
			"files": checked,
			"note":  "--execute を付けるとファイルをアップロードして提出します。スタッフの確認後に公開・採点されます",
		})
	}
	if len(files) > 0 {
		keys, err := uploadFiles(files, prefix)
		if err != nil {
			return err
		}
		input["objectKeys"] = keys
	}
	resp, err := garoopapi.NewClient().WithTimeout(2*time.Minute).Query(query, map[string]any{"input": input})
	if err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
	}
	return printJSON(map[string]any{"mode": "execute", "submission": resp.Data[field]})
}

func optionalKind(v string) (string, error) {
	k := strings.ToUpper(strings.TrimSpace(v))
	if k == "" {
		return "", nil
	}
	if !containsString(submissionKinds, k) {
		return "", fmt.Errorf("--kind は %s のいずれかです", strings.Join(submissionKinds, ", "))
	}
	return k, nil
}

func init() {
	submissionMineCmd.Flags().StringVar(&submissionKind, "kind", "", strings.Join(submissionKinds, "|"))
	submissionListCmd.Flags().StringVar(&submissionKind, "kind", "", strings.Join(submissionKinds, "|"))
	submissionListCmd.Flags().StringVar(&submissionStatus, "status", "PENDING", "PENDING|APPROVED|REJECTED（空で全件）")
	submissionListCmd.Flags().IntVar(&submissionLimit, "limit", 50, "件数（最大500）")
	submissionReviewCmd.Flags().BoolVar(&submissionApprove, "approve", false, "承認する")
	submissionReviewCmd.Flags().BoolVar(&submissionReject, "reject", false, "差し戻す")
	submissionReviewCmd.Flags().StringVar(&submissionComment, "comment", "", "提出者へのコメント")

	markMutates(submissionReviewCmd)
	submissionCmd.AddCommand(submissionMineCmd, submissionListCmd, submissionReviewCmd)
	rootCmd.AddCommand(submissionCmd)
}
