package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/model"

	"github.com/gin-gonic/gin"
)

type wechatLoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func getWeChatIdByCode(code string) (string, error) {
	if code == "" {
		return "", errors.New("无效的参数")
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/wechat/user?code=%s", common.WeChatServerAddress, code), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", common.WeChatServerToken)
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	httpResponse, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error("Failed to close response body", "error", err)
		}
	}(httpResponse.Body)
	var res wechatLoginResponse
	err = json.NewDecoder(httpResponse.Body).Decode(&res)
	if err != nil {
		return "", err
	}
	if !res.Success {
		return "", errors.New(res.Message)
	}
	if res.Data == "" {
		return "", errors.New("验证码错误或已过期")
	}
	return res.Data, nil
}

func WeChatAuth(c *gin.Context) {
	if !common.WeChatAuthEnabled {
		response.RespondFailure(c, "管理员未开启通过微信登录以及注册")
		return
	}
	code := c.Query("code")
	wechatId, err := getWeChatIdByCode(code)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	user := model.User{
		WeChatId: wechatId,
	}
	if model.IsWeChatIdAlreadyTaken(wechatId) {
		err := user.FillUserByWeChatId()
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

func WeChatBind(c *gin.Context) {
	if !common.WeChatAuthEnabled {
		response.RespondFailure(c, "管理员未开启通过微信登录以及注册")
		return
	}
	code := c.Query("code")
	wechatId, err := getWeChatIdByCode(code)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	if model.IsWeChatIdAlreadyTaken(wechatId) {
		response.RespondFailure(c, "该微信账号已被绑定")
		return
	}
	id := c.GetInt("id")
	user := model.User{
		Id: id,
	}
	err = user.FillUserById()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	user.WeChatId = wechatId
	err = user.Update(false)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
	return
}
