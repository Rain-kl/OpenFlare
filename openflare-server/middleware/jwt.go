package middleware

import (
	"log"
	"time"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/model"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

var JWTMiddleware *jwt.GinJWTMiddleware

// jwtSigningKey returns JWT_SECRET when set, falling back to SESSION_SECRET
// for backward compatibility with deployments that only configure SESSION_SECRET.
func jwtSigningKey() []byte {
	if common.JWTSecret != "" {
		return []byte(common.JWTSecret)
	}
	return []byte(common.SessionSecret)
}

func InitJWTMiddleware() {
	var err error
	JWTMiddleware, err = jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "openflare",
		Key:         jwtSigningKey(),
		Timeout:     24 * time.Hour,
		MaxRefresh:  24 * time.Hour,
		IdentityKey: "identity",
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*model.User); ok {
				return jwt.MapClaims{
					"id":       v.Id,
					"username": v.Username,
					"role":     v.Role,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			id, ok := claims["id"].(float64)
			if !ok {
				return nil
			}
			username, _ := claims["username"].(string)
			role, _ := claims["role"].(float64)
			return &model.User{
				Id:       int(id),
				Username: username,
				Role:     int(role),
			}
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			return data != nil
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			response.RespondErrorWithStatus(c, code, "无权进行此操作，未登录或 token 无效: "+message)
		},
		TokenLookup:   "header: OpenFlare-Token",
		TokenHeadName: "", // Empty for raw token value directly
		SendCookie:    false,
	})

	if err != nil {
		log.Fatalf("JWT Init Error: %s", err.Error())
	}
}
