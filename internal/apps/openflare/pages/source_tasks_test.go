// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
)

func TestSourceActionPayloadValidationIsStrictAndCredentialFree(t *testing.T) {
	handler := &SourceActionHandler{}
	valid := SourceActionPayload{
		SourceID:      7,
		ConfigVersion: 3,
		Action:        sourceActionSync,
		Actor:         "user:42",
		TriggerType:   pagesSourceTriggerManualSync,
	}
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatalf("json.Marshal(valid payload) error = %v, want nil", err)
	}
	normalized, err := handler.ValidatePayload(raw)
	if err != nil {
		t.Fatalf("ValidatePayload(valid) error = %v, want nil", err)
	}
	var got SourceActionPayload
	if err := json.Unmarshal(normalized, &got); err != nil {
		t.Fatalf("json.Unmarshal(normalized payload) error = %v, want nil", err)
	}
	if got != valid {
		t.Errorf("ValidatePayload(valid) = %+v, want %+v", got, valid)
	}
	legacy := valid
	legacy.TriggerType = ""
	legacyRaw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("json.Marshal(legacy payload) error = %v, want nil", err)
	}
	legacyNormalized, err := handler.ValidatePayload(legacyRaw)
	if err != nil {
		t.Fatalf("ValidatePayload(legacy payload) error = %v, want nil", err)
	}
	var legacyGot SourceActionPayload
	if err := json.Unmarshal(legacyNormalized, &legacyGot); err != nil {
		t.Fatalf("json.Unmarshal(legacy normalized payload) error = %v, want nil", err)
	}
	if legacyGot.TriggerType != pagesSourceTriggerManualSync {
		t.Errorf("legacy payload trigger_type = %q, want %q", legacyGot.TriggerType, pagesSourceTriggerManualSync)
	}
	for _, forbidden := range []string{"remote_url", "content_config_version", "expected_revision", "lease_token", "etag"} {
		if strings.Contains(string(normalized), forbidden) {
			t.Errorf("normalized payload = %s, want no forbidden field %q", normalized, forbidden)
		}
	}

	invalidPayloads := []struct {
		name string
		raw  string
	}{
		{
			name: "unknown remote URL field",
			raw:  `{"source_id":7,"config_version":3,"action":"sync","actor":"user:42","remote_url":"https://example.com/site.zip?token=secret"}`,
		},
		{
			name: "unknown content version field",
			raw:  `{"source_id":7,"config_version":3,"action":"sync","actor":"user:42","content_config_version":9}`,
		},
		{
			name: "empty actor",
			raw:  `{"source_id":7,"config_version":3,"action":"sync","actor":""}`,
		},
		{
			name: "untrusted system actor",
			raw:  `{"source_id":7,"config_version":3,"action":"sync","actor":"system"}`,
		},
		{
			name: "zero user actor",
			raw:  `{"source_id":7,"config_version":3,"action":"sync","actor":"user:0"}`,
		},
		{
			name: "invalid action",
			raw:  `{"source_id":7,"config_version":3,"action":"activate","actor":"user:42"}`,
		},
		{
			name: "multiple JSON values",
			raw:  `{"source_id":7,"config_version":3,"action":"sync","actor":"user:42"} {}`,
		},
	}
	for _, test := range invalidPayloads {
		t.Run(test.name, func(t *testing.T) {
			normalized, err := handler.ValidatePayload([]byte(test.raw))
			if err == nil {
				t.Errorf("ValidatePayload(%s) = %s, nil; want non-nil error", test.raw, normalized)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Errorf("ValidatePayload(%s) error = %q, want credential-free error", test.name, err)
			}
		})
	}
}

func TestSourceActionPayloadAcceptsOnlyRealActors(t *testing.T) {
	tests := []struct {
		actor string
		want  bool
	}{
		{actor: "user:1", want: true},
		{actor: "user:18446744073709551615", want: true},
		{actor: pagesSourceCreatedBySystem, want: true},
		{actor: "", want: false},
		{actor: "user:0", want: false},
		{actor: "user:-1", want: false},
		{actor: "user:not-a-number", want: false},
		{actor: "system", want: false},
	}
	for _, test := range tests {
		if got := validPagesSourceActor(test.actor); got != test.want {
			t.Errorf("validPagesSourceActor(%q) = %t, want %t", test.actor, got, test.want)
		}
	}
}

func TestRemoteCheckActionIsPermanentWithoutExposingURL(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "task-remote-check")
	secret := "task-query-secret"
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		"https://example.com/site.zip?token="+secret,
		RemoteNetworkPolicyPublic,
	)
	raw, err := json.Marshal(SourceActionPayload{
		SourceID:      source.ID,
		ConfigVersion: source.ConfigVersion,
		Action:        sourceActionCheck,
		Actor:         "user:9",
	})
	if err != nil {
		t.Fatalf("json.Marshal(check payload) error = %v, want nil", err)
	}

	result, err := (&SourceActionHandler{}).Execute(ctx, raw)
	if result != nil {
		t.Errorf("SourceActionHandler.Execute(remote check) result = %+v, want nil", result)
	}
	if err == nil {
		t.Fatal("SourceActionHandler.Execute(remote check) error = nil, want permanent error")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("SourceActionHandler.Execute(remote check) error = %v, want errors.Is(asynq.SkipRetry)", err)
	}
	if got, want := err.Error(), errPagesSourceCheckUnsupported; got != want {
		t.Errorf("SourceActionHandler.Execute(remote check) error = %q, want %q", got, want)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(string(raw), secret) {
		t.Errorf("remote check result error/payload = %q / %s, want no URL secret", err, raw)
	}
}

func TestPagesSourceActionMetaIsInternalOnly(t *testing.T) {
	if !PagesSourceActionMeta.InternalOnly {
		t.Error("PagesSourceActionMeta.InternalOnly = false, want true")
	}
	if PagesSourceActionMeta.Type != TaskTypePagesSourceAction {
		t.Errorf("PagesSourceActionMeta.Type = %q, want %q", PagesSourceActionMeta.Type, TaskTypePagesSourceAction)
	}
	if PagesSourceActionMeta.AsynqTask != PagesSourceActionTask {
		t.Errorf("PagesSourceActionMeta.AsynqTask = %q, want %q", PagesSourceActionMeta.AsynqTask, PagesSourceActionTask)
	}
	if PagesSourceActionMeta.Retryable {
		t.Error("PagesSourceActionMeta.Retryable = true, want false for manual retry API")
	}
	if PagesSourceActionMeta.MaxRetry <= 0 {
		t.Errorf("PagesSourceActionMeta.MaxRetry = %d, want bounded transient retries", PagesSourceActionMeta.MaxRetry)
	}
}
