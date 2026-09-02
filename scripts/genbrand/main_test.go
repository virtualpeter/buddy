package main

import (
	"image"
	"image/color"
	"testing"
)

func TestLetterboxSize(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	got := letterbox(src, 1280, 640, bgDark, 0)
	if got.Bounds().Dx() != 1280 || got.Bounds().Dy() != 640 {
		t.Fatalf("social canvas %v", got.Bounds())
	}
	bg := letterbox(src, 620, 418, bgLight, 240)
	if bg.Bounds().Dx() != 620 || bg.Bounds().Dy() != 418 {
		t.Fatalf("installer canvas %v", bg.Bounds())
	}
}

func TestLetterboxFill(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	bg := color.NRGBA{R: 0x0f, G: 0x14, B: 0x19, A: 0xff}
	got := letterbox(src, 40, 20, bg, 0)
	c := got.NRGBAAt(0, 0)
	if c != bg {
		t.Fatalf("corner %+v want %+v", c, bg)
	}
}
