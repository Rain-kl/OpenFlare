package controller

import (
	"openflare/common/response"
	"openflare/controller/bind"
	"openflare/service"

	"github.com/gin-gonic/gin"
)

func GetOrigins(c *gin.Context) {
	origins, err := service.ListOrigins()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, origins)
}

func GetOrigin(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	origin, err := service.GetOriginDetail(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, origin)
}

func CreateOrigin(c *gin.Context) {
	var input service.OriginInput
	if !bind.JSON(c, &input) {
		return
	}
	origin, err := service.CreateOrigin(input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, origin)
}

func UpdateOrigin(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	var input service.OriginInput
	if !bind.JSON(c, &input) {
		return
	}
	origin, err := service.UpdateOrigin(id, input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, origin)
}

func DeleteOrigin(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	if err := service.DeleteOrigin(id); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, nil)
}
