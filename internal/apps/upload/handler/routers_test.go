// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	uploadstats "github.com/Rain-kl/Wavelet/internal/apps/upload/stats"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/storage"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type testResponse struct {
	ErrorMsg string          `json:"error_msg"`
	Data     json.RawMessage `json:"data"`
}

func setupTestRouter(authUser *model.User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(response.ErrorHandlerMiddleware())

	authMiddleware := func(c *gin.Context) {
		if authUser != nil {
			oauth.SetToContext(c, oauth.UserObjKey, authUser)
		}
		c.Next()
	}

	uploadGroup := r.Group("/api/v1/upload")
	uploadGroup.Use(authMiddleware)
	{
		uploadGroup.POST("", UploadFile)
		uploadGroup.GET("/my", ListMyFiles)
		uploadGroup.DELETE("/:id", DeleteMyFile)
		uploadGroup.PUT("/:id", UpdateMyFile)
		uploadGroup.GET("/download/:id", DownloadFile)
		uploadGroup.POST("/download/batch", BatchDownloadFiles)
	}

	adminGroup := r.Group("/api/v1/admin/uploads")
	adminGroup.Use(authMiddleware)
	{
		adminGroup.GET("", ListFiles)
		adminGroup.GET("/stats", GetFileStats)
		adminGroup.DELETE("/:id", DeleteFile)
		adminGroup.GET("/download/:id", DownloadFile)
		adminGroup.POST("/download/batch", BatchDownloadFiles)
	}

	return r
}

func createMultipartRequest(t *testing.T, fieldName, fileName string, fileContent []byte, extraFields map[string]string) (string, *bytes.Buffer) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}

	_, err = part.Write(fileContent)
	if err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}

	for k, v := range extraFields {
		err = writer.WriteField(k, v)
		if err != nil {
			t.Fatalf("failed to write form field: %v", err)
		}
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	return writer.FormDataContentType(), body
}

func TestUploadFile(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	authUser := &model.User{ID: 1001, Username: "test_user"}
	router := setupTestRouter(authUser)

	// Mock Storage Client
	mockFiles := make(map[string][]byte)
	var putCount int

	restoreStorage := storage.MockStorage(
		func(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
			data, err := io.ReadAll(body)
			if err != nil {
				return err
			}
			mockFiles[key] = data
			putCount++
			return nil
		},
		func(ctx context.Context, key string) (*storage.Object, error) {
			data, ok := mockFiles[key]
			if !ok {
				return nil, os.ErrNotExist
			}
			return &storage.Object{
				Body:          io.NopCloser(bytes.NewReader(data)),
				ContentLength: int64(len(data)),
				ContentType:   "application/octet-stream",
			}, nil
		},
		func(ctx context.Context, key string) error {
			delete(mockFiles, key)
			return nil
		},
	)
	defer restoreStorage()

	// 开启 S3 Storage
	storage.IsEnabledFunc = func() bool { return true }
	defer func() {
		storage.IsEnabledFunc = func() bool { return false }
	}()

	t.Run("upload allowed image file successfully", func(t *testing.T) {
		putCount = 0
		imgContent := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89") // Valid PNG header
		contentType, body := createMultipartRequest(t, "file", "test.png", imgContent, map[string]string{
			"type":     "avatar",
			"metadata": `{"extra":{"source":"test_runner"}}`,
		})

		req, _ := http.NewRequest("POST", "/api/v1/upload", body)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp testResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.ErrorMsg != "" {
			t.Fatalf("expected success response, got failure: %s", resp.ErrorMsg)
		}

		// Verify database record
		var uploadRecord model.Upload
		if err := json.Unmarshal(resp.Data, &uploadRecord); err != nil {
			t.Fatalf("failed to unmarshal upload record: %v", err)
		}

		var dbRecord model.Upload
		if err := dbConn.First(&dbRecord, uploadRecord.ID).Error; err != nil {
			t.Fatalf("failed to retrieve database record: %v", err)
		}

		if dbRecord.FileName != "test.png" || dbRecord.Extension != "png" {
			t.Errorf("incorrect filename or extension: %s, %s", dbRecord.FileName, dbRecord.Extension)
		}

		if dbRecord.MimeType != "image/png" {
			t.Errorf("incorrect mime type detected: %s", dbRecord.MimeType)
		}

		if dbRecord.Metadata.Extra["source"] != "test_runner" {
			t.Errorf("expected extra meta 'source' to be 'test_runner', got %v", dbRecord.Metadata.Extra)
		}

		if putCount != 1 {
			t.Errorf("expected 1 storage Put operation, got %d", putCount)
		}
	})

	t.Run("upload blocked extension file", func(t *testing.T) {
		// System config allowed: jpg,png,webp. Uploading docx should be blocked.
		contentType, body := createMultipartRequest(t, "file", "contract.docx", []byte("fake docx content"), nil)
		req, _ := http.NewRequest("POST", "/api/v1/upload", body)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp testResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.ErrorMsg == "" || !strings.Contains(resp.ErrorMsg, shared.ErrUnsupportedFormat) {
			t.Errorf("expected unsupported format error, got: %v", resp)
		}
	})

	t.Run("upload rejects Pages reserved type", func(t *testing.T) {
		putCountBefore := putCount
		imgContent := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01")
		contentType, body := createMultipartRequest(t, "file", "pages.png", imgContent, map[string]string{
			"type": shared.ReservedPagesDeploymentType,
		})
		req, _ := http.NewRequest("POST", "/api/v1/upload", body)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d. Body: %s", w.Code, w.Body.String())
		}
		var resp testResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal reserved type response: %v", err)
		}
		if resp.ErrorMsg != shared.ErrReservedUploadType {
			t.Fatalf("reserved type error = %q, want %q", resp.ErrorMsg, shared.ErrReservedUploadType)
		}
		if putCount != putCountBefore {
			t.Fatalf("reserved upload wrote storage object: put count %d -> %d", putCountBefore, putCount)
		}
		var count int64
		if err := dbConn.Model(&model.Upload{}).
			Where("type = ?", shared.ReservedPagesDeploymentType).
			Count(&count).Error; err != nil {
			t.Fatalf("count reserved uploads: %v", err)
		}
		if count != 0 {
			t.Fatalf("reserved upload record count = %d, want 0", count)
		}
	})

	t.Run("instant upload deduplication (秒传)", func(t *testing.T) {
		putCount = 0
		imgContent := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01")

		// Upload first time
		contentType1, body1 := createMultipartRequest(t, "file", "avatar1.png", imgContent, map[string]string{"type": "avatar"})
		req1, _ := http.NewRequest("POST", "/api/v1/upload", body1)
		req1.Header.Set("Content-Type", contentType1)
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Fatalf("first upload failed: %s", w1.Body.String())
		}
		if putCount != 1 {
			t.Errorf("expected 1 put count on first upload, got %d", putCount)
		}

		// Upload same file second time (different filename, same content)
		contentType2, body2 := createMultipartRequest(t, "file", "avatar2.png", imgContent, map[string]string{"type": "avatar"})
		req2, _ := http.NewRequest("POST", "/api/v1/upload", body2)
		req2.Header.Set("Content-Type", contentType2)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Fatalf("second upload failed: %s", w2.Body.String())
		}

		var resp2 testResponse
		_ = json.Unmarshal(w2.Body.Bytes(), &resp2)

		if resp2.ErrorMsg != "" {
			t.Fatalf("second upload was unsuccessful: %s", resp2.ErrorMsg)
		}

		var uploadRecord2 model.Upload
		if err := json.Unmarshal(resp2.Data, &uploadRecord2); err != nil {
			t.Fatalf("failed to unmarshal second upload record: %v", err)
		}

		// Check if it triggered another storage put
		if putCount != 1 {
			t.Errorf("PutObject was triggered again! Expected deduplication (putCount=1), got putCount=%d", putCount)
		}

		// Check if database contains both records sharing the same FilePath
		var records []model.Upload
		dbConn.Where("hash = ?", uploadRecord2.Hash).Find(&records)
		if len(records) != 2 {
			t.Errorf("expected 2 database records sharing the same hash, got %d", len(records))
		}
		if records[0].FilePath != records[1].FilePath {
			t.Errorf("file paths are different: %s vs %s", records[0].FilePath, records[1].FilePath)
		}
		if records[0].ID == records[1].ID {
			t.Error("database record IDs should be unique")
		}

		t.Logf("Instant upload success. Record 1: %d, Record 2: %d", records[0].ID, records[1].ID)
	})

	t.Run("upload in local storage fallback mode", func(t *testing.T) {
		// Turn off S3
		storage.IsEnabledFunc = func() bool { return false }

		// Seed allowed extensions configuration to allow txt files
		var sc model.SystemConfig
		dbConn.Where("key = ?", model.ConfigKeyUploadAllowedExtensions).First(&sc)
		sc.Value = "jpg,png,webp,txt"
		dbConn.Save(&sc)
		_ = db.HSetJSON(context.Background(), repository.SystemConfigRedisHashKey, sc.Key, &sc)
		repository.ResetSystemConfigRAMCacheForTest()

		contentType, body := createMultipartRequest(t, "file", "doc.txt", []byte("hello world generic document file"), map[string]string{
			"type": "document",
		})
		req, _ := http.NewRequest("POST", "/api/v1/upload", body)
		req.Header.Set("Content-Type", contentType)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp testResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		if resp.ErrorMsg != "" {
			t.Fatalf("local upload failed: %s", resp.ErrorMsg)
		}

		var localRecord model.Upload
		if err := json.Unmarshal(resp.Data, &localRecord); err != nil {
			t.Fatalf("failed to unmarshal local upload record: %v", err)
		}

		// Confirm file was actually written to local disk
		fileContent, err := os.ReadFile(filepath.Join(tempDir, localRecord.FilePath))
		if err != nil {
			t.Fatalf("failed to read local file: %v", err)
		}

		if string(fileContent) != "hello world generic document file" {
			t.Errorf("unexpected local file contents: %s", string(fileContent))
		}
	})
}

func TestDownloadFile(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	authUser := &model.User{ID: 1001, Username: "test_user"}
	router := setupTestRouter(authUser)

	// Seed upload records in DB
	localUpload := model.Upload{
		ID:        2001,
		UserID:    1001,
		FileName:  "中文文件名.txt",
		FilePath:  "test_download.txt",
		FileSize:  12,
		MimeType:  "text/plain",
		Extension: "txt",
		Status:    model.UploadStatusUsed,
	}

	// Create local file
	err := os.WriteFile(filepath.Join(tempDir, "test_download.txt"), []byte("hello download"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	dbConn.Create(&localUpload)

	t.Run("download file successfully", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/uploads/download/2001", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		if w.Body.String() != "hello download" {
			t.Errorf("expected body 'hello download', got '%s'", w.Body.String())
		}

		// Verify Content-Disposition header (supports UTF-8 escaping)
		contentDisp := w.Header().Get("Content-Disposition")
		expectedDisp := "attachment; filename*=UTF-8''%E4%B8%AD%E6%96%87%E6%96%87%E4%BB%B6%E5%90%8D.txt"
		if contentDisp != expectedDisp {
			t.Errorf("expected Content-Disposition header %q, got %q", expectedDisp, contentDisp)
		}

		if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
			t.Errorf("expected Content-Type starting with text/plain, got %s", w.Header().Get("Content-Type"))
		}
	})

	t.Run("download non-existent file", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/uploads/download/9999", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestListFiles(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	authUser := &model.User{ID: 1001, Username: "test_user"}
	router := setupTestRouter(authUser)

	uploads := []model.Upload{
		{
			ID:        2101,
			UserID:    authUser.ID,
			FileName:  "first-report.txt",
			FilePath:  "first-report.txt",
			FileSize:  10,
			MimeType:  "text/plain",
			Extension: "txt",
			Status:    model.UploadStatusUsed,
		},
		{
			ID:        2102,
			UserID:    authUser.ID,
			FileName:  "Second-Photo.PNG",
			FilePath:  "second-photo.png",
			FileSize:  20,
			MimeType:  "image/png",
			Extension: "png",
			Status:    model.UploadStatusUsed,
		},
		{
			ID:        2103,
			UserID:    authUser.ID,
			FileName:  "third-notes.md",
			FilePath:  "third-notes.md",
			FileSize:  30,
			MimeType:  "text/markdown",
			Extension: "md",
			Status:    model.UploadStatusUsed,
		},
		{
			ID:        2104,
			UserID:    2002,
			FileName:  "other-user.txt",
			FilePath:  "other-user.txt",
			FileSize:  40,
			MimeType:  "text/plain",
			Extension: "txt",
			Status:    model.UploadStatusUsed,
		},
	}
	for i := range uploads {
		if err := dbConn.Create(&uploads[i]).Error; err != nil {
			t.Fatalf("failed to create upload %d: %v", uploads[i].ID, err)
		}
	}

	t.Run("returns requested page", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/uploads?page=2&page_size=2", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var resp testResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.ErrorMsg != "" {
			t.Fatalf("ListFiles() error = %q, want empty", resp.ErrorMsg)
		}

		var got listFilesResponse
		if err := json.Unmarshal(resp.Data, &got); err != nil {
			t.Fatalf("failed to parse list response: %v", err)
		}
		if got.Page != 2 {
			t.Errorf("ListFiles(page=2).Page = %d, want 2", got.Page)
		}
		if got.PageSize != 2 {
			t.Errorf("ListFiles(page_size=2).PageSize = %d, want 2", got.PageSize)
		}
		if got.Total != 4 {
			t.Errorf("ListFiles().Total = %d, want 4", got.Total)
		}
		if len(got.Items) != 2 {
			t.Fatalf("ListFiles(page=2, page_size=2) returned %d items, want 2", len(got.Items))
		}
	})

	t.Run("filters filename case insensitively", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/uploads?keyword=photo", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var resp testResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.ErrorMsg != "" {
			t.Fatalf("ListFiles(keyword=photo) error = %q, want empty", resp.ErrorMsg)
		}

		var got listFilesResponse
		if err := json.Unmarshal(resp.Data, &got); err != nil {
			t.Fatalf("failed to parse list response: %v", err)
		}
		if got.Total != 1 {
			t.Errorf("ListFiles(keyword=photo).Total = %d, want 1", got.Total)
		}
		if len(got.Items) != 1 {
			t.Fatalf("ListFiles(keyword=photo) returned %d items, want 1", len(got.Items))
		}
		if got.Items[0].FileName != "Second-Photo.PNG" {
			t.Errorf("ListFiles(keyword=photo).Items[0].FileName = %q, want %q", got.Items[0].FileName, "Second-Photo.PNG")
		}
	})

	t.Run("filters by user_id", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/uploads?user_id=1001", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var resp testResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.ErrorMsg != "" {
			t.Fatalf("ListFiles(user_id=1001) error = %q, want empty", resp.ErrorMsg)
		}

		var got listFilesResponse
		if err := json.Unmarshal(resp.Data, &got); err != nil {
			t.Fatalf("failed to parse list response: %v", err)
		}
		if got.Total != 3 {
			t.Errorf("ListFiles(user_id=1001).Total = %d, want 3", got.Total)
		}
		if len(got.Items) != 3 {
			t.Fatalf("ListFiles(user_id=1001) returned %d items, want 3", len(got.Items))
		}
	})
}

func TestBatchDownloadFiles(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	authUser := &model.User{ID: 1001, Username: "test_user"}
	router := setupTestRouter(authUser)

	if err := os.WriteFile(filepath.Join(tempDir, "f1.txt"), []byte("file1 content"), 0644); err != nil {
		t.Fatalf("failed to write f1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "f2.txt"), []byte("file2 content"), 0644); err != nil {
		t.Fatalf("failed to write f2.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "f3.txt"), []byte("duplicate name file content"), 0644); err != nil {
		t.Fatalf("failed to write f3.txt: %v", err)
	}

	// Seed upload records. Note f2 and f3 have the same FileName "file_a.txt" to trigger name collision resolution.
	uploads := []model.Upload{
		{
			ID:        3001,
			UserID:    1001,
			FileName:  "file_a.txt",
			FilePath:  "f1.txt",
			FileSize:  13,
			MimeType:  "text/plain",
			Extension: "txt",
			Status:    model.UploadStatusUsed,
		},
		{
			ID:        3002,
			UserID:    1001,
			FileName:  "file_b.txt",
			FilePath:  "f2.txt",
			FileSize:  13,
			MimeType:  "text/plain",
			Extension: "txt",
			Status:    model.UploadStatusUsed,
		},
		{
			ID:        3003,
			UserID:    1001,
			FileName:  "file_a.txt", // COLLISION with 3001!
			FilePath:  "f3.txt",
			FileSize:  28,
			MimeType:  "text/plain",
			Extension: "txt",
			Status:    model.UploadStatusUsed,
		},
	}

	for _, up := range uploads {
		dbConn.Create(&up)
	}

	t.Run("batch download zip successfully and check duplicate renaming", func(t *testing.T) {
		reqBody, _ := json.Marshal(batchDownloadRequest{
			IDs: []string{"3001", "3002", "3003"},
		})
		req, _ := http.NewRequest("POST", "/api/v1/admin/uploads/download/batch", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		if w.Header().Get("Content-Type") != "application/zip" {
			t.Errorf("expected Content-Type application/zip, got %s", w.Header().Get("Content-Type"))
		}

		// Unzip in-memory
		zipReader, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
		if err != nil {
			t.Fatalf("failed to read zip buffer: %v", err)
		}

		if len(zipReader.File) != 3 {
			t.Errorf("expected 3 files inside the ZIP, got %d", len(zipReader.File))
		}

		// Extract files to check their contents and name collision resolutions
		extracted := make(map[string]string)
		for _, f := range zipReader.File {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open zip file entry %s: %v", f.Name, err)
			}
			content, _ := io.ReadAll(rc)
			_ = rc.Close()
			extracted[f.Name] = string(content)
		}

		// Checks
		if extracted["file_a.txt"] != "file1 content" {
			t.Errorf("file_a.txt content incorrect: %q", extracted["file_a.txt"])
		}
		if extracted["file_b.txt"] != "file2 content" {
			t.Errorf("file_b.txt content incorrect: %q", extracted["file_b.txt"])
		}
		// The second file_a.txt should be renamed to file_a_1.txt
		if extracted["file_a_1.txt"] != "duplicate name file content" {
			t.Errorf("file_a_1.txt content incorrect: %q. Extracted files: %v", extracted["file_a_1.txt"], extracted)
		}

		t.Logf("Successfully unzipped batch. Extracted files: %+v", extracted)
	})
}

func TestUploadAccessModeAccessControl(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	user1 := &model.User{ID: 1001, Username: "user1"}
	user2 := &model.User{ID: 1002, Username: "user2"}

	// Seed user1
	if err := dbConn.Create(user1).Error; err != nil {
		t.Fatalf("create test user1 failed: %v", err)
	}
	// Seed user2
	if err := dbConn.Create(user2).Error; err != nil {
		t.Fatalf("create test user2 failed: %v", err)
	}

	router := setupTestRouter(user1)

	// 1. Upload private file for user1 (explicitly specifying access_mode = 0)
	imgContent := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
	contentType, body := createMultipartRequest(t, "file", "private.png", imgContent, map[string]string{
		"type":        "generic",
		"access_mode": "0",
	})
	req, _ := http.NewRequest("POST", "/api/v1/upload", body)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Upload failed: %d, %s", w.Code, w.Body.String())
	}
	t.Logf("Raw upload response: %s", w.Body.String())
	var resp1 testResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp1)
	var upload1 model.Upload
	_ = json.Unmarshal(resp1.Data, &upload1)

	if upload1.AccessMode != 0 {
		t.Errorf("expected access_mode 0, got %d", upload1.AccessMode)
	}

	// 2. Upload public file for user1 (type avatar, should default to public 1)
	contentType2, body2 := createMultipartRequest(t, "file", "public.png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89"), map[string]string{
		"type": "avatar",
	})
	req2, _ := http.NewRequest("POST", "/api/v1/upload", body2)
	req2.Header.Set("Content-Type", contentType2)

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	var resp2 testResponse
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	var upload2 model.Upload
	_ = json.Unmarshal(resp2.Data, &upload2)

	if upload2.AccessMode != 1 {
		t.Errorf("expected access_mode 1 (public) for avatar, got %d", upload2.AccessMode)
	}

	// 3. Verify accessing private file as user1 (owner) succeeds
	wAccessOwner := httptest.NewRecorder()
	reqAccessOwner, _ := http.NewRequest("GET", "/api/v1/admin/uploads/download/"+strconv.FormatUint(upload1.ID, 10), nil)
	router.ServeHTTP(wAccessOwner, reqAccessOwner)
	if wAccessOwner.Code != http.StatusOK {
		t.Errorf("owner should be allowed to download private file, got status %d", wAccessOwner.Code)
	}

	// 4. Verify accessing private file as user2 (non-owner) fails
	routerUser2 := setupTestRouter(user2)
	wAccessOther := httptest.NewRecorder()
	reqAccessOther, _ := http.NewRequest("GET", "/api/v1/admin/uploads/download/"+strconv.FormatUint(upload1.ID, 10), nil)
	routerUser2.ServeHTTP(wAccessOther, reqAccessOther)
	if wAccessOther.Code != http.StatusUnauthorized {
		t.Errorf("non-owner should be denied download of private file, got status %d, want 401", wAccessOther.Code)
	}

	// 5. Verify accessing public file as user2 (non-owner) succeeds
	wAccessPublic := httptest.NewRecorder()
	reqAccessPublic, _ := http.NewRequest("GET", "/api/v1/admin/uploads/download/"+strconv.FormatUint(upload2.ID, 10), nil)
	routerUser2.ServeHTTP(wAccessPublic, reqAccessPublic)
	if wAccessPublic.Code != http.StatusOK {
		t.Errorf("any logged-in user should be allowed to download public file, got status %d", wAccessPublic.Code)
	}
}

func TestGetFileStats(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	authUser := &model.User{ID: 1001, Username: "test_user"}
	router := setupTestRouter(authUser)

	// Insert some dummy uploads
	uploads := []model.Upload{
		{
			ID:        3101,
			UserID:    authUser.ID,
			FileName:  "photo.png",
			FilePath:  "photo.png",
			FileSize:  100,
			MimeType:  "image/png",
			Extension: "png",
			Type:      "generic",
			Status:    model.UploadStatusUsed,
			CreatedAt: time.Now(),
		},
		{
			ID:        3102,
			UserID:    authUser.ID,
			FileName:  "video.mp4",
			FilePath:  "video.mp4",
			FileSize:  500,
			MimeType:  "video/mp4",
			Extension: "mp4",
			Type:      "generic",
			Status:    model.UploadStatusUsed,
			CreatedAt: time.Now().AddDate(0, 0, -2), // 2 days ago
		},
		{
			ID:        3103,
			UserID:    authUser.ID,
			FileName:  "document.pdf",
			FilePath:  "document.pdf",
			FileSize:  200,
			MimeType:  "application/pdf",
			Extension: "pdf",
			Type:      "avatar", // different type
			Status:    model.UploadStatusUsed,
			CreatedAt: time.Now().AddDate(0, 0, -10), // older than 7 days
		},
	}

	for i := range uploads {
		if err := dbConn.Create(&uploads[i]).Error; err != nil {
			t.Fatalf("failed to create upload: %v", err)
		}
	}
	if err := uploadstats.RebuildUploadStats(context.Background()); err != nil {
		t.Fatalf("failed to rebuild upload stats: %v", err)
	}

	req, _ := http.NewRequest("GET", "/api/v1/admin/uploads/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ErrorMsg string            `json:"error_msg"`
		Data     fileStatsResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ErrorMsg != "" {
		t.Fatalf("expected no error, got: %s", resp.ErrorMsg)
	}

	// Verify total count and size
	if resp.Data.TotalCount != 3 {
		t.Errorf("expected 3 total files, got %d", resp.Data.TotalCount)
	}
	if resp.Data.TotalSize != 800 {
		t.Errorf("expected 800 total size, got %d", resp.Data.TotalSize)
	}

	// Verify trend (last 7 days should include photo.png (100) and video.mp4 (500), but NOT pdf (older))
	// Total size in trend should be 600
	var trendSizeSum int64
	for _, trendItem := range resp.Data.Trend {
		trendSizeSum += trendItem.Size
	}
	if trendSizeSum != 600 {
		t.Errorf("expected 7-day trend size sum to be 600, got %d", trendSizeSum)
	}

	// Verify categories
	categoryMap := make(map[string]int64)
	for _, cat := range resp.Data.Categories {
		categoryMap[cat.Name] = cat.Count
	}
	if categoryMap["图片"] != 1 {
		t.Errorf("expected 1 image category, got %d", categoryMap["图片"])
	}
	if categoryMap["视频"] != 1 {
		t.Errorf("expected 1 video category, got %d", categoryMap["视频"])
	}
	if categoryMap["文档"] != 1 {
		t.Errorf("expected 1 document category, got %d", categoryMap["文档"])
	}
}

func TestUserUploadManagement(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	user1 := &model.User{ID: 1001, Username: "user1"}
	user2 := &model.User{ID: 1002, Username: "user2"}

	_ = dbConn.Create(user1)
	_ = dbConn.Create(user2)

	router1 := setupTestRouter(user1)
	router2 := setupTestRouter(user2)

	// Seed upload records
	upload1 := model.Upload{
		ID:        4001,
		UserID:    1001,
		FileName:  "user1-file.txt",
		FilePath:  "user1-file.txt",
		FileSize:  100,
		MimeType:  "text/plain",
		Extension: "txt",
		Status:    model.UploadStatusUsed,
		CreatedAt: time.Now(),
	}
	upload2 := model.Upload{
		ID:        4002,
		UserID:    1002,
		FileName:  "user2-file.png",
		FilePath:  "user2-file.png",
		FileSize:  200,
		MimeType:  "image/png",
		Extension: "png",
		Status:    model.UploadStatusUsed,
		CreatedAt: time.Now(),
	}

	_ = dbConn.Create(&upload1)
	_ = dbConn.Create(&upload2)

	t.Run("ListMyFiles only returns own files", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/upload/my", nil)
		w := httptest.NewRecorder()
		router1.ServeHTTP(w, req)

		var resp struct {
			ErrorMsg string              `json:"error_msg"`
			Data     listMyFilesResponse `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		if resp.ErrorMsg != "" {
			t.Fatalf("ListMyFiles error: %s", resp.ErrorMsg)
		}
		if resp.Data.Total != 1 {
			t.Errorf("expected 1 file for user1, got %d", resp.Data.Total)
		}
		if len(resp.Data.Items) != 1 || resp.Data.Items[0].ID != 4001 {
			t.Errorf("expected file 4001, got items: %+v", resp.Data.Items)
		}
	})

	t.Run("UpdateMyFile updates file name and access mode successfully", func(t *testing.T) {
		newMode := 1
		reqBody, _ := json.Marshal(updateMyFileRequest{
			FileName:   "renamed.txt",
			AccessMode: &newMode,
		})
		req, _ := http.NewRequest("PUT", "/api/v1/upload/4001", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router1.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var updated model.Upload
		dbConn.First(&updated, 4001)
		if updated.FileName != "renamed.txt" {
			t.Errorf("expected file name renamed.txt, got %s", updated.FileName)
		}
		if updated.AccessMode != 1 {
			t.Errorf("expected access mode 1, got %d", updated.AccessMode)
		}
	})

	t.Run("UpdateMyFile blocks non-owners", func(t *testing.T) {
		reqBody, _ := json.Marshal(updateMyFileRequest{
			FileName: "hack.txt",
		})
		req, _ := http.NewRequest("PUT", "/api/v1/upload/4001", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router2.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("DeleteMyFile blocks non-owners", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/upload/4001", nil)
		w := httptest.NewRecorder()
		router2.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("DeleteMyFile deletes file successfully", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/upload/4001", nil)
		w := httptest.NewRecorder()
		router1.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var deleted model.Upload
		dbConn.First(&deleted, 4001)
		if deleted.Status != model.UploadStatusDeleted {
			t.Errorf("expected status deleted, got %s", deleted.Status)
		}
	})
}

func TestDeleteReservedUploadType(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	authUser := &model.User{ID: 1001, Username: "reserved_owner"}
	router := setupTestRouter(authUser)
	reserved := model.Upload{
		ID:        4101,
		UserID:    authUser.ID,
		FileName:  "pages.zip",
		FilePath:  "uploads/pages.zip",
		FileSize:  128,
		MimeType:  "application/zip",
		Extension: "zip",
		Hash:      "pages-reserved-hash",
		Type:      shared.ReservedPagesDeploymentType,
		Status:    model.UploadStatusUsed,
		CreatedAt: time.Now(),
	}
	if err := dbConn.Create(&reserved).Error; err != nil {
		t.Fatalf("seed reserved upload: %v", err)
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "admin delete", path: "/api/v1/admin/uploads/4101"},
		{name: "owner delete", path: "/api/v1/upload/4101"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodDelete, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusConflict {
				t.Fatalf("expected status 409, got %d. Body: %s", w.Code, w.Body.String())
			}
			var resp testResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal delete response: %v", err)
			}
			if resp.ErrorMsg != shared.ErrReservedUploadType {
				t.Fatalf("reserved delete error = %q, want %q", resp.ErrorMsg, shared.ErrReservedUploadType)
			}
		})
	}

	var persisted model.Upload
	if err := dbConn.First(&persisted, reserved.ID).Error; err != nil {
		t.Fatalf("reload reserved upload: %v", err)
	}
	if persisted.Status != model.UploadStatusUsed {
		t.Fatalf("reserved upload status = %s, want used", persisted.Status)
	}
}

func configureLocalStorageRoot(t *testing.T, dbConn *gorm.DB, tempDir string) {
	var sc model.SystemConfig
	if err := dbConn.Where("key = ?", model.ConfigKeyStorageConfig).First(&sc).Error; err != nil {
		t.Fatalf("failed to find storage config: %v", err)
	}
	var cfg storage.Config
	if err := json.Unmarshal([]byte(sc.Value), &cfg); err != nil {
		t.Fatalf("failed to unmarshal storage config: %v", err)
	}
	cfg.Local.Root = tempDir
	newVal, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal storage config: %v", err)
	}
	sc.Value = string(newVal)
	if err := dbConn.Save(&sc).Error; err != nil {
		t.Fatalf("failed to save storage config: %v", err)
	}
	_ = db.HSetJSON(context.Background(), repository.SystemConfigRedisHashKey, sc.Key, &sc)
	repository.ResetSystemConfigRAMCacheForTest()
	storage.ResetCache()
}
