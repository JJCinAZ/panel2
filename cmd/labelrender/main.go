package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jjcinaz/panel2/labelrender"
)

func main() {
	var err error
	var inputPath string
	var outputPath string
	var chromePath string
	var firefoxPath string
	var engine string
	var width int
	var height int
	var captureExtraHeight int
	var waitMS int
	var timeoutSec int
	var threshold int
	var gamma float64
	var contrast float64
	var keepTemp bool
	var opts labelrender.Options

	flag.StringVar(&inputPath, "input", "architecturalLabel.html", "Path to source HTML file")
	flag.StringVar(&outputPath, "output", "architecturalLabel-1bit.png", "Path to output PNG file")
	flag.StringVar(&engine, "engine", "chrome", "Browser engine: chrome or firefox")
	flag.StringVar(&chromePath, "chrome", "", "Chrome/Chromium binary path (used when -engine chrome)")
	flag.StringVar(&firefoxPath, "firefox", "", "Firefox binary path (used when -engine firefox)")
	flag.IntVar(&width, "width", 800, "Output width in pixels")
	flag.IntVar(&height, "height", 480, "Output height in pixels")
	flag.IntVar(&captureExtraHeight, "capture-extra-height", 160, "Extra browser window height used to avoid headless viewport clipping")
	flag.IntVar(&waitMS, "wait-ms", 5000, "Virtual time budget to allow fonts/assets to load")
	flag.IntVar(&timeoutSec, "timeout-sec", 45, "Timeout for the browser process")
	flag.IntVar(&threshold, "threshold", 150, "Threshold (0-255) for final black/white conversion")
	flag.Float64Var(&gamma, "gamma", 1.15, "Gamma preprocessing value (higher values darken mids)")
	flag.Float64Var(&contrast, "contrast", 1.2, "Contrast preprocessing multiplier")
	flag.BoolVar(&keepTemp, "keep-temp", false, "Keep temporary render files for debugging")
	flag.Parse()

	engine = strings.ToLower(strings.TrimSpace(engine))
	if threshold < 0 || threshold > 255 {
		fmt.Fprintf(os.Stderr, "invalid -threshold value %d (must be 0..255)\n", threshold)
		os.Exit(2)
	}
	if engine != "chrome" && engine != "firefox" {
		fmt.Fprintf(os.Stderr, "invalid -engine value %q (must be chrome or firefox)\n", engine)
		os.Exit(2)
	}
	if gamma <= 0 {
		fmt.Fprintf(os.Stderr, "invalid -gamma value %.4f (must be > 0)\n", gamma)
		os.Exit(2)
	}
	if contrast <= 0 {
		fmt.Fprintf(os.Stderr, "invalid -contrast value %.4f (must be > 0)\n", contrast)
		os.Exit(2)
	}
	if err = ensureInputExists(inputPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	opts = labelrender.DefaultOptions()
	opts.Engine = engine
	opts.Width = width
	opts.Height = height
	opts.FirefoxPath = firefoxPath
	opts.CaptureExtraHeight = captureExtraHeight
	opts.ChromePath = chromePath
	opts.VirtualTimeBudget = time.Duration(waitMS) * time.Millisecond
	opts.ChromeTimeout = time.Duration(timeoutSec) * time.Second
	opts.Threshold = uint8(threshold)
	opts.Gamma = gamma
	opts.Contrast = contrast
	opts.KeepTemporary = keepTemp
	opts.TemplateData = struct {
		SuiteNumber    string
		TenantName     string
		TenantSubtitle string
	}{
		SuiteNumber:    "126",
		TenantName:     "BJM SABINO, LLC",
		TenantSubtitle: `"A few small beers"`,
	}

	if err = labelrender.RenderFileTo1BitPNG(inputPath, outputPath, opts); err != nil {
		fmt.Fprintf(os.Stderr, "render failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s\n", outputPath)
}

func ensureInputExists(path string) error {
	var err error
	var absPath string
	var stat os.FileInfo

	if absPath, err = filepath.Abs(path); err != nil {
		return fmt.Errorf("resolve input path: %w", err)
	}
	if stat, err = os.Stat(absPath); err != nil {
		return fmt.Errorf("input file %q not found: %w", absPath, err)
	}
	if stat.IsDir() {
		return fmt.Errorf("input path %q is a directory", absPath)
	}
	return nil
}
