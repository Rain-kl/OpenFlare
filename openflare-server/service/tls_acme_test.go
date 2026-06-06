package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rain-kl/openflare/openflare-server/model"
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

func TestConvertTLSCertificateToAcmePreservesUploadUntilSuccess(t *testing.T) {
	setupServiceTestDB(t)

	originalCertPEM, originalKeyPEM := generateCertificatePair(t, []string{"manual.example.com"})
	cert, err := CreateTLSCertificate(TLSCertificateInput{
		Name:    "manual-cert",
		CertPEM: originalCertPEM,
		KeyPEM:  originalKeyPEM,
		Remark:  "manual upload",
	})
	if err != nil {
		t.Fatalf("CreateTLSCertificate failed: %v", err)
	}
	originalCertPEM = cert.CertPEM
	originalKeyPEM = cert.KeyPEM

	newCertPEM, newKeyPEM := generateCertificatePair(t, []string{"managed.example.com"})
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	restore := SetTLSCertificateObtainFuncForTest(func(c *model.TLSCertificate) error {
		started <- struct{}{}
		<-release
		c.CertPEM = newCertPEM
		c.KeyPEM = newKeyPEM
		c.NotBefore = time.Now().Add(-time.Hour)
		c.NotAfter = time.Now().Add(90 * 24 * time.Hour)
		c.ApplyStatus = "ready"
		c.ApplyMessage = ""
		return model.DB.Save(c).Error
	})
	t.Cleanup(restore)

	converted, err := ConvertTLSCertificateToAcme(cert.ID, TLSApplyInput{
		Name:          "managed-cert",
		Remark:        "converted",
		AcmeAccountID: 1,
		DnsAccountID:  2,
		KeyAlgorithm:  "EC256",
		AutoRenew:     true,
		PrimaryDomain: "managed.example.com",
		OtherDomains:  "www.managed.example.com",
	})
	if err != nil {
		t.Fatalf("ConvertTLSCertificateToAcme failed: %v", err)
	}
	if converted.ID != cert.ID {
		t.Fatalf("expected converted certificate to keep id %d, got %d", cert.ID, converted.ID)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected conversion obtain task to start")
	}

	applying, err := model.GetTLSCertificateByID(cert.ID)
	if err != nil {
		t.Fatalf("reload applying certificate failed: %v", err)
	}
	if applying.Provider != "upload" {
		t.Fatalf("expected provider to remain upload while applying, got %s", applying.Provider)
	}
	if applying.ApplyStatus != "applying" {
		t.Fatalf("expected applying status, got %s", applying.ApplyStatus)
	}
	if applying.CertPEM != originalCertPEM || applying.KeyPEM != originalKeyPEM {
		t.Fatal("expected original PEM payloads to be preserved while applying")
	}

	close(release)

	finalCert := waitForCertificateState(t, cert.ID, func(c *model.TLSCertificate) bool {
		return c.Provider == "acme" && c.ApplyStatus == "ready"
	})
	if finalCert.CertPEM != newCertPEM || finalCert.KeyPEM != newKeyPEM {
		t.Fatal("expected successful conversion to replace PEM payloads")
	}
	if !finalCert.AutoRenew {
		t.Fatal("expected converted certificate to keep auto renew enabled")
	}
	if finalCert.PrimaryDomain != "managed.example.com" || finalCert.OtherDomains != "www.managed.example.com" {
		t.Fatalf("expected converted certificate to persist ACME domains, got %+v", finalCert)
	}
}

func TestConvertTLSCertificateToAcmePreservesUploadOnFailure(t *testing.T) {
	setupServiceTestDB(t)

	originalCertPEM, originalKeyPEM := generateCertificatePair(t, []string{"manual.example.com"})
	cert, err := CreateTLSCertificate(TLSCertificateInput{
		Name:    "manual-cert",
		CertPEM: originalCertPEM,
		KeyPEM:  originalKeyPEM,
	})
	if err != nil {
		t.Fatalf("CreateTLSCertificate failed: %v", err)
	}
	originalCertPEM = cert.CertPEM
	originalKeyPEM = cert.KeyPEM

	restore := SetTLSCertificateObtainFuncForTest(func(c *model.TLSCertificate) error {
		err := errors.New("dns challenge failed")
		updateCertError(c, err.Error())
		return err
	})
	t.Cleanup(restore)

	if _, err := ConvertTLSCertificateToAcme(cert.ID, TLSApplyInput{
		Name:          "manual-cert",
		DnsAccountID:  1,
		PrimaryDomain: "manual.example.com",
	}); err != nil {
		t.Fatalf("ConvertTLSCertificateToAcme failed: %v", err)
	}

	finalCert := waitForCertificateState(t, cert.ID, func(c *model.TLSCertificate) bool {
		return c.ApplyStatus == "error"
	})
	if finalCert.Provider != "upload" {
		t.Fatalf("expected failed conversion to keep upload provider, got %s", finalCert.Provider)
	}
	if finalCert.CertPEM != originalCertPEM || finalCert.KeyPEM != originalKeyPEM {
		t.Fatal("expected failed conversion to preserve original PEM payloads")
	}
	if !strings.Contains(finalCert.ApplyMessage, "dns challenge failed") {
		t.Fatalf("expected conversion error message, got %q", finalCert.ApplyMessage)
	}
}

func TestConvertTLSCertificateToAcmeRejectsInvalidStates(t *testing.T) {
	setupServiceTestDB(t)

	certPEM, keyPEM := generateCertificatePair(t, []string{"manual.example.com"})
	cert, err := CreateTLSCertificate(TLSCertificateInput{
		Name:    "manual-cert",
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	})
	if err != nil {
		t.Fatalf("CreateTLSCertificate failed: %v", err)
	}

	cert.Provider = "acme"
	if err := cert.Update(); err != nil {
		t.Fatalf("failed to mark certificate acme: %v", err)
	}
	if _, err := ConvertTLSCertificateToAcme(cert.ID, TLSApplyInput{Name: "manual-cert"}); err == nil || !strings.Contains(err.Error(), "only uploaded") {
		t.Fatalf("expected non-upload conversion to fail, got %v", err)
	}

	cert.Provider = "upload"
	cert.ApplyStatus = "applying"
	if err := cert.Update(); err != nil {
		t.Fatalf("failed to mark certificate applying: %v", err)
	}
	if _, err := ConvertTLSCertificateToAcme(cert.ID, TLSApplyInput{Name: "manual-cert"}); err == nil || !strings.Contains(err.Error(), "already applying") {
		t.Fatalf("expected applying conversion to fail, got %v", err)
	}
}

func waitForCertificateState(t *testing.T, id uint, matches func(*model.TLSCertificate) bool) *model.TLSCertificate {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cert, err := model.GetTLSCertificateByID(id)
		if err != nil {
			t.Fatalf("reload certificate %d failed: %v", id, err)
		}
		if matches(cert) {
			return cert
		}
		time.Sleep(10 * time.Millisecond)
	}

	cert, err := model.GetTLSCertificateByID(id)
	if err != nil {
		t.Fatalf("reload certificate %d failed: %v", id, err)
	}
	t.Fatalf("certificate %d did not reach expected state: %+v", id, cert)
	return nil
}
