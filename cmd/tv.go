package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yamashitadaiki/garoop-cli/internal/garoopdata"
)

// GaroopTV の番組表は garoop-data/public/garoop-tv 配下の静的JSON。
// マージ規則は garoop_tv/app/video-player.tsx の mergeScheduleData / getCurrentProgram に合わせている。
const (
	tvChannelsPath        = "garoop-tv/channels.json"
	tvDefaultSchedulePath = "garoop-tv/schedules/default.json"
	tvWeeklySchedulePath  = "garoop-tv/schedules/weekly.json"
	tvMonthlySchedulePath = "garoop-tv/schedules/monthly.json"
)

func tvDateSchedulePath(date string) string {
	return "garoop-tv/schedules/dates/" + date + ".json"
}

type tvSlot struct {
	Start    string   `json:"start"`
	End      string   `json:"end"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle"`
	Mission  string   `json:"mission"`
	Reward   string   `json:"reward"`
	Tags     []string `json:"tags,omitempty"`
}

type tvEvent struct {
	ID       string `json:"id"`
	Start    string `json:"start,omitempty"`
	End      string `json:"end,omitempty"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Mission  string `json:"mission"`
	Reward   string `json:"reward"`
	Special  bool   `json:"special,omitempty"`
}

type tvChannel struct {
	ID       int      `json:"id"`
	Emoji    string   `json:"emoji"`
	Name     string   `json:"name"`
	Facts    []string `json:"facts,omitempty"`
	Schedule []tvSlot `json:"schedule,omitempty"`
}

type tvEventChannel struct {
	ChannelID int       `json:"channelId"`
	Programs  []tvEvent `json:"programs"`
}

type tvDateFile struct {
	Date     string           `json:"date,omitempty"`
	Title    string           `json:"title,omitempty"`
	Channels []tvEventChannel `json:"channels"`
}

// tvRawDateFile は既存番組を json.RawMessage のまま保持し、未知フィールドやキー順を壊さずに追記するための型。
type tvRawDateFile struct {
	Date     string `json:"date,omitempty"`
	Title    string `json:"title,omitempty"`
	Channels []struct {
		ChannelID int               `json:"channelId"`
		Programs  []json.RawMessage `json:"programs"`
	} `json:"channels"`
}

var (
	tvDate        string
	tvChannelID   int
	tvNow         bool
	tvStart       string
	tvEnd         string
	tvTitle       string
	tvSubtitle    string
	tvMission     string
	tvReward      string
	tvSpecial     bool
	tvProgramID   string
	tvDateTitle   string
	tvDescription string
	tvURL         string
	tvTargetAge   string
	tvProposer    string
)

var tvTimePattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

var tvCmd = serviceCommand("tv", "GaroopTV: 番組表の閲覧・番組追加・コンテンツ提案", ProfileGaroop, ProfileGaroopTV)

var tvChannelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "チャンネル一覧を表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		channels, err := tvLoadChannels()
		if err != nil {
			return err
		}
		out := make([]map[string]any, 0, len(channels))
		for _, c := range channels {
			out = append(out, map[string]any{"id": c.ID, "emoji": c.Emoji, "name": c.Name, "facts": c.Facts})
		}
		return printJSON(out)
	},
}

var tvScheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "番組表を表示（通常枠＋日付・月次・曜日イベントをサイトと同じ規則でマージ）",
	Example: `  garooptv-cli tv schedule --now
  garooptv-cli tv schedule --date 2026-10-01 --channel 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jst := tvJST()
		now := time.Now().In(jst)
		day := now
		if strings.TrimSpace(tvDate) != "" {
			d, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(tvDate), jst)
			if err != nil {
				return fmt.Errorf("--date は YYYY-MM-DD 形式で指定してください")
			}
			day = d
		}

		channels, err := tvLoadChannels()
		if err != nil {
			return err
		}
		var defaults struct {
			Channels []struct {
				ChannelID int      `json:"channelId"`
				Schedule  []tvSlot `json:"schedule"`
			} `json:"channels"`
		}
		var weekly struct {
			Weekly []struct {
				Weekday  int              `json:"weekday"`
				Channels []tvEventChannel `json:"channels"`
			} `json:"weekly"`
		}
		var monthly struct {
			Monthly []struct {
				MonthDate string           `json:"monthDate"`
				Channels  []tvEventChannel `json:"channels"`
			} `json:"monthly"`
		}
		var dated tvDateFile
		for path, target := range map[string]any{
			tvDefaultSchedulePath:                        &defaults,
			tvWeeklySchedulePath:                         &weekly,
			tvMonthlySchedulePath:                        &monthly,
			tvDateSchedulePath(day.Format("2006-01-02")): &dated,
		} {
			if _, err := garoopdata.FetchJSON(path, target); err != nil {
				return err
			}
		}

		var weeklyChannels, monthlyChannels []tvEventChannel
		for _, w := range weekly.Weekly {
			if w.Weekday == int(day.Weekday()) {
				weeklyChannels = w.Channels
			}
		}
		for _, m := range monthly.Monthly {
			if m.MonthDate == day.Format("01-02") {
				monthlyChannels = m.Channels
			}
		}

		showNow := tvNow && day.Format("2006-01-02") == now.Format("2006-01-02")
		result := make([]map[string]any, 0, len(channels))
		for _, c := range channels {
			if tvChannelID > 0 && c.ID != tvChannelID {
				continue
			}
			slots := c.Schedule
			for _, d := range defaults.Channels {
				if d.ChannelID == c.ID {
					slots = d.Schedule
				}
			}
			events := []tvEvent{}
			for _, group := range [][]tvEventChannel{dated.Channels, monthlyChannels, weeklyChannels} {
				events = append(events, tvProgramsFor(group, c.ID)...)
			}
			entry := map[string]any{
				"id":     c.ID,
				"emoji":  c.Emoji,
				"name":   c.Name,
				"slots":  slots,
				"events": events,
			}
			if showNow {
				current := tvCurrentProgram(slots, events, now)
				entry["current"] = current
				entry["next"] = tvNextSlot(slots, current)
			}
			result = append(result, entry)
		}
		out := map[string]any{
			"date":     day.Format("2006-01-02"),
			"weekday":  day.Weekday().String(),
			"channels": result,
		}
		if dated.Title != "" {
			out["dateTitle"] = dated.Title
		}
		if showNow {
			out["now"] = now.Format("15:04")
		}
		return printJSON(out)
	},
}

var tvScheduleAddCmd = &cobra.Command{
	Use:   "schedule-add",
	Short: "指定日の特別番組を番組表に追加（garoop-data へPR。既定はdry-run）",
	Example: `  garooptv-cli tv schedule-add --date 2026-10-10 --channel 5 --start 18:00 --end 18:59 \
    --title "AI起業ピッチ大会SP" --subtitle "小学生がAIプロダクトを発表" \
    --mission "自分ならどんなAIを作るか考えよう" --reward "AIメダル" --special`,
	RunE: func(cmd *cobra.Command, args []string) error {
		date := strings.TrimSpace(tvDate)
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return fmt.Errorf("--date は YYYY-MM-DD 形式で指定してください")
		}
		if tvChannelID <= 0 {
			return fmt.Errorf("--channel が必要です（`tv channels` で確認）")
		}
		if err := requireValue("title", tvTitle); err != nil {
			return err
		}
		if (tvStart == "") != (tvEnd == "") {
			return fmt.Errorf("--start と --end は両方指定するか、両方省略（終日）してください")
		}
		for _, t := range []string{tvStart, tvEnd} {
			if t != "" && !tvTimePattern.MatchString(t) {
				return fmt.Errorf("時刻は HH:MM 形式で指定してください: %s", t)
			}
		}

		p := garoopdata.NewPublisher()
		channels, err := tvLoadChannelsFrom(p)
		if err != nil {
			return err
		}
		channelName := ""
		for _, c := range channels {
			if c.ID == tvChannelID {
				channelName = c.Name
			}
		}
		if channelName == "" {
			return fmt.Errorf("チャンネル %d は存在しません", tvChannelID)
		}

		path := tvDateSchedulePath(date)
		file := tvRawDateFile{Date: date, Title: strings.TrimSpace(tvDateTitle)}
		existing, found, err := p.ReadPublic(path)
		if err != nil {
			return err
		}
		if found {
			if err := json.Unmarshal(existing, &file); err != nil {
				return fmt.Errorf("%s を解釈できません: %w", path, err)
			}
			if strings.TrimSpace(tvDateTitle) != "" {
				file.Title = strings.TrimSpace(tvDateTitle)
			}
		}

		id := strings.TrimSpace(tvProgramID)
		if id == "" {
			id = fmt.Sprintf("cli-%s-ch%d-%s", date, tvChannelID, time.Now().Format("150405"))
		}
		event := tvEvent{
			ID: id, Start: tvStart, End: tvEnd,
			Title: strings.TrimSpace(tvTitle), Subtitle: strings.TrimSpace(tvSubtitle),
			Mission: strings.TrimSpace(tvMission), Reward: strings.TrimSpace(tvReward),
			Special: tvSpecial,
		}
		eventRaw, err := json.Marshal(event)
		if err != nil {
			return err
		}

		warnings := []string{}
		added := false
		for i := range file.Channels {
			if file.Channels[i].ChannelID != tvChannelID {
				continue
			}
			for _, raw := range file.Channels[i].Programs {
				var e tvEvent
				if err := json.Unmarshal(raw, &e); err != nil {
					return err
				}
				if e.ID == id {
					return fmt.Errorf("番組ID %s は既に存在します", id)
				}
				if tvOverlaps(e, event) {
					warnings = append(warnings, fmt.Sprintf("既存番組「%s」(%s-%s) と時間が重なっています。サイトでは先に登録された番組が優先されます", e.Title, e.Start, e.End))
				}
			}
			file.Channels[i].Programs = append(file.Channels[i].Programs, eventRaw)
			added = true
		}
		if !added {
			file.Channels = append(file.Channels, struct {
				ChannelID int               `json:"channelId"`
				Programs  []json.RawMessage `json:"programs"`
			}{ChannelID: tvChannelID, Programs: []json.RawMessage{eventRaw}})
			sort.SliceStable(file.Channels, func(a, b int) bool { return file.Channels[a].ChannelID < file.Channels[b].ChannelID })
		}

		content, err := garoopdata.MarshalPretty(file)
		if err != nil {
			return err
		}
		title := fmt.Sprintf("garoop-tv: %s %s に「%s」を追加", date, channelName, event.Title)
		body := fmt.Sprintf("garoop-cli `tv schedule-add` で作成。\n\n- 日付: %s\n- チャンネル: %d %s\n- 時間: %s\n- 番組: %s\n- ミッション: %s\n- ごほうび: %s\n",
			date, tvChannelID, channelName, tvTimeLabel(event), event.Title, event.Mission, event.Reward)
		return publishOrDryRun(p, garoopdata.PullRequest{
			Branch:        garoopdata.BranchName("tv-schedule", date),
			Title:         title,
			Body:          body,
			CommitMessage: title,
			Files:         []garoopdata.FileChange{{Path: "public/" + path, Content: content}},
		}, map[string]any{"program": event, "warnings": warnings})
	},
}

var tvProposeCmd = &cobra.Command{
	Use:   "propose",
	Short: "GaroopTVで扱ってほしいコンテンツを提案（スタッフが確認。既定はdry-run。ログイン必須）",
	Example: `  garooptv-cli tv propose --title "宇宙飛行士の1日" --description "ISSでの生活を子ども向けに" \
    --channel 1 --target-age "小学生" --url https://example.com/ref --execute`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireValue("title", tvTitle); err != nil {
			return err
		}
		description, err := readTextOrFile(tvDescription)
		if err != nil {
			return err
		}
		if description == "" {
			return fmt.Errorf("--description が必要です（@file でファイル指定も可）")
		}
		lines := []string{description}
		if v := strings.TrimSpace(tvTargetAge); v != "" {
			lines = append(lines, "対象: "+v)
		}
		if v := strings.TrimSpace(tvURL); v != "" {
			lines = append(lines, "参考URL: "+v)
		}
		if v := strings.TrimSpace(tvProposer); v != "" {
			lines = append(lines, "提案者: "+v)
		}
		input := map[string]any{
			"kind":        "TV_PROPOSAL",
			"title":       strings.TrimSpace(tvTitle),
			"description": strings.Join(lines, "\n"),
		}
		if tvChannelID > 0 {
			input["targetId"] = fmt.Sprint(tvChannelID)
			if channels, err := tvLoadChannels(); err == nil {
				for _, c := range channels {
					if c.ID == tvChannelID {
						input["targetTitle"] = c.Name
					}
				}
			}
		}
		return submitContent("submitContent", input, nil, "", 0)
	},
}

func tvJST() *time.Location {
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		return loc
	}
	return time.FixedZone("JST", 9*60*60)
}

func tvLoadChannels() ([]tvChannel, error) {
	var channels []tvChannel
	found, err := garoopdata.FetchJSON(tvChannelsPath, &channels)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%s が見つかりません", garoopdata.PublicURL(tvChannelsPath))
	}
	return channels, nil
}

func tvLoadChannelsFrom(p *garoopdata.Publisher) ([]tvChannel, error) {
	b, found, err := p.ReadPublic(tvChannelsPath)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%s が見つかりません", tvChannelsPath)
	}
	var channels []tvChannel
	if err := json.Unmarshal(b, &channels); err != nil {
		return nil, err
	}
	return channels, nil
}

func tvProgramsFor(group []tvEventChannel, channelID int) []tvEvent {
	for _, g := range group {
		if g.ChannelID == channelID {
			return g.Programs
		}
	}
	return nil
}

func tvMinutes(t string) int {
	var h, m int
	fmt.Sscanf(t, "%d:%d", &h, &m)
	return h*60 + m
}

func tvInRange(now int, start, end string) bool {
	s, e := tvMinutes(start), tvMinutes(end)
	if s <= e {
		return now >= s && now <= e
	}
	return now >= s || now <= e
}

func tvCurrentProgram(slots []tvSlot, events []tvEvent, now time.Time) *tvSlot {
	minutes := now.Hour()*60 + now.Minute()
	for _, e := range events {
		if e.Start == "" || e.End == "" || tvInRange(minutes, e.Start, e.End) {
			start, end := e.Start, e.End
			if start == "" {
				start = "00:00"
			}
			if end == "" {
				end = "23:59"
			}
			tags := []string{"イベント", "特別放送"}
			if e.Special {
				tags = []string{"イベント", "今日だけ", "プレミアム"}
			}
			return &tvSlot{Start: start, End: end, Title: e.Title, Subtitle: e.Subtitle, Mission: e.Mission, Reward: e.Reward, Tags: tags}
		}
	}
	for i := range slots {
		if tvInRange(minutes, slots[i].Start, slots[i].End) {
			return &slots[i]
		}
	}
	return nil
}

func tvNextSlot(slots []tvSlot, current *tvSlot) *tvSlot {
	if len(slots) == 0 {
		return nil
	}
	if current != nil {
		for i, s := range slots {
			if s.Start == current.Start && s.End == current.End && s.Title == current.Title {
				return &slots[(i+1)%len(slots)]
			}
		}
	}
	return &slots[0]
}

func tvOverlaps(a, b tvEvent) bool {
	if a.Start == "" || b.Start == "" {
		return true
	}
	as, ae, bs, be := tvMinutes(a.Start), tvMinutes(a.End), tvMinutes(b.Start), tvMinutes(b.End)
	if as > ae || bs > be {
		return true
	}
	return as <= be && bs <= ae
}

func tvTimeLabel(e tvEvent) string {
	if e.Start == "" {
		return "終日"
	}
	return e.Start + "-" + e.End
}

func init() {
	tvScheduleCmd.Flags().StringVar(&tvDate, "date", "", "対象日 YYYY-MM-DD（既定: 今日 JST）")
	tvScheduleCmd.Flags().IntVar(&tvChannelID, "channel", 0, "チャンネルIDで絞り込み")
	tvScheduleCmd.Flags().BoolVar(&tvNow, "now", false, "現在放送中・次の番組も表示（今日のみ）")

	tvScheduleAddCmd.Flags().StringVar(&tvDate, "date", "", "放送日 YYYY-MM-DD（必須）")
	tvScheduleAddCmd.Flags().IntVar(&tvChannelID, "channel", 0, "チャンネルID（必須）")
	tvScheduleAddCmd.Flags().StringVar(&tvStart, "start", "", "開始 HH:MM（省略時は終日）")
	tvScheduleAddCmd.Flags().StringVar(&tvEnd, "end", "", "終了 HH:MM")
	tvScheduleAddCmd.Flags().StringVar(&tvTitle, "title", "", "番組タイトル（必須）")
	tvScheduleAddCmd.Flags().StringVar(&tvSubtitle, "subtitle", "", "サブタイトル")
	tvScheduleAddCmd.Flags().StringVar(&tvMission, "mission", "", "視聴後のミッション")
	tvScheduleAddCmd.Flags().StringVar(&tvReward, "reward", "", "ごほうび")
	tvScheduleAddCmd.Flags().BoolVar(&tvSpecial, "special", false, "プレミアム特番として表示")
	tvScheduleAddCmd.Flags().StringVar(&tvProgramID, "id", "", "番組ID（省略時は自動採番）")
	tvScheduleAddCmd.Flags().StringVar(&tvDateTitle, "date-title", "", "その日の特集タイトル（例: 母の日ありがとう特別編成）")

	tvProposeCmd.Flags().StringVar(&tvTitle, "title", "", "提案タイトル（必須）")
	tvProposeCmd.Flags().StringVar(&tvDescription, "description", "", "提案内容（必須。@file 可）")
	tvProposeCmd.Flags().IntVar(&tvChannelID, "channel", 0, "希望チャンネルID")
	tvProposeCmd.Flags().StringVar(&tvTargetAge, "target-age", "", "対象年齢・対象者")
	tvProposeCmd.Flags().StringVar(&tvURL, "url", "", "参考URL")
	tvProposeCmd.Flags().StringVar(&tvProposer, "proposer", "", "提案者のニックネーム")

	markMutates(tvScheduleAddCmd, tvProposeCmd)
	tvCmd.AddCommand(tvChannelsCmd, tvScheduleCmd, tvScheduleAddCmd, tvProposeCmd)
	rootCmd.AddCommand(tvCmd)
}
