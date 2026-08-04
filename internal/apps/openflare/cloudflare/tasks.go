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

// SyncMemberMeta describes one-member reconciliation.
var SyncMemberMeta = task.TaskMeta{Type: TaskTypeSyncMember, AsynqTask: SyncMemberTask, Name: "Cloudflare 域名同步", Description: "同步单个域名的 Cloudflare A 记录", MaxRetry: 3, Queue: task.QueueDefault, Retryable: true, InternalOnly: true}

// SyncGroupMeta describes group reconciliation.
var SyncGroupMeta = task.TaskMeta{Type: TaskTypeSyncGroup, AsynqTask: SyncGroupTask, Name: "Cloudflare 分组同步", Description: "同步指向分组内全部域名", MaxRetry: 2, Queue: task.QueueDefault, Retryable: true, InternalOnly: true}

// SyncByNodeMeta describes node-triggered reconciliation.
var SyncByNodeMeta = task.TaskMeta{Type: TaskTypeSyncByNode, AsynqTask: SyncByNodeTask, Name: "Cloudflare 节点同步", Description: "同步当前指向指定节点的全部域名", MaxRetry: 2, Queue: task.QueueDefault, Retryable: true, InternalOnly: true}

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
	var input SyncMemberPayload
	if err := decodePayload(payload, &input); err != nil || input.MemberID == 0 {
		return nil, errors.New("无效的 Cloudflare 成员同步参数")
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
	task.AppendLog(ctx, "正在同步 Cloudflare 成员 ID=%d", input.MemberID)
	if err = ReconcileMember(ctx, input.MemberID); err != nil {
		return nil, fmt.Errorf("%s: %w", errSyncFailed, err)
	}
	return &task.TaskResult{Message: "Cloudflare 域名同步成功"}, nil
}

// SyncGroupTaskHandler reconciles every member in a group.
type SyncGroupTaskHandler struct{}

// ValidatePayload validates a group task payload.
func (handler *SyncGroupTaskHandler) ValidatePayload(payload []byte) ([]byte, error) {
	var input SyncGroupPayload
	if err := decodePayload(payload, &input); err != nil || input.GroupID == 0 {
		return nil, errors.New("无效的 Cloudflare 分组同步参数")
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
	members, err := repository.ListCFPointingMembersByGroupID(ctx, input.GroupID)
	return executeBatchSync(ctx, members, err, "分组")
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
	members, err := repository.ListCFPointingMembersByActiveNodeID(ctx, input.NodeID)
	return executeBatchSync(ctx, members, err, "节点")
}

func executeBatchSync(ctx context.Context, members []model.CFPointingMember, listErr error, scope string) (*task.TaskResult, error) {
	if listErr != nil {
		return nil, listErr
	}
	for _, member := range members {
		if err := ReconcileMember(ctx, member.ID); err != nil {
			return nil, err
		}
	}
	return &task.TaskResult{Message: fmt.Sprintf("Cloudflare %s同步完成，共 %d 个域名", scope, len(members))}, nil
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
