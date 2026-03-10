package controller

import (
	"atsflare/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func GetConfigVersions(c *gin.Context) {
	versions, err := service.ListConfigVersions()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    versions,
	})
}

func GetActiveConfigVersion(c *gin.Context) {
	version, err := service.GetActiveConfigVersion()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "当前没有激活版本",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    version,
	})
}

func PreviewConfigVersion(c *gin.Context) {
	preview, err := service.PreviewConfigVersion()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    preview,
	})
}

func DiffConfigVersion(c *gin.Context) {
	diff, err := service.DiffConfigVersion()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    diff,
	})
}

func PublishConfigVersion(c *gin.Context) {
	username := c.GetString("username")
	result, err := service.PublishConfigVersion(username)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result.Version,
	})
}

func ActivateConfigVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	version, err := service.ActivateConfigVersion(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    version,
	})
}
