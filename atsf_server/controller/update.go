package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const latestReleaseURL = "https://api.github.com/repos/Rain-kl/ATSFlare/releases/latest"

var updateHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

type latestReleaseResponse struct {
	TagName     string `json:"tag_name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

func GetLatestRelease(c *gin.Context) {
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "创建更新请求失败",
		})
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ATSFlare-Server")

	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取最新版本失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("GitHub 返回异常状态: %s", resp.Status),
		})
		return
	}

	var release latestReleaseResponse
	if err = json.NewDecoder(resp.Body).Decode(&release); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "解析最新版本信息失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    release,
	})
}

func UpdateHTTPClientForTest() *http.Client {
	return updateHTTPClient
}

func SetUpdateHTTPClientForTest(client *http.Client) {
	updateHTTPClient = client
}
