package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/yamashitadaiki/garoop-cli/internal/parenting"
	"github.com/spf13/cobra"
)

var (
	parentingStorePath string
	parentingChild     string
	parentingMinutes   int
	parentingAmount    float64
	parentingWhen      string
	parentingDays      int
)

var parentingCmd = &cobra.Command{
	Use:     "parenting",
	Short:   "子育てログの記録・集計",
	GroupID: "garuchan_cli",
}

var parentingLogCmd = &cobra.Command{
	Use:   "log [kind] [memo]",
	Short: "子育てログを記録します",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		kind := strings.ToLower(args[0])
		if err := parenting.ValidateKind(kind); err != nil {
			return err
		}

		t := time.Now()
		if parentingWhen != "" {
			parsed, err := time.Parse(time.RFC3339, parentingWhen)
			if err != nil {
				return fmt.Errorf("--time は RFC3339 形式で指定してください")
			}
			t = parsed
		}

		store := parenting.NewStore(parentingStorePath)
		entry := parenting.Entry{
			Time:    t,
			Child:   parentingChild,
			Kind:    kind,
			Memo:    args[1],
			Minutes: parentingMinutes,
			Amount:  parentingAmount,
		}
		if err := store.Append(entry); err != nil {
			return err
		}
		fmt.Printf("子育てログを保存しました: kind=%s child=%s memo=%s\n", entry.Kind, entry.Child, entry.Memo)
		return nil
	},
}

var parentingTodayCmd = &cobra.Command{
	Use:   "today",
	Short: "本日の子育てログを集計表示します",
	RunE: func(cmd *cobra.Command, args []string) error {
		store := parenting.NewStore(parentingStorePath)
		entries, err := store.Load()
		if err != nil {
			return err
		}
		start := time.Now().Truncate(24 * time.Hour)
		today := parenting.FilterSince(entries, start)
		if len(today) == 0 {
			fmt.Println("本日のログはまだありません")
			return nil
		}

		totalMinutes := 0
		totalAmount := 0.0
		countByKind := map[string]int{}
		for _, e := range today {
			totalMinutes += e.Minutes
			totalAmount += e.Amount
			countByKind[e.Kind]++
		}

		fmt.Printf("本日の件数: %d\n", len(today))
		fmt.Printf("合計時間(分): %d\n", totalMinutes)
		fmt.Printf("合計金額: %.0f\n", totalAmount)
		for k, v := range countByKind {
			fmt.Printf("  %s: %d件\n", k, v)
		}
		return nil
	},
}

var parentingListCmd = &cobra.Command{
	Use:   "list",
	Short: "過去ログを表示します",
	RunE: func(cmd *cobra.Command, args []string) error {
		store := parenting.NewStore(parentingStorePath)
		entries, err := store.Load()
		if err != nil {
			return err
		}

		since := time.Now().Add(-time.Duration(parentingDays) * 24 * time.Hour)
		filtered := parenting.FilterSince(entries, since)
		if len(filtered) == 0 {
			fmt.Println("対象期間のログはありません")
			return nil
		}
		for _, e := range filtered {
			fmt.Printf("%s child=%s kind=%s min=%d amount=%.0f memo=%s\n",
				e.Time.Format(time.RFC3339), e.Child, e.Kind, e.Minutes, e.Amount, e.Memo)
		}
		return nil
	},
}

func init() {
	parentingCmd.PersistentFlags().StringVar(&parentingStorePath, "store", "data/parenting_logs.json", "ログ保存先")

	parentingLogCmd.Flags().StringVar(&parentingChild, "child", "未設定", "子どもの名前")
	parentingLogCmd.Flags().IntVar(&parentingMinutes, "minutes", 0, "時間(分)")
	parentingLogCmd.Flags().Float64Var(&parentingAmount, "amount", 0, "金額（食費など）")
	parentingLogCmd.Flags().StringVar(&parentingWhen, "time", "", "記録時刻（RFC3339）")

	parentingListCmd.Flags().IntVar(&parentingDays, "days", 7, "表示する日数")

	parentingCmd.AddCommand(parentingLogCmd)
	parentingCmd.AddCommand(parentingTodayCmd)
	parentingCmd.AddCommand(parentingListCmd)
	rootCmd.AddCommand(parentingCmd)
}
