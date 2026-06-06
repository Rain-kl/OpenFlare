package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/model"

	"github.com/gin-gonic/gin"
)

type GitHubOAuthResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

type GitHubUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func getGitHubUserInfoByCode(code string) (*GitHubUser, error) {
	if code == "" {
		return nil, errors.New("无效的参数")
	}
	values := map[string]string{"client_id": common.GitHubClientId, "client_secret": common.GitHubClientSecret, "code": code}
	jsonData, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		slog.Error("github oauth access token request failed", "error", err)
		return nil, errors.New("无法连接至 GitHub 服务器，请稍后重试！")
	}
	defer res.Body.Close()
	var oAuthResponse GitHubOAuthResponse
	err = json.NewDecoder(res.Body).Decode(&oAuthResponse)
	if err != nil {
		return nil, err
	}
	req, err = http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oAuthResponse.AccessToken))
	res2, err := client.Do(req)
	if err != nil {
		slog.Error("github user info request failed", "error", err)
		return nil, errors.New("无法连接至 GitHub 服务器，请稍后重试！")
	}
	defer res2.Body.Close()
	var githubUser GitHubUser
	err = json.NewDecoder(res2.Body).Decode(&githubUser)
	if err != nil {
		return nil, err
	}
	if githubUser.Login == "" {
		return nil, errors.New("返回值非法，用户字段为空，请稍后重试！")
	}
	return &githubUser, nil
}

func GitHubOAuth(c *gin.Context) {
	if currentUserFromOpenFlareToken(c) != nil {
		GitHubBind(c)
		return
	}

	if !common.GitHubOAuthEnabled {
		response.RespondFailure(c, "管理员未开启通过 GitHub 登录以及注册")
		return
	}
	code := c.Query("code")
	githubUser, err := getGitHubUserInfoByCode(code)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	user := model.User{
		GitHubId: githubUser.Login,
	}
	if model.IsGitHubIdAlreadyTaken(user.GitHubId) {
		err := user.FillUserByGitHubId()
		if err != nil {
			response.RespondFailure(c, err.Error())
			return
		}
	} else {
		response.RespondFailure(c, "管理员关闭了新用户注册")
		return
	}

	if user.Status != common.UserStatusEnabled {
		response.RespondFailure(c, "用户已被封禁")
		return
	}
	setupLogin(&user, c)
}

func GitHubBind(c *gin.Context) {
	if !common.GitHubOAuthEnabled {
		response.RespondFailure(c, "管理员未开启通过 GitHub 登录以及注册")
		return
	}
	code := c.Query("code")
	githubUser, err := getGitHubUserInfoByCode(code)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	user := model.User{
		GitHubId: githubUser.Login,
	}
	if model.IsGitHubIdAlreadyTaken(user.GitHubId) {
		response.RespondFailure(c, "该 GitHub 账户已被绑定")
		return
	}
	currentUser := currentUserFromOpenFlareToken(c)
	if currentUser == nil {
		response.RespondFailure(c, "无权进行此操作，未登录或 token 无效")
		return
	}
	user.Id = currentUser.Id
	err = user.FillUserById()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	user.GitHubId = githubUser.Login
	err = user.Update(false)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "bind")
}
