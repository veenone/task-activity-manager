package reportout

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// samplePNG is a readable PNG of the given size, base64 as an Image carries
// it. The renderers size a picture from its own header, so the size a test
// asks for is the size it can assert on.
func samplePNG(t *testing.T, w, h int) string {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	m.Set(0, 0, color.RGBA{R: 31, G: 122, B: 82, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatalf("encode a test png: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func sampleImage(t *testing.T) Image {
	t.Helper()
	return Image{Name: "chart-1.png", Alt: "Burndown, drawn as a chart.", Data: samplePNG(t, 480, 220)}
}

func TestPNGAnswersTheBytesAndTheSizeInTheHeader(t *testing.T) {
	raw, cfg, err := sampleImage(t).PNG()
	if err != nil {
		t.Fatalf("a readable png was refused: %v", err)
	}
	if cfg.Width != 480 || cfg.Height != 220 {
		t.Errorf("size = %dx%d, want 480x220", cfg.Width, cfg.Height)
	}
	if !bytes.HasPrefix(raw, []byte("\x89PNG")) {
		t.Error("the bytes are not the PNG that went in")
	}
}

// I1: an image arrives over the Wails binding, so its name is a part name in
// a zip and its bytes are a decoder's input. Both are checked.
func TestCheckRefusesAnImageARendererCouldNotPlace(t *testing.T) {
	good := sampleImage(t)
	for _, tc := range []struct {
		name string
		im   Image
	}{
		{"no file name", Image{Alt: good.Alt, Data: good.Data}},
		{"no description", Image{Name: good.Name, Data: good.Data}},
		{"a path in the file name", Image{Name: "../media/chart.png", Alt: good.Alt, Data: good.Data}},
		{"not a png by extension", Image{Name: "chart-1.svg", Alt: good.Alt, Data: good.Data}},
		{"not base64", Image{Name: good.Name, Alt: good.Alt, Data: "not base64 at all!"}},
		{"not a png by content", Image{Name: good.Name, Alt: good.Alt, Data: base64.StdEncoding.EncodeToString([]byte("GIF89a"))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := sample()
			d.Sections[0].Images = []Image{tc.im}
			err := d.Check()
			if err == nil {
				t.Fatal("want a refusal, got nil")
			}
			if !strings.Contains(err.Error(), "chart") {
				t.Errorf("the refusal does not say a chart image is the problem: %v", err)
			}
		})
	}
	d := sample()
	d.Sections[0].Images = []Image{good}
	if err := d.Check(); err != nil {
		t.Fatalf("a document carrying a readable chart image was refused: %v", err)
	}
}
