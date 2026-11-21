// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"database/sql"
	"fmt"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

// CredentialRotationCount reports the number of sealed values per field.
type CredentialRotationCount struct {
	Field      string
	Rotated    int
	AlreadyNew int
}

type sealedCredentialField struct {
	table  string
	column string
}

var sealedCredentialFields = []sealedCredentialField{
	{(model.TLSCertificate{}).TableName(), "key_pem"},
	{(model.DNSAccount{}).TableName(), "authorization"},
	{(model.AcmeAccount{}).TableName(), "private_key"},
	{(model.CFConnection{}).TableName(), "authorization"},
}

type sealedCredentialUpdate struct {
	field       sealedCredentialField
	id          uint
	oldValue    string
	replacement string
}

// RotateSealedCredentials validates all replacements before optionally committing
// them in a single primary database transaction.
func RotateSealedCredentials(ctx context.Context, prefix string, apply bool, rewrap func(string) (string, error)) ([]CredentialRotationCount, error) {
	conn := db.DB(db.WithPrimaryReads(ctx)).Session(&gorm.Session{Logger: gormlogger.Discard})
	var counts []CredentialRotationCount
	err := conn.Transaction(func(tx *gorm.DB) error {
		var updates []sealedCredentialUpdate
		for _, field := range sealedCredentialFields {
			count, err := collectCredentialUpdates(tx, field, prefix, rewrap, &updates)
			if err != nil {
				return err
			}
			counts = append(counts, count)
		}
		if apply {
			return writeCredentialUpdates(tx, updates)
		}
		return nil
	}, &sql.TxOptions{ReadOnly: !apply})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

func collectCredentialUpdates(tx *gorm.DB, field sealedCredentialField, prefix string, rewrap func(string) (string, error), updates *[]sealedCredentialUpdate) (CredentialRotationCount, error) {
	count := CredentialRotationCount{Field: field.table + "." + field.column}
	var rows []struct {
		ID    uint
		Value string
	}
	err := tx.Table(field.table).
		Select([]string{"id", field.column + " AS value"}).
		Where("SUBSTR(TRIM(?), 1, ?) = ?", clause.Column{Name: field.column}, len(prefix), prefix).
		Order("id").Scan(&rows).Error
	if err != nil {
		return count, fmt.Errorf("read %s: %w", count.Field, err)
	}
	for _, row := range rows {
		replacement, err := rewrap(row.Value)
		if err != nil {
			return count, fmt.Errorf("%s record %d: %w", count.Field, row.ID, err)
		}
		if replacement == row.Value {
			count.AlreadyNew++
			continue
		}
		count.Rotated++
		*updates = append(*updates, sealedCredentialUpdate{
			field: field, id: row.ID, oldValue: row.Value, replacement: replacement,
		})
	}
	return count, nil
}

func writeCredentialUpdates(tx *gorm.DB, updates []sealedCredentialUpdate) error {
	for _, update := range updates {
		column := clause.Column{Name: update.field.column}
		result := tx.Table(update.field.table).
			Where("id = ? AND ? = ?", update.id, column, update.oldValue).
			UpdateColumn(update.field.column, update.replacement)
		if result.Error != nil {
			// Driver errors can contain field values. Keep only the record location.
			return fmt.Errorf("could not write %s.%s record %d", update.field.table, update.field.column, update.id)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("%s.%s record %d changed during rotation; stop all writers before retrying", update.field.table, update.field.column, update.id)
		}
	}
	return nil
}
