package controller

import (
	"fmt"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/controller/bind"
	"github.com/rain-kl/openflare/openflare-server/model"
	"github.com/rain-kl/openflare/openflare-server/service"
	"github.com/rain-kl/openflare/openflare-server/utils/mail"
	"github.com/rain-kl/openflare/openflare-server/utils/security"
	"github.com/rain-kl/openflare/openflare-server/utils/validation"

	"github.com/gin-gonic/gin"
)

// GetStatus godoc
// @Summary Get server status
// @Tags Public
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/status [get]
func GetStatus(c *gin.Context) {
	authSources, err := service.PublicAuthSources("/api")
	if err != nil {
		authSources = []service.PublicAuthSource{}
	}
	response.RespondSuccess(c, gin.H{
		"version":                   common.Version,
		"start_time":                common.StartTime,
		"email_verification":        common.EmailVerificationEnabled,
		"github_oauth":              common.GitHubOAuthEnabled,
		"github_client_id":          common.GitHubClientId,
		"system_name":               common.SystemName,
		"home_page_link":            common.HomePageLink,
		"footer_html":               common.Footer,
		"wechat_qrcode":             common.WeChatAccountQRCodeImageURL,
		"wechat_login":              common.WeChatAuthEnabled,
		"server_address":            common.ServerAddress,
		"password_register_enabled": common.PasswordRegisterEnabled,
		"auth_sources":              authSources,
	})
}

func GetNotice(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	response.RespondSuccess(c, common.OptionMap["Notice"])
}

func GetAbout(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	response.RespondSuccess(c, common.OptionMap["About"])
}

func SendEmailVerification(c *gin.Context) {
	email := c.Query("email")
	if err := validation.Validate.Var(email, "required,email"); err != nil {
		response.RespondFailure(c, "无效的参数")
		return
	}
	if model.IsEmailAlreadyTaken(email) {
		response.RespondFailure(c, "邮箱地址已被占用")
		return
	}
	code := security.GenerateVerificationCode(6)
	security.RegisterVerificationCodeWithKey(email, code, security.EmailVerificationPurpose)
	subject := fmt.Sprintf("%s邮箱验证邮件", common.SystemName)
	content := fmt.Sprintf("<p>您好，你正在进行%s邮箱验证。</p>"+
		"<p>您的验证码为: <strong>%s</strong></p>"+
		"<p>验证码 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.SystemName, code, security.VerificationValidMinutes)
	cfg := mail.SMTPConfig{
		Server:     common.SMTPServer,
		Port:       common.SMTPPort,
		Account:    common.SMTPAccount,
		Token:      common.SMTPToken,
		SystemName: common.SystemName,
	}
	err := mail.SendEmail(cfg, subject, email, content)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}

func SendPasswordResetEmail(c *gin.Context) {
	email := c.Query("email")
	if err := validation.Validate.Var(email, "required,email"); err != nil {
		response.RespondFailure(c, "无效的参数")
		return
	}
	if !model.IsEmailAlreadyTaken(email) {
		response.RespondFailure(c, "该邮箱地址未注册")
		return
	}
	code := security.GenerateVerificationCode(0)
	security.RegisterVerificationCodeWithKey(email, code, security.PasswordResetPurpose)
	link := fmt.Sprintf("%s/user/reset?email=%s&token=%s", common.ServerAddress, email, code)
	subject := fmt.Sprintf("%s密码重置", common.SystemName)
	content := fmt.Sprintf("<p>您好，你正在进行%s密码重置。</p>"+
		"<p>点击<a href='%s'>此处</a>进行密码重置。</p>"+
		"<p>重置链接 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.SystemName, link, security.VerificationValidMinutes)
	cfg := mail.SMTPConfig{
		Server:     common.SMTPServer,
		Port:       common.SMTPPort,
		Account:    common.SMTPAccount,
		Token:      common.SMTPToken,
		SystemName: common.SystemName,
	}
	err := mail.SendEmail(cfg, subject, email, content)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}

type PasswordResetRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

func ResetPassword(c *gin.Context) {
	var req PasswordResetRequest
	if !bind.JSON(c, &req) {
		return
	}
	if req.Email == "" || req.Token == "" {
		response.RespondFailure(c, "无效的参数")
		return
	}
	if !security.VerifyCodeWithKey(req.Email, req.Token, security.PasswordResetPurpose) {
		response.RespondFailure(c, "重置链接非法或已过期")
		return
	}
	password := security.GenerateVerificationCode(12)
	err := model.ResetUserPasswordByEmail(req.Email, password)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	security.DeleteKey(req.Email, security.PasswordResetPurpose)
	response.RespondSuccess(c, password)
}
