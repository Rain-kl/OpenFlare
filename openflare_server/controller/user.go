package controller

import (
	"errors"
	"openflare/common"
	"openflare/model"
	"openflare/utils/security"
	"openflare/utils/validation"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Login(c *gin.Context) {
	if !common.PasswordLoginEnabled {
		respondFailure(c, "管理员关闭了密码登录")
		return
	}
	var loginRequest LoginRequest
	if !bindJSON(c, &loginRequest) {
		return
	}
	username := loginRequest.Username
	password := loginRequest.Password
	if username == "" || password == "" {
		respondFailure(c, "无效的参数")
		return
	}
	user := model.User{
		Username: username,
		Password: password,
	}
	err := user.ValidateAndFill()
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	setupLogin(&user, c)
}

// setup token and then return user info
func setLoginToken(user *model.User) (*model.User, error) {
	token, err := ensureUserOpenFlareToken(user)
	if err != nil {
		return nil, err
	}
	cleanUser := &model.User{
		Id:          user.Id,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Status:      user.Status,
		Token:       token,
	}
	return cleanUser, nil
}

func setupLogin(user *model.User, c *gin.Context) {
	cleanUser, err := setLoginToken(user)
	if err != nil {
		respondFailure(c, "无法保存会话信息，请重试")
		return
	}
	respondSuccess(c, *cleanUser)
}

func Logout(c *gin.Context) {
	token := c.GetHeader("OpenFlare-Token")
	if token != "" {
		user := model.ValidateUserToken(token)
		if user != nil && user.Id != 0 {
			if err := model.DB.Model(user).Update("token", "").Error; err != nil {
				respondFailure(c, err.Error())
				return
			}
		}
	}
	respondSuccessMessage(c, "")
}

func ensureUserOpenFlareToken(user *model.User) (string, error) {
	if user.Token != "" {
		return user.Token, nil
	}
	for i := 0; i < 3; i++ {
		token := strings.Replace(uuid.New().String(), "-", "", -1)
		if model.DB.Where("token = ?", token).First(&model.User{}).RowsAffected != 0 {
			continue
		}
		if err := model.DB.Model(user).Update("token", token).Error; err != nil {
			return "", err
		}
		user.Token = token
		return token, nil
	}
	return "", errors.New("生成登录凭证失败，请重试")
}

func currentUserFromOpenFlareToken(c *gin.Context) *model.User {
	token := c.GetHeader("OpenFlare-Token")
	if token == "" {
		return nil
	}
	return model.ValidateUserToken(token)
}

func Register(c *gin.Context) {
	respondFailure(c, "非法请求")
}

func GetAllUsers(c *gin.Context) {
	p, _ := strconv.Atoi(c.Query("p"))
	if p < 0 {
		p = 0
	}
	users, err := model.GetAllUsers(p*common.ItemsPerPage, common.ItemsPerPage)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, users)
}

func SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	users, err := model.SearchUsers(keyword)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, users)
}

func GetUser(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	user, err := model.GetUserById(int(id), false)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	myRole := c.GetInt("role")
	if myRole <= user.Role {
		respondFailure(c, "无权获取同级或更高等级用户的信息")
		return
	}
	respondSuccess(c, user)
}

func GenerateToken(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, true)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	user.Token = uuid.New().String()
	user.Token = strings.Replace(user.Token, "-", "", -1)

	if model.DB.Where("token = ?", user.Token).First(user).RowsAffected != 0 {
		respondFailure(c, "请重试，系统生成的 UUID 竟然重复了！")
		return
	}

	if err := user.Update(false); err != nil {
		respondFailure(c, err.Error())
		return
	}

	respondSuccess(c, user.Token)
}

func GetSelf(c *gin.Context) {
	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, user)
}

func UpdateUser(c *gin.Context) {
	var updatedUser model.User
	if !bindJSON(c, &updatedUser) {
		return
	}
	if updatedUser.Id == 0 {
		respondFailure(c, "无效的参数")
		return
	}
	if updatedUser.Password == "" {
		updatedUser.Password = "$I_LOVE_U" // make Validator happy :)
	}
	if err := validation.Validate.Struct(&updatedUser); err != nil {
		respondFailure(c, "输入不合法 "+err.Error())
		return
	}
	originUser, err := model.GetUserById(updatedUser.Id, false)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	myRole := c.GetInt("role")
	if myRole <= originUser.Role {
		respondFailure(c, "无权更新同权限等级或更高权限等级的用户信息")
		return
	}
	if myRole <= updatedUser.Role {
		respondFailure(c, "无权将其他用户权限等级提升到大于等于自己的权限等级")
		return
	}
	if updatedUser.Password == "$I_LOVE_U" {
		updatedUser.Password = "" // rollback to what it should be
	}
	updatePassword := updatedUser.Password != ""
	if err := updatedUser.Update(updatePassword); err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccessMessage(c, "")
}

func UpdateSelf(c *gin.Context) {
	var user model.User
	if !bindJSON(c, &user) {
		return
	}
	if user.Password == "" {
		user.Password = "$I_LOVE_U" // make Validator happy :)
	}
	if err := validation.Validate.Struct(&user); err != nil {
		respondFailure(c, "输入不合法 "+err.Error())
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
		respondFailure(c, err.Error())
		return
	}

	respondSuccessMessage(c, "")
}

func DeleteUser(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	originUser, err := model.GetUserById(int(id), false)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	myRole := c.GetInt("role")
	if myRole <= originUser.Role {
		respondFailure(c, "无权删除同权限等级或更高权限等级的用户")
		return
	}
	err = model.DeleteUserById(int(id))
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccessMessage(c, "")
}

func DeleteSelf(c *gin.Context) {
	id := c.GetInt("id")
	err := model.DeleteUserById(id)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccessMessage(c, "")
}

func CreateUser(c *gin.Context) {
	var user model.User
	if !bindJSON(c, &user) {
		return
	}
	if user.Username == "" || user.Password == "" {
		respondFailure(c, "无效的参数")
		return
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	myRole := c.GetInt("role")
	if user.Role >= myRole {
		respondFailure(c, "无法创建权限大于等于自己的用户")
		return
	}
	// Even for admin users, we cannot fully trust them!
	cleanUser := model.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.DisplayName,
	}
	if err := cleanUser.Insert(); err != nil {
		respondFailure(c, err.Error())
		return
	}

	respondSuccessMessage(c, "")
}

type ManageRequest struct {
	Username string `json:"username"`
	Action   string `json:"action"`
}

// ManageUser Only admin user can do this
func ManageUser(c *gin.Context) {
	var req ManageRequest
	if !bindJSON(c, &req) {
		return
	}
	user := model.User{
		Username: req.Username,
	}
	// Fill attributes
	model.DB.Where(&user).First(&user)
	if user.Id == 0 {
		respondFailure(c, "用户不存在")
		return
	}
	myRole := c.GetInt("role")
	if myRole <= user.Role && myRole != common.RoleRootUser {
		respondFailure(c, "无权更新同权限等级或更高权限等级的用户信息")
		return
	}
	switch req.Action {
	case "disable":
		user.Status = common.UserStatusDisabled
		if user.Role == common.RoleRootUser {
			respondFailure(c, "无法禁用超级管理员用户")
			return
		}
	case "enable":
		user.Status = common.UserStatusEnabled
	case "delete":
		if user.Role == common.RoleRootUser {
			respondFailure(c, "无法删除超级管理员用户")
			return
		}
		if err := user.Delete(); err != nil {
			respondFailure(c, err.Error())
			return
		}
	case "promote":
		if myRole != common.RoleRootUser {
			respondFailure(c, "普通管理员用户无法提升其他用户为管理员")
			return
		}
		if user.Role >= common.RoleAdminUser {
			respondFailure(c, "该用户已经是管理员")
			return
		}
		user.Role = common.RoleAdminUser
	case "demote":
		if user.Role == common.RoleRootUser {
			respondFailure(c, "无法降级超级管理员用户")
			return
		}
		if user.Role == common.RoleCommonUser {
			respondFailure(c, "该用户已经是普通用户")
			return
		}
		user.Role = common.RoleCommonUser
	}

	if err := user.Update(false); err != nil {
		respondFailure(c, err.Error())
		return
	}
	clearUser := model.User{
		Role:   user.Role,
		Status: user.Status,
	}
	respondSuccess(c, clearUser)
}

func EmailBind(c *gin.Context) {
	email := c.Query("email")
	code := c.Query("code")
	if !security.VerifyCodeWithKey(email, code, security.EmailVerificationPurpose) {
		respondFailure(c, "验证码错误或已过期")
		return
	}
	id := c.GetInt("id")
	user := model.User{
		Id: id,
	}
	err := user.FillUserById()
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	user.Email = email
	// no need to check if this email already taken, because we have used verification code to check it
	err = user.Update(false)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccessMessage(c, "")
}
