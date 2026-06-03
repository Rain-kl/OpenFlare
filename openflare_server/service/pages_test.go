package service

import (
	"archive/zip"
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"openflare/model"
	"strings"
	"testing"
)

func TestPagesUploadActivateAndPublishStaticRoute(t *testing.T) {
	setupServiceTestDB(t)

	project, err := CreatePagesProject(PagesProjectInput{
		Name:               "Marketing Site",
		Slug:               "marketing-site",
		Enabled:            true,
		SPAFallbackEnabled: true,
		SPAFallbackPath:    "/app.html",
	})
	if err != nil {
		t.Fatalf("CreatePagesProject failed: %v", err)
	}
	uploadHeader := multipartFileHeader(t, "site.zip", testPagesZip(t, map[string]string{
		"index.html":       "<h1>Hello Pages</h1>",
		"assets/app.js":    "console.log('pages')",
		"assets/style.css": "body{color:#111}",
	}))
	deployment, err := UploadPagesDeployment(project.ID, uploadHeader, "index.html", "root")
	if err != nil {
		t.Fatalf("UploadPagesDeployment failed: %v", err)
	}
	if deployment.FileCount != 3 || deployment.TotalSize == 0 {
		t.Fatalf("unexpected deployment manifest: %+v", deployment)
	}
	project, err = ActivatePagesDeployment(project.ID, deployment.ID)
	if err != nil {
		t.Fatalf("ActivatePagesDeployment failed: %v", err)
	}
	if project.ActiveDeploymentID == nil || *project.ActiveDeploymentID != deployment.ID {
		t.Fatalf("expected active deployment %d, got %+v", deployment.ID, project.ActiveDeploymentID)
	}

	route, err := CreateProxyRoute(ProxyRouteInput{
		Domain:         "pages.example.com",
		Enabled:        true,
		UpstreamType:   "pages",
		PagesProjectID: &project.ID,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	if route.UpstreamType != "pages" || route.PagesProjectID == nil || *route.PagesProjectID != project.ID {
		t.Fatalf("expected route to bind Pages project, got %+v", route)
	}

	result, err := PublishConfigVersion("root", false)
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"upstream_type":"pages"`) {
		t.Fatalf("expected snapshot to include pages route, got %s", result.Version.SnapshotJSON)
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"deployment_id":`) {
		t.Fatalf("expected snapshot to include pages deployment, got %s", result.Version.SnapshotJSON)
	}
	if !strings.Contains(result.Version.RenderedConfig, "root \"__OPENFLARE_PAGES_DIR__/deployments/") {
		t.Fatalf("expected rendered config to use pages dir placeholder, got:\n%s", result.Version.RenderedConfig)
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"spa_fallback_path":"/app.html"`) {
		t.Fatalf("expected snapshot to include custom SPA fallback path, got %s", result.Version.SnapshotJSON)
	}
	if !strings.Contains(result.Version.RenderedConfig, "try_files $uri $uri/ /app.html;") {
		t.Fatalf("expected SPA fallback try_files, got:\n%s", result.Version.RenderedConfig)
	}
	if strings.Contains(result.Version.RenderedConfig, "proxy_pass") {
		t.Fatalf("Pages route must not render proxy_pass, got:\n%s", result.Version.RenderedConfig)
	}
}

func TestPagesProjectRejectsUnsafeFallbackPath(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreatePagesProject(PagesProjectInput{
		Name:               "Unsafe Fallback",
		Slug:               "unsafe-fallback",
		Enabled:            true,
		SPAFallbackEnabled: true,
		SPAFallbackPath:    "/index.html; proxy_pass http://evil",
	})
	if err == nil || !strings.Contains(err.Error(), "回退路径") {
		t.Fatalf("expected unsafe SPA fallback path rejection, got %v", err)
	}
}

func TestUploadPagesDeploymentRejectsZipSlip(t *testing.T) {
	setupServiceTestDB(t)

	project, err := CreatePagesProject(PagesProjectInput{
		Name:    "Unsafe Site",
		Slug:    "unsafe-site",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreatePagesProject failed: %v", err)
	}
	_, err = UploadPagesDeployment(project.ID, multipartFileHeader(t, "bad.zip", testPagesZip(t, map[string]string{
		"../escape.html": "bad",
		"index.html":     "ok",
	})), "index.html", "root")
	if err == nil || !strings.Contains(err.Error(), "逃逸目录") {
		t.Fatalf("expected zip-slip rejection, got %v", err)
	}
}

func TestPagesRouteRequiresActiveDeployment(t *testing.T) {
	setupServiceTestDB(t)

	project, err := CreatePagesProject(PagesProjectInput{
		Name:    "Draft Site",
		Slug:    "draft-site",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreatePagesProject failed: %v", err)
	}
	if _, err = CreateProxyRoute(ProxyRouteInput{
		Domain:         "draft.example.com",
		Enabled:        true,
		UpstreamType:   "pages",
		PagesProjectID: &project.ID,
	}); err == nil || !strings.Contains(err.Error(), "没有激活部署") {
		t.Fatalf("expected active deployment validation, got %v", err)
	}
}

func TestPagesDeploymentPackageRequiresActiveConfigSnapshot(t *testing.T) {
	setupServiceTestDB(t)

	project, err := CreatePagesProject(PagesProjectInput{Name: "Published Site", Slug: "published-site", Enabled: true})
	if err != nil {
		t.Fatalf("CreatePagesProject failed: %v", err)
	}
	deployment, err := UploadPagesDeployment(project.ID, multipartFileHeader(t, "site.zip", testPagesZip(t, map[string]string{
		"index.html": "ok",
	})), "index.html", "root")
	if err != nil {
		t.Fatalf("UploadPagesDeployment failed: %v", err)
	}
	if _, err = ActivatePagesDeployment(project.ID, deployment.ID); err != nil {
		t.Fatalf("ActivatePagesDeployment failed: %v", err)
	}
	if _, _, err = GetPagesDeploymentPackagePath(deployment.ID); err == nil || !strings.Contains(err.Error(), "激活配置") {
		t.Fatalf("expected package download to require active config, got %v", err)
	}
	if _, err = CreateProxyRoute(ProxyRouteInput{
		Domain:         "published.example.com",
		Enabled:        true,
		UpstreamType:   "pages",
		PagesProjectID: &project.ID,
	}); err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	if _, err = PublishConfigVersion("root", false); err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	filePath, fileName, err := GetPagesDeploymentPackagePath(deployment.ID)
	if err != nil {
		t.Fatalf("GetPagesDeploymentPackagePath failed after publish: %v", err)
	}
	if filePath == "" || fileName == "" {
		t.Fatalf("expected package path and file name, got path=%q name=%q", filePath, fileName)
	}
}

func testPagesZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry failed: %v", err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry failed: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buffer.Bytes()
}

func multipartFileHeader(t *testing.T, fileName string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("package", fileName)
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err = part.Write(content); err != nil {
		t.Fatalf("write multipart file failed: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}
	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err = req.ParseMultipartForm(int64(len(content)) + 1024); err != nil {
		t.Fatalf("ParseMultipartForm failed: %v", err)
	}
	file, header, err := req.FormFile("package")
	if err != nil {
		t.Fatalf("FormFile failed: %v", err)
	}
	file.Close()
	return header
}

func TestDeletePagesDeploymentRejectsActiveDeployment(t *testing.T) {
	setupServiceTestDB(t)

	project, err := CreatePagesProject(PagesProjectInput{Name: "Active", Slug: "active", Enabled: true})
	if err != nil {
		t.Fatalf("CreatePagesProject failed: %v", err)
	}
	deployment, err := UploadPagesDeployment(project.ID, multipartFileHeader(t, "site.zip", testPagesZip(t, map[string]string{"index.html": "ok"})), "index.html", "root")
	if err != nil {
		t.Fatalf("UploadPagesDeployment failed: %v", err)
	}
	if _, err = ActivatePagesDeployment(project.ID, deployment.ID); err != nil {
		t.Fatalf("ActivatePagesDeployment failed: %v", err)
	}
	if err = DeletePagesDeployment(project.ID, deployment.ID); err == nil {
		t.Fatal("expected active deployment deletion to fail")
	}
	var stored model.PagesDeployment
	if err = model.DB.First(&stored, deployment.ID).Error; err != nil {
		t.Fatalf("expected active deployment to remain: %v", err)
	}
}

func TestUploadPagesDeploymentWithTopLevelFolder(t *testing.T) {
	setupServiceTestDB(t)

	project, err := CreatePagesProject(PagesProjectInput{
		Name:    "Folder Site",
		Slug:    "folder-site",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreatePagesProject failed: %v", err)
	}
	// Upload a zip with all files inside a top-level directory "Speed-Test-source/"
	uploadHeader := multipartFileHeader(t, "site.zip", testPagesZip(t, map[string]string{
		"Speed-Test-source/index.html":    "<h1>Hello Pages</h1>",
		"Speed-Test-source/assets/app.js": "console.log('pages')",
	}))
	deployment, err := UploadPagesDeployment(project.ID, uploadHeader, "index.html", "root")
	if err != nil {
		t.Fatalf("UploadPagesDeployment with folder failed: %v", err)
	}
	if deployment.FileCount != 2 {
		t.Fatalf("expected 2 files, got %d", deployment.FileCount)
	}
	if deployment.EntryFile != "index.html" {
		t.Fatalf("expected EntryFile to be index.html, got %q", deployment.EntryFile)
	}
}

func TestPagesProjectAPIProxyValidation(t *testing.T) {
	setupServiceTestDB(t)

	// 1. Invalid configuration: enabled but empty fields
	_, err := CreatePagesProject(PagesProjectInput{
		Name:            "API Proxy 1",
		Enabled:         true,
		APIProxyEnabled: true,
	})
	if err == nil || !strings.Contains(err.Error(), "匹配路径不能为空") {
		t.Fatalf("expected error for empty match path, got: %v", err)
	}

	// 2. Invalid path: must start with '/'
	_, err = CreatePagesProject(PagesProjectInput{
		Name:            "API Proxy 2",
		Enabled:         true,
		APIProxyEnabled: true,
		APIProxyPath:    "api",
		APIProxyPass:    "http://127.0.0.1:8080",
	})
	if err == nil || !strings.Contains(err.Error(), "必须以 '/' 开头") {
		t.Fatalf("expected error for path not starting with /, got: %v", err)
	}

	// 3. Invalid target URL
	_, err = CreatePagesProject(PagesProjectInput{
		Name:            "API Proxy 3",
		Enabled:         true,
		APIProxyEnabled: true,
		APIProxyPath:    "/api",
		APIProxyPass:    "127.0.0.1:8080",
	})
	if err == nil || !strings.Contains(err.Error(), "有效的 HTTP/HTTPS URL") {
		t.Fatalf("expected error for invalid pass URL, got: %v", err)
	}

	// 4. Valid configuration
	project, err := CreatePagesProject(PagesProjectInput{
		Name:            "API Proxy Valid",
		Enabled:         true,
		APIProxyEnabled: true,
		APIProxyPath:    "/api",
		APIProxyPass:    "http://127.0.0.1:8080",
		APIProxyRewrite: "/",
	})
	if err != nil {
		t.Fatalf("unexpected error creating valid project: %v", err)
	}
	if !project.APIProxyEnabled || project.APIProxyPath != "/api" || project.APIProxyPass != "http://127.0.0.1:8080" || project.APIProxyRewrite != "/" {
		t.Fatalf("unexpected project state: %+v", project)
	}
}
