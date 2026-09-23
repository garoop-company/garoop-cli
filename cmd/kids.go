package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopdata"
)

// kids_web（create.garoop.jp）のミッション・レッスン操作と、おうちミッション。
// ミッション本体は kids_api の mission テーブル（ポイント・期限など）と
// garoop-data/public/missions/{locale}/{id}.json（本文）に分かれている。

// 提出ファイル1つあたりの上限。
const kidsSubmitMaxBytes = 50 * 1024 * 1024

var (
	kidsLocale   string
	kidsCategory string
	kidsKeyword  string
	kidsNoDetail bool
	kidsFiles    []string
	kidsComment  string
)

type kidsMission struct {
	ID          string         `json:"id"`
	CompanyID   string         `json:"companyId"`
	CompanyName string         `json:"companyName"`
	Point       int            `json:"point"`
	Level       int            `json:"level"`
	DueDate     string         `json:"dueDate"`
	Locale      string         `json:"locale"`
	DataPath    string         `json:"dataPath"`
	Decided     bool           `json:"decided"`
	Detail      map[string]any `json:"detail,omitempty"`
}

const kidsMissionFields = `id companyId companyName point level dueDate locale dataPath decided`

var kidsCmd = serviceCommand("kids", "Garoop Kids (create.garoop.jp): ミッション・レッスン・おうちミッション", ProfileGaroop)

var kidsMissionCmd = &cobra.Command{
	Use:   "mission",
	Short: "ミッションの一覧・応募・提出・登録",
}

var kidsMissionListCmd = &cobra.Command{
	Use:   "list",
	Short: "ミッション一覧（本文JSONも取得してタイトル・カテゴリ付きで表示）",
	RunE: func(cmd *cobra.Command, args []string) error {
		missions, err := kidsFetchMissions(kidsLocale)
		if err != nil {
			return err
		}
		if !kidsNoDetail {
			kidsAttachDetails(missions)
		}
		filtered := make([]kidsMission, 0, len(missions))
		for _, m := range missions {
			if kidsCategory != "" && fmt.Sprint(m.Detail["category"]) != kidsCategory {
				continue
			}
			if kidsKeyword != "" {
				text := fmt.Sprint(m.Detail["title"]) + " " + fmt.Sprint(m.Detail["detailText"])
				if !strings.Contains(strings.ToLower(text), strings.ToLower(kidsKeyword)) {
					continue
				}
			}
			filtered = append(filtered, m)
		}
		return printJSON(filtered)
	},
}

var kidsMissionGetCmd = &cobra.Command{
	Use:   "get [missionId]",
	Short: "ミッション詳細（応募状況・本文を含む）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := kidsFetchMission(args[0])
		if err != nil {
			return err
		}
		return printJSON(m)
	},
}

var kidsMissionAcceptCmd = &cobra.Command{
	Use:   "accept [missionId]",
	Short: "ミッションに応募する（既定はdry-run）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("missionId は数値で指定してください")
		}
		return runMutationOrDryRun("acceptMission", `mutation AcceptMission($missionId: Int!) {
			acceptMission(input: { missionId: $missionId }) { success errorCode }
		}`, map[string]any{"missionId": id})
	},
}

var kidsMissionAcceptedCmd = &cobra.Command{
	Use:   "accepted",
	Short: "自分が応募済みのミッション一覧",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := garoopQueryAPI(`query GetAcceptMissionByLoginUserId {
			getAcceptMissionByLoginUserId { acceptMissionHistoryList { missionId acceptDate point dataPath } }
		}`, nil)
		if err != nil {
			return err
		}
		return printJSON(resp.Data["getAcceptMissionByLoginUserId"])
	},
}

var kidsMissionSubmitCmd = &cobra.Command{
	Use:   "submit [missionId]",
	Short: "ミッションの成果物を提出（スタッフが確認し、承認でポイントが付く。既定はdry-run）",
	Long: `ミッションの成果物ファイルを提出します。応募ずみ（kids mission accept）のミッションにだけ出せます。
ファイルは自分の領域にアップロードされ、スタッフが確認して承認するとミッション完了としてポイントが付きます。
確認状況は ` + "`submission mine --kind MISSION`" + ` で見られます。`,
	Example: `  garoop-cli kids mission submit 20 --file ./work.png --comment "NotebookLMで作りました" --execute`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(kidsFiles) == 0 {
			return fmt.Errorf("--file が必要です")
		}
		m, err := kidsFetchMission(args[0])
		if err != nil {
			return err
		}
		input := map[string]any{
			"kind":        "MISSION",
			"title":       fmt.Sprint(m.Detail["title"]),
			"targetId":    m.ID,
			"targetTitle": fmt.Sprint(m.Detail["title"]),
		}
		if c := strings.TrimSpace(kidsComment); c != "" {
			input["description"] = c
		}
		return submitContent("submitContent", input, kidsFiles, "missions", kidsSubmitMaxBytes)
	},
}

var kidsLessonCmd = &cobra.Command{
	Use:   "lesson",
	Short: "レッスン（トレーニング）の閲覧・クリア・コメント",
}

var kidsLessonListCmd = &cobra.Command{
	Use:   "list",
	Short: "親レッスン一覧（クリア済みフラグ付き）",
	RunE: func(cmd *cobra.Command, args []string) error {
		vars := map[string]any{"locale": kidsLocale}
		if kidsCategory != "" {
			vars["category"] = kidsCategory
		}
		resp, err := garoopQueryAPI(`query GetParentLessonList($locale: String!, $category: String) {
			getParentLessonList(locale: $locale, category: $category) { parentLessonList { id title level category disabled icon } }
		}`, vars)
		if err != nil {
			return err
		}
		var list struct {
			ParentLessonList []map[string]any `json:"parentLessonList"`
		}
		if err := graphQLField(resp, "getParentLessonList", &list); err != nil {
			return err
		}
		cleared := map[string]bool{}
		if cr, err := garoopQueryAPI(`query { getTrainClearedParentIdList }`, nil); err == nil {
			if ids, ok := cr.Data["getTrainClearedParentIdList"].([]any); ok {
				for _, id := range ids {
					cleared[fmt.Sprint(id)] = true
				}
			}
		}
		for _, l := range list.ParentLessonList {
			l["cleared"] = cleared[fmt.Sprint(l["id"])]
		}
		return printJSON(list.ParentLessonList)
	},
}

var kidsLessonBranchesCmd = &cobra.Command{
	Use:   "branches [parentId]",
	Short: "親レッスン配下の子レッスン一覧",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("parentId は数値で指定してください")
		}
		resp, err := garoopQueryAPI(`query GetBranchLessonList($parentId: Int!) {
			getBranchLessonList(parentId: $parentId) { id title level icon branchLesson { id parentId title level contentPath icon } }
			getTrainClearedBranchId(parentId: $parentId)
		}`, map[string]any{"parentId": id})
		if err != nil {
			return err
		}
		return printJSON(map[string]any{
			"lesson":          resp.Data["getBranchLessonList"],
			"clearedBranchId": resp.Data["getTrainClearedBranchId"],
		})
	},
}

var kidsLessonClearCmd = &cobra.Command{
	Use:   "clear [branchId]",
	Short: "子レッスンをクリア済みにする（既定はdry-run）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("branchId は数値で指定してください")
		}
		return runMutationOrDryRun("clearLesson", `mutation ClearLesson($branchId: Int!) {
			clearLesson(input: { branchId: $branchId }) { success errorCode }
		}`, map[string]any{"branchId": id})
	},
}

var kidsLessonCommentCmd = &cobra.Command{
	Use:   "comment [branchId] [text]",
	Short: "子レッスンにコメントする（既定はdry-run）",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("branchId は数値で指定してください")
		}
		return runMutationOrDryRun("addBranchComment", `mutation AddBranchComment($input: BranchCommentInput!) {
			addBranchComment(input: $input) { success errorCode }
		}`, map[string]any{"input": map[string]any{"branchId": id, "comment": strings.TrimSpace(args[1]), "locale": kidsLocale}})
	},
}

func kidsFetchMissions(locale string) ([]kidsMission, error) {
	resp, err := garoopQueryAPI(`query GetMissionList($locale: String!) {
		getMissionList(locale: $locale) { missionList { `+kidsMissionFields+` } }
	}`, map[string]any{"locale": locale})
	if err != nil {
		return nil, err
	}
	var list struct {
		MissionList []kidsMission `json:"missionList"`
	}
	if err := graphQLField(resp, "getMissionList", &list); err != nil {
		return nil, err
	}
	return list.MissionList, nil
}

func kidsFetchMission(idArg string) (*kidsMission, error) {
	id, err := strconv.Atoi(idArg)
	if err != nil {
		return nil, fmt.Errorf("missionId は数値で指定してください")
	}
	resp, err := garoopQueryAPI(`query GetMission($missionId: Int!) {
		getMission(input: { missionId: $missionId }) { mission { `+kidsMissionFields+` } }
	}`, map[string]any{"missionId": id})
	if err != nil {
		return nil, err
	}
	var result struct {
		Mission *kidsMission `json:"mission"`
	}
	if err := graphQLField(resp, "getMission", &result); err != nil {
		return nil, err
	}
	if result.Mission == nil {
		return nil, fmt.Errorf("ミッション %d が見つかりません", id)
	}
	missions := []kidsMission{*result.Mission}
	kidsAttachDetails(missions)
	return &missions[0], nil
}

// kidsAttachDetails は dataPath の本文JSONを並列取得して Detail に格納する。
func kidsAttachDetails(missions []kidsMission) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range missions {
		if missions[i].DataPath == "" {
			continue
		}
		wg.Add(1)
		go func(m *kidsMission) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			detail := map[string]any{}
			if found, err := garoopdata.FetchJSON(m.DataPath, &detail); err == nil && found {
				m.Detail = detail
			}
		}(&missions[i])
	}
	wg.Wait()
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func init() {
	kidsMissionListCmd.Flags().StringVar(&kidsLocale, "locale", "ja", "ロケール")
	kidsMissionListCmd.Flags().StringVar(&kidsCategory, "category", "", "カテゴリで絞り込み（例: daily_mission, sns, AI Challenge）")
	kidsMissionListCmd.Flags().StringVar(&kidsKeyword, "keyword", "", "タイトル・本文のキーワード")
	kidsMissionListCmd.Flags().BoolVar(&kidsNoDetail, "no-detail", false, "本文JSONを取得しない（高速）")
	kidsMissionSubmitCmd.Flags().StringSliceVar(&kidsFiles, "file", nil, "提出ファイル（必須。複数可、各50MBまで）")
	kidsMissionSubmitCmd.Flags().StringVar(&kidsComment, "comment", "", "ひとこと")

	kidsLessonListCmd.Flags().StringVar(&kidsLocale, "locale", "ja", "ロケール")
	kidsLessonListCmd.Flags().StringVar(&kidsCategory, "category", "", "カテゴリで絞り込み")
	kidsLessonCommentCmd.Flags().StringVar(&kidsLocale, "locale", "ja", "ロケール")

	markMutates(kidsMissionAcceptCmd, kidsMissionSubmitCmd, kidsLessonClearCmd, kidsLessonCommentCmd)
	kidsMissionCmd.AddCommand(kidsMissionListCmd, kidsMissionGetCmd, kidsMissionAcceptCmd, kidsMissionAcceptedCmd, kidsMissionSubmitCmd)
	kidsLessonCmd.AddCommand(kidsLessonListCmd, kidsLessonBranchesCmd, kidsLessonClearCmd, kidsLessonCommentCmd)
	kidsCmd.AddCommand(kidsMissionCmd, kidsLessonCmd, kidsFamilyCmd)
	rootCmd.AddCommand(kidsCmd)
}
