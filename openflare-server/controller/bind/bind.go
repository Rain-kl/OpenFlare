package bind

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/rain-kl/openflare/openflare-server/common/response"
)

// DecodeJSONBody decodes JSON reader to target
func DecodeJSONBody(body io.Reader, target any) error {
	return json.NewDecoder(body).Decode(target)
}

// OptionalJSON decodes optional JSON body of reader to target, allowing EOF
func OptionalJSON(body io.Reader, target any) error {
	if err := json.NewDecoder(body).Decode(target); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// IDParam parses "id" parameter from context path
func IDParam(c *gin.Context) (uint, bool) {
	return IDParamByName(c, "id")
}

// IDParamByName parses target parameter from context path
func IDParamByName(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		response.RespondBadRequest(c, "")
		return 0, false
	}
	return uint(id), true
}

// JSON binds JSON body of context request to target
func JSON(c *gin.Context, target any) bool {
	if err := DecodeJSONBody(c.Request.Body, target); err != nil {
		response.RespondBadRequest(c, "")
		return false
	}
	return true
}
