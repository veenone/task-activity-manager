package reportout

import (
	"fmt"
	"strings"
)

// The fixed parts of a pptx: everything PowerPoint insists on that carries
// none of the report. A deck is a zip of XML and the standard library has
// both, which is why there is no new dependency here; the Go pptx libraries
// are nowhere near excelize's maturity and this file is the whole price of
// not taking one.
//
// Nothing here is decorative. A missing content type, a missing
// relationship, a master with no colour map or a theme with an incomplete
// format scheme all make PowerPoint offer to repair the file instead of
// opening it, which is why the parts are spelled out rather than trimmed to
// what a reader of the XML would call necessary.

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

const relsNS = `xmlns="http://schemas.openxmlformats.org/package/2006/relationships"`

const nsDecls = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`

const relType = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"

// emptyTree is the group shape every spTree opens with. A slide, a layout
// and a master all need one even when they hold nothing else.
const emptyTree = `<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
	`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/>` +
	`<a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`

func rootRels() string {
	return xmlHeader + `<Relationships ` + relsNS + `>` +
		`<Relationship Id="rId1" Type="` + relType + `officeDocument" Target="ppt/presentation.xml"/>` +
		`</Relationships>`
}

func contentTypes(slides int) string {
	var b strings.Builder
	b.WriteString(xmlHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		override("/ppt/presentation.xml", "presentationml.presentation.main") +
		override("/ppt/slideMasters/slideMaster1.xml", "presentationml.slideMaster") +
		override("/ppt/slideLayouts/slideLayout1.xml", "presentationml.slideLayout") +
		`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`)
	for i := 1; i <= slides; i++ {
		b.WriteString(override(fmt.Sprintf("/ppt/slides/slide%d.xml", i), "presentationml.slide"))
	}
	b.WriteString(`</Types>`)
	return b.String()
}

func override(part, kind string) string {
	return `<Override PartName="` + part + `" ContentType="application/vnd.openxmlformats-officedocument.` + kind + `+xml"/>`
}

func presentation(slides int) string {
	var ids strings.Builder
	for i := 0; i < slides; i++ {
		ids.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="rId%d"/>`, 256+i, i+2))
	}
	return xmlHeader + `<p:presentation ` + nsDecls + `>` +
		`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
		`<p:sldIdLst>` + ids.String() + `</p:sldIdLst>` +
		fmt.Sprintf(`<p:sldSz cx="%d" cy="%d"/>`, slideWidth, slideHeight) +
		`<p:notesSz cx="6858000" cy="9144000"/>` +
		`</p:presentation>`
}

func presentationRels(slides int) string {
	var b strings.Builder
	b.WriteString(xmlHeader + `<Relationships ` + relsNS + `>` +
		`<Relationship Id="rId1" Type="` + relType + `slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	for i := 1; i <= slides; i++ {
		b.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="%sslide" Target="slides/slide%d.xml"/>`, i+1, relType, i))
	}
	b.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="%stheme" Target="theme/theme1.xml"/>`, slides+2, relType))
	b.WriteString(`</Relationships>`)
	return b.String()
}

func slideMaster() string {
	return xmlHeader + `<p:sldMaster ` + nsDecls + `>` +
		`<p:cSld><p:spTree>` + emptyTree + `</p:spTree></p:cSld>` +
		`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" ` +
		`accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
		`<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>` +
		`</p:sldMaster>`
}

func slideMasterRels() string {
	return xmlHeader + `<Relationships ` + relsNS + `>` +
		`<Relationship Id="rId1" Type="` + relType + `slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
		`<Relationship Id="rId2" Type="` + relType + `theme" Target="../theme/theme1.xml"/>` +
		`</Relationships>`
}

func slideLayout() string {
	return xmlHeader + `<p:sldLayout ` + nsDecls + ` type="blank" preserve="1">` +
		`<p:cSld name="Blank"><p:spTree>` + emptyTree + `</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
		`</p:sldLayout>`
}

func slideLayoutRels() string {
	return xmlHeader + `<Relationships ` + relsNS + `>` +
		`<Relationship Id="rId1" Type="` + relType + `slideMaster" Target="../slideMasters/slideMaster1.xml"/>` +
		`</Relationships>`
}

func slideRels() string {
	return xmlHeader + `<Relationships ` + relsNS + `>` +
		`<Relationship Id="rId1" Type="` + relType + `slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
		`</Relationships>`
}

// theme1 is the Office theme, cut to what the schema requires: a colour
// scheme, a font scheme, and a format scheme with three fills, three lines,
// three effects and three background fills. PowerPoint validates the counts.
func theme() string {
	fill := `<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>`
	line := `<a:ln w="9525" cap="flat" cmpd="sng" algn="ctr">` + fill +
		`<a:prstDash val="solid"/></a:ln>`
	effect := `<a:effectStyle><a:effectLst/></a:effectStyle>`
	var scheme strings.Builder
	scheme.WriteString(`<a:fmtScheme name="Office"><a:fillStyleLst>` + fill + fill + fill + `</a:fillStyleLst>`)
	scheme.WriteString(`<a:lnStyleLst>` + line + line + line + `</a:lnStyleLst>`)
	scheme.WriteString(`<a:effectStyleLst>` + effect + effect + effect + `</a:effectStyleLst>`)
	scheme.WriteString(`<a:bgFillStyleLst>` + fill + fill + fill + `</a:bgFillStyleLst></a:fmtScheme>`)

	colours := `<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>` +
		`<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
		`<a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2>` +
		`<a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2>` +
		`<a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4>` +
		`<a:accent5><a:srgbClr val="5B9BD5"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6>` +
		`<a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink>`
	font := func(tag string) string {
		return `<a:` + tag + `><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:` + tag + `>`
	}
	return xmlHeader + `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office">` +
		`<a:themeElements><a:clrScheme name="Office">` + colours + `</a:clrScheme>` +
		`<a:fontScheme name="Office">` + font("majorFont") + font("minorFont") + `</a:fontScheme>` +
		scheme.String() + `</a:themeElements>` +
		`<a:objectDefaults/><a:extraClrSchemeLst/></a:theme>`
}
