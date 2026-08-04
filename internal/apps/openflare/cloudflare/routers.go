// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func abortLogic(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.AbortNotFound(c, "Cloudflare 资源不存在")
	case err.Error() == errMemberExists:
		response.AbortConflict(c, err.Error())
	case err.Error() == errTaskDispatchFailed:
		response.AbortInternal(c, err.Error())
	default:
		response.AbortBadRequest(c, err.Error())
	}
	return true
}

// GetConnectionHandler returns Cloudflare connection readiness.
// @Summary 获取 Cloudflare 连接
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=cloudflare.ConnectionView}
// @Router /api/v1/d/cloudflare/connection [get]
func GetConnectionHandler(c *gin.Context) {
	item, err := GetConnection(c.Request.Context())
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// SaveConnectionHandler saves a Cloudflare credential source.
// @Summary 保存 Cloudflare 连接
// @Tags openflare-cloudflare
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body cloudflare.ConnectionInput true "连接参数"
// @Success 200 {object} response.Any{data=cloudflare.ConnectionView}
// @Failure 400 {object} response.Any
// @Router /api/v1/d/cloudflare/connection [put]
func SaveConnectionHandler(c *gin.Context) {
	var input ConnectionInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	item, err := SaveConnection(c.Request.Context(), input)
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// VerifyConnectionHandler verifies the configured Cloudflare token.
// @Summary 测试 Cloudflare 连接
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=cloudflare.ConnectionView}
// @Failure 400 {object} response.Any
// @Router /api/v1/d/cloudflare/connection/verify [post]
func VerifyConnectionHandler(c *gin.Context) {
	item, err := VerifyConnection(c.Request.Context())
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// ClearConnectionHandler clears the Cloudflare credential source.
// @Summary 清除 Cloudflare 连接
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any
// @Router /api/v1/d/cloudflare/connection/clear [post]
func ClearConnectionHandler(c *gin.Context) {
	if abortLogic(c, ClearConnection(c.Request.Context())) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// OverviewHandler returns Cloudflare pointing health.
// @Summary 获取 Cloudflare 指向总览
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=cloudflare.Overview}
// @Router /api/v1/d/cloudflare/overview [get]
func OverviewHandler(c *gin.Context) {
	item, err := GetOverview(c.Request.Context())
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// ListGroupsHandler lists pointing groups.
// @Summary 获取 Cloudflare 指向分组
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]cloudflare.GroupItem}
// @Router /api/v1/d/cloudflare/groups [get]
func ListGroupsHandler(c *gin.Context) {
	items, err := ListGroups(c.Request.Context())
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(items))
}

// CreateGroupHandler creates a pointing group.
// @Summary 创建 Cloudflare 指向分组
// @Tags openflare-cloudflare
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body cloudflare.GroupInput true "分组参数"
// @Success 200 {object} response.Any{data=cloudflare.GroupItem}
// @Failure 400 {object} response.Any
// @Router /api/v1/d/cloudflare/groups [post]
func CreateGroupHandler(c *gin.Context) {
	var input GroupInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	item, err := CreateGroup(c.Request.Context(), input)
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// GetGroupHandler returns one pointing group and its members.
// @Summary 获取 Cloudflare 指向分组详情
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Param id path int true "分组 ID"
// @Success 200 {object} response.Any{data=cloudflare.GroupDetail}
// @Failure 404 {object} response.Any
// @Router /api/v1/d/cloudflare/groups/{id} [get]
func GetGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	item, err := GetGroup(c.Request.Context(), id)
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// UpdateGroupHandler updates a pointing group.
// @Summary 更新 Cloudflare 指向分组
// @Tags openflare-cloudflare
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "分组 ID"
// @Param body body cloudflare.GroupInput true "分组参数"
// @Success 200 {object} response.Any{data=cloudflare.GroupItem}
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/d/cloudflare/groups/{id}/update [post]
func UpdateGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input GroupInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	item, err := UpdateGroup(c.Request.Context(), id, input)
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// DeleteGroupHandler deletes a pointing group and its managed remote A records.
// @Summary 删除 Cloudflare 指向分组
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Param id path int true "分组 ID"
// @Success 200 {object} response.Any
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/d/cloudflare/groups/{id}/delete [post]
func DeleteGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if abortLogic(c, DeleteGroup(c.Request.Context(), id)) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// SyncGroupHandler queues a full group synchronization.
// @Summary 同步 Cloudflare 指向分组
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Param id path int true "分组 ID"
// @Success 200 {object} response.Any{data=cloudflare.SyncReceipt}
// @Failure 500 {object} response.Any
// @Router /api/v1/d/cloudflare/groups/{id}/sync [post]
func SyncGroupHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	taskID, err := DispatchGroupSync(c.Request.Context(), id, "cloudflare_manual_group_sync")
	if err != nil {
		response.AbortInternal(c, errTaskDispatchFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(&SyncReceipt{TaskID: taskID}))
}

// ListMembersHandler lists group members.
// @Summary 获取 Cloudflare 指向分组成员
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Param id path int true "分组 ID"
// @Success 200 {object} response.Any{data=[]cloudflare.MemberItem}
// @Router /api/v1/d/cloudflare/groups/{id}/members [get]
func ListMembersHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	item, err := GetGroup(c.Request.Context(), id)
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item.Members))
}

// CreateMemberHandler adds a ZoneDomain to a pointing group.
// @Summary 添加 Cloudflare 指向成员
// @Tags openflare-cloudflare
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "分组 ID"
// @Param body body cloudflare.MemberCreateInput true "成员参数"
// @Success 200 {object} response.Any{data=cloudflare.MemberItem}
// @Failure 400 {object} response.Any
// @Failure 409 {object} response.Any
// @Router /api/v1/d/cloudflare/groups/{id}/members [post]
func CreateMemberHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input MemberCreateInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	item, err := CreateMember(c.Request.Context(), id, input)
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// UpdateMemberHandler updates a member's orange-cloud state.
// @Summary 更新 Cloudflare 指向成员
// @Tags openflare-cloudflare
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "分组 ID"
// @Param memberId path int true "成员 ID"
// @Param body body cloudflare.MemberUpdateInput true "成员参数"
// @Success 200 {object} response.Any{data=cloudflare.MemberItem}
// @Router /api/v1/d/cloudflare/groups/{id}/members/{memberId}/update [post]
func UpdateMemberHandler(c *gin.Context) {
	groupID, memberID, ok := memberParams(c)
	if !ok {
		return
	}
	var input MemberUpdateInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	item, err := UpdateMember(c.Request.Context(), groupID, memberID, input)
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// RemoveMemberHandler removes a member and its managed remote A record.
// @Summary 移出 Cloudflare 指向成员
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Param id path int true "分组 ID"
// @Param memberId path int true "成员 ID"
// @Success 200 {object} response.Any
// @Router /api/v1/d/cloudflare/groups/{id}/members/{memberId}/remove [post]
func RemoveMemberHandler(c *gin.Context) {
	groupID, memberID, ok := memberParams(c)
	if !ok {
		return
	}
	if abortLogic(c, RemoveMember(c.Request.Context(), groupID, memberID)) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// SyncMemberHandler queues one member synchronization.
// @Summary 同步 Cloudflare 指向成员
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Param id path int true "分组 ID"
// @Param memberId path int true "成员 ID"
// @Success 200 {object} response.Any{data=cloudflare.SyncReceipt}
// @Router /api/v1/d/cloudflare/groups/{id}/members/{memberId}/sync [post]
func SyncMemberHandler(c *gin.Context) {
	_, memberID, ok := memberParams(c)
	if !ok {
		return
	}
	taskID, err := DispatchMemberSync(c.Request.Context(), memberID, "cloudflare_manual_member_sync")
	if err != nil {
		response.AbortInternal(c, errTaskDispatchFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(&SyncReceipt{TaskID: taskID}))
}

// ListAvailableDomainsHandler lists ZoneDomains not assigned to another group.
// @Summary 获取可加入 Cloudflare 指向的域名
// @Tags openflare-cloudflare
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]cloudflare.AvailableDomain}
// @Router /api/v1/d/cloudflare/domains/available [get]
func ListAvailableDomainsHandler(c *gin.Context) {
	items, err := ListAvailableDomains(c.Request.Context())
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(items))
}

func memberParams(c *gin.Context) (uint, uint, bool) {
	groupID, ok := apiutil.IDParam(c)
	if !ok {
		return 0, 0, false
	}
	memberID, ok := apiutil.NamedIDParam(c, "memberId")
	return groupID, memberID, ok
}
