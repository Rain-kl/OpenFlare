package controller

import (
	"openflare/common/response"
	"openflare/controller/bind"
	"openflare/service"

	"github.com/gin-gonic/gin"
)

func ListPagesProjects(c *gin.Context) {
	projects, err := service.ListPagesProjects()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, projects)
}

func GetPagesProject(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	project, err := service.GetPagesProject(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, project)
}

func CreatePagesProject(c *gin.Context) {
	var input service.PagesProjectInput
	if !bind.JSON(c, &input) {
		return
	}
	project, err := service.CreatePagesProject(input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, project)
}

func UpdatePagesProject(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	var input service.PagesProjectInput
	if !bind.JSON(c, &input) {
		return
	}
	project, err := service.UpdatePagesProject(id, input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, project)
}

func DeletePagesProject(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	if err := service.DeletePagesProject(id); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, nil)
}

func ListPagesDeployments(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	deployments, err := service.ListPagesProjectDeployments(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, deployments)
}

func UploadPagesDeployment(c *gin.Context) {
	id, ok := bind.IDParam(c)
	if !ok {
		return
	}
	file, err := c.FormFile("package")
	if err != nil {
		response.RespondBadRequest(c, "缺少 Pages 部署包")
		return
	}
	deployment, err := service.UploadPagesDeployment(
		id,
		file,
		c.PostForm("root_dir"),
		c.PostForm("entry_file"),
		c.GetString("username"),
	)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, deployment)
}

func ActivatePagesDeployment(c *gin.Context) {
	projectID, ok := bind.IDParam(c)
	if !ok {
		return
	}
	deploymentID, ok := bind.IDParamByName(c, "deployment_id")
	if !ok {
		return
	}
	project, err := service.ActivatePagesDeployment(projectID, deploymentID)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, project)
}

func DeletePagesDeployment(c *gin.Context) {
	projectID, ok := bind.IDParam(c)
	if !ok {
		return
	}
	deploymentID, ok := bind.IDParamByName(c, "deployment_id")
	if !ok {
		return
	}
	if err := service.DeletePagesDeployment(projectID, deploymentID); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, nil)
}

func ListPagesDeploymentFiles(c *gin.Context) {
	deploymentID, ok := bind.IDParamByName(c, "deployment_id")
	if !ok {
		return
	}
	files, err := service.ListPagesDeploymentFiles(deploymentID)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, files)
}

func AgentDownloadPagesDeploymentPackage(c *gin.Context) {
	deploymentID, ok := bind.IDParamByName(c, "deployment_id")
	if !ok {
		return
	}
	filePath, fileName, err := service.GetPagesDeploymentPackagePath(deploymentID)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.File(filePath)
}
