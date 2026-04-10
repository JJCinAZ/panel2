package labelserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjcinaz/panel2/labelrender"
)

func TestResolveTemplatePathRejectsTraversal(t *testing.T) {
	var err error
	var root string
	var templatePath string

	if root, err = os.MkdirTemp("", "labelserver-root-*"); err != nil {
		t.Fatalf("create temp root: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(root)
	}()

	if templatePath, err = resolveTemplatePath(root, "../secret.html"); err == nil {
		t.Fatalf("expected traversal error, got path %q", templatePath)
	}
	if !errors.Is(err, errTemplatePath) {
		t.Fatalf("expected errTemplatePath, got %v", err)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	var err error
	var root string
	var handler *Handler
	var req *http.Request
	var rec *httptest.ResponseRecorder

	if root, err = os.MkdirTemp("", "labelserver-root-*"); err != nil {
		t.Fatalf("create temp root: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(root)
	}()

	if handler, err = newHandler(root, func(_ string, _ labelrender.Options) ([]byte, error) {
		return []byte("unused"), nil
	}); err != nil {
		t.Fatalf("newHandler failed: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/render", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("expected Allow=POST, got %q", got)
	}
}

func TestHandlerTemplateNotFound(t *testing.T) {
	var err error
	var root string
	var handler *Handler
	var req *http.Request
	var rec *httptest.ResponseRecorder
	var response map[string]string

	if root, err = os.MkdirTemp("", "labelserver-root-*"); err != nil {
		t.Fatalf("create temp root: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(root)
	}()

	if handler, err = newHandler(root, func(_ string, _ labelrender.Options) ([]byte, error) {
		return []byte("unused"), nil
	}); err != nil {
		t.Fatalf("newHandler failed: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(`{"template":"missing.tmpl"}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if err = json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if !strings.Contains(response["error"], "template not found") {
		t.Fatalf("expected template not found error, got %q", response["error"])
	}
}

func TestHandlerInvalidOption(t *testing.T) {
	var err error
	var root string
	var template string
	var handler *Handler
	var req *http.Request
	var rec *httptest.ResponseRecorder
	var response map[string]string

	if root, err = os.MkdirTemp("", "labelserver-root-*"); err != nil {
		t.Fatalf("create temp root: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(root)
	}()

	template = filepath.Join(root, "label.tmpl")
	if err = os.WriteFile(template, []byte("<h1>ok</h1>"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	if handler, err = newHandler(root, func(_ string, _ labelrender.Options) ([]byte, error) {
		return []byte("unused"), nil
	}); err != nil {
		t.Fatalf("newHandler failed: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(`{
		"template":"label.tmpl",
		"options":{"threshold":999}
	}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	if err = json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if !strings.Contains(response["error"], "threshold") {
		t.Fatalf("expected threshold validation message, got %q", response["error"])
	}
}

func TestHandlerRenderSuccess(t *testing.T) {
	var err error
	var root string
	var templatePath string
	var handler *Handler
	var req *http.Request
	var rec *httptest.ResponseRecorder
	var gotTemplatePath string
	var gotOptions labelrender.Options
	var data map[string]interface{}
	var ok bool

	if root, err = os.MkdirTemp("", "labelserver-root-*"); err != nil {
		t.Fatalf("create temp root: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(root)
	}()

	templatePath = filepath.Join(root, "architecturalLabel.tmpl")
	if err = os.WriteFile(templatePath, []byte("<h1>{{ .TenantName }}</h1>"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	if handler, err = newHandler(root, func(path string, opts labelrender.Options) ([]byte, error) {
		gotTemplatePath = path
		gotOptions = opts
		return []byte("PNGDATA"), nil
	}); err != nil {
		t.Fatalf("newHandler failed: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(`{
		"template":"architecturalLabel.tmpl",
		"options":{
			"engine":"firefox",
			"width":1024,
			"height":512,
			"threshold":150,
			"wait_ms":7000,
			"timeout_sec":60
		},
		"data":{
			"TenantName":"BJM SABINO, LLC",
			"SuiteNumber":"126"
		}
	}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected image/png content type, got %q", got)
	}
	if rec.Body.String() != "PNGDATA" {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
	if gotTemplatePath != templatePath {
		t.Fatalf("expected template path %q, got %q", templatePath, gotTemplatePath)
	}
	if gotOptions.Engine != "firefox" {
		t.Fatalf("expected engine firefox, got %q", gotOptions.Engine)
	}
	if gotOptions.Width != 1024 {
		t.Fatalf("expected width 1024, got %d", gotOptions.Width)
	}
	if gotOptions.Height != 512 {
		t.Fatalf("expected height 512, got %d", gotOptions.Height)
	}
	if gotOptions.Threshold != 150 {
		t.Fatalf("expected threshold 150, got %d", gotOptions.Threshold)
	}
	if gotOptions.VirtualTimeBudget.Milliseconds() != 7000 {
		t.Fatalf("expected wait_ms 7000, got %d", gotOptions.VirtualTimeBudget.Milliseconds())
	}
	if gotOptions.ChromeTimeout.Seconds() != 60 {
		t.Fatalf("expected timeout_sec 60, got %.0f", gotOptions.ChromeTimeout.Seconds())
	}

	if data, ok = gotOptions.TemplateData.(map[string]interface{}); !ok {
		t.Fatalf("expected template data map, got %T", gotOptions.TemplateData)
	}
	if data["TenantName"] != "BJM SABINO, LLC" {
		t.Fatalf("expected TenantName value, got %#v", data["TenantName"])
	}
	if data["SuiteNumber"] != "126" {
		t.Fatalf("expected SuiteNumber value, got %#v", data["SuiteNumber"])
	}
}
