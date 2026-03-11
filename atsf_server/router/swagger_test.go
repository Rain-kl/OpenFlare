package router_test

import (
	"os"
	"strings"
	"testing"
)

func TestGeneratedSwaggerSpecExists(t *testing.T) {
	data, err := os.ReadFile("../docs/swagger.json")
	if err != nil {
		t.Fatalf("failed to read generated swagger spec: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "\"title\": \"ATSFlare Server API\"") {
		t.Fatal("expected swagger spec title to exist")
	}
	if !strings.Contains(content, "\"/api/proxy-routes/\"") {
		t.Fatal("expected swagger spec to contain proxy route endpoint")
	}
}
