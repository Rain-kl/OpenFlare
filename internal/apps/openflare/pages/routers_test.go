// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadDeploymentHandlerRecordsCurrentUserActor(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()

	project, err := CreateProject(t.Context(), Input{Name: "Actor Upload", Slug: "actor-upload", Enabled: true})
	require.NoError(t, err)
	packageBytes := testPagesZip(t, map[string]string{"index.html": "ok"})

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("package", "site.zip")
	require.NoError(t, err)
	_, err = part.Write(packageBytes)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/d/pages/1/deployments/upload", &requestBody)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(project.ID), 10)}}
	oauth.SetToContext(c, oauth.UserObjKey, &model.User{ID: 42})

	UploadDeploymentHandler(c)
	assert.Equal(t, http.StatusOK, recorder.Code)
	deployments, err := model.ListPagesDeployments(t.Context(), project.ID)
	require.NoError(t, err)
	require.Len(t, deployments, 1)
	assert.Equal(t, "user:42", deployments[0].CreatedBy)
}

func TestUploadDeploymentFromURLHandlerRecordsCurrentUserActor(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()

	project, err := CreateProject(t.Context(), Input{Name: "Actor URL", Slug: "actor-url", Enabled: true})
	require.NoError(t, err)
	packageBytes := testPagesZip(t, map[string]string{"index.html": "ok"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="site.zip"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(packageBytes)
	}))
	defer server.Close()

	body, err := json.Marshal(UploadFromURLInput{URL: server.URL + "/site.zip"})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/d/pages/1/deployments/upload-from-url", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(project.ID)}}
	oauth.SetToContext(c, oauth.UserObjKey, &model.User{ID: 77})

	UploadDeploymentFromURLHandler(c)
	assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	deployments, err := model.ListPagesDeployments(t.Context(), project.ID)
	require.NoError(t, err)
	require.Len(t, deployments, 1)
	assert.Equal(t, "user:77", deployments[0].CreatedBy)
}

func TestCurrentPagesActorRejectsMissingUser(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	actor, ok := currentPagesActor(c)
	assert.False(t, ok)
	assert.Empty(t, actor)
	assert.True(t, c.IsAborted())
}
