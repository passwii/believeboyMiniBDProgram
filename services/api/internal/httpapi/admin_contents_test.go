package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/adminauth"
	"github.com/believeboy/mini-bd-program/services/api/internal/content"
	"github.com/believeboy/mini-bd-program/services/api/internal/storage"
)

const contentTestAdminID = "55555555-5555-4555-8555-555555555555"

type fakeContentRepository struct {
	created []content.PersistInput
	records []content.Record
	get     content.Record
	getErr  error
}

func (f *fakeContentRepository) Create(_ context.Context, input content.PersistInput) (content.Record, error) {
	f.created = append(f.created, input)
	return content.Record{
		ID:            input.ContentID,
		Title:         input.Title,
		Status:        "ACTIVE",
		VersionID:     input.VersionID,
		VersionNo:     1,
		SHA256:        input.SHA256,
		ByteSize:      input.ByteSize,
		AnalysisNotes: input.AnalysisNotes,
		CreatedAt:     time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (f *fakeContentRepository) List(context.Context, int32, int32) ([]content.Record, error) {
	return f.records, nil
}

func (f *fakeContentRepository) Get(context.Context, uuid.UUID) (content.Record, error) {
	return f.get, f.getErr
}

func newTestRouterWithContent(t *testing.T) (http.Handler, *fakeContentRepository) {
	t.Helper()
	adminStore := newMemoryAdminStore(t)
	adminService := adminauth.New(adminStore, 12*time.Hour)
	repository := &fakeContentRepository{}
	contentService := content.NewService(repository, storage.NewMemoryStore(), 20<<20)
	router := NewRouter(Dependencies{
		AdminAuth:          adminService,
		AdminCookieName:    "bb_admin_session",
		AdminSessionTTL:    12 * time.Hour,
		AllowedAdminOrigin: httpTestOrigin,
		Content:            contentService,
		MaxUploadBytes:     20 << 20,
	})
	return router, repository
}

func authCookie(t *testing.T, router http.Handler) *http.Cookie {
	t.Helper()
	login := loginRequest(t, router)
	if login.Code != http.StatusNoContent {
		t.Fatalf("login status = %d", login.Code)
	}
	return login.Result().Cookies()[0]
}

func multipartFile(t *testing.T, filename, fieldName string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &body, writer.FormDataContentType()
}

func uploadRequest(t *testing.T, router http.Handler, cookie *http.Cookie, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/contents", body)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Origin", httpTestOrigin)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func validUploadHTML() []byte {
	return []byte(`<!doctype html><html><head><title>Uploaded</title><meta name="viewport" content="width=device-width"></head><body><script>1</script></body></html>`)
}

func TestContentUploadRequiresAdminSession(t *testing.T) {
	router, _ := newTestRouterWithContent(t)
	body, contentType := multipartFile(t, "proposal.html", "file", validUploadHTML())

	response := uploadRequest(t, router, nil, body, contentType)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestContentUploadCreatesVersion(t *testing.T) {
	router, repository := newTestRouterWithContent(t)
	cookie := authCookie(t, router)
	body, contentType := multipartFile(t, "proposal.html", "file", validUploadHTML())

	response := uploadRequest(t, router, cookie, body, contentType)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["title"] != "Uploaded" {
		t.Fatalf("title = %v", payload["title"])
	}
	if payload["version_no"] != float64(1) {
		t.Fatalf("version_no = %v", payload["version_no"])
	}
	sha, _ := payload["sha256"].(string)
	if len(sha) != 64 {
		t.Fatalf("sha256 = %q", sha)
	}
	if _, exists := payload["object_key"]; exists {
		t.Fatal("response leaked object_key")
	}
	if len(repository.created) != 1 {
		t.Fatalf("repository calls = %d", len(repository.created))
	}
}

func TestContentUploadRejectsMissingFile(t *testing.T) {
	router, _ := newTestRouterWithContent(t)
	cookie := authCookie(t, router)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("note", "no file"); err != nil {
		t.Fatalf("WriteField() error = %v", err)
	}
	_ = writer.Close()

	response := uploadRequest(t, router, cookie, &body, writer.FormDataContentType())

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestContentUploadRejectsOversizedFile(t *testing.T) {
	router, _ := newTestRouterWithContent(t)
	cookie := authCookie(t, router)
	body, contentType := multipartFile(t, "large.html", "file", bytes.Repeat([]byte("x"), (20<<20)+1))

	response := uploadRequest(t, router, cookie, body, contentType)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestContentUploadRejectsExternalResources(t *testing.T) {
	router, _ := newTestRouterWithContent(t)
	cookie := authCookie(t, router)
	body, contentType := multipartFile(
		t,
		"external.html",
		"file",
		[]byte(`<html><head><title>x</title><script src="https://cdn.example/app.js"></script></head></html>`),
	)

	response := uploadRequest(t, router, cookie, body, contentType)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestContentListAndDetail(t *testing.T) {
	router, repository := newTestRouterWithContent(t)
	cookie := authCookie(t, router)
	id := uuid.New()
	repository.records = []content.Record{{
		ID:        id,
		Title:     "Listed",
		Status:    "ACTIVE",
		VersionID: uuid.New(),
		VersionNo: 1,
		ByteSize:  42,
		CreatedAt: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC),
	}}
	repository.get = content.Record{
		ID:            id,
		Title:         "Detail",
		Status:        "ACTIVE",
		VersionID:     uuid.New(),
		VersionNo:     1,
		ObjectKey:     "must-not-leak",
		ByteSize:      100,
		AnalysisNotes: []byte(`{"viewport_present":true,"script_count":2,"external_resources":[],"warnings":[]}`),
		CreatedAt:     time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC),
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/contents", nil)
	listRequest.AddCookie(cookie)
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(listResponse.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0]["title"] != "Listed" {
		t.Fatalf("list = %v", list)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/contents/"+id.String(), nil)
	detailRequest.AddCookie(cookie)
	detailResponse := httptest.NewRecorder()
	router.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", detailResponse.Code, detailResponse.Body.String())
	}
	var detail map[string]any
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail["title"] != "Detail" {
		t.Fatalf("detail = %v", detail)
	}
	if strings.Contains(detailResponse.Body.String(), "must-not-leak") {
		t.Fatal("detail leaked object key")
	}
}
