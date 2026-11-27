// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/config_version"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/node"
	"github.com/Rain-kl/Wavelet/internal/automationcontract"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	connectionOperation = "openflare_connection_info"
	nodeListOperation   = "openflare_node_list"
	nodeGetOperation    = "openflare_node_get"
	edgeNodeType        = "edge_node"
	boolTrueString      = "true"
)

type operationFunc func(context.Context, identity, map[string]string) (any, *operationError)

type binding struct {
	automationcontract.Operation
	run operationFunc
}

type operationError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ConnectionInfo identifies the connected instance and effective OAuth grant.
type ConnectionInfo struct {
	InstanceID   string   `json:"instance_id"`
	InstanceName string   `json:"instance_name"`
	Scopes       []string `json:"scopes"`
}

// NodeSummary exposes node health and apply status without agent credentials.
type NodeSummary struct {
	NodeRecordID        string     `json:"node_record_id"`
	NodeID              string     `json:"node_id"`
	Name                string     `json:"name"`
	IP                  string     `json:"ip"`
	NodeType            string     `json:"node_type"`
	Status              string     `json:"status"`
	Version             string     `json:"version"`
	CurrentVersion      string     `json:"current_version"`
	DesiredVersion      string     `json:"desired_site_version,omitempty"`
	LastHeartbeatAt     *time.Time `json:"last_heartbeat_at,omitempty"`
	LatestApplyResult   string     `json:"latest_apply_result"`
	LatestApplyChecksum string     `json:"latest_apply_checksum"`
	LatestApplyAt       *time.Time `json:"latest_apply_at,omitempty"`
}

// NodePage contains one page of node summaries.
type NodePage struct {
	Total    int64         `json:"total"`
	Results  []NodeSummary `json:"results"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// Catalog lists contracts available to the authenticated credential.
type Catalog struct {
	InstanceID string                         `json:"instance_id"`
	Operations []automationcontract.Operation `json:"operations"`
}

func bindings() []binding {
	handlers := map[string]operationFunc{
		connectionOperation:             connectionInfo,
		nodeListOperation:               listNodes,
		nodeGetOperation:                getNode,
		"openflare_zone_list":           listZones,
		"openflare_zone_create":         createZone,
		"openflare_domain_list":         listDomains,
		"openflare_domain_create":       createDomain,
		"openflare_certificate_list":    listCertificates,
		"openflare_site_list":           listSites,
		"openflare_site_overview":       siteOverview,
		"openflare_site_create":         createSite,
		"openflare_site_bind_domain":    bindDomain,
		"openflare_site_set_upstreams":  setUpstreams,
		"openflare_logs_list":           listAccessLogs,
		"openflare_metrics_traffic":     trafficMetrics,
		"openflare_node_metrics":        nodeMetrics,
		"openflare_performance_list":    listPerformance,
		"openflare_performance_set":     setPerformance,
		"openflare_site_get_settings":   getSiteSettings,
		"openflare_site_patch_settings": patchSiteSettings,
		"openflare_site_set_auth":       setSiteAuth,
		"openflare_site_get_pow":        getSitePoW,
		"openflare_site_set_pow":        setSitePoW,
		"openflare_waf_list":            listWAFFlows,
		"openflare_site_set_waf":        setSiteWAF,
		"openflare_attack_group_list":   listAttackGroups,
		"openflare_attack_group_create": createAttackGroup,
		"openflare_attack_group_update": updateAttackGroup,
		"openflare_attack_group_delete": deleteAttackGroup,
		"openflare_ip_group_list":       listIPGroups,
		"openflare_ip_group_get":        getIPGroup,
		"openflare_node_health":         nodeHealth,
		"openflare_ping_task_list":      listPingTasks,
		"openflare_ping_task_create":    createPingTask,
		"openflare_ping_task_update":    updatePingTask,
		"openflare_ping_task_delete":    deletePingTask,
		"openflare_ping_task_reorder":   reorderPingTasks,
		"openflare_node_ping_metrics":   nodePingMetrics,
		"openflare_exec_run":            runRemoteExec,
		"openflare_exec_list":           listRemoteExecTasks,
		"openflare_exec_get":            getRemoteExecTask,
	}
	var result []binding
	for _, operation := range automationcontract.Operations() {
		handler, exists := handlers[operation.ID]
		if !exists {
			panic("missing automation handler: " + operation.ID)
		}
		delete(handlers, operation.ID)
		result = append(result, binding{Operation: operation, run: handler})
	}
	for id := range handlers {
		panic("missing generated automation operation: " + id)
	}
	return result
}

var bodyContracts = func() map[string]automationcontract.Operation {
	result := make(map[string]automationcontract.Operation)
	for _, operation := range automationcontract.Operations() {
		if operation.Effect == automationcontract.EffectWrite {
			result[operation.ID] = operation
		}
	}
	return result
}()

// decodeBodyFields converts already validated transport arguments into their documented DTO.
func decodeBodyFields(operationID string, input map[string]string, dst any) error {
	operation, exists := bodyContracts[operationID]
	if !exists {
		panic("missing body contract: " + operationID)
	}
	raw, err := operation.JSONBody(input)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return errors.New("request body does not match the operation's field types")
	}
	return nil
}

func recordID(input map[string]string, key string) (uint, error) {
	value, err := strconv.ParseUint(input[key], 10, 63)
	if err != nil || value == 0 || uint64(uint(value)) != value {
		return 0, errors.New(key + " must be a positive decimal record ID")
	}
	return uint(value), nil
}

func inputInt(input map[string]string, key string) int {
	value, _ := strconv.Atoi(input[key])
	return value
}

func resourceFailure(ctx context.Context, operation string, err error) *operationError {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, repository.ErrZoneDomainNotFound):
		return &operationError{http.StatusNotFound, "NOT_FOUND", "A referenced record does not exist. Resolve its ID with the corresponding list operation."}
	case errors.Is(err, repository.ErrZoneDomainBoundToAnotherRoute):
		return &operationError{http.StatusConflict, "DOMAIN_ALREADY_BOUND", "The domain already belongs to another site."}
	default:
		return operationFailed(ctx, operation, err)
	}
}

func (operation binding) permitted(actor identity) bool {
	if !actor.permits(operation.Scope) {
		return false
	}
	for _, scope := range operation.Requires {
		if !actor.permits(scope) {
			return false
		}
	}
	return true
}

func (operation binding) execute(ctx context.Context, actor identity, input map[string]string) (any, *operationError) {
	var result any
	var failure *operationError
	if !operation.permitted(actor) {
		failure = &operationError{http.StatusForbidden, "INSUFFICIENT_SCOPE", "The grant does not permit this operation."}
	} else if err := operation.Validate(input); err != nil {
		failure = invalidArguments(err)
	} else {
		result, failure = operation.run(db.WithPrimaryReads(ctx), actor, input)
		if failure == nil && operation.Effect == automationcontract.EffectWrite {
			config_version.RequestRefresh()
		}
	}
	status := "OK"
	if failure != nil {
		status = failure.Code
	}
	logger.InfoF(ctx, "Automation operation=%s user_id=%d grant_id=%s result=%s", operation.ID, actor.userID, actor.grantID, status)
	return result, failure
}

func invalidArguments(err error) *operationError {
	return &operationError{http.StatusBadRequest, "INVALID_ARGUMENT", err.Error()}
}

func operationFailed(ctx context.Context, operation string, err error) *operationError {
	logger.ErrorF(ctx, "Automation %s failed: %v", operation, err)
	return &operationError{http.StatusInternalServerError, "INTERNAL_ERROR", "The operation could not be completed."}
}

// @ID openflare_connection_info
// @Summary Verify the OpenFlare connection
// @Description Return the connected instance ID, display name and effective permissions. Use this read-only operation to verify a newly configured connection.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Success 200 {object} OperationResponse{data=ConnectionInfo}
// @Failure 401 {object} OperationResponse
// @Failure 403 {object} OperationResponse
// @Failure 404 {object} OperationResponse
// @Failure 500 {object} OperationResponse
// @x-automation {"cli":"connection info","scope":"connection:read","effect":"read"}
// @Router /api/v1/automation/connection [get]
func connectionInfo(ctx context.Context, actor identity, _ map[string]string) (any, *operationError) {
	setting, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySiteName)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, operationFailed(ctx, connectionOperation, err)
	}
	name := setting.Value
	if name == "" {
		name = "OpenFlare"
	}
	return ConnectionInfo{InstanceID: actor.instanceID, InstanceName: name, Scopes: strings.Fields(actor.scope)}, nil
}

// @ID openflare_node_list
// @Summary List nodes by page
// @Description List nodes in the current instance with their record IDs, registration identifiers, health, and current configuration versions. Use search to find candidate records before requesting details. This operation is read-only.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param page query integer false "Page number, starting at 1" minimum(1) default(1)
// @Param page_size query integer false "Number of records per page" minimum(1) maximum(100) default(20)
// @Param search query string false "Literal substring of the node name, registration identifier, IP address, region or operating system, at most 256 bytes" maxlength(256)
// @Param status query string false "Filter by current health. Omit for all statuses." Enums(pending,online,offline,quarantined)
// @Param node_type query string false "Filter by node type. Omit for all types." Enums(edge_node)
// @Success 200 {object} OperationResponse{data=NodePage}
// @Failure 400 {object} OperationResponse
// @Failure 401 {object} OperationResponse
// @Failure 403 {object} OperationResponse
// @Failure 404 {object} OperationResponse
// @Failure 500 {object} OperationResponse
// @x-automation {"cli":"node list","scope":"nodes:read","effect":"read"}
// @Router /api/v1/automation/nodes [get]
func listNodes(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	result, failure := nodePage(ctx, nodeListOperation, input)
	if failure != nil {
		return nil, failure
	}
	results := make([]NodeSummary, 0, len(result.Results))
	for _, item := range result.Results {
		results = append(results, summarizeNode(item))
	}
	return NodePage{Total: result.Total, Results: results, Page: result.Page, PageSize: result.PageSize}, nil
}

func nodePage(ctx context.Context, operation string, input map[string]string) (*node.ListResult, *operationError) {
	page, _ := strconv.Atoi(input["page"])
	pageSize, _ := strconv.Atoi(input["page_size"])
	if input["page"] != "" && page == 0 || input["page_size"] != "" && pageSize == 0 {
		return nil, invalidArguments(errors.New("pagination exceeds the supported range"))
	}
	result, err := node.ListPage(ctx, node.ListQuery{
		Page: page, PageSize: pageSize, Search: input["search"], Status: input["status"], NodeType: input["node_type"],
	})
	if err != nil {
		if node.IsInvalidListQuery(err) {
			return nil, invalidArguments(errors.New("invalid node pagination or filters"))
		}
		return nil, operationFailed(ctx, operation, err)
	}
	return result, nil
}

// @ID openflare_node_get
// @Summary Get a node by its control-plane record ID
// @Description Read one node and its latest configuration application summary. For an edge node, compare current_version with desired_site_version to check whether the controller's current site snapshot has been applied. Pass node_record_id from openflare_node_list.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Param node_record_id path string true "Decimal control-plane record ID returned by openflare_node_list"
// @Success 200 {object} OperationResponse{data=NodeSummary}
// @Failure 400 {object} OperationResponse
// @Failure 401 {object} OperationResponse
// @Failure 403 {object} OperationResponse
// @Failure 404 {object} OperationResponse
// @Failure 500 {object} OperationResponse
// @x-automation {"cli":"node get","scope":"nodes:read","effect":"read","parameters":{"node_record_id":{"pattern":"^[1-9][0-9]{0,18}$"}}}
// @Router /api/v1/automation/nodes/{node_record_id} [get]
func getNode(ctx context.Context, _ identity, input map[string]string) (any, *operationError) {
	id, err := recordID(input, "node_record_id")
	if err != nil {
		return nil, invalidArguments(errors.New("node_record_id is outside the supported range"))
	}
	view, err := node.GetNode(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &operationError{http.StatusNotFound, "NODE_NOT_FOUND", "The requested node record does not exist."}
	}
	if err != nil {
		return nil, operationFailed(ctx, nodeGetOperation, err)
	}
	result := summarizeNode(view)
	if view.NodeType == edgeNodeType {
		active, err := repository.GetActivePingoraConfigMetadata(ctx)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, operationFailed(ctx, nodeGetOperation, err)
		}
		if err == nil {
			result.DesiredVersion = active.Version
		}
	}
	return result, nil
}

func summarizeNode(view *node.View) NodeSummary {
	return NodeSummary{
		NodeRecordID: strconv.FormatUint(uint64(view.ID), 10), NodeID: view.NodeID,
		Name: view.Name, IP: view.IP, NodeType: view.NodeType, Status: view.Status,
		Version: view.Version, CurrentVersion: view.CurrentVersion, LatestApplyResult: view.LatestApplyResult,
		LastHeartbeatAt:     view.LastHeartbeatAt,
		LatestApplyChecksum: view.LatestApplyChecksum, LatestApplyAt: view.LatestApplyAt,
	}
}

func requestID() string {
	return uuid.NewString()
}
