// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	// SyncMemberTask is the Asynq task type for one Cloudflare member.
	SyncMemberTask = "cloudflare:sync_member"
	// SyncGroupTask is the Asynq task type for a Cloudflare group.
	SyncGroupTask = "cloudflare:sync_group"
	// SyncByNodeTask is the Asynq task type for members targeting one node.
	SyncByNodeTask = "cloudflare:sync_by_node"

	// TaskTypeSyncMember is the task metadata type for member synchronization.
	TaskTypeSyncMember = "of_cloudflare_sync_member"
	// TaskTypeSyncGroup is the task metadata type for group synchronization.
	TaskTypeSyncGroup = "of_cloudflare_sync_group"
	// TaskTypeSyncByNode is the task metadata type for node-triggered synchronization.
	TaskTypeSyncByNode = "of_cloudflare_sync_by_node"
)

// SyncMemberMeta describes one-member reconciliation (admin-dispatchable).
var SyncMemberMeta = task.TaskMeta{
	Type:         TaskTypeSyncMember,
	AsynqTask:    SyncMemberTask,
	Name:         "Cloudflare 域名同步",
	Description:  "同步单个域名的 Cloudflare A 记录",
	SupportsTime: false,
	MaxRetry:     3,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "member_id",
			Label:       "成员 ID",
			Type:        "number",
			Required:    true,
			Placeholder: "请输入 Cloudflare 指向成员 ID",
			Description: "of_cf_pointing_members 表中的成员主键 ID",
		},
	},
}

// SyncGroupMeta describes group reconciliation (admin-dispatchable).
var SyncGroupMeta = task.TaskMeta{
	Type:         TaskTypeSyncGroup,
	AsynqTask:    SyncGroupTask,
	Name:         "Cloudflare 分组同步",
	Description:  "同步指向分组内全部域名",
	SupportsTime: false,
	MaxRetry:     2,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "group_id",
			Label:       "分组 ID",
			Type:        "number",
			Required:    true,
			Placeholder: "请输入 Cloudflare 指向分组 ID",
			Description: "of_cf_pointing_groups 表中的分组主键 ID",
		},
	},
}

// SyncByNodeMeta describes node-triggered reconciliation (internal only).
var SyncByNodeMeta = task.TaskMeta{
	Type:         TaskTypeSyncByNode,
	AsynqTask:    SyncByNodeTask,
	Name:         "Cloudflare 节点同步",
	Description:  "同步当前指向指定节点的全部域名",
	SupportsTime: false,
	MaxRetry:     2,
	Queue:        task.QueueDefault,
	Retryable:    true,
	InternalOnly: true,
}

// SyncMemberPayload identifies one member.
type SyncMemberPayload struct {
	MemberID uint `json:"member_id"`
}

// SyncGroupPayload identifies one group.
type SyncGroupPayload struct {
	GroupID uint `json:"group_id"`
}

// SyncByNodePayload identifies one active node.
type SyncByNodePayload struct {
	NodeID uint `json:"node_id"`
}

var dispatchTaskFn = task.DispatchTask

// SetDispatchTaskForTest replaces task dispatch for tests.
func SetDispatchTaskForTest(fn func(context.Context, string, []byte, string) (string, error)) func() {
	previous := dispatchTaskFn
	dispatchTaskFn = fn
	return func() { dispatchTaskFn = previous }
}

// DispatchMemberSync queues one member reconciliation.
func DispatchMemberSync(ctx context.Context, memberID uint, triggeredBy string) (string, error) {
	return dispatch(ctx, TaskTypeSyncMember, SyncMemberPayload{MemberID: memberID}, triggeredBy)
}

// DispatchGroupSync queues a group reconciliation.
func DispatchGroupSync(ctx context.Context, groupID uint, triggeredBy string) (string, error) {
	return dispatch(ctx, TaskTypeSyncGroup, SyncGroupPayload{GroupID: groupID}, triggeredBy)
}

// DispatchNodeSync queues reconciliation for members targeting a node.
func DispatchNodeSync(ctx context.Context, nodeID uint, triggeredBy string) (string, error) {
	return dispatch(ctx, TaskTypeSyncByNode, SyncByNodePayload{NodeID: nodeID}, triggeredBy)
}

func dispatch(ctx context.Context, taskType string, payload any, triggeredBy string) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return dispatchTaskFn(ctx, taskType, encoded, triggeredBy)
}

// SyncMemberTaskHandler reconciles one member.
type SyncMemberTaskHandler struct{}

// ValidatePayload validates a one-member task payload.
func (handler *SyncMemberTaskHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("任务参数不能为空")
	}
	var input SyncMemberPayload
	if err := decodePayload(payload, &input); err != nil {
		return nil, fmt.Errorf("无效的 Cloudflare 成员同步参数: %w", err)
	}
	if input.MemberID == 0 {
		return nil, errors.New("成员 ID 不能为空或零")
	}
	return json.Marshal(input)
}

// Execute reconciles one member.
func (handler *SyncMemberTaskHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	normalized, err := handler.ValidatePayload(payload)
	if err != nil {
		return nil, task.PermanentError(err.Error())
	}
	var input SyncMemberPayload
	_ = json.Unmarshal(normalized, &input)

	state, loadErr := repository.GetCFPointingMemberContext(ctx, input.MemberID)
	if loadErr != nil {
		task.AppendLog(ctx, "加载成员上下文失败: member_id=%d error=%v", input.MemberID, loadErr)
	} else {
		task.AppendLog(ctx,
			"开始域名同步: domain=%s zone=%s group=%s(#%d) node=%s(%s) proxied=%v member_id=%d",
			state.Domain.Domain,
			state.Zone.Domain,
			state.Group.Name,
			state.Group.ID,
			state.Node.Name,
			strings.TrimSpace(state.Node.IP),
			state.Member.Proxied,
			input.MemberID,
		)
	}

	if err = ReconcileMember(ctx, input.MemberID); err != nil {
		if state != nil {
			task.AppendLog(ctx, "域名同步失败: domain=%s member_id=%d error=%v",
				state.Domain.Domain, input.MemberID, err)
		} else {
			task.AppendLog(ctx, "域名同步失败: member_id=%d error=%v", input.MemberID, err)
		}
		return nil, fmt.Errorf("%s: %w", errSyncFailed, err)
	}

	message := "Cloudflare 域名同步成功"
	if state != nil {
		ip := strings.TrimSpace(state.Node.IP)
		message = fmt.Sprintf("Cloudflare 域名同步成功: %s → %s (proxied=%v)",
			state.Domain.Domain, ip, state.Member.Proxied)
		task.AppendLog(ctx,
			"域名同步成功: domain=%s desired_ip=%s proxied=%v group=%s node=%s",
			state.Domain.Domain, ip, state.Member.Proxied, state.Group.Name, state.Node.Name,
		)
	} else {
		task.AppendLog(ctx, "域名同步成功: member_id=%d", input.MemberID)
	}
	return &task.TaskResult{Message: message}, nil
}

// SyncGroupTaskHandler reconciles every member in a group.
type SyncGroupTaskHandler struct{}

// ValidatePayload validates a group task payload.
func (handler *SyncGroupTaskHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("任务参数不能为空")
	}
	var input SyncGroupPayload
	if err := decodePayload(payload, &input); err != nil {
		return nil, fmt.Errorf("无效的 Cloudflare 分组同步参数: %w", err)
	}
	if input.GroupID == 0 {
		return nil, errors.New("分组 ID 不能为空或零")
	}
	return json.Marshal(input)
}

// Execute reconciles every member in a group.
func (handler *SyncGroupTaskHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	normalized, err := handler.ValidatePayload(payload)
	if err != nil {
		return nil, task.PermanentError(err.Error())
	}
	var input SyncGroupPayload
	if err = json.Unmarshal(normalized, &input); err != nil {
		return nil, task.PermanentError(err.Error())
	}

	scopeName := fmt.Sprintf("#%d", input.GroupID)
	activeNode := ""
	if group, groupErr := repository.GetCFPointingGroup(ctx, input.GroupID); groupErr != nil {
		task.AppendLog(ctx, "加载分组失败: group_id=%d error=%v", input.GroupID, groupErr)
	} else {
		scopeName = group.Name
		if node, nodeErr := repository.GetOpenFlareNodeByID(ctx, group.ActiveNodeID); nodeErr != nil {
			task.AppendLog(ctx, "加载生效节点失败: group=%s active_node_id=%d error=%v",
				group.Name, group.ActiveNodeID, nodeErr)
		} else {
			activeNode = fmt.Sprintf("%s(%s)", node.Name, strings.TrimSpace(node.IP))
		}
		task.AppendLog(ctx,
			"准备分组同步: group=%s id=%d enabled=%v active_node=%s default_proxied=%v",
			group.Name, group.ID, group.Enabled, activeNode, group.DefaultProxied,
		)
	}

	members, err := repository.ListCFPointingMembersByGroupID(ctx, input.GroupID)
	return executeBatchSync(ctx, members, err, "分组", scopeName, input.GroupID, activeNode)
}

// SyncByNodeTaskHandler reconciles every member targeting a node.
type SyncByNodeTaskHandler struct{}

// ValidatePayload validates a node task payload.
func (handler *SyncByNodeTaskHandler) ValidatePayload(payload []byte) ([]byte, error) {
	var input SyncByNodePayload
	if err := decodePayload(payload, &input); err != nil || input.NodeID == 0 {
		return nil, errors.New("无效的 Cloudflare 节点同步参数")
	}
	return json.Marshal(input)
}

// Execute reconciles every member targeting a node.
func (handler *SyncByNodeTaskHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	normalized, err := handler.ValidatePayload(payload)
	if err != nil {
		return nil, task.PermanentError(err.Error())
	}
	var input SyncByNodePayload
	if err = json.Unmarshal(normalized, &input); err != nil {
		return nil, task.PermanentError(err.Error())
	}

	scopeName := fmt.Sprintf("#%d", input.NodeID)
	activeNode := ""
	if node, nodeErr := repository.GetOpenFlareNodeByID(ctx, input.NodeID); nodeErr != nil {
		task.AppendLog(ctx, "加载节点失败: node_id=%d error=%v", input.NodeID, nodeErr)
	} else {
		scopeName = node.Name
		activeNode = fmt.Sprintf("%s(%s)", node.Name, strings.TrimSpace(node.IP))
		task.AppendLog(ctx, "准备节点同步: node=%s id=%d ip=%s",
			node.Name, node.ID, strings.TrimSpace(node.IP))
	}

	members, err := repository.ListCFPointingMembersByActiveNodeID(ctx, input.NodeID)
	return executeBatchSync(ctx, members, err, "节点", scopeName, input.NodeID, activeNode)
}

func executeBatchSync(
	ctx context.Context,
	members []model.CFPointingMember,
	listErr error,
	scope, scopeName string,
	scopeID uint,
	activeNode string,
) (*task.TaskResult, error) {
	if listErr != nil {
		task.AppendLog(ctx, "列出%s成员失败: name=%s id=%d error=%v",
			scope, scopeName, scopeID, listErr)
		return nil, listErr
	}

	task.AppendLog(ctx, "开始%s同步: name=%s id=%d active_node=%s 域名数=%d",
		scope, scopeName, scopeID, activeNode, len(members))
	if len(members) == 0 {
		message := fmt.Sprintf("Cloudflare %s同步完成: %s 无域名成员", scope, scopeName)
		task.AppendLog(ctx, "%s", message)
		return &task.TaskResult{Message: message}, nil
	}

	for index, member := range members {
		domainName := fmt.Sprintf("zone_domain_id=%d", member.ZoneDomainID)
		if domain, domainErr := repository.GetZoneDomainByID(ctx, member.ZoneDomainID); domainErr == nil {
			domainName = domain.Domain
		}
		task.AppendLog(ctx, "[%d/%d] 同步域名 %s (member_id=%d proxied=%v)",
			index+1, len(members), domainName, member.ID, member.Proxied)
		if err := ReconcileMember(ctx, member.ID); err != nil {
			task.AppendLog(ctx, "[%d/%d] 失败: domain=%s member_id=%d error=%v",
				index+1, len(members), domainName, member.ID, err)
			return nil, err
		}
		task.AppendLog(ctx, "[%d/%d] 成功: domain=%s", index+1, len(members), domainName)
	}

	message := fmt.Sprintf("Cloudflare %s同步完成: %s 共 %d 个域名", scope, scopeName, len(members))
	if activeNode != "" {
		message = fmt.Sprintf("Cloudflare %s同步完成: %s → %s，共 %d 个域名",
			scope, scopeName, activeNode, len(members))
	}
	task.AppendLog(ctx, "%s", message)
	return &task.TaskResult{Message: message}, nil
}

func decodePayload(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON value")
	}
	return nil
}
