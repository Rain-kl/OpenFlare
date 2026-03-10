package service

import (
	"atsflare/model"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	ManagedDomainMatchTypeExact    = "exact"
	ManagedDomainMatchTypeWildcard = "wildcard"
)

type ManagedDomainInput struct {
	Domain  string `json:"domain"`
	CertID  *uint  `json:"cert_id"`
	Enabled bool   `json:"enabled"`
	Remark  string `json:"remark"`
}

type ManagedDomainMatchCandidate struct {
	ManagedDomainID uint   `json:"managed_domain_id"`
	Domain          string `json:"domain"`
	MatchType       string `json:"match_type"`
	CertificateID   uint   `json:"certificate_id"`
	CertificateName string `json:"certificate_name"`
}

type ManagedDomainMatchResult struct {
	Domain     string                        `json:"domain"`
	Matched    bool                          `json:"matched"`
	Candidate  *ManagedDomainMatchCandidate  `json:"candidate,omitempty"`
	Candidates []ManagedDomainMatchCandidate `json:"candidates"`
}

func ListManagedDomains() ([]*model.ManagedDomain, error) {
	return model.ListManagedDomains()
}

func CreateManagedDomain(input ManagedDomainInput) (*model.ManagedDomain, error) {
	domain, err := buildManagedDomain(nil, input)
	if err != nil {
		return nil, err
	}
	if err = domain.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("域名已存在")
		}
		return nil, err
	}
	return domain, nil
}

func UpdateManagedDomain(id uint, input ManagedDomainInput) (*model.ManagedDomain, error) {
	domain, err := model.GetManagedDomainByID(id)
	if err != nil {
		return nil, err
	}
	domain, err = buildManagedDomain(domain, input)
	if err != nil {
		return nil, err
	}
	if err = domain.Update(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("域名已存在")
		}
		return nil, err
	}
	return domain, nil
}

func DeleteManagedDomain(id uint) error {
	domain, err := model.GetManagedDomainByID(id)
	if err != nil {
		return err
	}
	return domain.Delete()
}

func MatchManagedDomainCertificate(rawDomain string) (*ManagedDomainMatchResult, error) {
	domain := normalizeManagedDomain(rawDomain)
	if err := validateManagedDomainPattern(domain); err != nil {
		return nil, err
	}
	managedDomains, err := model.ListEnabledManagedDomainsWithCertificate()
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
		certificate, err := model.GetTLSCertificateByID(*item.CertID)
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

func buildManagedDomain(existing *model.ManagedDomain, input ManagedDomainInput) (*model.ManagedDomain, error) {
	domain := normalizeManagedDomain(input.Domain)
	remark := strings.TrimSpace(input.Remark)
	if err := validateManagedDomainPattern(domain); err != nil {
		return nil, err
	}
	if input.CertID != nil && *input.CertID != 0 {
		if _, err := model.GetTLSCertificateByID(*input.CertID); err != nil {
			return nil, errors.New("所选证书不存在")
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
		return errors.New("域名不能为空")
	}
	if strings.Contains(domain, "://") || strings.Contains(domain, "/") {
		return errors.New("域名格式不合法")
	}
	if strings.Contains(domain, "*") {
		if !strings.HasPrefix(domain, "*.") || strings.Count(domain, "*") != 1 {
			return errors.New("通配符域名仅支持 *.example.com 格式")
		}
		return validateHostname(strings.TrimPrefix(domain, "*."))
	}
	return validateHostname(domain)
}

func validateHostname(domain string) error {
	if domain == "" {
		return errors.New("域名不能为空")
	}
	if len(domain) > 253 {
		return errors.New("域名格式不合法")
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return errors.New("域名格式不合法")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return errors.New("域名格式不合法")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("域名格式不合法")
		}
		for _, r := range label {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
				continue
			}
			return errors.New("域名格式不合法")
		}
	}
	return nil
}

func detectManagedDomainMatchType(pattern string, domain string) string {
	if pattern == domain {
		return ManagedDomainMatchTypeExact
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
	return ManagedDomainMatchTypeWildcard
}

func sortManagedDomainCandidates(candidates []ManagedDomainMatchCandidate) {
	sort.Slice(candidates, func(i int, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.MatchType != right.MatchType {
			return left.MatchType == ManagedDomainMatchTypeExact
		}
		if len(left.Domain) != len(right.Domain) {
			return len(left.Domain) > len(right.Domain)
		}
		return left.ManagedDomainID < right.ManagedDomainID
	})
}
