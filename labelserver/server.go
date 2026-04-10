package labelserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jjcinaz/panel2/labelrender"
)

const maxRequestBodyBytes = 1 << 20

var errInvalidJSON = errors.New("invalid json body")
var errInvalidOption = errors.New("invalid render option")
var errTemplateRequired = errors.New("template is required")
var errTemplatePath = errors.New("invalid template path")
var errTemplateNotFound = errors.New("template not found")
var errTemplateDirectory = errors.New("template path is a directory")

// RenderRequest is the JSON payload accepted by POST /render.
type RenderRequest struct {
	Template string                 `json:"template"`
	Options  RenderRequestOptions   `json:"options"`
	Data     map[string]interface{} `json:"data"`
}

// RenderRequestOptions controls browser rendering and monochrome conversion options.
type RenderRequestOptions struct {
	Width              *int     `json:"width,omitempty"`
	Height             *int     `json:"height,omitempty"`
	Engine             string   `json:"engine,omitempty"`
	ChromePath         string   `json:"chrome_path,omitempty"`
	FirefoxPath        string   `json:"firefox_path,omitempty"`
	CaptureExtraHeight *int     `json:"capture_extra_height,omitempty"`
	WaitMS             *int     `json:"wait_ms,omitempty"`
	TimeoutSec         *int     `json:"timeout_sec,omitempty"`
	Threshold          *int     `json:"threshold,omitempty"`
	Gamma              *float64 `json:"gamma,omitempty"`
	Contrast           *float64 `json:"contrast,omitempty"`
	KeepTemp           *bool    `json:"keep_temp,omitempty"`
}

// Handler serves the render API.
type Handler struct {
	templateRoot string
	renderPNG    func(string, labelrender.Options) ([]byte, error)
}

func NewHandler(templateRoot string) (*Handler, error) {
	return newHandler(templateRoot, labelrender.RenderFileTo1BitPNGBytes)
}

func newHandler(templateRoot string, renderPNG func(string, labelrender.Options) ([]byte, error)) (*Handler, error) {
	var err error
	var absRoot string
	var stat os.FileInfo

	if absRoot, err = filepath.Abs(templateRoot); err != nil {
		return nil, fmt.Errorf("resolve template root: %w", err)
	}
	if stat, err = os.Stat(absRoot); err != nil {
		return nil, fmt.Errorf("stat template root: %w", err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("template root %q is not a directory", absRoot)
	}
	if renderPNG == nil {
		return nil, errors.New("render function cannot be nil")
	}

	return &Handler{
		templateRoot: absRoot,
		renderPNG:    renderPNG,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var req RenderRequest
	var templatePath string
	var renderOpts labelrender.Options
	var png []byte

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSONError(w, http.StatusMethodNotAllowed, errors.New("method not allowed; use POST"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if req, err = decodeRenderRequest(r); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	if templatePath, err = resolveTemplatePath(h.templateRoot, req.Template); err != nil {
		switch {
		case errors.Is(err, errTemplateRequired):
			writeJSONError(w, http.StatusBadRequest, err)
			return
		case errors.Is(err, errTemplatePath):
			writeJSONError(w, http.StatusBadRequest, err)
			return
		case errors.Is(err, errTemplateNotFound):
			writeJSONError(w, http.StatusNotFound, err)
			return
		case errors.Is(err, errTemplateDirectory):
			writeJSONError(w, http.StatusBadRequest, err)
			return
		default:
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
	}

	if renderOpts, err = buildRenderOptions(req.Options); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	renderOpts.TemplateData = req.Data

	if png, err = h.renderPNG(templatePath, renderOpts); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("render failed: %w", err))
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	if _, err = w.Write(png); err != nil {
		return
	}
}

func decodeRenderRequest(r *http.Request) (RenderRequest, error) {
	var err error
	var req RenderRequest
	var decoder *json.Decoder
	var extra interface{}
	var maxBytesErr *http.MaxBytesError

	decoder = json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err = decoder.Decode(&req); err != nil {
		if errors.As(err, &maxBytesErr) {
			return req, fmt.Errorf("%w: request exceeds %d bytes", errInvalidJSON, maxRequestBodyBytes)
		}
		return req, fmt.Errorf("%w: %v", errInvalidJSON, err)
	}
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return req, fmt.Errorf("%w: request body must contain a single JSON object", errInvalidJSON)
	}
	return req, nil
}

func buildRenderOptions(raw RenderRequestOptions) (labelrender.Options, error) {
	var opts labelrender.Options

	opts = labelrender.DefaultOptions()
	if raw.Width != nil {
		if *raw.Width <= 0 {
			return opts, fmt.Errorf("%w: width must be greater than zero", errInvalidOption)
		}
		opts.Width = *raw.Width
	}
	if raw.Height != nil {
		if *raw.Height <= 0 {
			return opts, fmt.Errorf("%w: height must be greater than zero", errInvalidOption)
		}
		opts.Height = *raw.Height
	}
	if raw.Engine != "" {
		opts.Engine = raw.Engine
	}
	if raw.ChromePath != "" {
		opts.ChromePath = raw.ChromePath
	}
	if raw.FirefoxPath != "" {
		opts.FirefoxPath = raw.FirefoxPath
	}
	if raw.CaptureExtraHeight != nil {
		if *raw.CaptureExtraHeight < 0 {
			return opts, fmt.Errorf("%w: capture_extra_height must be zero or greater", errInvalidOption)
		}
		opts.CaptureExtraHeight = *raw.CaptureExtraHeight
	}
	if raw.WaitMS != nil {
		if *raw.WaitMS <= 0 {
			return opts, fmt.Errorf("%w: wait_ms must be greater than zero", errInvalidOption)
		}
		opts.VirtualTimeBudget = time.Duration(*raw.WaitMS) * time.Millisecond
	}
	if raw.TimeoutSec != nil {
		if *raw.TimeoutSec <= 0 {
			return opts, fmt.Errorf("%w: timeout_sec must be greater than zero", errInvalidOption)
		}
		opts.ChromeTimeout = time.Duration(*raw.TimeoutSec) * time.Second
	}
	if raw.Threshold != nil {
		if *raw.Threshold < 0 || *raw.Threshold > 255 {
			return opts, fmt.Errorf("%w: threshold must be in 0..255", errInvalidOption)
		}
		opts.Threshold = uint8(*raw.Threshold)
	}
	if raw.Gamma != nil {
		if *raw.Gamma <= 0 {
			return opts, fmt.Errorf("%w: gamma must be greater than zero", errInvalidOption)
		}
		opts.Gamma = *raw.Gamma
	}
	if raw.Contrast != nil {
		if *raw.Contrast <= 0 {
			return opts, fmt.Errorf("%w: contrast must be greater than zero", errInvalidOption)
		}
		opts.Contrast = *raw.Contrast
	}
	if raw.KeepTemp != nil {
		opts.KeepTemporary = *raw.KeepTemp
	}
	return opts, nil
}

func resolveTemplatePath(templateRoot string, templateName string) (string, error) {
	var err error
	var cleanedName string
	var joinedPath string
	var rel string
	var stat os.FileInfo

	templateName = strings.TrimSpace(templateName)
	if templateName == "" {
		return "", errTemplateRequired
	}
	if filepath.IsAbs(templateName) {
		return "", fmt.Errorf("%w: template path must be relative", errTemplatePath)
	}

	cleanedName = filepath.Clean(templateName)
	if cleanedName == "." || cleanedName == ".." || strings.HasPrefix(cleanedName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", errTemplatePath, templateName)
	}

	joinedPath = filepath.Join(templateRoot, cleanedName)
	if rel, err = filepath.Rel(templateRoot, joinedPath); err != nil {
		return "", fmt.Errorf("resolve template path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", errTemplatePath, templateName)
	}
	if stat, err = os.Stat(joinedPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: %q", errTemplateNotFound, templateName)
		}
		return "", fmt.Errorf("stat template: %w", err)
	}
	if stat.IsDir() {
		return "", fmt.Errorf("%w: %q", errTemplateDirectory, templateName)
	}

	return joinedPath, nil
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	var err error

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err = json.NewEncoder(w).Encode(payload); err != nil {
		return
	}
}
