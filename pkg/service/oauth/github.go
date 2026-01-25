package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/narasux/goblog/pkg/envs"
)

const (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubUserAPIURL   = "https://api.github.com/user"
)

// GitHubUser GitHub 用户信息
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubTokenResponse GitHub OAuth Token 响应
type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// GetAuthorizeURL 获取 GitHub 授权 URL
func GetAuthorizeURL(state string) string {
	params := url.Values{
		"client_id":    {envs.GithubClientID},
		"redirect_uri": {envs.GithubCallbackURL},
		"scope":        {"read:user user:email"},
		"state":        {state},
	}
	return fmt.Sprintf("%s?%s", githubAuthorizeURL, params.Encode())
}

// ExchangeToken 用 code 换取 access_token
func ExchangeToken(ctx context.Context, code string) (string, error) {
	data := url.Values{
		"client_id":     {envs.GithubClientID},
		"client_secret": {envs.GithubClientSecret},
		"code":          {code},
		"redirect_uri":  {envs.GithubCallbackURL},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", errors.Wrap(err, "create request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "exchange token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "read response")
	}

	var tokenResp GitHubTokenResponse
	if err = json.Unmarshal(body, &tokenResp); err != nil {
		return "", errors.Wrap(err, "parse response")
	}

	if tokenResp.Error != "" {
		return "", errors.Errorf("github oauth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return tokenResp.AccessToken, nil
}

// GetUserInfo 获取 GitHub 用户信息
func GetUserInfo(ctx context.Context, accessToken string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserAPIURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "create request")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "get user info")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("github api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response")
	}

	var user GitHubUser
	if err = json.Unmarshal(body, &user); err != nil {
		return nil, errors.Wrap(err, "parse user info")
	}

	return &user, nil
}
