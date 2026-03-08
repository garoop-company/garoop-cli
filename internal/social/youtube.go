package social

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/yamashitadaiki/garoop-cli/internal/authutil"
)

type YouTubeClient struct {
	accessToken string
	execute     bool
	httpClient  *http.Client
}

type YouTubeVideo struct {
	VideoID     string
	Title       string
	Description string
	PublishedAt string
}

type ReplyResult struct {
	TargetCommentID string
	ReplyCommentID  string
	VideoID         string
}

type commentThreadItem struct {
	Snippet struct {
		VideoID         string `json:"videoId"`
		TopLevelComment struct {
			ID      string `json:"id"`
			Snippet struct {
				TextDisplay     string `json:"textDisplay"`
				AuthorChannelID struct {
					Value string `json:"value"`
				} `json:"authorChannelId"`
			} `json:"snippet"`
		} `json:"topLevelComment"`
	} `json:"snippet"`
	Replies struct {
		Comments []struct {
			Snippet struct {
				AuthorChannelID struct {
					Value string `json:"value"`
				} `json:"authorChannelId"`
			} `json:"snippet"`
		} `json:"comments"`
	} `json:"replies"`
}

type commentThreadListResponse struct {
	Items []commentThreadItem `json:"items"`
}

func NewYouTubeClient(execute bool) (*YouTubeClient, error) {
	if !execute {
		return &YouTubeClient{
			execute:    false,
			httpClient: http.DefaultClient,
		}, nil
	}
	token := strings.TrimSpace(os.Getenv("YOUTUBE_ACCESS_TOKEN"))
	if token == "" {
		var saved struct {
			AccessToken string `json:"access_token"`
		}
		if err := authutil.LoadJSON("tokens/youtube.json", &saved); err == nil {
			token = strings.TrimSpace(saved.AccessToken)
		}
	}
	if token == "" {
		return nil, fmt.Errorf("YouTubeの実行には環境変数か tokens/youtube.json の access_token が必要です")
	}
	return &YouTubeClient{
		accessToken: token,
		execute:     true,
		httpClient:  http.DefaultClient,
	}, nil
}

func (c *YouTubeClient) UploadVideo(videoPath string, title string, description string, thumbnailPath string) (string, error) {
	if !c.execute {
		return fmt.Sprintf("[dry-run] YouTube upload: video=%q title=%q thumbnail=%q", videoPath, title, thumbnailPath), nil
	}

	videoBytes, err := os.ReadFile(videoPath)
	if err != nil {
		return "", fmt.Errorf("動画読み込み失敗: %w", err)
	}

	metadata := map[string]any{
		"snippet": map[string]any{
			"title":       title,
			"description": description,
		},
		"status": map[string]any{
			"privacyStatus": "public",
		},
	}
	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	metaPart, err := writer.CreatePart(textProtoHeader(map[string]string{
		"Content-Type": "application/json; charset=UTF-8",
	}))
	if err != nil {
		return "", err
	}
	if _, err := metaPart.Write(metaBytes); err != nil {
		return "", err
	}

	videoPart, err := writer.CreatePart(textProtoHeader(map[string]string{
		"Content-Type": "application/octet-stream",
	}))
	if err != nil {
		return "", err
	}
	if _, err := videoPart.Write(videoBytes); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://www.googleapis.com/upload/youtube/v3/videos?part=snippet,status&uploadType=multipart", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("YouTube upload失敗: %s", string(respBody))
	}

	var uploaded struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&uploaded); err != nil {
		return "", err
	}

	if thumbnailPath != "" {
		if err := c.setThumbnail(uploaded.ID, thumbnailPath); err != nil {
			return uploaded.ID, fmt.Errorf("動画アップロード成功, サムネ設定失敗: %w", err)
		}
	}
	return uploaded.ID, nil
}

func (c *YouTubeClient) Comment(videoID string, text string) (string, error) {
	if !c.execute {
		return fmt.Sprintf("[dry-run] YouTube comment: video_id=%q text=%q", videoID, text), nil
	}

	payload := map[string]any{
		"snippet": map[string]any{
			"videoId": videoID,
			"topLevelComment": map[string]any{
				"snippet": map[string]any{
					"textOriginal": text,
				},
			},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://www.googleapis.com/youtube/v3/commentThreads?part=snippet", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("YouTubeコメント失敗: %s", string(respBody))
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *YouTubeClient) AutoReplyChannelComments(channelID string, ownChannelID string, replyText string, limit int, maxReplies int) ([]ReplyResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if maxReplies <= 0 {
		maxReplies = 5
	}
	if !c.execute {
		return []ReplyResult{
			{
				TargetCommentID: "[dry-run-parent-comment]",
				ReplyCommentID:  "[dry-run]",
				VideoID:         "[dry-run-video]",
			},
		}, nil
	}

	threads, err := c.fetchCommentThreads(channelID, limit)
	if err != nil {
		return nil, err
	}

	results := make([]ReplyResult, 0, maxReplies)
	for _, item := range threads.Items {
		parentID := item.Snippet.TopLevelComment.ID
		if parentID == "" {
			continue
		}

		authorChannelID := item.Snippet.TopLevelComment.Snippet.AuthorChannelID.Value
		if ownChannelID != "" && authorChannelID == ownChannelID {
			continue
		}
		if hasReplyFromChannel(item, ownChannelID) {
			continue
		}

		if !c.execute {
			results = append(results, ReplyResult{
				TargetCommentID: parentID,
				ReplyCommentID:  "[dry-run]",
				VideoID:         item.Snippet.VideoID,
			})
		} else {
			replyID, err := c.replyToComment(parentID, replyText)
			if err != nil {
				return results, err
			}
			results = append(results, ReplyResult{
				TargetCommentID: parentID,
				ReplyCommentID:  replyID,
				VideoID:         item.Snippet.VideoID,
			})
		}

		if len(results) >= maxReplies {
			break
		}
	}

	return results, nil
}

func (c *YouTubeClient) fetchCommentThreads(channelID string, limit int) (*commentThreadListResponse, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channelIDが空です")
	}

	u, _ := url.Parse("https://www.googleapis.com/youtube/v3/commentThreads")
	q := u.Query()
	q.Set("part", "snippet,replies")
	q.Set("allThreadsRelatedToChannelId", channelID)
	q.Set("maxResults", strconv.Itoa(limit))
	q.Set("order", "time")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if c.execute {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("YouTubeコメント一覧取得失敗: %s", string(respBody))
	}

	var out commentThreadListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *YouTubeClient) replyToComment(parentID string, text string) (string, error) {
	payload := map[string]any{
		"snippet": map[string]any{
			"parentId":     parentID,
			"textOriginal": text,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://www.googleapis.com/youtube/v3/comments?part=snippet", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("YouTubeコメント返信失敗: %s", string(respBody))
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func hasReplyFromChannel(item commentThreadItem, ownChannelID string) bool {
	if ownChannelID == "" {
		return false
	}
	for _, reply := range item.Replies.Comments {
		if reply.Snippet.AuthorChannelID.Value == ownChannelID {
			return true
		}
	}
	return false
}

func (c *YouTubeClient) setThumbnail(videoID string, imagePath string) error {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("サムネ読み込み失敗: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://www.googleapis.com/upload/youtube/v3/thumbnails/set?videoId="+videoID, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "image/jpeg")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("サムネ設定失敗: %s", string(respBody))
	}
	return nil
}

func (c *YouTubeClient) RecentChannelVideos(channelID string, limit int) ([]YouTubeVideo, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	if !c.execute {
		return []YouTubeVideo{
			{
				VideoID:     "dry-run-video-1",
				Title:       "ガルちゃん向けYouTubeサンプル",
				Description: "サンプル動画説明",
				PublishedAt: "2026-03-05T00:00:00Z",
			},
		}, nil
	}
	if strings.TrimSpace(channelID) == "" {
		return nil, fmt.Errorf("channelIDが空です")
	}

	u, _ := url.Parse("https://www.googleapis.com/youtube/v3/search")
	q := u.Query()
	q.Set("part", "snippet")
	q.Set("channelId", channelID)
	q.Set("order", "date")
	q.Set("type", "video")
	q.Set("maxResults", strconv.Itoa(limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("YouTube動画一覧取得失敗: %s", string(body))
	}

	var out struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				PublishedAt string `json:"publishedAt"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	videos := make([]YouTubeVideo, 0, len(out.Items))
	for _, item := range out.Items {
		videos = append(videos, YouTubeVideo{
			VideoID:     item.ID.VideoID,
			Title:       item.Snippet.Title,
			Description: item.Snippet.Description,
			PublishedAt: item.Snippet.PublishedAt,
		})
	}
	return videos, nil
}
