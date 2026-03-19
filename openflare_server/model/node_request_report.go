package model

import (
	"sort"
	"time"

	"gorm.io/gorm"
)

type NodeRequestReport struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	NodeID              string    `json:"node_id" gorm:"index;size:64;not null"`
	WindowStartedAt     time.Time `json:"window_started_at" gorm:"index"`
	WindowEndedAt       time.Time `json:"window_ended_at" gorm:"index"`
	RequestCount        int64     `json:"request_count"`
	ErrorCount          int64     `json:"error_count"`
	UniqueVisitorCount  int64     `json:"unique_visitor_count"`
	StatusCodesJSON     string    `json:"status_codes_json" gorm:"type:text"`
	TopDomainsJSON      string    `json:"top_domains_json" gorm:"type:text"`
	SourceCountriesJSON string    `json:"source_countries_json" gorm:"type:text"`
	RawJSON             string    `json:"raw_json" gorm:"type:text"`
	CreatedAt           time.Time `json:"created_at"`
}

func (report *NodeRequestReport) Insert() error {
	return DB.Create(report).Error
}

func ListNodeRequestReports(nodeID string, since time.Time, limit int) (reports []*NodeRequestReport, err error) {
	query := DB.Where("node_id = ?", nodeID).Order("window_ended_at desc")
	if !since.IsZero() {
		query = query.Where("window_ended_at >= ?", since)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err = query.Find(&reports).Error
	return reports, err
}

func ListRequestReportsSince(since time.Time) (reports []*NodeRequestReport, err error) {
	rows, err := queryAcrossShards("node_request_reports", func(tx *gorm.DB) ([]*NodeRequestReport, error) {
		var shardRows []*NodeRequestReport
		query := tx.Order("window_ended_at desc")
		if !since.IsZero() {
			query = query.Where("window_ended_at >= ?", since)
		}
		if err := query.Find(&shardRows).Error; err != nil {
			return nil, err
		}
		return shardRows, nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i int, j int) bool {
		if rows[i].WindowEndedAt.Equal(rows[j].WindowEndedAt) {
			return rows[i].ID > rows[j].ID
		}
		return rows[i].WindowEndedAt.After(rows[j].WindowEndedAt)
	})
	return rows, nil
}
