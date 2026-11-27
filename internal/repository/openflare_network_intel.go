// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/Rain-kl/Wavelet/pkg/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	colNodeID         = "node_id"
	colClientASN      = "client_asn"
	colTCPRTTP50Ms    = "tcp_rtt_p50_ms"
	colTCPSampleCount = "tcp_sample_count"
	colTraceRTTP50Ms  = "trace_rtt_p50_ms"
	colEffectiveRTTMs = "effective_rtt_ms"
	colConfidence     = "confidence"
	colUpdatedAt      = "updated_at"
	colLastSeenAt     = "last_seen_at"

	networkIntelDefaultPageSize = 20
	networkQualityBatchSize     = 200
	networkQualityMaxPageSize   = 200
	asTopologyMaxEdgeLimit      = 200
	tracerouteMaxPageSize       = 100
)

// ObservedASEdge describes a newly observed AS-to-AS hop measurement to merge into of_as_topology_edges.
type ObservedASEdge struct {
	SrcASN   uint32
	DstASN   uint32
	EdgeType string
	HopRTTMs float64
	SeenAt   time.Time
}

// InsertOpenFlareNetworkQualitySample enqueues an aggregated network quality sample into logstore.
func InsertOpenFlareNetworkQualitySample(ctx context.Context, record *model.OpenFlareNetworkQualitySample) error {
	s, err := logstore.Active(ctx)
	if err != nil {
		return err
	}
	return s.Observability.InsertNetworkQualitySample(ctx, record)
}

// ListOpenFlareNetworkQualitySamplesSince reads recent network quality samples from the active logstore backend.
func ListOpenFlareNetworkQualitySamplesSince(ctx context.Context, since time.Time, limit int) ([]analyticsmodel.NetworkQualitySample, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	return s.Observability.ListNetworkQualitySamplesSince(ctx, since, limit)
}

// UpsertOpenFlareNetworkQualityMatrixBatch atomically upserts (node_id, client_asn) quality rows.
func UpsertOpenFlareNetworkQualityMatrixBatch(ctx context.Context, rows []*model.OpenFlareNetworkQualityMatrix) error {
	if len(rows) == 0 {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	filtered := make([]*model.OpenFlareNetworkQualityMatrix, 0, len(rows))
	for _, row := range rows {
		if row == nil || strings.TrimSpace(row.NodeID) == "" || row.ClientASN == 0 {
			continue
		}
		row.NodeID = strings.TrimSpace(row.NodeID)
		row.ClientCountry = strings.ToUpper(strings.TrimSpace(row.ClientCountry))
		row.ClientASNName = strings.TrimSpace(row.ClientASNName)
		if strings.TrimSpace(row.TraceASPath) == "" {
			row.TraceASPath = "[]"
		}
		if strings.TrimSpace(row.DataSource) == "" {
			row.DataSource = model.QualityMatrixDataSourceMeasured
		}
		filtered = append(filtered, row)
	}
	if len(filtered) == 0 {
		return nil
	}
	return conn.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: colNodeID}, {Name: colClientASN}},
		DoUpdates: clause.AssignmentColumns([]string{
			"node_asn",
			"client_country",
			"client_asn_name",
			colTCPRTTP50Ms,
			"tcp_rtt_p95_ms",
			"tcp_min_rtt_ms",
			"tcp_retrans_rate",
			colTCPSampleCount,
			"tcp_last_seen_at",
			colTraceRTTP50Ms,
			"trace_loss_rate",
			"trace_hop_count",
			"trace_as_path",
			"trace_sample_count",
			"trace_last_seen_at",
			colEffectiveRTTMs,
			"predicted_rtt_ms",
			"quality_score",
			colConfidence,
			"data_source",
			colUpdatedAt,
		}),
	}).CreateInBatches(filtered, networkQualityBatchSize).Error
}

// ListOpenFlareNetworkQualityMatrix returns filtered, paginated network quality matrix entries.
func ListOpenFlareNetworkQualityMatrix(ctx context.Context, query model.NetworkQualityMatrixQuery) ([]*model.OpenFlareNetworkQualityMatrix, int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, 0, errors.New(errDatabaseNotInitialized)
	}
	q := conn.Model(&model.OpenFlareNetworkQualityMatrix{})
	if nodeID := strings.TrimSpace(query.NodeID); nodeID != "" {
		q = q.Where("node_id = ?", nodeID)
	}
	if query.ClientASN > 0 {
		q = q.Where("client_asn = ?", query.ClientASN)
	}
	if country := strings.ToUpper(strings.TrimSpace(query.ClientCountry)); country != "" {
		q = q.Where("client_country = ?", country)
	}
	if src := strings.ToLower(strings.TrimSpace(query.DataSource)); src != "" {
		q = q.Where("data_source = ?", src)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		escaped := "%" + util.EscapeLike(search) + "%"
		if parsedASN, err := strconv.ParseUint(strings.TrimPrefix(strings.ToUpper(search), "AS"), 10, 32); err == nil && parsedASN > 0 {
			q = q.Where(
				`client_asn = ? OR node_id LIKE ? ESCAPE '\' OR client_asn_name LIKE ? ESCAPE '\' OR client_country LIKE ? ESCAPE '\'`,
				uint32(parsedASN), escaped, escaped, escaped,
			)
		} else {
			q = q.Where(
				`node_id LIKE ? ESCAPE '\' OR client_asn_name LIKE ? ESCAPE '\' OR client_country LIKE ? ESCAPE '\'`,
				escaped, escaped, escaped,
			)
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		if isMissingTableError(err) {
			return []*model.OpenFlareNetworkQualityMatrix{}, 0, nil
		}
		return nil, 0, err
	}

	page := max(query.Page, 1)
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = networkIntelDefaultPageSize
	} else if pageSize > networkQualityMaxPageSize {
		pageSize = networkQualityMaxPageSize
	}

	orderCol := networkQualitySortColumn(query.SortBy)
	dir := "DESC"
	if strings.EqualFold(strings.TrimSpace(query.SortOrder), "asc") {
		dir = "ASC"
	}

	var rows []*model.OpenFlareNetworkQualityMatrix
	if err := q.Order(orderCol + " " + dir + ", id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func networkQualitySortColumn(sortBy string) string {
	switch strings.TrimSpace(strings.ToLower(sortBy)) {
	case colEffectiveRTTMs:
		return colEffectiveRTTMs
	case colTCPRTTP50Ms:
		return colTCPRTTP50Ms
	case colTraceRTTP50Ms:
		return colTraceRTTP50Ms
	case colTCPSampleCount:
		return colTCPSampleCount
	case colConfidence:
		return colConfidence
	case colUpdatedAt:
		return colUpdatedAt
	case colClientASN:
		return colClientASN
	default:
		return "quality_score"
	}
}

// ListOpenFlareNetworkQualityMatrixByClientASN returns all measured/predicted node quality rows for a client ASN ordered by score.
func ListOpenFlareNetworkQualityMatrixByClientASN(ctx context.Context, clientASN uint32) ([]*model.OpenFlareNetworkQualityMatrix, error) {
	if clientASN == 0 {
		return []*model.OpenFlareNetworkQualityMatrix{}, nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var rows []*model.OpenFlareNetworkQualityMatrix
	if err := conn.Where("client_asn = ?", clientASN).Order("quality_score DESC, effective_rtt_ms ASC").Find(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*model.OpenFlareNetworkQualityMatrix{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// DeleteStaleOpenFlareNetworkQualityMatrixBefore removes matrix rows not updated since cutoff.
func DeleteStaleOpenFlareNetworkQualityMatrixBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	res := conn.Where("updated_at < ?", cutoff.UTC()).Delete(&model.OpenFlareNetworkQualityMatrix{})
	return res.RowsAffected, res.Error
}

// UpsertOpenFlareASTopologyEdges merges observed AS-to-AS edges into of_as_topology_edges.
func UpsertOpenFlareASTopologyEdges(ctx context.Context, edges []ObservedASEdge) error {
	if len(edges) == 0 {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		for _, edge := range edges {
			if edge.SrcASN == 0 || edge.DstASN == 0 || edge.SrcASN == edge.DstASN {
				continue
			}
			edgeType := strings.TrimSpace(edge.EdgeType)
			if edgeType == "" {
				edgeType = "observed"
			}
			seenAt := edge.SeenAt.UTC()
			if seenAt.IsZero() {
				seenAt = now
			}
			rtt := max(edge.HopRTTMs, 0)

			var existing model.OpenFlareASTopologyEdge
			err := tx.Where("src_asn = ? AND dst_asn = ?", edge.SrcASN, edge.DstASN).First(&existing).Error
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				row := model.OpenFlareASTopologyEdge{
					SrcASN:      edge.SrcASN,
					DstASN:      edge.DstASN,
					EdgeType:    edgeType,
					AvgHopRTTMs: rtt,
					MinHopRTTMs: rtt,
					SampleCount: 1,
					LastSeenAt:  &seenAt,
				}
				if createErr := tx.Create(&row).Error; createErr != nil {
					return createErr
				}
				continue
			}

			totalSamples := existing.SampleCount + 1
			avgRTT := existing.AvgHopRTTMs
			if rtt > 0 {
				if existing.SampleCount <= 0 || existing.AvgHopRTTMs <= 0 {
					avgRTT = rtt
				} else {
					avgRTT = (existing.AvgHopRTTMs*float64(existing.SampleCount) + rtt) / float64(totalSamples)
				}
			}
			minRTT := existing.MinHopRTTMs
			if rtt > 0 && (minRTT <= 0 || rtt < minRTT) {
				minRTT = rtt
			}
			if err := tx.Model(&existing).Updates(map[string]any{
				"edge_type":      edgeType,
				"avg_hop_rtt_ms": avgRTT,
				"min_hop_rtt_ms": minRTT,
				"sample_count":   totalSamples,
				colLastSeenAt:    seenAt,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListOpenFlareASTopologyEdges lists observed AS topology edges ordered by sample count desc.
func ListOpenFlareASTopologyEdges(ctx context.Context, limit int) ([]*model.OpenFlareASTopologyEdge, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	q := conn.Model(&model.OpenFlareASTopologyEdge{}).Order("sample_count DESC, id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var rows []*model.OpenFlareASTopologyEdge
	if err := q.Find(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*model.OpenFlareASTopologyEdge{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// ListOpenFlareASTopologyEdgesByASN returns observed AS topology edges connected to the given ASN.
func ListOpenFlareASTopologyEdgesByASN(ctx context.Context, asn uint32) ([]*model.OpenFlareASTopologyEdge, error) {
	if asn == 0 {
		return []*model.OpenFlareASTopologyEdge{}, nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var rows []*model.OpenFlareASTopologyEdge
	if err := conn.Where("src_asn = ? OR dst_asn = ?", asn, asn).
		Order("sample_count DESC, id DESC").
		Limit(asTopologyMaxEdgeLimit).
		Find(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*model.OpenFlareASTopologyEdge{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// CreateOpenFlareTracerouteRecord persists a traceroute result record.
func CreateOpenFlareTracerouteRecord(ctx context.Context, record *model.OpenFlareTracerouteRecord) error {
	if record == nil {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	if strings.TrimSpace(record.HopsJSON) == "" {
		record.HopsJSON = "[]"
	}
	if strings.TrimSpace(record.ASPathJSON) == "" {
		record.ASPathJSON = "[]"
	}
	return conn.Create(record).Error
}

// ListOpenFlareTracerouteRecords returns filtered paginated traceroute history records.
func ListOpenFlareTracerouteRecords(ctx context.Context, query model.TracerouteRecordQuery) ([]*model.OpenFlareTracerouteRecord, int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, 0, errors.New(errDatabaseNotInitialized)
	}
	q := conn.Model(&model.OpenFlareTracerouteRecord{})
	if nodeID := strings.TrimSpace(query.NodeID); nodeID != "" {
		q = q.Where("node_id = ?", nodeID)
	}
	if target := strings.TrimSpace(query.Target); target != "" {
		q = q.Where(`target LIKE ? ESCAPE '\'`, "%"+util.EscapeLike(target)+"%")
	}
	if triggerType := strings.TrimSpace(query.TriggerType); triggerType != "" {
		q = q.Where("trigger_type = ?", triggerType)
	}
	if query.TargetASN > 0 {
		q = q.Where("target_asn = ?", query.TargetASN)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		if isMissingTableError(err) {
			return []*model.OpenFlareTracerouteRecord{}, 0, nil
		}
		return nil, 0, err
	}

	page := max(query.Page, 1)
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = networkIntelDefaultPageSize
	} else if pageSize > tracerouteMaxPageSize {
		pageSize = tracerouteMaxPageSize
	}

	var rows []*model.OpenFlareTracerouteRecord
	if err := q.Order("created_at DESC, id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ListRecentOpenFlareTracerouteRecordsSince returns recent traceroute records since cutoff for matrix rollup.
func ListRecentOpenFlareTracerouteRecordsSince(ctx context.Context, since time.Time, limit int) ([]*model.OpenFlareTracerouteRecord, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	q := conn.Model(&model.OpenFlareTracerouteRecord{})
	if !since.IsZero() {
		q = q.Where("created_at >= ?", since.UTC())
	}
	q = q.Order("created_at DESC, id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var rows []*model.OpenFlareTracerouteRecord
	if err := q.Find(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*model.OpenFlareTracerouteRecord{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// DeleteOpenFlareTracerouteRecordsBefore deletes historical traceroute records older than cutoff.
func DeleteOpenFlareTracerouteRecordsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	res := conn.Where("created_at < ?", cutoff.UTC()).Delete(&model.OpenFlareTracerouteRecord{})
	return res.RowsAffected, res.Error
}

// UpdateOpenFlareNodeASN updates a node's ASN and ASN organization fields.
func UpdateOpenFlareNodeASN(ctx context.Context, nodeID string, asn uint32, asnOrg string) error {
	trimmed := strings.TrimSpace(nodeID)
	if trimmed == "" {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Model(&model.OpenFlareNode{}).
		Where("node_id = ?", trimmed).
		Updates(map[string]any{
			"asn":     asn,
			"asn_org": strings.TrimSpace(asnOrg),
		}).Error
}

// ListOpenFlareNodesMissingASN returns nodes with non-empty IP whose ASN has not been resolved yet.
func ListOpenFlareNodesMissingASN(ctx context.Context, limit int) ([]model.OpenFlareNode, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	q := conn.Model(&model.OpenFlareNode{}).Where("ip <> '' AND asn = 0")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var nodes []model.OpenFlareNode
	if err := q.Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}
