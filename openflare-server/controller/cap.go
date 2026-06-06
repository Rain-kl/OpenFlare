package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rain-kl/openflare/openflare-server/controller/bind"
	"github.com/rain-kl/openflare/openflare-server/service"
	"github.com/rain-kl/openflare/openflare-server/utils/cap"
)

// GetCapChallenge generates a new CAPTCHA challenge
func GetCapChallenge(c *gin.Context) {
	scope := c.Param("scope")
	if scope == "" {
		scope = c.Query("scope")
	}
	resp, err := service.CapManager.Generate(scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// RedeemCapChallenge validates CAPTCHA solutions and yields a one-time redeem token
func RedeemCapChallenge(c *gin.Context) {
	scope := c.Param("scope")
	if scope == "" {
		scope = c.Query("scope")
	}

	var req cap.RedeemRequest
	if !bind.JSON(c, &req) {
		return
	}

	resp, err := service.CapManager.Redeem(c.Request.Context(), req.Token, req.Solutions, scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}
