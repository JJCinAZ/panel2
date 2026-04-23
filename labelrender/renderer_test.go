package labelrender

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

type templateRenderData struct {
	SuiteNumber    string
	TenantName     string
	TenantSubtitle string
}

func TestInjectRenderOverride(t *testing.T) {
	var source string
	var rendered string

	source = "<html><head><title>x</title></head><body></body></html>"
	rendered = injectRenderOverride(source, 800, 480)
	if !strings.Contains(rendered, `id="labelrender-override"`) {
		t.Fatalf("render override style block was not injected")
	}
	if !strings.Contains(rendered, "width: 800px;") {
		t.Fatalf("expected injected width rule")
	}
	if !strings.Contains(rendered, "height: 480px;") {
		t.Fatalf("expected injected height rule")
	}
}

func TestConvertTo1BitCleanThreshold(t *testing.T) {
	var src *image.RGBA
	var dst *image.Gray
	var opts Options

	src = image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, color.RGBA{R: 20, G: 20, B: 20, A: 255})
	src.Set(1, 0, color.RGBA{R: 240, G: 240, B: 240, A: 255})
	opts = DefaultOptions()
	opts.Gamma = 1
	opts.Contrast = 1
	opts.Threshold = 128

	dst = convertTo1Bit(src, opts)
	if got := dst.GrayAt(0, 0).Y; got != 0 {
		t.Fatalf("expected first pixel black (0), got %d", got)
	}
	if got := dst.GrayAt(1, 0).Y; got != 255 {
		t.Fatalf("expected second pixel white (255), got %d", got)
	}
}

func TestConvertTo1BitEncodedPNGColorTypeIsGrayscale(t *testing.T) {
	var src *image.RGBA
	var dst *image.Gray
	var opts Options
	var b bytes.Buffer
	var encoded []byte
	var err error

	src = image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, color.RGBA{R: 20, G: 20, B: 20, A: 255})
	src.Set(1, 0, color.RGBA{R: 240, G: 240, B: 240, A: 255})
	opts = DefaultOptions()
	opts.Gamma = 1
	opts.Contrast = 1
	opts.Threshold = 128

	dst = convertTo1Bit(src, opts)
	if err = png.Encode(&b, dst); err != nil {
		t.Fatalf("png.Encode failed: %v", err)
	}
	encoded = b.Bytes()
	if len(encoded) < 26 {
		t.Fatalf("encoded png unexpectedly short: %d bytes", len(encoded))
	}
	if !bytes.Equal(encoded[0:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		t.Fatalf("missing png signature")
	}
	if !bytes.Equal(encoded[12:16], []byte("IHDR")) {
		t.Fatalf("first chunk is not IHDR")
	}
	if got := encoded[25]; got != 0 {
		t.Fatalf("expected PNG color type 0 (grayscale), got %d", got)
	}
}

func TestNormalizeImageSizeCropTopLeft(t *testing.T) {
	var src *image.RGBA
	var out image.Image
	var err error
	var c color.Color
	var got color.RGBA

	src = image.NewRGBA(image.Rect(0, 0, 100, 60))
	src.Set(0, 0, color.RGBA{R: 255, A: 255})
	src.Set(99, 59, color.RGBA{B: 255, A: 255})
	out, err = normalizeImageSize(src, 80, 40)
	if err != nil {
		t.Fatalf("normalizeImageSize failed: %v", err)
	}
	if out.Bounds().Dx() != 80 || out.Bounds().Dy() != 40 {
		t.Fatalf("unexpected output bounds: %v", out.Bounds())
	}
	c = out.At(0, 0)
	got = color.RGBAModel.Convert(c).(color.RGBA)
	if got.R != 255 || got.G != 0 || got.B != 0 {
		t.Fatalf("expected top-left crop to retain source top-left pixel, got %#v", got)
	}
}

func TestRenderInputHTMLRawWhenNoTemplateMode(t *testing.T) {
	var raw []byte
	var out string
	var err error

	raw = []byte("<h1>{{ .TenantName }}</h1>")
	out, err = renderInputHTML("/tmp/input.html", raw, nil)
	if err != nil {
		t.Fatalf("renderInputHTML returned error: %v", err)
	}
	if out != string(raw) {
		t.Fatalf("expected raw html passthrough, got %q", out)
	}
}

func TestRenderInputHTMLTemplateFromTmplExtension(t *testing.T) {
	var raw []byte
	var out string
	var err error
	var data templateRenderData

	raw = []byte("<h1>{{ .TenantName }}</h1>")
	data = templateRenderData{TenantName: "ARCHITECTURAL SYNERGY"}
	out, err = renderInputHTML("/tmp/input.tmpl", raw, data)
	if err != nil {
		t.Fatalf("renderInputHTML returned error: %v", err)
	}
	if !strings.Contains(out, "ARCHITECTURAL SYNERGY") {
		t.Fatalf("expected template output to include tenant name, got %q", out)
	}
}

func TestRenderInputHTMLTemplateWhenDataProvidedForHTML(t *testing.T) {
	var raw []byte
	var out string
	var err error
	var data templateRenderData

	raw = []byte("<h1>{{ .SuiteNumber }}</h1>")
	data = templateRenderData{SuiteNumber: "402"}
	out, err = renderInputHTML("/tmp/input.html", raw, data)
	if err != nil {
		t.Fatalf("renderInputHTML returned error: %v", err)
	}
	if !strings.Contains(out, ">402<") {
		t.Fatalf("expected rendered suite number, got %q", out)
	}
}

func TestChromeCandidatesForOSDarwinIncludesAppBundle(t *testing.T) {
	var candidates []string
	var foundBundle bool
	var foundCommand bool
	var candidate string

	candidates = chromeCandidatesForOS("darwin", "/Users/tester")
	for _, candidate = range candidates {
		if candidate == "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" {
			foundBundle = true
		}
		if candidate == "google-chrome" {
			foundCommand = true
		}
	}

	if !foundBundle {
		t.Fatalf("expected darwin candidates to include standard Chrome app bundle path")
	}
	if !foundCommand {
		t.Fatalf("expected darwin candidates to include PATH command candidate")
	}
}

func TestChromeCandidatesForOSLinuxDoesNotIncludeMacBundle(t *testing.T) {
	var candidates []string
	var candidate string

	candidates = chromeCandidatesForOS("linux", "/home/tester")
	for _, candidate = range candidates {
		if strings.Contains(candidate, "/Applications/") {
			t.Fatalf("linux candidates should not include macOS app bundle paths")
		}
	}
}

func TestFirefoxCandidatesForOSDarwinIncludesAppBundle(t *testing.T) {
	var candidates []string
	var foundBundle bool
	var foundCommand bool
	var candidate string

	candidates = firefoxCandidatesForOS("darwin", "/Users/tester")
	for _, candidate = range candidates {
		if candidate == "/Applications/Firefox.app/Contents/MacOS/firefox" {
			foundBundle = true
		}
		if candidate == "firefox" {
			foundCommand = true
		}
	}

	if !foundBundle {
		t.Fatalf("expected darwin firefox candidates to include standard Firefox app bundle path")
	}
	if !foundCommand {
		t.Fatalf("expected darwin firefox candidates to include PATH command candidate")
	}
}

func TestFirefoxCandidatesForOSLinuxDoesNotIncludeMacBundle(t *testing.T) {
	var candidates []string
	var candidate string

	candidates = firefoxCandidatesForOS("linux", "/home/tester")
	for _, candidate = range candidates {
		if strings.Contains(candidate, "/Applications/") {
			t.Fatalf("linux firefox candidates should not include macOS app bundle paths")
		}
	}
}

func TestNormalizeEngine(t *testing.T) {
	if got := normalizeEngine("  FireFox "); got != "firefox" {
		t.Fatalf("expected firefox, got %q", got)
	}
	if got := normalizeEngine("CHROME"); got != "chrome" {
		t.Fatalf("expected chrome, got %q", got)
	}
	if !isValidEngine("chrome") {
		t.Fatalf("expected chrome to be valid")
	}
	if !isValidEngine("firefox") {
		t.Fatalf("expected firefox to be valid")
	}
	if isValidEngine("safari") {
		t.Fatalf("expected safari to be invalid")
	}
}
