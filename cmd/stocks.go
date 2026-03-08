package cmd

import (
	"fmt"
	"strconv"

	"github.com/yamashitadaiki/garoop-cli/internal/social"
	"github.com/spf13/cobra"
)

var (
	orderSide        string
	orderType        string
	orderTimeInForce string
)

var stocksCmd = &cobra.Command{
	Use:     "stocks",
	Short:   "株式取引・口座確認（Alpaca API）",
	GroupID: "garoop_cli",
}

var stocksOrderCmd = &cobra.Command{
	Use:   "order [symbol] [qty]",
	Short: "株式注文を実行します",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		qty, err := strconv.Atoi(args[1])
		if err != nil || qty <= 0 {
			return fmt.Errorf("qtyは1以上の整数で指定してください")
		}

		client, err := social.NewStocksClient(executeMode)
		if err != nil {
			return err
		}
		out, err := client.PlaceOrder(args[0], qty, orderSide, orderType, orderTimeInForce)
		if err != nil {
			return err
		}
		fmt.Printf("注文結果: id=%s symbol=%s qty=%s side=%s type=%s status=%s\n", out.ID, out.Symbol, out.Qty, out.Side, out.Type, out.Status)
		return nil
	},
}

var stocksAccountCmd = &cobra.Command{
	Use:   "account",
	Short: "口座サマリを表示します",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewStocksClient(executeMode)
		if err != nil {
			return err
		}
		s, err := client.AccountSummary()
		if err != nil {
			return err
		}
		fmt.Printf("口座情報: %s\n", s)
		return nil
	},
}

var stocksPositionsCmd = &cobra.Command{
	Use:   "positions",
	Short: "保有ポジションを表示します",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := social.NewStocksClient(executeMode)
		if err != nil {
			return err
		}
		s, err := client.Positions()
		if err != nil {
			return err
		}
		fmt.Printf("保有ポジション: %s\n", s)
		return nil
	},
}

func init() {
	stocksOrderCmd.Flags().StringVar(&orderSide, "side", "buy", "注文方向 buy/sell")
	stocksOrderCmd.Flags().StringVar(&orderType, "type", "market", "注文種別 market/limitなど")
	stocksOrderCmd.Flags().StringVar(&orderTimeInForce, "tif", "day", "time in force")

	stocksCmd.AddCommand(stocksOrderCmd)
	stocksCmd.AddCommand(stocksAccountCmd)
	stocksCmd.AddCommand(stocksPositionsCmd)
	rootCmd.AddCommand(stocksCmd)
}
