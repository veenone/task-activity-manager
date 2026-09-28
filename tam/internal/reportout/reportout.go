// Package reportout renders a sprint report somewhere other than the screen:
// a Confluence page, a spreadsheet and a deck.
//
// It words nothing. Every sentence and every table cell in a Document was
// built by the frontend, in lib/reportText and lib/reportTables, and the
// reason is in lib/reportDocument's own comment: the report already has one
// vocabulary, in TypeScript, and a Go renderer that built its own would be
// a second one for the same figures. So this package lays a document out
// and never decides what it says. Nothing here reads a clock, a database or
// Jira, and nothing here may ever call Jira at all.
//
// Notes are the caveats, and a renderer that has to drop something never
// drops those: the figures are rebuilt from a changelog walk and can
// disagree with Jira's own report, so numbers without their qualification
// look authoritative and are not.
package reportout

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"regexp"
)

// Table is a section's figures. Columns empty means the section has no
// table, which is how a burndown with no days or a board with no closed
// sprint travels: it says so in Lines instead.
type Table struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}

// Image is a section's chart as a picture: the file name it travels under
// inside the output, the description a reader who cannot see it is given,
// and its bytes.
//
// The frontend rasterises the SVG it already draws, because the chart's
// colours are CSS custom properties that only a browser can resolve, and
// that is also why the bytes are base64: they cross the Wails binding as
// JSON, where a Go []byte would arrive in TypeScript as an array of numbers.
//
// PNG only. One image type is one decoder to trust here, and the frontend
// has a canvas that writes one.
type Image struct {
	Name string `json:"name"`
	Alt  string `json:"alt"`
	Data string `json:"data"`
}

// imageName is the whole of a file name an image may carry. It becomes a
// part name in a zip, so a slash or a dot pair in it would write outside the
// media folder.
var imageName = regexp.MustCompile(`^[a-z0-9-]+\.png$`)

// PNG is an image's bytes and the size its own header declares, which is
// what a renderer places it at.
//
// It refuses anything it could not place (I1: what arrives over the binding
// is checked, not assumed), because a spreadsheet or a deck carrying bytes
// Office cannot decode is a file that asks the user to repair it.
func (im Image) PNG() ([]byte, image.Config, error) {
	var none image.Config
	if im.Alt == "" {
		return nil, none, errors.New("a chart image has no description, so a reader who cannot see it would be told nothing")
	}
	if !imageName.MatchString(im.Name) {
		return nil, none, fmt.Errorf("a chart image's file name %q is not a plain .png name", im.Name)
	}
	raw, err := base64.StdEncoding.DecodeString(im.Data)
	if err != nil {
		return nil, none, fmt.Errorf("the chart image %s did not arrive as readable base64: %w", im.Name, err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, none, fmt.Errorf("the chart image %s is not a PNG this can read: %w", im.Name, err)
	}
	return raw, cfg, nil
}

// Section is one part of the report: a heading, the sentences that open it,
// its figures, the caveats on those figures, and the charts drawn from them.
//
// Images is plural because one section's chart is sometimes several: the
// velocity chart splits into one panel per unit when a board changed how it
// estimates, and each panel is its own picture.
type Section struct {
	Heading string   `json:"heading"`
	Lines   []string `json:"lines"`
	Table   Table    `json:"table"`
	Notes   []string `json:"notes"`
	Images  []Image  `json:"images"`
}

// Document is a whole sprint report. Title is the sprint's own, and it is
// the Confluence page's title, the spreadsheet's first row and the deck's
// title slide.
type Document struct {
	Title    string    `json:"title"`
	Sections []Section `json:"sections"`
}

// Check refuses a document there is nothing to render (I1: what arrives
// over the binding is checked, not assumed). The frontend already refuses
// to build one for an unavailable report, and this is the second line of
// defence: an empty page, a spreadsheet of headings and a deck of zeroes
// all look like a report that says nothing rather than like no report.
func (d Document) Check() error {
	if d.Title == "" {
		return errors.New("this report has no title, so there is nothing to publish or export")
	}
	if len(d.Sections) == 0 {
		return errors.New("this report has no sections, so there is nothing to publish or export")
	}
	for _, s := range d.Sections {
		for _, im := range s.Images {
			if _, _, err := im.PNG(); err != nil {
				return err
			}
		}
	}
	return nil
}
