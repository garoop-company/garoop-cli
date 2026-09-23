package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// おうちミッション（kids_api の familyMission）。
// 保護者が「子どもにやってほしいこと」を登録し、子どもが「できた」を報告し、保護者が確認してごほうびを渡す。
// 子どもは Garoop Pay の子ども（pay_child）。保護者の操作には合言葉（kids family unlock）が必要。

const familyMissionFields = `id childId title detail rewardGaru rewardNote dueDate status reportComment reportObjectKey reportedAt reviewComment reviewedAt createdAt`

var (
	familyChildID    string
	familyTitle      string
	familyDetail     string
	familyRewardGaru int
	familyRewardNote string
	familyDue        string
	familyStatus     string
	familyComment    string
	familyPhoto      string
	familyApprove    bool
	familyReject     bool
)

var kidsFamilyCmd = &cobra.Command{
	Use:   "family",
	Short: "おうちミッション: 子どもにやってほしいことを登録・報告・承認（ごほうびのガル付き）",
	Long: `おうちミッションは、保護者が子どもに出すミッションです。

  1. kids family children     子ども（Garoop Pay で登録済み）の一覧
  2. kids family unlock       保護者の合言葉を入力（35分有効）
  3. kids family create       ミッションを登録（ごほうびのガルは500まで）
  4. kids family report       子どもが「できた」を報告（写真も添付可）
  5. kids family review       保護者が確認。--approve でガルを渡す

子どもが1人なら --child は省略できます。`,
}

var kidsFamilyChildrenCmd = &cobra.Command{
	Use:   "children",
	Short: "子どもの一覧（Garoop Pay で登録した子ども）",
	RunE: func(cmd *cobra.Command, args []string) error {
		children, err := familyChildren()
		if err != nil {
			return err
		}
		return printJSON(children)
	},
}

var kidsFamilyUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "保護者の合言葉を通す（GAROOP_PARENT_PASSCODE か標準入力から読む）",
	Long: `保護者の合言葉を通します。このセッションで35分間、保護者の操作（登録・承認・削除）ができます。
合言葉はコマンドライン引数では受け取りません（履歴に残るため）。
環境変数 GAROOP_PARENT_PASSCODE か、標準入力から渡してください。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		passcode := strings.TrimSpace(os.Getenv("GAROOP_PARENT_PASSCODE"))
		if passcode == "" {
			fmt.Fprint(os.Stderr, "保護者の合言葉: ")
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil && line == "" {
				return fmt.Errorf("合言葉を読み取れませんでした")
			}
			passcode = strings.TrimSpace(line)
		}
		if passcode == "" {
			return fmt.Errorf("合言葉が空です")
		}
		resp, err := garoopQueryAPI(`mutation Unlock($passcode: String!) {
			verifyPayParentPasscode(passcode: $passcode) { ok }
		}`, map[string]any{"passcode": passcode})
		if err != nil {
			return err
		}
		return printJSON(resp.Data["verifyPayParentPasscode"])
	},
}

var kidsFamilyListCmd = &cobra.Command{
	Use:   "list",
	Short: "おうちミッションの一覧",
	RunE: func(cmd *cobra.Command, args []string) error {
		childID, err := familyResolveChild()
		if err != nil {
			return err
		}
		vars := map[string]any{"childId": childID}
		if s := strings.ToUpper(strings.TrimSpace(familyStatus)); s != "" {
			vars["status"] = s
		}
		resp, err := garoopQueryAPI(`query List($childId: ID!, $status: FamilyMissionStatus) {
			getFamilyMissions(childId: $childId, status: $status) { `+familyMissionFields+` }
		}`, vars)
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getFamilyMissions"])
	},
}

var kidsFamilyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "子どもにやってほしいミッションを登録（保護者。既定はdry-run）",
	Example: `  garoop-cli kids family create --title "おうちの人の肩たたき券を作ろう" \
    --detail "紙とペンで3枚作ってプレゼントしよう" --reward-garu 100 --reward-note "アイス1こ" --due 2026-12-31 --execute`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireValue("title", familyTitle); err != nil {
			return err
		}
		if n := len([]rune(strings.TrimSpace(familyTitle))); n > 60 {
			return fmt.Errorf("--title は60文字までです（現在%d文字）", n)
		}
		if familyRewardGaru < 0 || familyRewardGaru > 500 {
			return fmt.Errorf("--reward-garu は0〜500で指定してください")
		}
		if d := strings.TrimSpace(familyDue); d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return fmt.Errorf("--due は YYYY-MM-DD 形式で指定してください")
			}
		}
		detail, err := readTextOrFile(familyDetail)
		if err != nil {
			return err
		}
		childID, err := familyResolveChild()
		if err != nil {
			return err
		}
		input := map[string]any{"childId": childID, "title": strings.TrimSpace(familyTitle), "rewardGaru": familyRewardGaru}
		for k, v := range map[string]string{"detail": detail, "rewardNote": familyRewardNote, "dueDate": familyDue} {
			if v = strings.TrimSpace(v); v != "" {
				input[k] = v
			}
		}
		return runMutationOrDryRun("createFamilyMission", `mutation Create($input: CreateFamilyMissionInput!) {
			createFamilyMission(input: $input) { `+familyMissionFields+` }
		}`, map[string]any{"input": input})
	},
}

var kidsFamilyReportCmd = &cobra.Command{
	Use:     "report [missionId]",
	Short:   "子どもが「できた」を報告（写真を添付可。既定はdry-run）",
	Example: `  garoop-cli kids family report 5 --comment "3まいできたよ" --photo ./ticket.jpg --execute`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		childID, err := familyResolveChild()
		if err != nil {
			return err
		}
		input := map[string]any{"childId": childID, "missionId": args[0]}
		if c := strings.TrimSpace(familyComment); c != "" {
			input["comment"] = c
		}
		photo := strings.TrimSpace(familyPhoto)
		if photo != "" {
			if _, err := checkFiles([]string{photo}, kidsSubmitMaxBytes); err != nil {
				return err
			}
		}
		query := `mutation Report($input: ReportFamilyMissionInput!) {
			reportFamilyMission(input: $input) { ` + familyMissionFields + ` }
		}`
		if !executeMode {
			if photo != "" {
				input["objectKey"] = "<--execute 時に " + photo + " をアップロード>"
			}
			return runMutationOrDryRun("reportFamilyMission", query, map[string]any{"input": input})
		}
		if photo != "" {
			keys, err := uploadFiles([]string{photo}, "family")
			if err != nil {
				return err
			}
			input["objectKey"] = keys[0]
		}
		return runMutationOrDryRun("reportFamilyMission", query, map[string]any{"input": input})
	},
}

var kidsFamilyReviewCmd = &cobra.Command{
	Use:   "review [missionId]",
	Short: "保護者が確認（--approve でごほうびのガルを渡す / --reject でやり直し。既定はdry-run）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if familyApprove == familyReject {
			return fmt.Errorf("--approve か --reject のどちらかを指定してください")
		}
		childID, err := familyResolveChild()
		if err != nil {
			return err
		}
		input := map[string]any{"childId": childID, "missionId": args[0], "approve": familyApprove}
		if c := strings.TrimSpace(familyComment); c != "" {
			input["comment"] = c
		}
		return runMutationOrDryRun("reviewFamilyMission", `mutation Review($input: ReviewFamilyMissionInput!) {
			reviewFamilyMission(input: $input) { `+familyMissionFields+` }
		}`, map[string]any{"input": input})
	},
}

var kidsFamilyDeleteCmd = &cobra.Command{
	Use:   "delete [missionId]",
	Short: "おうちミッションを消す（保護者。既定はdry-run）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		childID, err := familyResolveChild()
		if err != nil {
			return err
		}
		return runMutationOrDryRun("deleteFamilyMission", `mutation Delete($childId: ID!, $missionId: ID!) {
			deleteFamilyMission(childId: $childId, missionId: $missionId) { success }
		}`, map[string]any{"childId": childID, "missionId": args[0]})
	},
}

func familyChildren() ([]map[string]any, error) {
	resp, err := garoopQueryAPI(`query { getPayParentSession { children { childId nickname gradeLabel balanceGaru frozen } } }`, nil)
	if err != nil {
		return nil, err
	}
	var session struct {
		Children []map[string]any `json:"children"`
	}
	if err := graphQLField(resp, "getPayParentSession", &session); err != nil {
		return nil, err
	}
	return session.Children, nil
}

// familyResolveChild は --child を返す。省略時、子どもが1人ならその子にする。
func familyResolveChild() (string, error) {
	if id := strings.TrimSpace(familyChildID); id != "" {
		return id, nil
	}
	children, err := familyChildren()
	if err != nil {
		return "", err
	}
	switch len(children) {
	case 0:
		return "", fmt.Errorf("子どもが登録されていません。Garoop Pay（pay.garoop.jp）でおさいふを開設してください")
	case 1:
		return fmt.Sprint(children[0]["childId"]), nil
	default:
		return "", fmt.Errorf("子どもが複数います。--child で指定してください（`kids family children` で確認）")
	}
}

func init() {
	for _, c := range []*cobra.Command{kidsFamilyListCmd, kidsFamilyCreateCmd, kidsFamilyReportCmd, kidsFamilyReviewCmd, kidsFamilyDeleteCmd} {
		c.Flags().StringVar(&familyChildID, "child", "", "子どものID（1人なら省略可）")
	}
	kidsFamilyListCmd.Flags().StringVar(&familyStatus, "status", "", "OPEN|SUBMITTED|APPROVED|REJECTED")
	kidsFamilyCreateCmd.Flags().StringVar(&familyTitle, "title", "", "ミッション名（必須、60文字まで）")
	kidsFamilyCreateCmd.Flags().StringVar(&familyDetail, "detail", "", "やってほしいことの説明（@file 可）")
	kidsFamilyCreateCmd.Flags().IntVar(&familyRewardGaru, "reward-garu", 0, "承認時に渡すガル（0〜500）")
	kidsFamilyCreateCmd.Flags().StringVar(&familyRewardNote, "reward-note", "", "ガル以外のごほうび（表示用）")
	kidsFamilyCreateCmd.Flags().StringVar(&familyDue, "due", "", "期限 YYYY-MM-DD")
	kidsFamilyReportCmd.Flags().StringVar(&familyComment, "comment", "", "ひとこと")
	kidsFamilyReportCmd.Flags().StringVar(&familyPhoto, "photo", "", "写真などのファイル（50MBまで）")
	kidsFamilyReviewCmd.Flags().BoolVar(&familyApprove, "approve", false, "承認してごほうびを渡す")
	kidsFamilyReviewCmd.Flags().BoolVar(&familyReject, "reject", false, "やり直しにする")
	kidsFamilyReviewCmd.Flags().StringVar(&familyComment, "comment", "", "子どもへのコメント")

	markMutates(kidsFamilyCreateCmd, kidsFamilyReportCmd, kidsFamilyReviewCmd, kidsFamilyDeleteCmd)
	kidsFamilyCmd.AddCommand(kidsFamilyChildrenCmd, kidsFamilyUnlockCmd, kidsFamilyListCmd, kidsFamilyCreateCmd, kidsFamilyReportCmd, kidsFamilyReviewCmd, kidsFamilyDeleteCmd)
}
