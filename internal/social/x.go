package social

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"

	"garoop-cli/internal/authutil"
	"github.com/ChimeraCoder/anaconda"
	"github.com/dghubble/oauth1"
	twitterv2 "github.com/g8rswimmer/go-twitter/v2"
)

type noopAuthorizer struct{}

func (noopAuthorizer) Add(_ *http.Request) {}

type XClient struct {
	v1      *anaconda.TwitterApi
	v2      *twitterv2.Client
	execute bool
}

type XPost struct {
	ID        string
	Text      string
	CreatedAt string
}

func NewXClient(execute bool) (*XClient, error) {
	if !execute {
		return &XClient{execute: false}, nil
	}

	consumerKey := strings.TrimSpace(os.Getenv("X_CONSUMER_KEY"))
	consumerSecret := strings.TrimSpace(os.Getenv("X_CONSUMER_SECRET"))
	accessToken := strings.TrimSpace(os.Getenv("X_ACCESS_TOKEN"))
	accessSecret := strings.TrimSpace(os.Getenv("X_ACCESS_TOKEN_SECRET"))
	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		var token struct {
			ConsumerKey       string `json:"consumer_key"`
			ConsumerSecret    string `json:"consumer_secret"`
			AccessToken       string `json:"access_token"`
			AccessTokenSecret string `json:"access_token_secret"`
		}
		if err := authutil.LoadJSON("tokens/x.json", &token); err == nil {
			if consumerKey == "" {
				consumerKey = strings.TrimSpace(token.ConsumerKey)
			}
			if consumerSecret == "" {
				consumerSecret = strings.TrimSpace(token.ConsumerSecret)
			}
			if accessToken == "" {
				accessToken = strings.TrimSpace(token.AccessToken)
			}
			if accessSecret == "" {
				accessSecret = strings.TrimSpace(token.AccessTokenSecret)
			}
		}
	}
	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessSecret == "" {
		return nil, fmt.Errorf("Xの実行には環境変数か tokens/x.json の認証情報が必要です")
	}

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	httpClient := config.Client(context.Background(), token)

	v2Client := &twitterv2.Client{
		Authorizer: noopAuthorizer{},
		Client:     httpClient,
		Host:       "https://api.twitter.com",
	}
	v1Client := anaconda.NewTwitterApiWithCredentials(accessToken, accessSecret, consumerKey, consumerSecret)

	return &XClient{
		v1:      v1Client,
		v2:      v2Client,
		execute: true,
	}, nil
}

func (c *XClient) Post(text string, imagePath string) (string, error) {
	if !c.execute {
		return fmt.Sprintf("[dry-run] X post: %q image=%q", text, imagePath), nil
	}

	req := twitterv2.CreateTweetRequest{Text: text}
	if imagePath != "" {
		mediaID, err := c.uploadMedia(imagePath)
		if err != nil {
			return "", err
		}
		req.Media = &twitterv2.CreateTweetMedia{
			IDs: []string{mediaID},
		}
	}

	resp, err := c.v2.CreateTweet(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("X post失敗: %w", err)
	}
	if resp == nil || resp.Tweet == nil || resp.Tweet.ID == "" {
		return "", fmt.Errorf("X post失敗: レスポンスにtweet idがありません")
	}
	return resp.Tweet.ID, nil
}

func (c *XClient) Reply(tweetID string, text string, imagePath string) (string, error) {
	if !c.execute {
		return fmt.Sprintf("[dry-run] X reply: to=%s text=%q image=%q", tweetID, text, imagePath), nil
	}

	req := twitterv2.CreateTweetRequest{
		Text: text,
		Reply: &twitterv2.CreateTweetReply{
			InReplyToTweetID: tweetID,
		},
	}
	if imagePath != "" {
		mediaID, err := c.uploadMedia(imagePath)
		if err != nil {
			return "", err
		}
		req.Media = &twitterv2.CreateTweetMedia{
			IDs: []string{mediaID},
		}
	}

	resp, err := c.v2.CreateTweet(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("X reply失敗: %w", err)
	}
	if resp == nil || resp.Tweet == nil || resp.Tweet.ID == "" {
		return "", fmt.Errorf("X reply失敗: レスポンスにtweet idがありません")
	}
	return resp.Tweet.ID, nil
}

func (c *XClient) uploadMedia(path string) (string, error) {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("画像読み込み失敗: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(fileBytes)

	media, err := c.v1.UploadMedia(encoded)
	if err != nil {
		return "", fmt.Errorf("X media upload失敗: %w", err)
	}
	if media.MediaIDString == "" {
		return "", fmt.Errorf("X media upload失敗: media idが空です")
	}
	return media.MediaIDString, nil
}

func (c *XClient) RecentOwnPosts(limit int) ([]XPost, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	if !c.execute {
		return []XPost{
			{
				ID:        "dry-run-x-1",
				Text:      "ガルちゃん向けXサンプル投稿",
				CreatedAt: "2026-03-05T00:00:00Z",
			},
		}, nil
	}

	me, err := c.v2.AuthUserLookup(context.Background(), twitterv2.UserLookupOpts{})
	if err != nil {
		return nil, err
	}
	if me == nil || me.Raw == nil || len(me.Raw.Users) == 0 || me.Raw.Users[0] == nil {
		return nil, fmt.Errorf("認証ユーザー情報を取得できませんでした")
	}
	userID := me.Raw.Users[0].ID
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("認証ユーザーIDが空です")
	}

	opts := twitterv2.UserTweetTimelineOpts{
		MaxResults:  limit,
		TweetFields: []twitterv2.TweetField{twitterv2.TweetFieldText, twitterv2.TweetFieldCreatedAt},
	}
	timeline, err := c.v2.UserTweetTimeline(context.Background(), userID, opts)
	if err != nil {
		return nil, err
	}
	if timeline == nil || timeline.Raw == nil {
		return []XPost{}, nil
	}

	posts := make([]XPost, 0, len(timeline.Raw.Tweets))
	for _, tw := range timeline.Raw.Tweets {
		if tw == nil {
			continue
		}
		posts = append(posts, XPost{
			ID:        tw.ID,
			Text:      tw.Text,
			CreatedAt: tw.CreatedAt,
		})
	}
	return posts, nil
}
