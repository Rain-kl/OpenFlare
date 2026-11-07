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
	"gorm.io/gorm"
)

const (
	// SyncMemberTask is the Asynq task type for one Cloudflare member.
	SyncMemberTask = "cloudflare:sync_member"
	// SyncGroupTask is the Asynq task type for a Cloudflare group.
	SyncGroupTask = "cloudflare:sync_group"
	// SyncByNodeTask is the Asynq task type for members targeting one node.
	SyncByNodeTask = "cloudflare:sync_by_node"
	// FailoverCheckTask checks node availability and updates active Cloudflare targets.
	FailoverCheckTask = "cloudflare:failover_check"

	// TaskTypeSyncMember is the task metadata type for member synchronization.
	TaskTypeSyncMember = "of_cloudflare_sync_member"
	// TaskTypeSyncGroup is the task metadata type for group synchronization.
	TaskTypeSyncGroup = "of_cloudflare_sync_group"
	// TaskTypeSyncByNode is the task metadata type for node-triggered synchronization.
	TaskTypeSyncByNode = "of_cloudflare_sync_by_node"
	// TaskTypeFailoverCheck is the schedule metadata type for node failover checks.
	TaskTypeFailoverCheck = "of_cloudflare_failover_check"
)

// FailoverCheckMeta describes the scheduled node failover reconciliation.
var FailoverCheckMeta = task.TaskMeta{
	Type:         TaskTypeFailoverCheck,
	AsynqTask:    FailoverCheckTask,
	Name:         "Cloudflare node failover check",
	Description:  "Switch group primary and standby nodes based on node availability and configuration status",
	SupportsTime: false,
	MaxRetry:     2,
	Queue:        task.QueueDefault,
	Retryable:    true,
	InternalOnly: true,
}

// SyncMemberMeta describes one-member reconciliation (admin-dispatchable).
var SyncMemberMeta = task.TaskMeta{
	Type:         TaskTypeSyncMember,
	AsynqTask:    SyncMemberTask,
	Name:         "Cloudflare domain synchronization",
	Description:  "Synchronize Cloudflare DNS records for a single domain",
	SupportsTime: false,
	MaxRetry:     3,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "member_id",
			Label:       "Member ID",
			Type:        "number",
			Required:    true,
			Placeholder: "Enter the Cloudflare target member ID",
			Description: "Member primary key in the of_cf_pointing_members table",
		},
	},
}

// SyncGroupMeta describes group reconciliation (admin-dispatchable).
var SyncGroupMeta = task.TaskMeta{
	Type:         TaskTypeSyncGroup,
	AsynqTask:    SyncGroupTask,
	Name:         "Cloudflare group synchronization",
	Description:  "Synchronize all domains in a target group",
	SupportsTime: false,
	MaxRetry:     2,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "group_id",
			Label:       "Group ID",
			Type:        "number",
			Required:    true,
			Placeholder: "Enter the Cloudflare target group ID",
			Description: "Group primary key in the of_cf_pointing_groups table",
		},
	},
}

// SyncByNodeMeta describes node-triggered reconciliation (internal only).
var SyncByNodeMeta = task.TaskMeta{
	Type:         TaskTypeSyncByNode,
	AsynqTask:    SyncByNodeTask,
	Name:         "Cloudflare node synchronization",
	Description:  "Synchronize all domains currently targeting the specified node",
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
		return nil, errors.New("task parameters must not be empty")
	}
	var input SyncMemberPayload
	if err := decodePayload(payload, &input); err != nil {
		return nil, fmt.Errorf("invalid Cloudflare member synchronization parameters: %w", err)
	}
	if input.MemberID == 0 {
		return nil, errors.New("member ID must be nonzero")
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
		task.AppendLog(ctx, "Failed to load member context: member_id=%d error=%v", input.MemberID, loadErr)
	} else {
		target := fmt.Sprintf("%s(%s)", state.Node.Name, strings.TrimSpace(state.Node.IP))
		if state.Group.TargetMode == model.CFPointingTargetModeCustom {
			target = fmt.Sprintf("%s %s", state.Group.RecordType, state.Group.RecordContent)
		}
		task.AppendLog(ctx,
			"Starting domain synchronization: domain=%s zone=%s group=%s(#%d) target=%s proxied=%v member_id=%d",
			state.Domain.Domain,
			state.Zone.Domain,
			state.Group.Name,
			state.Group.ID,
			target,
			state.Member.Proxied,
			input.MemberID,
		)
	}

	if err = ReconcileMember(ctx, input.MemberID); err != nil {
		if state != nil {
			task.AppendLog(ctx, "Domain synchronization failed: domain=%s member_id=%d error=%v",
				state.Domain.Domain, input.MemberID, err)
		} else {
			task.AppendLog(ctx, "Domain synchronization failed: member_id=%d error=%v", input.MemberID, err)
		}
		return nil, fmt.Errorf("%s: %w", errSyncFailed, err)
	}

	message := "Cloudflare domain synchronization succeeded"
	if state != nil {
		target := strings.TrimSpace(state.Node.IP)
		if state.Group.TargetMode == model.CFPointingTargetModeCustom {
			target = fmt.Sprintf("%s %s", state.Group.RecordType, state.Group.RecordContent)
		}
		message = fmt.Sprintf("Cloudflare domain synchronization succeeded: %s -> %s (proxied=%v)",
			state.Domain.Domain, target, state.Member.Proxied)
		task.AppendLog(ctx,
			"Domain synchronization succeeded: domain=%s desired_target=%s proxied=%v group=%s target_mode=%s",
			state.Domain.Domain, target, state.Member.Proxied, state.Group.Name, state.Group.TargetMode,
		)
	} else {
		task.AppendLog(ctx, "Domain synchronization succeeded: member_id=%d", input.MemberID)
	}
	return &task.TaskResult{Message: message}, nil
}

// SyncGroupTaskHandler reconciles every member in a group.
type SyncGroupTaskHandler struct{}

// ValidatePayload validates a group task payload.
func (handler *SyncGroupTaskHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("task parameters must not be empty")
	}
	var input SyncGroupPayload
	if err := decodePayload(payload, &input); err != nil {
		return nil, fmt.Errorf("invalid Cloudflare group synchronization parameters: %w", err)
	}
	if input.GroupID == 0 {
		return nil, errors.New("group ID must be nonzero")
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
		task.AppendLog(ctx, "Failed to load group: group_id=%d error=%v", input.GroupID, groupErr)
	} else {
		scopeName = group.Name
		activeNode = groupSyncTarget(ctx, group)
		task.AppendLog(ctx,
			"Preparing group synchronization: group=%s id=%d enabled=%v target=%s default_proxied=%v",
			group.Name, group.ID, group.Enabled, activeNode, group.DefaultProxied,
		)
	}

	members, err := repository.ListCFPointingMembersByGroupID(ctx, input.GroupID)
	return executeBatchSync(ctx, members, err, "group", scopeName, input.GroupID, activeNode)
}

func groupSyncTarget(ctx context.Context, group *model.CFPointingGroup) string {
	if group.TargetMode == model.CFPointingTargetModeCustom {
		return fmt.Sprintf("%s %s", group.RecordType, group.RecordContent)
	}
	node, err := repository.GetOpenFlareNodeByID(ctx, group.ActiveNodeID)
	if err != nil {
		task.AppendLog(ctx, "Failed to load active node: group=%s active_node_id=%d error=%v",
			group.Name, group.ActiveNodeID, err)
		return ""
	}
	return fmt.Sprintf("%s(%s)", node.Name, strings.TrimSpace(node.IP))
}

// SyncByNodeTaskHandler reconciles every member targeting a node.
type SyncByNodeTaskHandler struct{}

// ValidatePayload validates a node task payload.
func (handler *SyncByNodeTaskHandler) ValidatePayload(payload []byte) ([]byte, error) {
	var input SyncByNodePayload
	if err := decodePayload(payload, &input); err != nil || input.NodeID == 0 {
		return nil, errors.New("invalid Cloudflare node synchronization parameters")
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
		task.AppendLog(ctx, "Failed to load node: node_id=%d error=%v", input.NodeID, nodeErr)
	} else {
		scopeName = node.Name
		activeNode = fmt.Sprintf("%s(%s)", node.Name, strings.TrimSpace(node.IP))
		task.AppendLog(ctx, "Preparing node synchronization: node=%s id=%d ip=%s",
			node.Name, node.ID, strings.TrimSpace(node.IP))
	}

	members, err := repository.ListCFPointingMembersByActiveNodeID(ctx, input.NodeID)
	return executeBatchSync(ctx, members, err, "node", scopeName, input.NodeID, activeNode)
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
		task.AppendLog(ctx, "Failed to list %s members: name=%s id=%d error=%v",
			scope, scopeName, scopeID, listErr)
		return nil, listErr
	}

	task.AppendLog(ctx, "Starting %s synchronization: name=%s id=%d target=%s domains=%d",
		scope, scopeName, scopeID, activeNode, len(members))
	if len(members) == 0 {
		message := fmt.Sprintf("Cloudflare %s synchronization completed: %s has no domain members", scope, scopeName)
		task.AppendLog(ctx, "%s", message)
		return &task.TaskResult{Message: message}, nil
	}

	syncedCount := 0
	for index, member := range members {
		domainName := fmt.Sprintf("zone_domain_id=%d", member.ZoneDomainID)
		if domain, domainErr := repository.GetZoneDomainByID(ctx, member.ZoneDomainID); domainErr == nil {
			domainName = domain.Domain
		} else if errors.Is(domainErr, gorm.ErrRecordNotFound) {
			task.AppendLog(ctx, "[%d/%d] Domain record no longer exists; removing orphaned member: member_id=%d zone_domain_id=%d",
				index+1, len(members), member.ID, member.ZoneDomainID)
			if delErr := repository.DeleteCFPointingMember(ctx, &member); delErr != nil {
				task.AppendLog(ctx, "[%d/%d] Failed to remove orphaned member: member_id=%d error=%v",
					index+1, len(members), member.ID, delErr)
			}
			continue
		}
		task.AppendLog(ctx, "[%d/%d] Synchronizing domain %s (member_id=%d proxied=%v)",
			index+1, len(members), domainName, member.ID, member.Proxied)
		if err := ReconcileMember(ctx, member.ID); err != nil {
			task.AppendLog(ctx, "[%d/%d] Failed: domain=%s member_id=%d error=%v",
				index+1, len(members), domainName, member.ID, err)
			return nil, err
		}
		syncedCount++
		task.AppendLog(ctx, "[%d/%d] Succeeded: domain=%s", index+1, len(members), domainName)
	}

	message := fmt.Sprintf("Cloudflare %s synchronization completed: %s, %d domains", scope, scopeName, syncedCount)
	if activeNode != "" {
		message = fmt.Sprintf("Cloudflare %s synchronization completed: %s -> %s, %d domains",
			scope, scopeName, activeNode, syncedCount)
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

// FailoverCheckTaskHandler checks enabled node groups for failover and recovery.
type FailoverCheckTaskHandler struct{}

// ValidatePayload validates the empty scheduled-task payload.
func (handler *FailoverCheckTaskHandler) ValidatePayload(payload []byte) ([]byte, error) {
	var input struct{}
	if err := decodePayload(payload, &input); err != nil {
		return nil, fmt.Errorf("invalid Cloudflare failover check parameters: %w", err)
	}
	return json.Marshal(input)
}

// Execute switches affected groups and queues DNS reconciliation.
func (handler *FailoverCheckTaskHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	if _, err := handler.ValidatePayload(payload); err != nil {
		return nil, task.PermanentError(err.Error())
	}
	task.AppendLog(ctx, "Checking Cloudflare group primary and standby node status")
	changed, err := CheckNodeFailover(ctx)
	if err != nil {
		task.AppendLog(ctx, "Cloudflare node failover check failed: error=%v", err)
		return nil, err
	}
	message := fmt.Sprintf("Cloudflare node failover check completed; switched %d groups", changed)
	task.AppendLog(ctx, "%s", message)
	return &task.TaskResult{Message: message}, nil
}
