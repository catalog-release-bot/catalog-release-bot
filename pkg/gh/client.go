package gh

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/google/go-github/v42/github"
	"golang.org/x/oauth2"
)

type Client struct {
	*github.Client
}

func NewClient(ctx context.Context, token string) *Client {
	var httpClient *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient = oauth2.NewClient(ctx, ts)
	}
	ghClient := github.NewClient(httpClient)
	if baseURLStr, ok := os.LookupEnv("GITHUB_API_URL"); ok {
		baseURL, err := url.Parse(baseURLStr)
		if err != nil {
			panic(fmt.Sprintf("parse GITHUB_API_URL %q: %v", baseURLStr, err))
		}
		ghClient.BaseURL = baseURL
	}

	return &Client{Client: ghClient}
}

func (c *Client) AppRepos(ctx context.Context) ([]*github.Repository, error) {
	lrs, _, err := c.Apps.ListRepos(ctx, nil)
	if err != nil {
		return nil, err
	}
	return lrs.Repositories, nil
}
