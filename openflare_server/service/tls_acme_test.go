package service

import (
	"openflare/model"
	"testing"
	"time"
)

func TestAcmeAndDnsIntegration(t *testing.T) {
	setupServiceTestDB(t)

	// 1. Create a DNS Account
	dnsAccount := &model.DnsAccount{
		Name:          "Test Cloudflare",
		Type:          "cloudflare",
		Authorization: `{"api_token": "dummy_token"}`,
	}
	if err := dnsAccount.Insert(); err != nil {
		t.Fatalf("Failed to insert DNS Account: %v", err)
	}

	// 2. Apply for TLS Certificate (using the new ApplyTLSCertificate function)
	certInput := TLSApplyInput{
		Name:          "Test ACME Cert",
		PrimaryDomain: "example.com",
		OtherDomains:  "*.example.com",
		DnsAccountID:  dnsAccount.ID,
		KeyAlgorithm:  "RSA2048",
		AutoRenew:     true,
	}

	cert, err := ApplyTLSCertificate(certInput)
	if err != nil {
		t.Fatalf("ApplyTLSCertificate failed: %v", err)
	}

	if cert.ApplyStatus != "applying" {
		t.Fatalf("Expected cert ApplyStatus to be applying, got %s", cert.ApplyStatus)
	}

	if cert.Provider != "acme" {
		t.Fatalf("Expected cert Provider to be acme, got %s", cert.Provider)
	}

	// 3. Try to delete the DNS account (should fail since it's used by the cert)
	// Actually, the delete logic is in the controller for the foreign key check.
	// But let's check if the controller logic can be tested here, or we just trust the DB setup.
	var count int64
	model.DB.Model(&model.TLSCertificate{}).Where("dns_account_id = ?", dnsAccount.ID).Count(&count)
	if count != 1 {
		t.Fatalf("Expected 1 certificate associated with DNS account, got %d", count)
	}

	// 4. Test RenewTLSCertificate
	renewedCert, err := RenewTLSCertificate(cert.ID)
	if err != nil {
		t.Fatalf("RenewTLSCertificate failed: %v", err)
	}
	if renewedCert.ApplyStatus != "applying" {
		t.Fatalf("Expected renewed cert ApplyStatus to be applying, got %s", renewedCert.ApplyStatus)
	}

	// Wait for the async goroutine to fail (it now registers an LE account, which takes longer)
	time.Sleep(5 * time.Second)

	// Reload cert and verify error status
	finalCert, err := model.GetTLSCertificateByID(renewedCert.ID)
	if err != nil {
		t.Fatalf("Failed to reload cert: %v", err)
	}
	if finalCert.ApplyStatus != "error" {
		t.Fatalf("Expected final cert ApplyStatus to be error, got %s", finalCert.ApplyStatus)
	}
	if finalCert.ApplyMessage == "" {
		t.Fatalf("Expected final cert ApplyMessage to be populated, got empty")
	}

	// Clean up
	if err := DeleteTLSCertificate(cert.ID); err != nil {
		t.Fatalf("DeleteTLSCertificate failed: %v", err)
	}

	if err := dnsAccount.Delete(); err != nil {
		t.Fatalf("Failed to delete DNS Account after cert cleanup: %v", err)
	}
}
