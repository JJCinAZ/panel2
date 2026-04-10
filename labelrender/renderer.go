package labelrender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"
)

const (
	defaultWidth             = 800
	defaultHeight            = 480
	defaultVirtualTimeBudget = 5 * time.Second
	defaultChromeTimeout     = 45 * time.Second
	defaultThreshold         = 150
	defaultGamma             = 1.15
	defaultContrast          = 1.2
	windowExtraHeight        = 160
)

const (
	engineChrome  = "chrome"
	engineFirefox = "firefox"
)

var errUnsupportedOutputSize = errors.New("browser screenshot is smaller than the requested output size")
var errInvalidEngine = errors.New("invalid browser engine")

// Options controls HTML rendering and 1-bit conversion behavior.
type Options struct {
	Width              int
	Height             int
	TemplateData       any
	Engine             string
	ChromePath         string
	FirefoxPath        string
	CaptureExtraHeight int
	VirtualTimeBudget  time.Duration
	ChromeTimeout      time.Duration
	Threshold          uint8
	Gamma              float64
	Contrast           float64
	KeepTemporary      bool
}

// DefaultOptions returns sane defaults for 800x480 monochrome rendering.
func DefaultOptions() Options {
	return Options{
		Width:              defaultWidth,
		Height:             defaultHeight,
		Engine:             engineChrome,
		CaptureExtraHeight: windowExtraHeight,
		VirtualTimeBudget:  defaultVirtualTimeBudget,
		ChromeTimeout:      defaultChromeTimeout,
		Threshold:          defaultThreshold,
		Gamma:              defaultGamma,
		Contrast:           defaultContrast,
	}
}

// RenderFileTo1BitPNG renders a local HTML file into an 800x480 1-bit PNG.
func RenderFileTo1BitPNG(inputHTMLPath string, outputPNGPath string, opts Options) error {
	var err error
	var inputAbsPath string
	var outputAbsPath string
	var tempDir string
	var htmlBytes []byte
	var sourceHTML string
	var renderedHTML string
	var wrappedHTMLPath string
	var screenshotPath string
	var screenshotFile *os.File
	var screenshotImage image.Image
	var normalizedImage image.Image
	var oneBitImage *image.Paletted
	var outputFile *os.File

	if inputAbsPath, err = filepath.Abs(inputHTMLPath); err != nil {
		return fmt.Errorf("resolve input path: %w", err)
	}
	if outputAbsPath, err = filepath.Abs(outputPNGPath); err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if opts.Width <= 0 {
		opts.Width = defaultWidth
	}
	if opts.Height <= 0 {
		opts.Height = defaultHeight
	}
	if opts.VirtualTimeBudget <= 0 {
		opts.VirtualTimeBudget = defaultVirtualTimeBudget
	}
	if opts.ChromeTimeout <= 0 {
		opts.ChromeTimeout = defaultChromeTimeout
	}
	if opts.Gamma <= 0 {
		opts.Gamma = defaultGamma
	}
	if opts.Contrast <= 0 {
		opts.Contrast = defaultContrast
	}
	opts.Engine = normalizeEngine(opts.Engine)
	if opts.Engine == "" {
		opts.Engine = engineChrome
	}
	if !isValidEngine(opts.Engine) {
		return fmt.Errorf("%w %q (valid: %s, %s)", errInvalidEngine, opts.Engine, engineChrome, engineFirefox)
	}

	if htmlBytes, err = os.ReadFile(inputAbsPath); err != nil {
		return fmt.Errorf("read input html: %w", err)
	}
	if sourceHTML, err = renderInputHTML(inputAbsPath, htmlBytes, opts.TemplateData); err != nil {
		return err
	}
	renderedHTML = injectRenderOverride(sourceHTML, opts.Width, opts.Height)

	if tempDir, err = os.MkdirTemp("", "labelrender-*"); err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	if !opts.KeepTemporary {
		defer func() {
			_ = os.RemoveAll(tempDir)
		}()
	}

	wrappedHTMLPath = filepath.Join(tempDir, "render.html")
	screenshotPath = filepath.Join(tempDir, "render.png")
	if err = os.WriteFile(wrappedHTMLPath, []byte(renderedHTML), 0o644); err != nil {
		return fmt.Errorf("write wrapped html: %w", err)
	}

	if err = captureWithBrowser(wrappedHTMLPath, screenshotPath, opts); err != nil {
		return err
	}

	if screenshotFile, err = os.Open(screenshotPath); err != nil {
		return fmt.Errorf("open screenshot: %w", err)
	}
	defer func() {
		_ = screenshotFile.Close()
	}()

	if screenshotImage, err = png.Decode(screenshotFile); err != nil {
		return fmt.Errorf("decode screenshot png: %w", err)
	}

	if normalizedImage, err = normalizeImageSize(screenshotImage, opts.Width, opts.Height); err != nil {
		return err
	}
	oneBitImage = convertTo1Bit(normalizedImage, opts)

	if err = os.MkdirAll(filepath.Dir(outputAbsPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if outputFile, err = os.Create(outputAbsPath); err != nil {
		return fmt.Errorf("create output png: %w", err)
	}
	defer func() {
		_ = outputFile.Close()
	}()

	if err = png.Encode(outputFile, oneBitImage); err != nil {
		return fmt.Errorf("encode 1-bit png: %w", err)
	}
	return nil
}

// RenderFileTo1BitPNGBytes renders a local HTML file and returns the final 1-bit PNG bytes.
func RenderFileTo1BitPNGBytes(inputHTMLPath string, opts Options) ([]byte, error) {
	var err error
	var outputFile *os.File
	var outputPath string
	var pngBytes []byte

	if outputFile, err = os.CreateTemp("", "labelrender-output-*.png"); err != nil {
		return nil, fmt.Errorf("create temp output file: %w", err)
	}
	outputPath = outputFile.Name()
	if err = outputFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp output file: %w", err)
	}
	defer func() {
		_ = os.Remove(outputPath)
	}()

	if err = RenderFileTo1BitPNG(inputHTMLPath, outputPath, opts); err != nil {
		return nil, err
	}
	if pngBytes, err = os.ReadFile(outputPath); err != nil {
		return nil, fmt.Errorf("read output png: %w", err)
	}
	return pngBytes, nil
}

func renderInputHTML(inputAbsPath string, rawHTML []byte, templateData any) (string, error) {
	var ext string

	ext = strings.ToLower(filepath.Ext(inputAbsPath))
	if templateData == nil && ext != ".tmpl" && ext != ".tpl" && ext != ".gotmpl" {
		return string(rawHTML), nil
	}
	return executeHTMLTemplate(inputAbsPath, rawHTML, templateData)
}

func executeHTMLTemplate(inputAbsPath string, rawHTML []byte, templateData any) (string, error) {
	var err error
	var tpl *template.Template
	var out bytes.Buffer

	if templateData == nil {
		templateData = map[string]string{}
	}
	if tpl, err = template.New(filepath.Base(inputAbsPath)).Option("missingkey=zero").Parse(string(rawHTML)); err != nil {
		return "", fmt.Errorf("parse html template %q: %w", inputAbsPath, err)
	}
	if err = tpl.Execute(&out, templateData); err != nil {
		return "", fmt.Errorf("execute html template %q: %w", inputAbsPath, err)
	}
	return out.String(), nil
}

func captureWithBrowser(htmlPath string, screenshotPath string, opts Options) error {
	var err error
	var fileURL string
	var args []string
	var chromePath string
	var chromeUserDataDir string
	var chromeHomeDir string
	var chromeXDGConfigHome string
	var chromeXDGCacheHome string
	var firefoxPath string
	var captureHeight int
	var captureExtraHeight int
	var ctx context.Context
	var cancel context.CancelFunc
	var cmd *exec.Cmd
	var output []byte
	var env []string

	ctx, cancel = context.WithTimeout(context.Background(), opts.ChromeTimeout)
	defer cancel()

	fileURL = (&url.URL{Scheme: "file", Path: htmlPath}).String()
	captureExtraHeight = opts.CaptureExtraHeight
	if captureExtraHeight < 0 {
		captureExtraHeight = 0
	}
	if captureExtraHeight == 0 {
		captureExtraHeight = windowExtraHeight
	}
	captureHeight = opts.Height + captureExtraHeight

	if opts.Engine == engineFirefox {
		if firefoxPath, err = resolveFirefoxBinary(opts.FirefoxPath); err != nil {
			return err
		}
		args = []string{
			"-headless",
			"-screenshot", screenshotPath,
			fmt.Sprintf("--window-size=%d,%d", opts.Width, captureHeight),
			fileURL,
		}
		cmd = exec.CommandContext(ctx, firefoxPath, args...)
		if output, err = cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("headless firefox failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	}

	if chromePath, err = resolveChromeBinary(opts.ChromePath); err != nil {
		return err
	}
	chromeUserDataDir = filepath.Join(filepath.Dir(screenshotPath), "chrome-user-data")
	chromeHomeDir = filepath.Join(filepath.Dir(screenshotPath), "chrome-home")
	chromeXDGConfigHome = filepath.Join(chromeHomeDir, ".config")
	chromeXDGCacheHome = filepath.Join(chromeHomeDir, ".cache")
	if err = os.MkdirAll(chromeUserDataDir, 0o755); err != nil {
		return fmt.Errorf("create chrome user data dir: %w", err)
	}
	if err = os.MkdirAll(chromeXDGConfigHome, 0o755); err != nil {
		return fmt.Errorf("create chrome xdg config dir: %w", err)
	}
	if err = os.MkdirAll(chromeXDGCacheHome, 0o755); err != nil {
		return fmt.Errorf("create chrome xdg cache dir: %w", err)
	}
	args = []string{
		"--headless=new",
		"--disable-gpu",
		"--hide-scrollbars",
		"--mute-audio",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-crash-reporter",
		"--disable-breakpad",
		"--disable-crashpad",
		"--disable-features=Crashpad",
		fmt.Sprintf("--user-data-dir=%s", chromeUserDataDir),
		fmt.Sprintf("--window-size=%d,%d", opts.Width, captureHeight),
		"--force-device-scale-factor=1",
		fmt.Sprintf("--virtual-time-budget=%d", opts.VirtualTimeBudget.Milliseconds()),
		fmt.Sprintf("--screenshot=%s", screenshotPath),
		fileURL,
	}
	cmd = exec.CommandContext(ctx, chromePath, args...)
	env = os.Environ()
	env = append(env, fmt.Sprintf("HOME=%s", chromeHomeDir))
	env = append(env, fmt.Sprintf("XDG_CONFIG_HOME=%s", chromeXDGConfigHome))
	env = append(env, fmt.Sprintf("XDG_CACHE_HOME=%s", chromeXDGCacheHome))
	cmd.Env = env
	if output, err = cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("headless chrome failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func normalizeEngine(engine string) string {
	return strings.ToLower(strings.TrimSpace(engine))
}

func isValidEngine(engine string) bool {
	return engine == engineChrome || engine == engineFirefox
}

func resolveFirefoxBinary(explicitPath string) (string, error) {
	var err error
	var candidates []string
	var candidate string
	var resolved string
	var homeDir string

	if explicitPath != "" {
		if strings.ContainsRune(explicitPath, filepath.Separator) {
			if _, err = os.Stat(explicitPath); err != nil {
				return "", fmt.Errorf("firefox binary not found at %q: %w", explicitPath, err)
			}
			return explicitPath, nil
		}
		if resolved, err = exec.LookPath(explicitPath); err != nil {
			return "", fmt.Errorf("firefox binary %q not found in PATH: %w", explicitPath, err)
		}
		return resolved, nil
	}

	homeDir, _ = os.UserHomeDir()
	candidates = firefoxCandidatesForOS(runtime.GOOS, homeDir)
	for _, candidate = range candidates {
		if strings.ContainsRune(candidate, filepath.Separator) {
			if _, err = os.Stat(candidate); err == nil {
				return candidate, nil
			}
			continue
		}
		if resolved, err = exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
	}
	return "", errors.New("could not find headless Firefox binary in PATH or standard install locations")
}

func resolveChromeBinary(explicitPath string) (string, error) {
	var err error
	var candidates []string
	var candidate string
	var resolved string
	var homeDir string

	if explicitPath != "" {
		if strings.ContainsRune(explicitPath, filepath.Separator) {
			if _, err = os.Stat(explicitPath); err != nil {
				return "", fmt.Errorf("chrome binary not found at %q: %w", explicitPath, err)
			}
			return explicitPath, nil
		}
		if resolved, err = exec.LookPath(explicitPath); err != nil {
			return "", fmt.Errorf("chrome binary %q not found in PATH: %w", explicitPath, err)
		}
		return resolved, nil
	}

	homeDir, _ = os.UserHomeDir()
	candidates = chromeCandidatesForOS(runtime.GOOS, homeDir)
	for _, candidate = range candidates {
		if strings.ContainsRune(candidate, filepath.Separator) {
			if _, err = os.Stat(candidate); err == nil {
				return candidate, nil
			}
			continue
		}
		if resolved, err = exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
	}
	return "", errors.New("could not find headless Chrome/Chromium binary in PATH or standard install locations")
}

func chromeCandidatesForOS(goos string, homeDir string) []string {
	var candidates []string

	candidates = []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
	}

	if goos == "darwin" {
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
			"/Applications/Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"/opt/homebrew/bin/chromium",
			"/usr/local/bin/chromium",
		)
		if homeDir != "" {
			candidates = append(candidates,
				filepath.Join(homeDir, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
				filepath.Join(homeDir, "Applications/Chromium.app/Contents/MacOS/Chromium"),
				filepath.Join(homeDir, "Applications/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"),
			)
		}
	}

	return candidates
}

func firefoxCandidatesForOS(goos string, homeDir string) []string {
	var candidates []string

	candidates = []string{
		"firefox",
		"firefox-esr",
	}
	if goos == "darwin" {
		candidates = append(candidates,
			"/Applications/Firefox.app/Contents/MacOS/firefox",
			"/Applications/Firefox Developer Edition.app/Contents/MacOS/firefox",
			"/Applications/Firefox Nightly.app/Contents/MacOS/firefox",
			"/opt/homebrew/bin/firefox",
			"/usr/local/bin/firefox",
		)
		if homeDir != "" {
			candidates = append(candidates,
				filepath.Join(homeDir, "Applications/Firefox.app/Contents/MacOS/firefox"),
				filepath.Join(homeDir, "Applications/Firefox Developer Edition.app/Contents/MacOS/firefox"),
			)
		}
	}
	return candidates
}

func injectRenderOverride(sourceHTML string, width int, height int) string {
	var overrideCSS string
	var replacement string

	overrideCSS = fmt.Sprintf(`
<style id="labelrender-override">
html, body {
	width: %dpx;
	height: %dpx;
	margin: 0 !important;
	padding: 0 !important;
	min-height: 0 !important;
	background: #ffffff !important;
}
body {
	display: block !important;
	overflow: hidden !important;
}
.e-ink-canvas {
	position: absolute !important;
	left: 0 !important;
	top: 0 !important;
	width: %dpx !important;
	height: %dpx !important;
	box-sizing: border-box !important;
	box-shadow: none !important;
}
</style>`, width, height, width, height)
	replacement = overrideCSS + "\n</head>"
	if strings.Contains(sourceHTML, "</head>") {
		return strings.Replace(sourceHTML, "</head>", replacement, 1)
	}
	return "<head>" + overrideCSS + "</head>\n" + sourceHTML
}

func normalizeImageSize(src image.Image, width int, height int) (image.Image, error) {
	var bounds image.Rectangle
	var srcWidth int
	var srcHeight int
	var targetRect image.Rectangle
	var target *image.RGBA

	bounds = src.Bounds()
	srcWidth = bounds.Dx()
	srcHeight = bounds.Dy()
	if srcWidth == width && srcHeight == height {
		return src, nil
	}
	if srcWidth < width || srcHeight < height {
		return nil, fmt.Errorf("%w: got %dx%d, need at least %dx%d", errUnsupportedOutputSize, srcWidth, srcHeight, width, height)
	}

	// Crop from top-left. We intentionally capture with extra vertical headroom
	// to avoid viewport clipping differences across Chrome headless builds.
	targetRect = image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+width, bounds.Min.Y+height)

	target = image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(target, target.Bounds(), src, targetRect.Min, draw.Src)
	return target, nil
}

func convertTo1Bit(src image.Image, opts Options) *image.Paletted {
	var gray *image.Gray

	gray = prepareGrayImage(src, opts.Gamma, opts.Contrast)
	return thresholdBinarize(gray, opts.Threshold)
}

func prepareGrayImage(src image.Image, gamma float64, contrast float64) *image.Gray {
	var bounds image.Rectangle
	var out *image.Gray
	var y int
	var x int
	var r uint32
	var g uint32
	var b uint32
	var a uint32
	var luma float64
	var v float64
	var at color.Color

	bounds = src.Bounds()
	out = image.NewGray(bounds)
	if gamma <= 0 {
		gamma = defaultGamma
	}
	if contrast <= 0 {
		contrast = defaultContrast
	}

	for y = bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x = bounds.Min.X; x < bounds.Max.X; x++ {
			at = src.At(x, y)
			r, g, b, a = at.RGBA()
			if a == 0 {
				out.SetGray(x, y, color.Gray{Y: 255})
				continue
			}
			luma = ((0.299 * float64(r)) + (0.587 * float64(g)) + (0.114 * float64(b))) / 65535.0
			v = math.Pow(luma, gamma)
			v = ((v - 0.5) * contrast) + 0.5
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			out.SetGray(x, y, color.Gray{Y: uint8(math.Round(v * 255.0))})
		}
	}
	return out
}

func thresholdBinarize(src *image.Gray, threshold uint8) *image.Paletted {
	var bounds image.Rectangle
	var palette color.Palette
	var dst *image.Paletted
	var y int
	var x int

	bounds = src.Bounds()
	palette = color.Palette{
		color.White,
		color.Black,
	}
	dst = image.NewPaletted(bounds, palette)

	for y = bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x = bounds.Min.X; x < bounds.Max.X; x++ {
			if src.GrayAt(x, y).Y < threshold {
				dst.SetColorIndex(x, y, 1)
				continue
			}
			dst.SetColorIndex(x, y, 0)
		}
	}
	return dst
}
