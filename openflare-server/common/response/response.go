package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const invalidParamsMessage = "参数错误"

// RespondSuccess sends a successful response with data
func RespondSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// RespondSuccessWithExtras sends a successful response with data and extra fields
func RespondSuccessWithExtras(c *gin.Context, data any, extras gin.H) {
	payload := gin.H{
		"success": true,
		"message": "",
		"data":    data,
	}
	for key, value := range extras {
		payload[key] = value
	}
	c.JSON(http.StatusOK, payload)
}

// RespondSuccessMessage sends a successful response with a custom message
func RespondSuccessMessage(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
	})
}

// RespondFailure sends a failed response with http.StatusOK and a failure message
func RespondFailure(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": message,
	})
}

// RespondBadRequest sends a bad request response (400)
func RespondBadRequest(c *gin.Context, message string) {
	if message == "" {
		message = invalidParamsMessage
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"success": false,
		"message": message,
	})
}

// RespondUnauthorized sends an unauthorized response (401)
func RespondUnauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"message": message,
	})
}

// RespondForbidden sends a forbidden response (403)
func RespondForbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": message,
	})
}

// RespondErrorWithStatus sends a response with target HTTP status code and a message
func RespondErrorWithStatus(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"success": false,
		"message": message,
	})
}
