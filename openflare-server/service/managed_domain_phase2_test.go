package service

import "testing"

func TestMatchManagedDomainCertificatePrefersExactMatch(t *testing.T) {
	setupServiceTestDB(t)

	wildcardCertPEM, wildcardKeyPEM := generateCertificatePair(t, []string{"*.example.com"})
	wildcardCert, err := CreateTLSCertificate(TLSCertificateInput{
		Name:    "wildcard-cert",
		CertPEM: wildcardCertPEM,
		KeyPEM:  wildcardKeyPEM,
	})
	if err != nil {
		t.Fatalf("failed to create wildcard certificate: %v", err)
	}
	exactCertPEM, exactKeyPEM := generateCertificatePair(t, []string{"api.example.com"})
	exactCert, err := CreateTLSCertificate(TLSCertificateInput{
		Name:    "exact-cert",
		CertPEM: exactCertPEM,
		KeyPEM:  exactKeyPEM,
	})
	if err != nil {
		t.Fatalf("failed to create exact certificate: %v", err)
	}
	if _, err = CreateManagedDomain(ManagedDomainInput{
		Domain:  "*.example.com",
		CertID:  &wildcardCert.ID,
		Enabled: true,
	}); err != nil {
		t.Fatalf("failed to create wildcard managed domain: %v", err)
	}
	if _, err = CreateManagedDomain(ManagedDomainInput{
		Domain:  "api.example.com",
		CertID:  &exactCert.ID,
		Enabled: true,
	}); err != nil {
		t.Fatalf("failed to create exact managed domain: %v", err)
	}

	result, err := MatchManagedDomainCertificate("api.example.com")
	if err != nil {
		t.Fatalf("MatchManagedDomainCertificate failed: %v", err)
	}
	if !result.Matched || result.Candidate == nil {
		t.Fatal("expected exact domain to be matched")
	}
	if result.Candidate.MatchType != ManagedDomainMatchTypeExact {
		t.Fatalf("expected exact match first, got %s", result.Candidate.MatchType)
	}
	if result.Candidate.CertificateID != exactCert.ID {
		t.Fatalf("expected exact certificate %d, got %d", exactCert.ID, result.Candidate.CertificateID)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("expected 2 match candidates, got %d", len(result.Candidates))
	}
}

func TestMatchManagedDomainCertificateSupportsWildcard(t *testing.T) {
	setupServiceTestDB(t)

	certPEM, keyPEM := generateCertificatePair(t, []string{"*.example.com"})
	certificate, err := CreateTLSCertificate(TLSCertificateInput{
		Name:    "wildcard-cert",
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	})
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}
	if _, err = CreateManagedDomain(ManagedDomainInput{
		Domain:  "*.example.com",
		CertID:  &certificate.ID,
		Enabled: true,
	}); err != nil {
		t.Fatalf("failed to create managed domain: %v", err)
	}

	result, err := MatchManagedDomainCertificate("edge.example.com")
	if err != nil {
		t.Fatalf("MatchManagedDomainCertificate failed: %v", err)
	}
	if !result.Matched || result.Candidate == nil {
		t.Fatal("expected wildcard domain to be matched")
	}
	if result.Candidate.MatchType != ManagedDomainMatchTypeWildcard {
		t.Fatalf("expected wildcard match, got %s", result.Candidate.MatchType)
	}

	deepResult, err := MatchManagedDomainCertificate("deep.edge.example.com")
	if err != nil {
		t.Fatalf("MatchManagedDomainCertificate failed: %v", err)
	}
	if deepResult.Matched {
		t.Fatal("expected single-level wildcard not to match deep subdomain")
	}
}

func TestCreateManagedDomainRejectsInvalidWildcard(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateManagedDomain(ManagedDomainInput{
		Domain:  "*.*.example.com",
		Enabled: true,
	})
	if err == nil {
		t.Fatal("expected invalid wildcard domain to fail")
	}
}
