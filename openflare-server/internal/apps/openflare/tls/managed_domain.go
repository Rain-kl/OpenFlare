// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	managedDomainMatchTypeExact    = "exact"
	managedDomainMatchTypeWildcard = "wildcard"
)

// ManagedDomainInput 托管域名创建/更新请求。
type ManagedDomainInput struct {
	Domain  string `json:"domain"`
	CertID  *uint  `json:"cert_id"`
	Enabled bool   `json:"enabled"`
	Remark  string `json:"remark"`
}

// ManagedDomainMatchCandidate 证书匹配候选。
type ManagedDomainMatchCandidate struct {
	ManagedDomainID uint   `json:"managed_domain_id"`
	Domain          string `json:"domain"`
	MatchType       string `json:"match_type"`
	CertificateID   uint   `json:"certificate_id"`
	CertificateName string `json:"certificate_name"`
}

// ManagedDomainMatchResult 证书匹配结果。
type ManagedDomainMatchResult struct {
	Domain     string                        `json:"domain"`
	Matched    bool                          `json:"matched"`
	Candidate  *ManagedDomainMatchCandidate  `json:"candidate,omitempty"`
	Candidates []ManagedDomainMatchCandidate `json:"candidates"`
}

// ListManagedDomains 列出托管域名。
func ListManagedDomains(ctx context.Context) ([]model.ManagedDomain, error) {
	return model.ListManagedDomains(ctx)
}

// CreateManagedDomain 创建托管域名。
func CreateManagedDomain(ctx context.Context, input ManagedDomainInput) (*model.ManagedDomain, error) {
	domain, err := buildManagedDomain(ctx, nil, input)
	if err != nil {
		return nil, err
	}
	if err = model.CreateManagedDomainRecord(ctx, domain); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errManagedDomainExists)
		}
		return nil, err
	}
	return domain, nil
}

// UpdateManagedDomain 更新托管域名。
func UpdateManagedDomain(ctx context.Context, id uint, input ManagedDomainInput) (*model.ManagedDomain, error) {
	domain, err := model.GetManagedDomainByID(ctx, id)
	if err != nil {
		return nil, err
	}
	domain, err = buildManagedDomain(ctx, domain, input)
	if err != nil {
		return nil, err
	}
	if err = model.SaveManagedDomain(ctx, domain); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errManagedDomainExists)
		}
		return nil, err
	}
	return domain, nil
}

// DeleteManagedDomain 删除托管域名。
func DeleteManagedDomain(ctx context.Context, id uint) error {
	if _, err := model.GetManagedDomainByID(ctx, id); err != nil {
		return err
	}
	return model.DeleteManagedDomainRecord(ctx, id)
}

// MatchManagedDomainCertificate 为域名匹配证书。
func MatchManagedDomainCertificate(ctx context.Context, rawDomain string) (*ManagedDomainMatchResult, error) {
	domain := normalizeManagedDomain(rawDomain)
	if err := validateManagedDomainPattern(domain); err != nil {
		return nil, err
	}
	managedDomains, err := model.ListEnabledManagedDomainsWithCertificate(ctx)
	if err != nil {
		return nil, err
	}
	candidates := make([]ManagedDomainMatchCandidate, 0)
	for _, item := range managedDomains {
		if item.CertID == nil || *item.CertID == 0 {
			continue
		}
		matchType := detectManagedDomainMatchType(item.Domain, domain)
		if matchType == "" {
			continue
		}
		certificate, err := model.GetTLSCertificateByID(ctx, *item.CertID)
		if err != nil {
			return nil, fmt.Errorf("托管域名 %s 关联证书不存在", item.Domain)
		}
		candidates = append(candidates, ManagedDomainMatchCandidate{
			ManagedDomainID: item.ID,
			Domain:          item.Domain,
			MatchType:       matchType,
			CertificateID:   certificate.ID,
			CertificateName: certificate.Name,
		})
	}
	sortManagedDomainCandidates(candidates)
	result := &ManagedDomainMatchResult{
		Domain:     domain,
		Matched:    len(candidates) > 0,
		Candidates: candidates,
	}
	if len(candidates) > 0 {
		candidate := candidates[0]
		result.Candidate = &candidate
	}
	return result, nil
}

func buildManagedDomain(ctx context.Context, existing *model.ManagedDomain, input ManagedDomainInput) (*model.ManagedDomain, error) {
	domain := normalizeManagedDomain(input.Domain)
	remark := strings.TrimSpace(input.Remark)
	if err := validateManagedDomainPattern(domain); err != nil {
		return nil, err
	}
	if input.CertID != nil && *input.CertID != 0 {
		if _, err := model.GetTLSCertificateByID(ctx, *input.CertID); err != nil {
			return nil, errors.New(errManagedDomainCertNotFound)
		}
	} else {
		input.CertID = nil
	}
	if existing == nil {
		existing = &model.ManagedDomain{}
	}
	existing.Domain = domain
	existing.CertID = input.CertID
	existing.Enabled = input.Enabled
	existing.Remark = remark
	return existing, nil
}

func normalizeManagedDomain(domain string) string {
	return strings.ToLower(strings.TrimSpace(domain))
}

func validateManagedDomainPattern(domain string) error {
	if domain == "" {
		return errors.New(errManagedDomainRequired)
	}
	if strings.Contains(domain, "://") || strings.Contains(domain, "/") {
		return errors.New(errManagedDomainInvalid)
	}
	if strings.Contains(domain, "*") {
		if !strings.HasPrefix(domain, "*.") || strings.Count(domain, "*") != 1 {
			return errors.New(errManagedDomainWildcardInvalid)
		}
		return validateHostname(strings.TrimPrefix(domain, "*."))
	}
	return validateHostname(domain)
}

func validateHostname(domain string) error {
	if domain == "" {
		return errors.New(errManagedDomainRequired)
	}
	if len(domain) > 253 {
		return errors.New(errManagedDomainInvalid)
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return errors.New(errManagedDomainInvalid)
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return errors.New(errManagedDomainInvalid)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New(errManagedDomainInvalid)
		}
		for _, r := range label {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
				continue
			}
			return errors.New(errManagedDomainInvalid)
		}
	}
	return nil
}

func detectManagedDomainMatchType(pattern string, domain string) string {
	if pattern == domain {
		return managedDomainMatchTypeExact
	}
	if !strings.HasPrefix(pattern, "*.") {
		return ""
	}
	suffix := strings.TrimPrefix(pattern, "*.")
	if !strings.HasSuffix(domain, "."+suffix) {
		return ""
	}
	prefix := strings.TrimSuffix(domain, "."+suffix)
	if prefix == "" || strings.Contains(prefix, ".") {
		return ""
	}
	return managedDomainMatchTypeWildcard
}

func sortManagedDomainCandidates(candidates []ManagedDomainMatchCandidate) {
	sort.Slice(candidates, func(i int, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.MatchType != right.MatchType {
			return left.MatchType == managedDomainMatchTypeExact
		}
		if len(left.Domain) != len(right.Domain) {
			return len(left.Domain) > len(right.Domain)
		}
		return left.ManagedDomainID < right.ManagedDomainID
	})
}
