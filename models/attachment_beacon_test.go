package models

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
	"strings"

	check "gopkg.in/check.v1"
)

// These tests build minimal, synthetic Office files entirely in code (no real
// documents, so no author/path metadata is ever committed) and assert that the
// attachment-open beacon is injected correctly for each format.

const beaconTrackURL = "https://track.example/track?a=1&rid=abc123"

func beaconCtx(enabled bool) PhishingTemplateContext {
	ptx := PhishingTemplateContext{}
	if enabled {
		ptx.TrackAttachments = true
		ptx.AttachmentTrackingURL = beaconTrackURL
	}
	return ptx
}

// officeB64 packs the given {part path: content} map into a zip and returns it
// base64-encoded, the form Attachment.Content expects.
func officeB64(parts map[string]string) string {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for name, body := range parts {
		w, err := zw.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			panic(err)
		}
	}
	zw.Close()
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// readerParts reads the Office archive produced by ApplyTemplate back into a
// {part path: content} map for assertions.
func readerParts(c *check.C, r io.Reader) map[string]string {
	data, err := ioutil.ReadAll(r)
	c.Assert(err, check.Equals, nil)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	c.Assert(err, check.Equals, nil)
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		c.Assert(err, check.Equals, nil)
		b, _ := ioutil.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
	}
	return parts
}

const relsOpen = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`

func docxParts() map[string]string {
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`</Types>`,
		"_rels/.rels": relsOpen +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
			`</Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
			`<w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": relsOpen + `</Relationships>`,
	}
}

func pptxParts() map[string]string {
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>` +
			`</Types>`,
		"_rels/.rels": relsOpen +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>` +
			`</Relationships>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
			`<p:cSld><p:spTree></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": relsOpen + `</Relationships>`,
	}
}

func xlsxParts() map[string]string {
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`</Types>`,
		"_rels/.rels": relsOpen +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
			`</Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": relsOpen +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
			`</Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheetData/></worksheet>`,
		"xl/worksheets/_rels/sheet1.xml.rels": relsOpen + `</Relationships>`,
	}
}

// --- HTML ---

func (s *ModelsSuite) TestHTMLBeaconInjectedWhenEnabled(c *check.C) {
	a := &Attachment{Name: "page.html", Content: base64.StdEncoding.EncodeToString([]byte("<html><body>Hi</body></html>"))}
	r, err := a.ApplyTemplate(beaconCtx(true))
	c.Assert(err, check.Equals, nil)
	out, _ := ioutil.ReadAll(r)
	c.Assert(strings.Contains(string(out), `src="`+beaconTrackURL+`"`), check.Equals, true)
	c.Assert(strings.Contains(string(out), `display:none`), check.Equals, true)
}

func (s *ModelsSuite) TestHTMLBeaconNotInjectedWhenDisabled(c *check.C) {
	a := &Attachment{Name: "page.html", Content: base64.StdEncoding.EncodeToString([]byte("<html><body>Hi</body></html>"))}
	r, err := a.ApplyTemplate(beaconCtx(false))
	c.Assert(err, check.Equals, nil)
	out, _ := ioutil.ReadAll(r)
	c.Assert(strings.Contains(string(out), beaconTrackURL), check.Equals, false)
}

// --- Word ---

func (s *ModelsSuite) TestDocxBeaconInjectedWhenEnabled(c *check.C) {
	a := &Attachment{Name: "doc.docx", Content: officeB64(docxParts())}
	r, err := a.ApplyTemplate(beaconCtx(true))
	c.Assert(err, check.Equals, nil)
	parts := readerParts(c, r)
	// The drawing links the external beacon relationship on the document body.
	c.Assert(strings.Contains(parts["word/document.xml"], `r:link="`+wordBeaconRelationshipID+`"`), check.Equals, true)
	// The relationship points at the tracking URL as an external target.
	rels := parts["word/_rels/document.xml.rels"]
	c.Assert(strings.Contains(rels, `Id="`+wordBeaconRelationshipID+`"`), check.Equals, true)
	c.Assert(strings.Contains(rels, `TargetMode="External"`), check.Equals, true)
	c.Assert(strings.Contains(rels, "track.example"), check.Equals, true)
}

func (s *ModelsSuite) TestDocxBeaconNotInjectedWhenDisabled(c *check.C) {
	a := &Attachment{Name: "doc.docx", Content: officeB64(docxParts())}
	r, err := a.ApplyTemplate(beaconCtx(false))
	c.Assert(err, check.Equals, nil)
	parts := readerParts(c, r)
	c.Assert(strings.Contains(parts["word/document.xml"], wordBeaconRelationshipID), check.Equals, false)
	c.Assert(strings.Contains(parts["word/_rels/document.xml.rels"], wordBeaconRelationshipID), check.Equals, false)
}

// --- PowerPoint ---

func (s *ModelsSuite) TestPptxBeaconInjectedWhenEnabled(c *check.C) {
	a := &Attachment{Name: "deck.pptx", Content: officeB64(pptxParts())}
	r, err := a.ApplyTemplate(beaconCtx(true))
	c.Assert(err, check.Equals, nil)
	parts := readerParts(c, r)
	c.Assert(strings.Contains(parts["ppt/slides/slide1.xml"], `r:link="`+wordBeaconRelationshipID+`"`), check.Equals, true)
	rels := parts["ppt/slides/_rels/slide1.xml.rels"]
	c.Assert(strings.Contains(rels, `Id="`+wordBeaconRelationshipID+`"`), check.Equals, true)
	c.Assert(strings.Contains(rels, `TargetMode="External"`), check.Equals, true)
	c.Assert(strings.Contains(rels, "track.example"), check.Equals, true)
}

func (s *ModelsSuite) TestPptxBeaconNotInjectedWhenDisabled(c *check.C) {
	a := &Attachment{Name: "deck.pptx", Content: officeB64(pptxParts())}
	r, err := a.ApplyTemplate(beaconCtx(false))
	c.Assert(err, check.Equals, nil)
	parts := readerParts(c, r)
	c.Assert(strings.Contains(parts["ppt/slides/slide1.xml"], wordBeaconRelationshipID), check.Equals, false)
}

// --- Excel ---

func (s *ModelsSuite) TestXlsxBeaconInjectedWhenEnabled(c *check.C) {
	a := &Attachment{Name: "book.xlsx", Content: officeB64(xlsxParts())}
	r, err := a.ApplyTemplate(beaconCtx(true))
	c.Assert(err, check.Equals, nil)
	parts := readerParts(c, r)
	// A <drawing> referencing the beacon relationship is injected on sheet1.
	c.Assert(strings.Contains(parts["xl/worksheets/sheet1.xml"], `r:id="`+wordBeaconRelationshipID+`"`), check.Equals, true)
	c.Assert(strings.Contains(parts["xl/worksheets/sheet1.xml"], "<drawing"), check.Equals, true)
	// sheet1's rels point at the beacon drawing part.
	c.Assert(strings.Contains(parts["xl/worksheets/_rels/sheet1.xml.rels"], "../drawings/beacon.xml"), check.Equals, true)
	// The standalone beacon drawing + its external-URL relationship exist.
	_, hasDrawing := parts["xl/drawings/beacon.xml"]
	c.Assert(hasDrawing, check.Equals, true)
	beaconRels := parts["xl/drawings/_rels/beacon.xml.rels"]
	c.Assert(strings.Contains(beaconRels, `TargetMode="External"`), check.Equals, true)
	c.Assert(strings.Contains(beaconRels, "track.example"), check.Equals, true)
	// The beacon drawing part is registered in [Content_Types].xml.
	c.Assert(strings.Contains(parts["[Content_Types].xml"], `PartName="/xl/drawings/beacon.xml"`), check.Equals, true)
}

func (s *ModelsSuite) TestXlsxBeaconNotInjectedWhenDisabled(c *check.C) {
	a := &Attachment{Name: "book.xlsx", Content: officeB64(xlsxParts())}
	r, err := a.ApplyTemplate(beaconCtx(false))
	c.Assert(err, check.Equals, nil)
	parts := readerParts(c, r)
	c.Assert(strings.Contains(parts["xl/worksheets/sheet1.xml"], wordBeaconRelationshipID), check.Equals, false)
	_, hasDrawing := parts["xl/drawings/beacon.xml"]
	c.Assert(hasDrawing, check.Equals, false)
}

// --- Error handling ---

func (s *ModelsSuite) TestOfficeAttachmentEmptyGivesClearError(c *check.C) {
	a := &Attachment{Name: "empty.docx", Content: ""}
	_, err := a.ApplyTemplate(beaconCtx(true))
	c.Assert(err, check.NotNil)
	c.Assert(strings.Contains(err.Error(), "not a valid Office file"), check.Equals, true)
}
