package github

import (
	"context"
	"errors"
	"fmt"

	"go-notes/internal/config"

	gogithub "github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

var ErrMissingToken = errors.New("GITHUB_TOKEN is required")

type Client struct {
	cfg config.Config
	api *gogithub.Client
}

func NewClient(cfg config.Config) *Client {
	var api *gogithub.Client
	if cfg.GitHubToken != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.GitHubToken})
		api = gogithub.NewClient(oauth2.NewClient(context.Background(), ts))
	}
	return &Client{cfg: cfg, api: api}
}

func (c *Client) CreateRepoFromTemplate(ctx context.Context, name string) (*gogithub.Repository, error) {
	if c.api == nil {
		return nil, ErrMissingToken
	}
	if c.cfg.GitHubTemplateOwner == "" || c.cfg.GitHubTemplateRepo == "" {
		return nil, errors.New("GITHUB_TEMPLATE_OWNER and GITHUB_TEMPLATE_REPO are required")
	}

	owner := c.cfg.GitHubRepoOwner
	if owner == "" {
		owner = c.cfg.GitHubTemplateOwner
	}

	req := &gogithub.TemplateRepoRequest{
		Name:    gogithub.String(name),
		Owner:   gogithub.String(owner),
		Private: gogithub.Bool(c.cfg.GitHubRepoPrivate),
	}

	repo, _, err := c.api.Repositories.CreateFromTemplate(ctx, c.cfg.GitHubTemplateOwner, c.cfg.GitHubTemplateRepo, req)
	if err != nil {
		return nil, fmt.Errorf("create repo from template: %w", err)
	}

	return repo, nil
}
