// Command genbrand composites the robot mascot PNG onto the GitHub social card
// and macOS installer background canvases.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
)

var (
	bgDark  = color.NRGBA{R: 0x0f, G: 0x14, B: 0x19, A: 0xff} // docs dark --bg
	bgLight = color.NRGBA{R: 0xf6, G: 0xf3, B: 0xee, A: 0xff} // docs light --bg
)

func main() {
	srcPath := flag.String("src", "internal/serve/buddy.png", "source logo PNG")
	socialPath := flag.String("social", "", "write 1280×640 GitHub social preview PNG")
	bgPath := flag.String("background", "", "write 620×418 installer background (light)")
	bgDarkPath := flag.String("background-dark", "", "write 620×418 installer background (dark Aqua)")
	flag.Parse()

	if *socialPath == "" && *bgPath == "" && *bgDarkPath == "" {
		fmt.Fprintln(os.Stderr, "genbrand: specify -social, -background, and/or -background-dark")
		os.Exit(2)
	}

	src, err := loadPNG(*srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genbrand: %v\n", err)
		os.Exit(1)
	}

	if *socialPath != "" {
		img := letterbox(src, 1280, 640, bgDark, 480)
		if err := writePNG(*socialPath, img); err != nil {
			fmt.Fprintf(os.Stderr, "genbrand: social: %v\n", err)
			os.Exit(1)
		}
	}
	if *bgPath != "" {
		img := letterbox(src, 620, 418, bgLight, 240)
		if err := writePNG(*bgPath, img); err != nil {
			fmt.Fprintf(os.Stderr, "genbrand: background: %v\n", err)
			os.Exit(1)
		}
	}
	if *bgDarkPath != "" {
		img := letterbox(src, 620, 418, bgDark, 240)
		if err := writePNG(*bgDarkPath, img); err != nil {
			fmt.Fprintf(os.Stderr, "genbrand: background-dark: %v\n", err)
			os.Exit(1)
		}
	}
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return img, nil
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, img); err != nil {
		return err
	}
	return f.Close()
}

// letterbox draws src centered on a w×h canvas. If maxSide > 0, src is
// scaled so its larger side equals maxSide (never upscaled).
func letterbox(src image.Image, w, h int, bg color.NRGBA, maxSide int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	logo := src
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if maxSide > 0 {
		side := sw
		if sh > side {
			side = sh
		}
		if side > maxSide {
			nw := sw * maxSide / side
			nh := sh * maxSide / side
			logo = scaleNearest(src, nw, nh)
			sw, sh = nw, nh
		}
	}
	x := (w - sw) / 2
	y := (h - sh) / 2
	r := image.Rect(x, y, x+sw, y+sh)
	draw.Draw(dst, r, logo, logo.Bounds().Min, draw.Over)
	return dst
}

func scaleNearest(src image.Image, w, h int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	for y := 0; y < h; y++ {
		sy := sb.Min.Y + y*sh/h
		for x := 0; x < w; x++ {
			sx := sb.Min.X + x*sw/w
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
