# panel2 label renderer

This repository includes a Go package and CLI that render `architecturalLabel.html` into an `800x480` monochrome (1-bit) PNG using headless Chrome/Chromium or Firefox on Linux and macOS.

## What it does

1. Loads your HTML file with headless Chrome.
2. Applies a temporary render override so `.e-ink-canvas` fills exactly `800x480`.
3. Captures a PNG screenshot.
4. Converts it to a strict black/white 1-bit PNG.

## Run it

```bash
go run ./cmd/labelrender \
  -input architecturalLabel.html \
  -output architecturalLabel-1bit.png
```

Optional flags:

- `-engine chrome|firefox` selects the rendering browser (default `chrome`).
- `-chrome /path/to/google-chrome` sets Chrome/Chromium binary path when `-engine chrome`.
- `-firefox /path/to/firefox` sets Firefox binary path when `-engine firefox`.
- `-capture-extra-height 160` adds vertical headroom before final crop to exact output size.
- `-wait-ms 7000` to allow more time for remote fonts/assets to load.
- `-gamma 1.15` tone curve before thresholding (higher values darken mids).
- `-contrast 1.2` contrast multiplier before thresholding.
- `-threshold 150` threshold used for final black/white conversion.
- `-keep-temp` to keep temporary render files for debugging.

## Use as a package

```go
package main

import "github.com/jjcinaz/panel2/labelrender"

type labelData struct {
	SuiteNumber    string
	TenantName     string
	TenantSubtitle string
}

func main() {
	opts := labelrender.DefaultOptions()
	opts.TemplateData = labelData{
		SuiteNumber:    "402",
		TenantName:     "ARCHITECTURAL SYNERGY",
		TenantSubtitle: "Principal Architects & Design Lab",
	}
	_ = labelrender.RenderFileTo1BitPNG("architecturalLabel.tmpl", "architecturalLabel-1bit.png", opts)
}
```

Template behavior:
- Files ending in `.tmpl`, `.tpl`, or `.gotmpl` are rendered as Go `text/template` before screenshot capture.
- If `opts.TemplateData` is set, template rendering is applied even when the filename ends in `.html`.

## Debian 12: install headless browsers

The renderer auto-detects:
- Chrome family: `google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser`
- Firefox family: `firefox`, `firefox-esr`

### Option A (recommended): Chromium from Debian repo

```bash
sudo apt update
sudo apt install -y chromium
chromium --headless --version
```

If the binary is installed as `chromium`, you can run:

```bash
go run ./cmd/labelrender -chrome chromium -input architecturalLabel.html -output architecturalLabel-1bit.png
```

### Option B: Google Chrome stable (official Google package)

```bash
sudo apt update
sudo apt install -y ca-certificates curl gnupg
sudo install -d -m 0755 /etc/apt/keyrings
curl -fsSL https://dl.google.com/linux/linux_signing_key.pub | sudo gpg --dearmor -o /etc/apt/keyrings/google-chrome.gpg
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/google-chrome.gpg] https://dl.google.com/linux/chrome/deb/ stable main" | sudo tee /etc/apt/sources.list.d/google-chrome.list >/dev/null
sudo apt update
sudo apt install -y google-chrome-stable
google-chrome --headless --version
```

Then run:

```bash
go run ./cmd/labelrender -chrome google-chrome -input architecturalLabel.html -output architecturalLabel-1bit.png
```

### Option C: Firefox ESR from Debian repo

```bash
sudo apt update
sudo apt install -y firefox-esr
firefox --headless --version
```

Then run:

```bash
go run ./cmd/labelrender -engine firefox -input architecturalLabel.html -output architecturalLabel-1bit.png
```

## macOS: install and run headless browsers

Install Chrome (or Chromium) with Homebrew:

```bash
brew install --cask google-chrome
# or:
# brew install --cask chromium
```

Verify:

```bash
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --headless=new --version
```

Or install Firefox:

```bash
brew install --cask firefox
/Applications/Firefox.app/Contents/MacOS/firefox --headless --version
```

Run renderer with Chrome:

```bash
go run ./cmd/labelrender -input architecturalLabel.html -output architecturalLabel-1bit.png
```

Run renderer with Firefox:

```bash
go run ./cmd/labelrender -engine firefox -input architecturalLabel.html -output architecturalLabel-1bit.png
```

The renderer auto-detects common macOS app locations under `/Applications/...`. If your browser is installed elsewhere, pass `-chrome /absolute/path/to/chrome-binary` or `-firefox /absolute/path/to/firefox`.

## Notes

- This renderer runs without a GUI session using headless Chrome/Chromium or Firefox.
- The HTML currently references Google Fonts; internet access improves visual fidelity.
- Output is always PNG with a 2-color palette (white + black).
