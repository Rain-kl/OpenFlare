package controller

import (
	"openflare/common"
	"openflare/common/response"
	"openflare/controller/bind"
	"openflare/middleware"
	"openflare/model"
	"openflare/utils/security"
	"openflare/utils/validation"
	"strconv"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Login(c *gin.Context) {
	if !common.PasswordLoginEnabled {
		response.RespondFailure(c, "管理员关闭了密码登录")
		return
	}
	var loginRequest LoginRequest
	if !bind.JSON(c, &loginRequest) {
		return
	}
	username := loginRequest.Username
	password := loginRequest.Password
	if username == "" || password == "" {
		response.RespondFailure(c, "无效的参数")
		return
	}
	user := model.User{
		Username: username,
		Password: password,
	}
	err := user.ValidateAndFill()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	setupLogin(&user, c)
}

// setup token and then return user info
func setLoginToken(user *model.User) (*model.User, error) {
	// Generate a signed JWT using gin-jwt middleware
	tokenString, _, err := middleware.JWTMiddleware.TokenGenerator(user)
	if err != nil {
		return nil, err
	}
	// Persist JWT in DB so we can invalidate it on logout
	if err := model.DB.Model(user).Update("token", tokenString).Error; err != nil {
		return nil, err
	}
	cleanUser := &model.User{
		Id:          user.Id,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Status:      user.Status,
		Token:       tokenString,
	}
	return cleanUser, nil
}

func setupLogin(user *model.User, c *gin.Context) {
	cleanUser, err := setLoginToken(user)
	if err != nil {
		response.RespondFailure(c, "无法保存会话信息，请重试")
		return
	}
	response.RespondSuccess(c, *cleanUser)
}

func Logout(c *gin.Context) {
	token := c.GetHeader("OpenFlare-Token")
	if token != "" {
		user := model.ValidateUserToken(token)
		if user != nil && user.Id != 0 {
			if err := model.DB.Model(user).Update("token", "").Error; err != nil {
				response.RespondFailure(c, err.Error())
				return
			}
		}
	}
	response.RespondSuccessMessage(c, "")
}

func currentUserFromOpenFlareToken(c *gin.Context) *model.User {
	token := c.GetHeader("OpenFlare-Token")
	if token == "" {
		return nil
	}
	return model.ValidateUserToken(token)
}

func Register(c *gin.Context) {
	response.RespondFailure(c, "非法请求")
}

func GetAllUsers(c *gin.Context) {
	p, _ := strconv.Atoi(c.Query("p"))
	if p < 0 {
		p = 0
	}
	users, err := model.GetAllUsers(p*common.ItemsPerPage, common.ItemsPerPage)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, users)
}

func SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	users, err := model.SearchUsers(keyword)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, users)
}

func GetUser(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	user, err := model.GetUserById(int(id), false)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	myRole := c.GetInt("role")
	if myRole <= user.Role {
		response.RespondFailure(c, "无权获取同级或更高等级用户的信息")
		return
	}
	response.RespondSuccess(c, user)
}

func GenerateToken(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, true)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	// Generate a fresh JWT for the user
	tokenString, _, err := middleware.JWTMiddleware.TokenGenerator(user)
	if err != nil {
		response.RespondFailure(c, "生成 Token 失败: "+err.Error())
		return
	}
	user.Token = tokenString
	if err := user.Update(false); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, user.Token)
}

func GetSelf(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, user)
}

func UpdateUser(c *gin.Context) {
	var updatedUser model.User
	if !bind.JSON(c, &updatedUser) {
		return
	}
	if updatedUser.Id == 0 {
		response.RespondFailure(c, "无效的参数")
		return
	}
	if updatedUser.Password == "" {
		updatedUser.Password = "$I_LOVE_U" // make Validator happy :)
	}
	if err := validation.Validate.Struct(&updatedUser); err != nil {
		response.RespondFailure(c, "输入不合法 "+err.Error())
		return
	}
	originUser, err := model.GetUserById(updatedUser.Id, false)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	myRole := c.GetInt("role")
	if myRole <= originUser.Role {
		response.RespondFailure(c, "无权更新同权限等级或更高权限等级的用户信息")
		return
	}
	if myRole <= updatedUser.Role {
		response.RespondFailure(c, "无权将其他用户权限等级提升到大于等于自己的权限等级")
		return
	}
	if updatedUser.Password == "$I_LOVE_U" {
		updatedUser.Password = "" // rollback to what it should be
	}
	updatePassword := updatedUser.Password != ""
	if err := updatedUser.Update(updatePassword); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}

func UpdateSelf(c *gin.Context) {
	var user model.User
	if !bind.JSON(c, &user) {
		return
	}
	if user.Password == "" {
		user.Password = "$I_LOVE_U" // make Validator happy :)
	}
	if err := validation.Validate.Struct(&user); err != nil {
		response.RespondFailure(c, "输入不合法 "+err.Error())
		return
	}

	cleanUser := model.User{
		Id:          c.GetInt("id"),
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.DisplayName,
	}
	if user.Password == "$I_LOVE_U" {
		user.Password = "" // rollback to what it should be
		cleanUser.Password = ""
	}
	updatePassword := user.Password != ""
	if err := cleanUser.Update(updatePassword); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}

	response.RespondSuccessMessage(c, "")
}

func DeleteUser(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	originUser, err := model.GetUserById(int(id), false)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	myRole := c.GetInt("role")
	if myRole <= originUser.Role {
		response.RespondFailure(c, "无权删除同权限等级或更高权限等级的用户")
		return
	}
	err = model.DeleteUserById(int(id))
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}

func DeleteSelf(c *gin.Context) {
	id := c.GetInt("id")
	err := model.DeleteUserById(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}

func CreateUser(c *gin.Context) {
	var user model.User
	if !bind.JSON(c, &user) {
		return
	}
	if user.Username == "" || user.Password == "" {
		response.RespondFailure(c, "无效的参数")
		return
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	myRole := c.GetInt("role")
	if user.Role >= myRole {
		response.RespondFailure(c, "无法创建权限大于等于自己的用户")
		return
	}
	// Even for admin users, we cannot fully trust them!
	cleanUser := model.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.DisplayName,
	}
	if err := cleanUser.Insert(); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}

	response.RespondSuccessMessage(c, "")
}

type ManageRequest struct {
	Username string `json:"username"`
	Action   string `json:"action"`
}

// ManageUser Only admin user can do this
func ManageUser(c *gin.Context) {
	var req ManageRequest
	if !bind.JSON(c, &req) {
		return
	}
	user := model.User{
		Username: req.Username,
	}
	// Fill attributes
	model.DB.Where(&user).First(&user)
	if user.Id == 0 {
		response.RespondFailure(c, "用户不存在")
		return
	}
	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		response.RespondFailure(c, "无权更新同权限等级或更高权限等级的用户信息")
		return
	}
	switch req.Action {
	case "disable":
		user.Status = common.UserStatusDisabled
		if user.Role == common.RoleRootUser {
			response.RespondFailure(c, "无法禁用超级管理员用户")
			return
		}
	case "enable":
		user.Status = common.UserStatusEnabled
	case "delete":
		if user.Role == common.RoleRootUser {
			response.RespondFailure(c, "无法删除超级管理员用户")
			return
		}
		if err := user.Delete(); err != nil {
			response.RespondFailure(c, err.Error())
			return
		}
	case "promote":
		if myRole != common.RoleRootUser {
			response.RespondFailure(c, "普通管理员用户无法提升其他用户为管理员")
			return
		}
		if user.Role >= common.RoleAdminUser {
			response.RespondFailure(c, "该用户已经是管理员")
			return
		}
		user.Role = common.RoleAdminUser
	case "demote":
		if user.Role == common.RoleRootUser {
			response.RespondFailure(c, "无法降级超级管理员用户")
			return
		}
		if user.Role == common.RoleCommonUser {
			response.RespondFailure(c, "该用户已经是普通用户")
			return
		}
		user.Role = common.RoleCommonUser
	}

	if err := user.Update(false); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	clearUser := model.User{
		Role:   user.Role,
		Status: user.Status,
	}
	response.RespondSuccess(c, clearUser)
}

func EmailBind(c *gin.Context) {
	email := c.Query("email")
	code := c.Query("code")
	if !security.VerifyCodeWithKey(email, code, security.EmailVerificationPurpose) {
		response.RespondFailure(c, "验证码错误或已过期")
		return
	}
	id := c.GetInt("id")
	user := model.User{
		Id: id,
	}
	err := user.FillUserById()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	user.Email = email
	// no need to check if this email already taken, because we have used verification code to check it
	err = user.Update(false)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}
