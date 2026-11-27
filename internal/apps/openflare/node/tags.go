// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"errors"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	errTagIDInvalid        = "invalid tag ID"
	errTagNameRequired     = "tag name is required and must not exceed 64 characters"
	errTagSlugInvalid      = "tag slug must contain only lowercase letters, digits, and hyphens"
	errTagColorInvalid     = "tag color is not allowed"
	errTagWeightInvalid    = "tag weight must be between 0.01 and 100"
	errTagNotFound         = "node tag not found"
	errTagSlugConflict     = "tag slug already exists"
	errTagUnknownSlug      = "unknown node tag slug"
	errNodeCapacityInvalid = "node capacity values must be non-negative"
	errListNodeTagsFailed  = "failed to list node tags"
	errCreateNodeTagFailed = "failed to create node tag"
	errUpdateNodeTagFailed = "failed to update node tag"
	errDeleteNodeTagFailed = "failed to delete node tag"
	defaultTagColor        = "blue"
	defaultTagWeight       = 1.0
	defaultNodeCapacityWt  = 100
	minTagWeight           = 0.01
	maxTagWeight           = 100.0
	maxTagNameLength       = 64
	maxTagDescLength       = 255
)

type tagLogicError struct {
	status int
	msg    string
}

func (e *tagLogicError) Error() string {
	return e.msg
}

func newTagLogicError(status int, msg string) error {
	return &tagLogicError{status: status, msg: msg}
}

var (
	tagSlugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)
	allowedTagCols = map[string]struct{}{
		"slate": {}, "gray": {}, "zinc": {}, "neutral": {}, "stone": {},
		"red": {}, "orange": {}, "amber": {}, "yellow": {}, "lime": {},
		"green": {}, "emerald": {}, "teal": {}, "cyan": {}, "sky": {},
		"blue": {}, "indigo": {}, "violet": {}, "purple": {}, "fuchsia": {},
		"pink": {}, "rose": {},
	}
)

// TagBindingInput specifies a tag slug and optional node-level weight override attached to a node.
type TagBindingInput struct {
	Slug           string   `json:"slug"`
	WeightOverride *float64 `json:"weight_override,omitempty"`
}

// TagInput is the create/update payload for node routing tags.
type TagInput struct {
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Color         string   `json:"color"`
	Description   string   `json:"description"`
	DefaultWeight *float64 `json:"default_weight"`
}

// TagView is the API representation of an OpenFlareNodeTag.
type TagView struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Color         string    `json:"color"`
	Description   string    `json:"description"`
	DefaultWeight float64   `json:"default_weight"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toTagView(tag *model.OpenFlareNodeTag) *TagView {
	if tag == nil {
		return nil
	}
	return &TagView{
		ID:            tag.ID,
		Name:          tag.Name,
		Slug:          tag.Slug,
		Color:         tag.Color,
		Description:   tag.Description,
		DefaultWeight: tag.DefaultWeight,
		CreatedAt:     tag.CreatedAt,
		UpdatedAt:     tag.UpdatedAt,
	}
}

func validateWeight(w float64) error {
	if math.IsNaN(w) || math.IsInf(w, 0) || w < minTagWeight || w > maxTagWeight {
		return newTagLogicError(http.StatusBadRequest, errTagWeightInvalid)
	}
	return nil
}

func normalizeAndValidateTagInput(input TagInput) (name, slug, color, desc string, weight float64, err error) {
	name = strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > maxTagNameLength {
		return "", "", "", "", 0, newTagLogicError(http.StatusBadRequest, errTagNameRequired)
	}
	slug = strings.ToLower(strings.TrimSpace(input.Slug))
	if !tagSlugPattern.MatchString(slug) {
		return "", "", "", "", 0, newTagLogicError(http.StatusBadRequest, errTagSlugInvalid)
	}
	color = strings.ToLower(strings.TrimSpace(input.Color))
	if color == "" {
		color = defaultTagColor
	}
	if _, ok := allowedTagCols[color]; !ok {
		return "", "", "", "", 0, newTagLogicError(http.StatusBadRequest, errTagColorInvalid)
	}
	desc = strings.TrimSpace(input.Description)
	if len([]rune(desc)) > maxTagDescLength {
		desc = string([]rune(desc)[:maxTagDescLength])
	}
	weight = defaultTagWeight
	if input.DefaultWeight != nil {
		weight = *input.DefaultWeight
	}
	if err := validateWeight(weight); err != nil {
		return "", "", "", "", 0, err
	}
	return name, slug, color, desc, weight, nil
}

func validateNodeCapacityAndTags(ctx context.Context, input Input) ([]repository.NodeTagBindingSpec, error) {
	if input.CapacityBandwidthMbps < 0 || input.CapacityConnections < 0 || input.CapacityWeight < 0 {
		return nil, newTagLogicError(http.StatusBadRequest, errNodeCapacityInvalid)
	}
	if len(input.Tags) == 0 {
		return nil, nil
	}
	slugs := make([]string, 0, len(input.Tags))
	seen := make(map[string]struct{}, len(input.Tags))
	for _, item := range input.Tags {
		slug := strings.ToLower(strings.TrimSpace(item.Slug))
		if !tagSlugPattern.MatchString(slug) {
			return nil, newTagLogicError(http.StatusBadRequest, errTagSlugInvalid)
		}
		if item.WeightOverride != nil {
			if err := validateWeight(*item.WeightOverride); err != nil {
				return nil, err
			}
		}
		if _, ok := seen[slug]; !ok {
			seen[slug] = struct{}{}
			slugs = append(slugs, slug)
		}
	}
	found, err := repository.ListOpenFlareNodeTagsBySlugs(ctx, slugs)
	if err != nil {
		return nil, err
	}
	bySlug := make(map[string]model.OpenFlareNodeTag, len(found))
	for _, tag := range found {
		bySlug[tag.Slug] = tag
	}
	specs := make([]repository.NodeTagBindingSpec, 0, len(slugs))
	dedup := make(map[string]struct{}, len(slugs))
	for _, item := range input.Tags {
		slug := strings.ToLower(strings.TrimSpace(item.Slug))
		if _, ok := dedup[slug]; ok {
			continue
		}
		dedup[slug] = struct{}{}
		tag, ok := bySlug[slug]
		if !ok {
			return nil, newTagLogicError(http.StatusBadRequest, errTagUnknownSlug)
		}
		var override *float64
		if item.WeightOverride != nil {
			v := *item.WeightOverride
			override = &v
		}
		specs = append(specs, repository.NodeTagBindingSpec{
			TagID:          tag.ID,
			WeightOverride: override,
		})
	}
	return specs, nil
}

func normalizeNodeCapacityWeight(w int) int {
	if w <= 0 {
		return defaultNodeCapacityWt
	}
	return w
}

func attachNodeTagsToViews(ctx context.Context, views []*View) {
	if len(views) == 0 {
		return
	}
	nodeIDs := make([]string, 0, len(views))
	for _, view := range views {
		if view != nil && view.NodeID != "" {
			nodeIDs = append(nodeIDs, view.NodeID)
		}
	}
	if len(nodeIDs) == 0 {
		return
	}
	byNode, err := repository.ListNodeTagSummariesByNodeIDs(ctx, nodeIDs)
	if err != nil {
		logger.WarnF(ctx, "failed to load node tag summaries: %v", err)
		return
	}
	for _, view := range views {
		if view == nil {
			continue
		}
		if tags := byNode[view.NodeID]; len(tags) > 0 {
			view.Tags = tags
		} else {
			view.Tags = []model.NodeTagSummary{}
		}
	}
}

// ListTags returns all configured node routing tags.
func ListTags(ctx context.Context) ([]TagView, error) {
	rows, err := repository.ListOpenFlareNodeTags(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]TagView, 0, len(rows))
	for i := range rows {
		views = append(views, *toTagView(&rows[i]))
	}
	return views, nil
}

// CreateTag validates and creates a new node routing tag.
func CreateTag(ctx context.Context, input TagInput) (*TagView, error) {
	name, slug, color, desc, weight, err := normalizeAndValidateTagInput(input)
	if err != nil {
		return nil, err
	}
	existing, err := repository.GetOpenFlareNodeTagBySlug(ctx, slug)
	if err == nil && existing != nil {
		return nil, newTagLogicError(http.StatusConflict, errTagSlugConflict)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item := &model.OpenFlareNodeTag{
		Name:          name,
		Slug:          slug,
		Color:         color,
		Description:   desc,
		DefaultWeight: weight,
	}
	if err := repository.CreateOpenFlareNodeTag(ctx, item); err != nil {
		return nil, err
	}
	return toTagView(item), nil
}

// UpdateTag validates and updates an existing node routing tag.
func UpdateTag(ctx context.Context, id uint, input TagInput) (*TagView, error) {
	item, err := repository.GetOpenFlareNodeTagByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newTagLogicError(http.StatusNotFound, errTagNotFound)
		}
		return nil, err
	}
	name, slug, color, desc, weight, err := normalizeAndValidateTagInput(input)
	if err != nil {
		return nil, err
	}
	if slug != item.Slug {
		conflict, err := repository.GetOpenFlareNodeTagBySlug(ctx, slug)
		if err == nil && conflict != nil && conflict.ID != item.ID {
			return nil, newTagLogicError(http.StatusConflict, errTagSlugConflict)
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	item.Name = name
	item.Slug = slug
	item.Color = color
	item.Description = desc
	item.DefaultWeight = weight
	if err := repository.SaveOpenFlareNodeTag(ctx, item); err != nil {
		return nil, err
	}
	return toTagView(item), nil
}

// DeleteTag removes a node routing tag and its node bindings.
func DeleteTag(ctx context.Context, id uint) error {
	if err := repository.DeleteOpenFlareNodeTag(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newTagLogicError(http.StatusNotFound, errTagNotFound)
		}
		return err
	}
	return nil
}

func abortTagLogicError(c *gin.Context, err error, internalMsg string) {
	ctx := c.Request.Context()
	var logicErr *tagLogicError
	if errors.As(err, &logicErr) {
		switch logicErr.status {
		case http.StatusBadRequest:
			response.AbortBadRequest(c, logicErr.msg)
		case http.StatusNotFound:
			response.AbortNotFound(c, logicErr.msg)
		case http.StatusConflict:
			response.AbortConflict(c, logicErr.msg)
		default:
			response.AbortInternal(c, logicErr.msg)
		}
		return
	}
	logger.ErrorF(ctx, "%s: %v", internalMsg, err)
	response.AbortInternal(c, internalMsg)
}

// ListNodeTagsHandler 获取节点路由标签列表
// @Summary 获取节点路由标签列表
// @Description 获取所有节点路由分类标签与默认权重，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]node.TagView} "节点标签列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/node-tags [get]
func ListNodeTagsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	views, err := ListTags(ctx)
	if err != nil {
		logger.ErrorF(ctx, "list node tags failed: %v", err)
		response.AbortInternal(c, errListNodeTagsFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(views))
}

// CreateNodeTagHandler 创建节点路由标签
// @Summary 创建节点路由标签
// @Description 创建节点路由标签并设置默认权重，需要管理员权限
// @Tags openflare-node
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body node.TagInput true "节点标签参数"
// @Success 200 {object} response.Any{data=node.TagView} "创建成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 409 {object} response.Any "标识符冲突"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/node-tags [post]
func CreateNodeTagHandler(c *gin.Context) {
	var input TagInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := CreateTag(c.Request.Context(), input)
	if err != nil {
		abortTagLogicError(c, err, errCreateNodeTagFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// UpdateNodeTagHandler 更新节点路由标签
// @Summary 更新节点路由标签
// @Description 更新指定节点路由标签名称、标识符、颜色与默认权重，需要管理员权限
// @Tags openflare-node
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "标签 ID"
// @Param body body node.TagInput true "节点标签参数"
// @Success 200 {object} response.Any{data=node.TagView} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 409 {object} response.Any "标识符冲突"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/node-tags/{id}/update [post]
func UpdateNodeTagHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input TagInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	view, err := UpdateTag(c.Request.Context(), id, input)
	if err != nil {
		abortTagLogicError(c, err, errUpdateNodeTagFailed)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// DeleteNodeTagHandler 删除节点路由标签
// @Summary 删除节点路由标签
// @Description 删除指定节点路由标签及其所有节点绑定，需要管理员权限
// @Tags openflare-node
// @Produce json
// @Security SessionCookie
// @Param id path int true "标签 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "服务器错误"
// @Router /api/v1/d/node-tags/{id}/delete [post]
func DeleteNodeTagHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteTag(c.Request.Context(), id); err != nil {
		abortTagLogicError(c, err, errDeleteNodeTagFailed)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
