package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"openflare/common"
	"openflare/model"
)

const OpenFlareTokenHeader = "OpenFlare-Token"

func authHelper(c *gin.Context, minRole int) {
	token := c.GetHeader(OpenFlareTokenHeader)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无权进行此操作，未登录或 token 无效",
		})
		c.Abort()
		return
	}

	user := model.ValidateUserToken(token)
	if user == nil || user.Username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无权进行此操作，token 无效",
		})
		c.Abort()
		return
	}
	if user.Status == common.UserStatusDisabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户已被封禁",
		})
		c.Abort()
		return
	}
	if user.Role < minRole {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权进行此操作，权限不足",
		})
		c.Abort()
		return
	}
	c.Set("username", user.Username)
	c.Set("role", user.Role)
	c.Set("id", user.Id)
	c.Set("authByToken", true)
	c.Next()
}

func UserAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, common.RoleCommonUser)
	}
}

func AdminAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, common.RoleAdminUser)
	}
}

func RootAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, common.RoleRootUser)
	}
}

// NoTokenAuth is kept as a compatibility no-op because admin APIs now always use OPENFLARE_TOKEN.
func NoTokenAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Next()
	}
}

// TokenOnlyAuth is kept as a compatibility no-op because admin APIs now always use OPENFLARE_TOKEN.
func TokenOnlyAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Next()
	}
}
