package reportout

import "fmt"

// The picture side of the deck: the chart the frontend rasterised, the media
// part it travels in, and the shape that shows it. It is beside pptx.go
// rather than in it because that file is at the length this repository holds
// a file to.

// media is one file inside the deck. The parts are a slice and not a map so
// they are written in the order the document named them, which is what keeps
// two exports of one report byte for byte the same.
type media struct{ name, body string }

// pictures is the <p:pic> for every image in the document, by file name, and
// the media parts they are drawn from.
//
// They are built here rather than in the slide builders because a picture's
// extent comes from its own PNG header, which is the one part of a slide that
// can fail to be read.
func pictures(d Document) (map[string]string, []media, error) {
	pics := map[string]string{}
	var files []media
	for _, s := range d.Sections {
		for _, im := range s.Images {
			raw, cfg, err := im.PNG()
			if err != nil {
				return nil, nil, err
			}
			// Two parts of one zip cannot share a name, and a deck that held
			// both would be a file PowerPoint offers to repair.
			if _, taken := pics[im.Name]; taken {
				return nil, nil, fmt.Errorf("two of this report's charts are both called %s", im.Name)
			}
			cx, cy := fit(cfg.Width*emuPerPixel, cfg.Height*emuPerPixel, tableWidth, tableBottom-tableTop)
			pics[im.Name] = picture(im, cx, cy)
			files = append(files, media{"ppt/media/" + im.Name, string(raw)})
		}
	}
	return pics, files, nil
}

// fit is the largest w by h that sits inside the box without changing shape.
// A chart stretched to the box would misread its own axis.
func fit(w, h, boxW, boxH int) (int, int) {
	if w*boxH > h*boxW {
		return boxW, h * boxW / w
	}
	return w * boxH / h, boxH
}

// picture is one image on a slide, where the table would have been, with its
// description in the attribute PowerPoint reads out.
func picture(im Image, cx, cy int) string {
	return fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="%d" name="%s" descr="%s"/>`+
		`<p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr>`+
		`<p:blipFill><a:blip r:embed="rId%d"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>`+
		`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`+
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`,
		pictureShapeID, esc(im.Name), esc(im.Alt), pictureRelID, tableLeft, tableTop, cx, cy)
}
