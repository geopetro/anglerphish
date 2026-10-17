package models

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Attachment contains the fields and methods for
// an email attachment
type Attachment struct {
	Id          int64  `json:"-"`
	TemplateId  int64  `json:"-"`
	Content     string `json:"content"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	vanillaFile bool   // Vanilla file has no template variables
}

// applyQRFallback replaces QR templates with fallback text
func (a *Attachment) applyQRFallback(content string, ptx PhishingTemplateContext) string {
	return strings.ReplaceAll(content, "{{.QR}}", ptx.QRFallbackText)
}

// processQRCodeForFileType determines the appropriate QR code replacement based on file type
func (a *Attachment) processQRCodeForFileType(content string, ptx PhishingTemplateContext, fileExtension string) string {
	if !strings.Contains(content, "{{.QR}}") {
		return content
	}

	switch fileExtension {
	case ".html":
		return strings.ReplaceAll(content, "{{.QR}}", ptx.QR)
	case ".txt", ".ics":
		return a.applyQRFallback(content, ptx)
	default:
		return a.applyQRFallback(content, ptx)
	}
}

// processQRCodeInOfficeDocument handles QR embedding in Office documents
func (a *Attachment) processQRCodeInOfficeDocument(content string, ptx PhishingTemplateContext, fileExtension string, relationshipId string) (string, string, bool) {
	// Skip metadata contexts
	if a.isMetadataContext(content) {
		return a.applyQRFallback(content, ptx), "", false
	}

	switch fileExtension {
	case ".docx", ".docm":
		if strings.Contains(content, "{{.QR}}") && ptx.QRImageData != nil {
			processedContent, imageName, success := a.embedQRInWordDocument(content, ptx, relationshipId)
			if success {
				return processedContent, imageName, true
			}
			return a.applyQRFallback(content, ptx), "", false
		}
		return content, "", false

	case ".xlsx", ".xlsm":
		if ptx.QRImageData != nil {
			processedContent, imageName, success := a.embedQRInExcelDocument(content, ptx, relationshipId)
			if success {
				return processedContent, imageName, true
			}
		}
		if strings.Contains(content, "{{.QR}}") {
			return a.applyQRFallback(content, ptx), "", false
		}
		return content, "", false

	case ".pptx":
		if strings.Contains(content, "{{.QR}}") && ptx.QRImageData != nil {
			processedContent, imageName, success := a.embedQRInPowerPointDocument(content, ptx, relationshipId)
			if success {
				return processedContent, imageName, true
			}
			return a.applyQRFallback(content, ptx), "", false
		}
		return content, "", false

	default:
		return a.applyQRFallback(content, ptx), "", false
	}
}

// isMetadataContext checks if content is in metadata context
func (a *Attachment) isMetadataContext(content string) bool {
	metadataIndicators := []string{
		"<dc:creator>",
		"<dc:title>",
		"<dc:subject>",
		"<dc:description>",
		"<cp:lastModifiedBy>",
		"<cp:revision>",
		"<cp:category>",
		"<cp:keywords>",
		"<Application>",
		"<Company>",
		"<Manager>",
	}

	for _, indicator := range metadataIndicators {
		if strings.Contains(content, indicator) {
			return true
		}
	}
	return false
}

// embedQRInWordDocument handles QR code embedding in Word documents
func (a *Attachment) embedQRInWordDocument(content string, ptx PhishingTemplateContext, relationshipId string) (string, string, bool) {
	// Only embed in document body content
	if !strings.Contains(content, "<w:body>") && !strings.Contains(content, "<w:p>") {
		return content, "", false
	}

	imageName := "qrcode_image.png"
	qrSizeEMU := a.calculateQRSizeEMU(ptx)
	imageXML := `</w:t></w:r><w:r><w:drawing>
      <wp:inline distT="0" distB="0" distL="0" distR="0" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">
        <wp:extent cx="` + qrSizeEMU + `" cy="` + qrSizeEMU + `"/>
        <wp:effectExtent l="0" t="0" r="0" b="0"/>
        <wp:docPr id="1" name="QR Code"/>
        <wp:cNvGraphicFramePr>
          <a:graphicFrameLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" noChangeAspect="1"/>
        </wp:cNvGraphicFramePr>
        <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
          <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
            <pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
              <pic:nvPicPr>
                <pic:cNvPr id="1" name="QR Code"/>
                <pic:cNvPicPr/>
              </pic:nvPicPr>
              <pic:blipFill>
                <a:blip r:embed="` + relationshipId + `" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
                <a:stretch>
                  <a:fillRect/>
                </a:stretch>
              </pic:blipFill>
              <pic:spPr>
                <a:xfrm>
                  <a:off x="0" y="0"/>
                  <a:ext cx="` + qrSizeEMU + `" cy="` + qrSizeEMU + `"/>
                </a:xfrm>
                <a:prstGeom prst="rect">
                  <a:avLst/>
                </a:prstGeom>
              </pic:spPr>
            </pic:pic>
          </a:graphicData>
        </a:graphic>
      </wp:inline>
    </w:drawing></w:r><w:r><w:t>`

	processedContent := strings.ReplaceAll(content, "{{.QR}}", imageXML)
	if processedContent == content {
		return content, "", false
	}

	return processedContent, imageName, true
}

// embedQRInExcelDocument handles QR code embedding in Excel documents
func (a *Attachment) embedQRInExcelDocument(content string, ptx PhishingTemplateContext, relationshipId string) (string, string, bool) {
	imageName := "qrcode_image.png"

	// Handle shared strings file
	if strings.Contains(content, "<sst") && strings.Contains(content, "{{.QR}}") {
		processedContent := strings.ReplaceAll(content, "{{.QR}}", " ")
		return processedContent, imageName, true
	}

	// Handle worksheet content
	if strings.Contains(content, "<worksheet") {
		if !strings.Contains(content, "<drawing") && ptx.QRImageData != nil {
			drawingXML := `<drawing r:id="` + relationshipId + `"/>`
			processedContent := strings.Replace(content, "</worksheet>", drawingXML+"</worksheet>", 1)
			if processedContent != content {
				return processedContent, imageName, true
			}
		}
	}

	return content, "", false
}

// embedQRInPowerPointDocument handles QR code embedding in PowerPoint documents
func (a *Attachment) embedQRInPowerPointDocument(content string, ptx PhishingTemplateContext, relationshipId string) (string, string, bool) {
	imageName := "qrcode_image.png"

	// Only process slide content
	if !strings.Contains(content, "<p:sld") {
		return content, "", false
	}

	// Check if this slide contains QR template
	if !strings.Contains(content, "{{.QR}}") {
		return content, "", false
	}

	qrSizeEMU := a.calculateQRSizeEMU(ptx)

	// Find the text shape containing {{.QR}} and extract its position
	// We need to replace the entire <p:sp> element, not just the text
	processedContent := content

	// Use regex to find the <p:sp> element that contains {{.QR}}
	// This pattern matches a complete shape element containing the QR template
	shapePattern := `<p:sp>.*?<a:t>{{\.QR}}</a:t>.*?</p:sp>`
	re := regexp.MustCompile(shapePattern)

	shapeMatch := re.FindString(processedContent)
	if shapeMatch == "" {
		return content, "", false
	}

	// Extract position from the matched shape (look within the specific shape)
	// Default position if we can't extract it
	xPos := "1524000" // Default X position
	yPos := "1524000" // Default Y position

	// Try to extract position from the matched shape
	posPattern := `<a:off x="(\d+)" y="(\d+)"/>`
	posRe := regexp.MustCompile(posPattern)
	if matches := posRe.FindStringSubmatch(shapeMatch); len(matches) == 3 {
		xPos = matches[1]
		yPos = matches[2]
	}

	// Create the replacement image shape with complete PowerPoint structure
	imageShapeXML := `<p:pic>
      <p:nvPicPr>
        <p:cNvPr id="999" name="QR Code">
          <a:extLst>
            <a:ext uri="{FF2B5EF4-FFF2-40B4-BE49-F238E27FC236}">
              <a16:creationId xmlns:a16="http://schemas.microsoft.com/office/drawing/2014/main" id="{9B87BFE3-F154-86B2-42A3-EA887281009D}"/>
            </a:ext>
          </a:extLst>
        </p:cNvPr>
        <p:cNvPicPr>
          <a:picLocks noChangeAspect="1"/>
        </p:cNvPicPr>
        <p:nvPr/>
      </p:nvPicPr>
      <p:blipFill>
        <a:blip r:embed="` + relationshipId + `">
          <a:extLst>
            <a:ext uri="{28A0092B-C50C-407E-A947-70E740481C1C}">
              <a14:useLocalDpi xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main" val="0"/>
            </a:ext>
          </a:extLst>
        </a:blip>
        <a:stretch>
          <a:fillRect/>
        </a:stretch>
      </p:blipFill>
      <p:spPr>
        <a:xfrm>
          <a:off x="` + xPos + `" y="` + yPos + `"/>
          <a:ext cx="` + qrSizeEMU + `" cy="` + qrSizeEMU + `"/>
        </a:xfrm>
        <a:prstGeom prst="rect">
          <a:avLst/>
        </a:prstGeom>
      </p:spPr>
    </p:pic>`

	// Replace the entire text shape with the image shape
	processedContent = re.ReplaceAllString(processedContent, imageShapeXML)

	if processedContent == content {
		return content, "", false
	}

	return processedContent, imageName, true
}

// calculateQRSizeEMU calculates QR code size in EMUs
func (a *Attachment) calculateQRSizeEMU(ptx PhishingTemplateContext) string {
	defaultSizePixels := 100
	sizePixels := defaultSizePixels

	// Use the actual QR size if provided
	if ptx.QRSize != "" {
		if parsedSize, err := strconv.Atoi(ptx.QRSize); err == nil && parsedSize > 0 {
			sizePixels = parsedSize
		}
	}

	sizeEMU := sizePixels * 9525 // Convert pixels to EMUs (96 DPI)
	return fmt.Sprintf("%d", sizeEMU)
}

// getMediaPath returns the appropriate media path for different Office document types
func (a *Attachment) getMediaPath(fileExtension string, imageName string) string {
	switch fileExtension {
	case ".docx", ".docm":
		return "word/media/" + imageName
	case ".xlsx", ".xlsm":
		return "xl/media/" + imageName
	case ".pptx":
		return "ppt/media/" + imageName
	default:
		return "media/" + imageName
	}
}

// createExcelDrawingXML creates the Excel drawing XML content, anchoring the
// QR picture at the given zero-indexed col/row (so the picture lands exactly
// where the author placed the {{.QR}} marker in the sheet).
func (a *Attachment) createExcelDrawingXML(imageRelId string, ptx PhishingTemplateContext, col, row int) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  ` + a.excelQRAnchorXML(imageRelId, ptx, col, row) + `
</xdr:wsDr>`
}

// excelQRAnchorXML returns the single <xdr:oneCellAnchor> element for the QR
// picture. Reusable so the same anchor can either stand alone in a brand-new
// drawing part (via createExcelDrawingXML) or be spliced into a user's existing
// drawing part (merge case).
func (a *Attachment) excelQRAnchorXML(imageRelId string, ptx PhishingTemplateContext, col, row int) string {
	qrSizeEMU := a.calculateQRSizeEMU(ptx)
	return `<xdr:oneCellAnchor>
    <xdr:from>
      <xdr:col>` + strconv.Itoa(col) + `</xdr:col>
      <xdr:colOff>0</xdr:colOff>
      <xdr:row>` + strconv.Itoa(row) + `</xdr:row>
      <xdr:rowOff>0</xdr:rowOff>
    </xdr:from>
    <xdr:ext cx="` + qrSizeEMU + `" cy="` + qrSizeEMU + `"/>
    <xdr:pic>
      <xdr:nvPicPr>
        <xdr:cNvPr id="9997" name="QR Code"/>
        <xdr:cNvPicPr/>
      </xdr:nvPicPr>
      <xdr:blipFill>
        <a:blip r:embed="` + imageRelId + `" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
        <a:stretch>
          <a:fillRect/>
        </a:stretch>
      </xdr:blipFill>
      <xdr:spPr>
        <a:xfrm>
          <a:off x="0" y="0"/>
          <a:ext cx="` + qrSizeEMU + `" cy="` + qrSizeEMU + `"/>
        </a:xfrm>
        <a:prstGeom prst="rect">
          <a:avLst/>
        </a:prstGeom>
      </xdr:spPr>
    </xdr:pic>
    <xdr:clientData/>
  </xdr:oneCellAnchor>`
}

// createExcelDrawingRelsXML creates the Excel drawing relationships XML content
func (a *Attachment) createExcelDrawingRelsXML(imageName string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId999" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/` + imageName + `"/>
</Relationships>`
}

// createExcelWorksheetRelsXML creates the Excel worksheet relationships XML content
func (a *Attachment) createExcelWorksheetRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId999" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>
</Relationships>`
}

// createPowerPointSlideRelsXML creates the PowerPoint slide relationships XML content
// This preserves the slideLayout relationship and adds the image relationship
func (a *Attachment) createPowerPointSlideRelsXML(imageName string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId999" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/` + imageName + `"/>
</Relationships>`
}

// extractSheetName extracts the sheet name from a worksheet file path
// e.g., "xl/worksheets/sheet1.xml" -> "sheet1"
func (a *Attachment) extractSheetName(filePath string) string {
	if !strings.Contains(filePath, "xl/worksheets/") {
		return ""
	}

	// Extract filename from path
	filename := filepath.Base(filePath)

	// Remove .xml extension
	if strings.HasSuffix(filename, ".xml") {
		filename = strings.TrimSuffix(filename, ".xml")
	}

	// Return sheet name (e.g., "sheet1", "sheet2", etc.)
	if strings.HasPrefix(filename, "sheet") {
		return filename
	}

	return ""
}

// extractSlideName extracts the slide name from a PowerPoint slide file path
// e.g., "ppt/slides/slide1.xml" -> "slide1"
func (a *Attachment) extractSlideName(filePath string) string {
	if !strings.Contains(filePath, "ppt/slides/") {
		return ""
	}

	// Extract filename from path
	filename := filepath.Base(filePath)

	// Remove .xml extension
	if strings.HasSuffix(filename, ".xml") {
		filename = strings.TrimSuffix(filename, ".xml")
	}

	// Return slide name (e.g., "slide1", "slide2", etc.)
	if strings.HasPrefix(filename, "slide") {
		return filename
	}

	return ""
}

// cellRefToColRow parses an Excel cell reference like "D5" or "AB12" into
// zero-indexed (col, row). "A1" → (0, 0), "B3" → (1, 2), "AA10" → (26, 9).
// Returns (-1, -1, false) on malformed input.
func cellRefToColRow(ref string) (int, int, bool) {
	col := 0
	i := 0
	for i < len(ref) {
		c := ref[i]
		if c >= 'A' && c <= 'Z' {
			col = col*26 + int(c-'A') + 1
			i++
			continue
		}
		if c >= 'a' && c <= 'z' {
			col = col*26 + int(c-'a') + 1
			i++
			continue
		}
		break
	}
	if col == 0 || i == len(ref) {
		return -1, -1, false
	}
	row, err := strconv.Atoi(ref[i:])
	if err != nil || row < 1 {
		return -1, -1, false
	}
	return col - 1, row - 1, true
}

// findExcelQRCellPosition scans sheet1.xml to locate the cell that holds
// `{{.QR}}` — either inline (`<c t="inlineStr"><is><t>{{.QR}}</t></is></c>`)
// or via a sharedStrings reference whose <si> contains `{{.QR}}`. Returns the
// zero-indexed (col, row) of the first match plus true; returns false if no
// cell actually renders `{{.QR}}`. An orphan `{{.QR}}` in sharedStrings that
// no cell references returns false by design — we don't make up a position.
func (a *Attachment) findExcelQRCellPosition(zipReader *zip.Reader) (int, int, bool) {
	var sheet1, sharedStrings string
	for _, zf := range zipReader.File {
		switch zf.Name {
		case "xl/worksheets/sheet1.xml":
			rc, err := zf.Open()
			if err != nil {
				continue
			}
			b, _ := ioutil.ReadAll(rc)
			rc.Close()
			sheet1 = string(b)
		case "xl/sharedStrings.xml":
			rc, err := zf.Open()
			if err != nil {
				continue
			}
			b, _ := ioutil.ReadAll(rc)
			rc.Close()
			sharedStrings = string(b)
		}
	}
	if sheet1 == "" {
		return -1, -1, false
	}

	// 1. Inline string case: find `<c r="XX" t="inlineStr">...<t>{{.QR}}</t>...</c>`.
	inlineRe := regexp.MustCompile(`<c\s[^>]*r="([^"]+)"[^>]*t="inlineStr"[^>]*>\s*<is>\s*<t[^>]*>\s*\{\{\.QR\}\}\s*</t>`)
	if m := inlineRe.FindStringSubmatch(sheet1); len(m) >= 2 {
		if col, row, ok := cellRefToColRow(m[1]); ok {
			return col, row, true
		}
	}

	// 2. Shared-string case: find the <si> index that contains `{{.QR}}`, then
	// find the first <c t="s"><v>N</v></c> that references it.
	if sharedStrings == "" {
		return -1, -1, false
	}
	siRe := regexp.MustCompile(`<si\b[^>]*>(?:[\s\S]*?)</si>`)
	for idx, si := range siRe.FindAllString(sharedStrings, -1) {
		if !strings.Contains(si, "{{.QR}}") {
			continue
		}
		cellRe := regexp.MustCompile(`<c\s[^>]*r="([^"]+)"[^>]*t="s"[^>]*>\s*<v>\s*` + strconv.Itoa(idx) + `\s*</v>\s*</c>`)
		if m := cellRe.FindStringSubmatch(sheet1); len(m) >= 2 {
			if col, row, ok := cellRefToColRow(m[1]); ok {
				return col, row, true
			}
		}
	}
	return -1, -1, false
}

// findExcelSheet1DrawingTarget scans sheet1 and its _rels to find the drawing
// part sheet1 already references, if any. Returns (true, drawingPath,
// drawingRelsPath) when sheet1 has a resolvable <drawing r:id="X"/>, with
// paths normalised to archive-root (e.g. "xl/drawings/drawing1.xml"); returns
// (false, "", "") otherwise. The beacon needs this so it can merge its own
// anchor into whichever drawing part Excel will render on open, rather than
// adding a second <drawing> element (invalid OOXML, one per worksheet).
func (a *Attachment) findExcelSheet1DrawingTarget(zipReader *zip.Reader) (bool, string, string) {
	var sheet1, sheet1Rels string
	for _, zf := range zipReader.File {
		switch zf.Name {
		case "xl/worksheets/sheet1.xml":
			rc, err := zf.Open()
			if err != nil {
				continue
			}
			b, _ := ioutil.ReadAll(rc)
			rc.Close()
			sheet1 = string(b)
		case "xl/worksheets/_rels/sheet1.xml.rels":
			rc, err := zf.Open()
			if err != nil {
				continue
			}
			b, _ := ioutil.ReadAll(rc)
			rc.Close()
			sheet1Rels = string(b)
		}
	}
	if sheet1 == "" || sheet1Rels == "" {
		return false, "", ""
	}
	drawingIDRe := regexp.MustCompile(`<drawing\s[^>]*r:id="([^"]+)"`)
	m := drawingIDRe.FindStringSubmatch(sheet1)
	if len(m) < 2 {
		return false, "", ""
	}
	rid := m[1]
	relRe := regexp.MustCompile(`<Relationship\s[^>]*Id="` + regexp.QuoteMeta(rid) + `"[^>]*Target="([^"]+)"`)
	rm := relRe.FindStringSubmatch(sheet1Rels)
	if len(rm) < 2 {
		return false, "", ""
	}
	target := rm[1]
	// Target is relative to xl/worksheets/_rels/, e.g. "../drawings/drawing1.xml".
	base := "xl/worksheets/"
	for strings.HasPrefix(target, "../") {
		target = target[3:]
		base = path.Dir(strings.TrimSuffix(base, "/")) + "/"
	}
	drawingPath := base + target
	drawingDir := path.Dir(drawingPath)
	drawingFile := path.Base(drawingPath)
	drawingRelsPath := drawingDir + "/_rels/" + drawingFile + ".rels"
	return true, drawingPath, drawingRelsPath
}

// injectHTMLBeacon appends an invisible remote-image beacon to an HTML
// attachment. The image fetch records an "Attachment Opened" event.
func injectHTMLBeacon(html, url string) string {
	img := `<img src="` + url + `" width="1" height="1" style="display:none" alt="">`
	lower := strings.ToLower(html)
	if idx := strings.LastIndex(lower, "</body>"); idx != -1 {
		return html[:idx] + img + html[idx:]
	}
	return html + img
}

const wordBeaconRelationshipID = "rIdBeacon"

// escapeXMLURL escapes a URL for use inside an XML attribute value.
func escapeXMLURL(url string) string {
	return strings.ReplaceAll(url, "&", "&amp;")
}

// wordBeaconRelationshipXML returns an external image relationship entry
// pointing at the beacon URL.
func wordBeaconRelationshipXML(url string) string {
	return `<Relationship Id="` + wordBeaconRelationshipID +
		`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="` +
		escapeXMLURL(url) + `" TargetMode="External"/>`
}

// wordBeaconDrawingXML returns a 1x1 EMU hidden inline drawing that links the
// external beacon image (r:link, not r:embed — nothing is packaged).
func wordBeaconDrawingXML() string {
	return `<w:p><w:r><w:drawing>
	  <wp:inline distT="0" distB="0" distL="0" distR="0" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">
	    <wp:extent cx="1" cy="1"/>
	    <wp:docPr id="9998" name="b"/>
	    <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
	      <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
	        <pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
	          <pic:nvPicPr><pic:cNvPr id="9998" name="b"/><pic:cNvPicPr/></pic:nvPicPr>
	          <pic:blipFill>
	            <a:blip r:link="` + wordBeaconRelationshipID + `" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
	            <a:stretch><a:fillRect/></a:stretch>
	          </pic:blipFill>
	          <pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="1" cy="1"/></a:xfrm>
	          <a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>
	        </pic:pic>
	      </a:graphicData>
	    </a:graphic>
	  </wp:inline>
	</w:drawing></w:r></w:p>`
}

// excelBeaconAnchorXML is a single 1-pixel anchor for the external beacon
// image. Dimensions are in EMUs (9525 EMU = 1 pixel at 96 DPI). Microsoft
// Excel's validator rejects sub-pixel sizes (e.g. 1 EMU) and may ask the user
// to "repair" the file, so we use the Excel-native minimum here even though
// the anchor lives on a veryHidden sheet nobody sees.
func excelBeaconAnchorXML() string {
	// xmlns:a is declared on the parent <xdr:wsDr>, so <a:picLocks> inherits
	// it — redeclaring it inline is redundant and Excel-produced drawings
	// never do that.
	return `<xdr:oneCellAnchor>
    <xdr:from><xdr:col>0</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>0</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>
    <xdr:ext cx="9525" cy="9525"/>
    <xdr:pic>
      <xdr:nvPicPr><xdr:cNvPr id="9998" name="Beacon"/><xdr:cNvPicPr><a:picLocks noChangeAspect="1"/></xdr:cNvPicPr></xdr:nvPicPr>
      <xdr:blipFill><a:blip xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:link="` + wordBeaconRelationshipID + `"/><a:stretch><a:fillRect/></a:stretch></xdr:blipFill>
      <xdr:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="9525" cy="9525"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></xdr:spPr>
    </xdr:pic>
    <xdr:clientData/>
  </xdr:oneCellAnchor>`
}

// excelBeaconDrawingXML is a standalone drawing part holding only the beacon.
func excelBeaconDrawingXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  ` + excelBeaconAnchorXML() + `
</xdr:wsDr>`
}

// excelBeaconDrawingRelsXML points the drawing's relationship at the external URL.
func excelBeaconDrawingRelsXML(url string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="` + wordBeaconRelationshipID + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="` + escapeXMLURL(url) + `" TargetMode="External"/>
</Relationships>`
}

// excelBeaconExternalRelXML returns the <Relationship> element used when the
// beacon anchor is merged into an existing drawing part's rels file. The
// relationship id is the globally-unique beacon id so it never collides with
// relationships already in that rels file.
func excelBeaconExternalRelXML(url string) string {
	return `<Relationship Id="` + wordBeaconRelationshipID + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="` + escapeXMLURL(url) + `" TargetMode="External"/>`
}

// pptBeaconPicXML is a 1x1 picture on a slide linking the external beacon.
func pptBeaconPicXML() string {
	return `<p:pic>
	  <p:nvPicPr><p:cNvPr id="9998" name="b"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>
	  <p:blipFill><a:blip xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:link="` + wordBeaconRelationshipID + `"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>
	  <p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="1" cy="1"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
	</p:pic>`
}

// Validate ensures that the provided attachment uses the supported template variables correctly.
func (a Attachment) Validate() error {
	vc := ValidationContext{
		FromAddress: "foo@bar.com",
		BaseURL:     "http://example.com",
	}
	td := Result{
		BaseRecipient: BaseRecipient{
			Email:     "foo@bar.com",
			FirstName: "Foo",
			LastName:  "Bar",
			Position:  "Test",
			Custom:    "Custom",
		},
		RId: "123456",
	}
	ptx, err := NewPhishingTemplateContext(vc, td.BaseRecipient, td.RId)
	if err != nil {
		return err
	}
	// The filename may itself contain template variables, so validate it too.
	if _, err = ExecuteTemplate(a.Name, ptx); err != nil {
		return err
	}
	_, err = a.ApplyTemplate(ptx)
	return err
}

// ApplyTemplate parses different attachment files and applies the supplied phishing template.
func (a *Attachment) ApplyTemplate(ptx PhishingTemplateContext) (io.Reader, error) {

	decodedAttachment := base64.NewDecoder(base64.StdEncoding, strings.NewReader(a.Content))

	// If we've already determined there are no template variables in this attachment return it immediately
	if a.vanillaFile == true {
		return decodedAttachment, nil
	}

	// Decided to use the file extension rather than the content type, as there seems to be quite
	//  a bit of variability with types. e.g sometimes a Word docx file would have:
	//   "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	fileExtension := filepath.Ext(a.Name)

	switch fileExtension {

	case ".docx", ".docm", ".pptx", ".xlsx", ".xlsm":
		// Most modern office formats are xml based and can be unarchived.
		// .docm and .xlsm files are comprised of xml, and a binary blob for the macro code

		// Zip archives require random access for reading, so it's hard to stream bytes. Solution seems to be to use a buffer.
		// See https://stackoverflow.com/questions/16946978/how-to-unzip-io-readcloser
		b := new(bytes.Buffer)
		b.ReadFrom(decodedAttachment)
		zipReader, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len())) // Create a new zip reader from the file

		if err != nil {
			// A modern Office file is a zip archive; this fails when the file is
			// empty (e.g. a 0-byte placeholder) or not a real .docx/.xlsx/.pptx.
			return nil, fmt.Errorf("not a valid Office file (empty or corrupt): %w", err)
		}

		newZipArchive := new(bytes.Buffer)
		zipWriter := zip.NewWriter(newZipArchive) // For writing the new archive

		// Check if QR code needs to be embedded - PHASE 1: Detection
		qrImageEmbedded := false
		qrImageName := ""
		qrRelationshipId := "rId999"             // Use a high ID to avoid conflicts
		qrEnabledSheets := make(map[string]bool) // Track which sheets got QR codes
		qrEnabledSlides := make(map[string]bool) // Track which slides got QR codes

		// Pre-scan to detect if any file contains QR template and we have QR image data
		hasQRCode := false
		if ptx.QRImageData != nil {
			for _, zipFile := range zipReader.File {
				ff, err := zipFile.Open()
				if err != nil {
					continue
				}
				contents, err := ioutil.ReadAll(ff)
				ff.Close()
				if err != nil {
					continue
				}
				if strings.Contains(string(contents), "{{.QR}}") {
					hasQRCode = true
					break
				}
			}
		}

		// For Excel: find the sheet1 cell that references `{{.QR}}` so the QR
		// picture lands exactly where the author placed the marker. If the
		// marker isn't actually referenced by any cell (orphan sharedString),
		// qrInCell stays false and we skip QR embedding entirely rather than
		// placing the image at some arbitrary default location.
		var (
			qrInCell  bool
			qrCellCol int
			qrCellRow int
		)
		if hasQRCode && (fileExtension == ".xlsx" || fileExtension == ".xlsm") {
			qrCellCol, qrCellRow, qrInCell = a.findExcelQRCellPosition(zipReader)
			if !qrInCell {
				// No cell references {{.QR}} — don't embed QR for this attachment.
				hasQRCode = false
			}
		}

		// Track beacon injection state for Word so we can create a document
		// relationships file if the original has none (otherwise the drawing
		// reference would dangle and corrupt the file).
		beaconWordInjected := false
		beaconWordRelsHandled := false

		// Excel beacon state. The beacon has to live on sheet1 (or whatever
		// Excel renders by default on open) because Excel defers fetching
		// external linked images on sheets that are not actively displayed —
		// and a non-rendered sheet never fires its beacon. Two cases:
		//   - sheet1 already has a <drawing> (user's picture or chart): merge
		//     the beacon anchor into THAT drawing part (one drawing per sheet
		//     is the OOXML rule).
		//   - sheet1 has no <drawing>: inject our own <drawing> on sheet1 and
		//     emit a dedicated beacon.xml drawing part.
		beaconExcelEnabled := (fileExtension == ".xlsx" || fileExtension == ".xlsm") &&
			ptx.TrackAttachments && ptx.AttachmentTrackingURL != ""
		// Pre-scan sheet1's drawing part (if any). Both beacon and QR need it
		// so they can merge into the user's existing drawing part rather than
		// collide with it.
		var (
			sheet1HasDrawing             bool
			sheet1DrawingPath            string // e.g. "xl/drawings/drawing1.xml"
			sheet1DrawingRelsPath        string // e.g. "xl/drawings/_rels/drawing1.xml.rels"
			beaconExcelInjectedOnSheet1  bool   // in-loop: we added <drawing> to sheet1
			beaconExcelSheet1RelsHandled bool   // in-loop: we added rIdBeacon to sheet1 rels
		)
		if fileExtension == ".xlsx" || fileExtension == ".xlsm" {
			sheet1HasDrawing, sheet1DrawingPath, sheet1DrawingRelsPath =
				a.findExcelSheet1DrawingTarget(zipReader)
		}
		beaconExcelMerge := beaconExcelEnabled && sheet1HasDrawing

		// Track every entry name written to the output archive. ZIP permits
		// duplicate names, but Microsoft Excel treats a workbook containing
		// them as corrupt and prompts to repair, so every post-loop writer
		// below checks this map before emitting its name.
		writtenNames := map[string]bool{}

		// i. Read each file from the Office document archive
		// ii. Apply the template to it with QR code handling
		// iii. Add the templated content to a new zip archive
		a.vanillaFile = true
		for _, zipFile := range zipReader.File {
			ff, err := zipFile.Open()
			if err != nil {
				return nil, err
			}
			defer ff.Close()
			contents, err := ioutil.ReadAll(ff)
			if err != nil {
				return nil, err
			}
			subFileExtension := filepath.Ext(zipFile.Name)
			var tFile string
			if subFileExtension == ".xml" || subFileExtension == ".rels" { // Process XML and relationship files
				// First we look for instances where Office has URL escaped our template variables. This seems to happen when inserting a remote image, converting {{.Foo}} to %7b%7b.foo%7d%7d.
				// See https://stackoverflow.com/questions/68287630/disable-url-encoding-for-includepicture-in-microsoft-word
				rx, _ := regexp.Compile("%7b%7b.([a-zA-Z]+)%7d%7d")
				contents := rx.ReplaceAllFunc(contents, func(m []byte) []byte {
					d, err := url.QueryUnescape(string(m))
					if err != nil {
						return m
					}
					return []byte(d)
				})

				// Handle QR code embedding for Office documents
				contentStr := string(contents)

				// Handle relationship files for Word documents
				if qrImageEmbedded && strings.Contains(zipFile.Name, "document.xml.rels") && fileExtension == ".docx" {
					// Add relationship for the QR code image
					if strings.Contains(contentStr, "</Relationships>") {
						relationshipXML := `<Relationship Id="` + qrRelationshipId + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/` + qrImageName + `"/>`
						contentStr = strings.Replace(contentStr, "</Relationships>", relationshipXML+"</Relationships>", 1)
					}
				}

				// Handle relationship files for Excel documents
				if qrImageEmbedded && strings.Contains(zipFile.Name, "worksheet") && strings.Contains(zipFile.Name, ".rels") && (fileExtension == ".xlsx" || fileExtension == ".xlsm") {
					// Add relationship for the QR code image in Excel worksheet
					if strings.Contains(contentStr, "</Relationships>") {
						relationshipXML := `<Relationship Id="` + qrRelationshipId + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>`
						contentStr = strings.Replace(contentStr, "</Relationships>", relationshipXML+"</Relationships>", 1)
					}
				}

				// Handle relationship files for PowerPoint slides
				if qrImageEmbedded && strings.Contains(zipFile.Name, "ppt/slides/_rels/") && strings.Contains(zipFile.Name, ".rels") && fileExtension == ".pptx" {
					// Add relationship for the QR code image in PowerPoint slide (preserve existing relationships)
					if strings.Contains(contentStr, "</Relationships>") {
						relationshipXML := `<Relationship Id="` + qrRelationshipId + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/` + qrImageName + `"/>`
						contentStr = strings.Replace(contentStr, "</Relationships>", relationshipXML+"</Relationships>", 1)
					}
				}

				// Handle Content Types file to register PNG images and drawing files
				if zipFile.Name == "[Content_Types].xml" && ptx.QRImageData != nil {
					// Add PNG content type if not already present
					if !strings.Contains(contentStr, `Extension="png"`) {
						if strings.Contains(contentStr, "</Types>") {
							pngContentType := `<Default Extension="png" ContentType="image/png"/>`
							contentStr = strings.Replace(contentStr, "</Types>", pngContentType+"\n</Types>", 1)
						}
					}
					// Add drawing content type override for Excel
					if (fileExtension == ".xlsx" || fileExtension == ".xlsm") && !strings.Contains(contentStr, `PartName="/xl/drawings/drawing1.xml"`) {
						if strings.Contains(contentStr, "</Types>") {
							drawingContentType := `<Override PartName="/xl/drawings/drawing1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/>`
							contentStr = strings.Replace(contentStr, "</Types>", drawingContentType+"\n</Types>", 1)
						}
					}
				}

				// Excel beacon handling. Beacon must live on sheet1 (what Excel
				// renders on open), so we either merge our 1x1 anchor into the
				// drawing part sheet1 already references, or inject our own
				// drawing on sheet1 if it has none. Case B (QR adds a new
				// drawing on an empty sheet1) is handled in the post-loop QR
				// drawing creation.
				if beaconExcelEnabled {
					switch {
					// Merge path: sheet1 already references an existing drawing.
					case beaconExcelMerge && zipFile.Name == sheet1DrawingPath &&
						strings.Contains(contentStr, "</xdr:wsDr>"):
						contentStr = strings.Replace(contentStr, "</xdr:wsDr>",
							excelBeaconAnchorXML()+"</xdr:wsDr>", 1)
						a.vanillaFile = false
					case beaconExcelMerge && zipFile.Name == sheet1DrawingRelsPath &&
						strings.Contains(contentStr, "</Relationships>"):
						contentStr = strings.Replace(contentStr, "</Relationships>",
							excelBeaconExternalRelXML(ptx.AttachmentTrackingURL)+"</Relationships>", 1)
						a.vanillaFile = false
					// Inject path: sheet1 has no drawing (and no QR will add one).
					// If QR will add one, we defer and merge into it post-loop.
					case !beaconExcelMerge && !hasQRCode &&
						zipFile.Name == "xl/worksheets/sheet1.xml" &&
						!strings.Contains(contentStr, "<drawing ") &&
						strings.Contains(contentStr, "</worksheet>"):
						contentStr = strings.Replace(contentStr, "</worksheet>",
							`<drawing xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="`+wordBeaconRelationshipID+`"/></worksheet>`, 1)
						beaconExcelInjectedOnSheet1 = true
						a.vanillaFile = false
					case !beaconExcelMerge && !hasQRCode &&
						zipFile.Name == "xl/worksheets/_rels/sheet1.xml.rels" &&
						strings.Contains(contentStr, "</Relationships>"):
						contentStr = strings.Replace(contentStr, "</Relationships>",
							`<Relationship Id="`+wordBeaconRelationshipID+`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/beacon.xml"/></Relationships>`, 1)
						beaconExcelSheet1RelsHandled = true
						a.vanillaFile = false
					// Register beacon.xml content-type override when we're
					// hosting our own drawing (not merging into an existing one).
					case !beaconExcelMerge && zipFile.Name == "[Content_Types].xml" &&
						!strings.Contains(contentStr, `PartName="/xl/drawings/beacon.xml"`) &&
						strings.Contains(contentStr, "</Types>"):
						override := `<Override PartName="/xl/drawings/beacon.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/>`
						contentStr = strings.Replace(contentStr, "</Types>", override+"\n</Types>", 1)
					}
				}

				// QR merge path: when sheet1 already has an existing drawing
				// (so the post-loop QR drawing creation would be skipped by
				// writeOnce), splice the QR picture anchor + relationship into
				// the user's existing drawing part during the loop. Same
				// one-drawing-per-sheet OOXML rule the beacon handles.
				if hasQRCode && sheet1HasDrawing &&
					(fileExtension == ".xlsx" || fileExtension == ".xlsm") {
					switch {
					case zipFile.Name == sheet1DrawingPath &&
						strings.Contains(contentStr, "</xdr:wsDr>"):
						contentStr = strings.Replace(contentStr, "</xdr:wsDr>",
							a.excelQRAnchorXML(qrRelationshipId, ptx, qrCellCol, qrCellRow)+"</xdr:wsDr>", 1)
						// Mark QR as embedded so the post-loop writes the
						// qrcode_image.png media file.
						if !qrImageEmbedded {
							qrImageEmbedded = true
							qrImageName = "qrcode_image.png"
						}
						a.vanillaFile = false
					case zipFile.Name == sheet1DrawingRelsPath &&
						strings.Contains(contentStr, "</Relationships>"):
						qrRelXML := `<Relationship Id="` + qrRelationshipId + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/qrcode_image.png"/>`
						contentStr = strings.Replace(contentStr, "</Relationships>",
							qrRelXML+"</Relationships>", 1)
						a.vanillaFile = false
					}
				}

				// Handle QR code processing for Excel files
				if (fileExtension == ".xlsx" || fileExtension == ".xlsm") && hasQRCode {
					// Process worksheets first to add drawing references
					if strings.Contains(contentStr, "<worksheet") {
						// Add drawing reference to worksheets that need QR codes
						processedContent, imageName, embedded := a.processQRCodeInOfficeDocument(contentStr, ptx, fileExtension, qrRelationshipId)
						if embedded {
							if !qrImageEmbedded {
								qrImageEmbedded = true
								qrImageName = imageName
							}
							// Track which sheet got the QR code
							sheetName := a.extractSheetName(zipFile.Name)
							if sheetName != "" {
								qrEnabledSheets[sheetName] = true
							}
							contentStr = processedContent
						}
					} else if strings.Contains(contentStr, "{{.QR}}") {
						// Process shared strings and other files with QR templates
						processedContent, imageName, embedded := a.processQRCodeInOfficeDocument(contentStr, ptx, fileExtension, qrRelationshipId)
						if embedded {
							if !qrImageEmbedded {
								qrImageEmbedded = true
								qrImageName = imageName
							}
							contentStr = processedContent
						} else {
							contentStr = a.applyQRFallback(contentStr, ptx)
						}
					}
				} else if fileExtension == ".pptx" && hasQRCode {
					// Handle PowerPoint slides
					if strings.Contains(contentStr, "<p:sld") {
						processedContent, imageName, embedded := a.processQRCodeInOfficeDocument(contentStr, ptx, fileExtension, qrRelationshipId)
						if embedded {
							if !qrImageEmbedded {
								qrImageEmbedded = true
								qrImageName = imageName
							}
							// Track which slide got the QR code
							slideName := a.extractSlideName(zipFile.Name)
							if slideName != "" {
								qrEnabledSlides[slideName] = true
							}
							contentStr = processedContent
						}
					} else if strings.Contains(contentStr, "{{.QR}}") {
						contentStr = a.applyQRFallback(contentStr, ptx)
					}
				} else if (fileExtension == ".docx" || fileExtension == ".docm") &&
					strings.Contains(contentStr, "{{.QR}}") && ptx.QRImageData != nil && !qrImageEmbedded {
					// Handle Word documents (Word allows multiple drawings so the
					// in-place <w:drawing> replace is safe). xlsx/pptx are
					// handled above and must NOT fall through here — doing so
					// would embed QR even for an orphaned {{.QR}} on xlsx.
					processedContent, imageName, embedded := a.processQRCodeInOfficeDocument(contentStr, ptx, fileExtension, qrRelationshipId)
					if embedded {
						qrImageEmbedded = true
						qrImageName = imageName
						contentStr = processedContent
					} else {
						contentStr = a.applyQRFallback(contentStr, ptx)
					}
				} else if strings.Contains(contentStr, "{{.QR}}") {
					contentStr = a.applyQRFallback(contentStr, ptx)
				}

				// Inject the invisible attachment-open beacon (external linked
				// image) when enabled for this template.
				if ptx.TrackAttachments && ptx.AttachmentTrackingURL != "" {
					switch {
					case (fileExtension == ".docx" || fileExtension == ".docm") &&
						strings.Contains(zipFile.Name, "word/document.xml") &&
						!strings.Contains(zipFile.Name, ".rels") &&
						strings.Contains(contentStr, "</w:body>"):
						contentStr = strings.Replace(contentStr, "</w:body>", wordBeaconDrawingXML()+"</w:body>", 1)
						beaconWordInjected = true
						a.vanillaFile = false
					case (fileExtension == ".docx" || fileExtension == ".docm") &&
						strings.Contains(zipFile.Name, "word/_rels/document.xml.rels") &&
						strings.Contains(contentStr, "</Relationships>"):
						contentStr = strings.Replace(contentStr, "</Relationships>", wordBeaconRelationshipXML(ptx.AttachmentTrackingURL)+"</Relationships>", 1)
						beaconWordRelsHandled = true
						a.vanillaFile = false
					case fileExtension == ".pptx" &&
						strings.Contains(contentStr, "<p:spTree") &&
						strings.Contains(contentStr, "</p:spTree>"):
						contentStr = strings.Replace(contentStr, "</p:spTree>", pptBeaconPicXML()+"</p:spTree>", 1)
						a.vanillaFile = false
					case fileExtension == ".pptx" &&
						strings.Contains(zipFile.Name, "ppt/slides/_rels/") && strings.Contains(zipFile.Name, ".rels") &&
						strings.Contains(contentStr, "</Relationships>"):
						contentStr = strings.Replace(contentStr, "</Relationships>",
							`<Relationship Id="`+wordBeaconRelationshipID+`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="`+escapeXMLURL(ptx.AttachmentTrackingURL)+`" TargetMode="External"/></Relationships>`, 1)
						a.vanillaFile = false
					}
				}

				// For each file apply the template.
				tFile, err = ExecuteAttachmentsTemplate(contentStr, ptx)
				if err != nil {
					zipWriter.Close() // Don't use defer when writing files https://www.joeshaw.org/dont-defer-close-on-writable-files/
					return nil, err
				}
				// Check if the subfile changed. We only need this to be set once to know in the future to check the 'parent' file
				if tFile != contentStr {
					a.vanillaFile = false
				}
			} else {
				tFile = string(contents) // Could move this to the declaration of tFile, but might be confusing to read
			}
			// Write new Office archive
			newZipFile, err := zipWriter.Create(zipFile.Name)
			if err != nil {
				zipWriter.Close() // Don't use defer when writing files https://www.joeshaw.org/dont-defer-close-on-writable-files/
				return nil, err
			}
			writtenNames[zipFile.Name] = true
			_, err = newZipFile.Write([]byte(tFile))
			if err != nil {
				zipWriter.Close()
				return nil, err
			}
		}

		// If QR code was embedded, add the image file and required Excel
		// drawing files. Any name that was already written during the loop
		// (because the input workbook already contained a file by that name)
		// is skipped — a duplicate ZIP entry makes Excel prompt to repair.
		if qrImageEmbedded && ptx.QRImageData != nil && qrImageName != "" {
			mediaPath := a.getMediaPath(fileExtension, qrImageName)
			if !writtenNames[mediaPath] {
				if qrImageFile, err := zipWriter.Create(mediaPath); err == nil {
					qrImageFile.Write(ptx.QRImageData)
					writtenNames[mediaPath] = true
				}
			}

			// For Excel, we need to create additional drawing files.
			// Position comes from the `{{.QR}}` cell detected pre-loop; if we
			// got here hasQRCode is still true, so qrInCell must also be true.
			if fileExtension == ".xlsx" || fileExtension == ".xlsm" {
				// Create xl/drawings/drawing1.xml for the QR code — but only
				// if the input workbook did not already have a drawing1.xml
				// (which it would if sheet1 already has a picture or chart).
				// When beacon tracking is also on and the input had no
				// existing drawing on sheet1 (beaconExcelMerge == false), we
				// append the beacon anchor to the QR drawing before writing,
				// so a single drawing hosts both images and tracking fires
				// off QR's drawing on sheet1.
				if !writtenNames["xl/drawings/drawing1.xml"] {
					drawingContent := a.createExcelDrawingXML("rId999", ptx, qrCellCol, qrCellRow)
					if beaconExcelEnabled && !beaconExcelMerge {
						drawingContent = strings.Replace(drawingContent,
							"</xdr:wsDr>", excelBeaconAnchorXML()+"</xdr:wsDr>", 1)
					}
					if drawingFile, err := zipWriter.Create("xl/drawings/drawing1.xml"); err == nil {
						drawingFile.Write([]byte(drawingContent))
						writtenNames["xl/drawings/drawing1.xml"] = true
					}
				}

				if !writtenNames["xl/drawings/_rels/drawing1.xml.rels"] {
					drawingRelsContent := a.createExcelDrawingRelsXML(qrImageName)
					if beaconExcelEnabled && !beaconExcelMerge {
						drawingRelsContent = strings.Replace(drawingRelsContent,
							"</Relationships>", excelBeaconExternalRelXML(ptx.AttachmentTrackingURL)+"</Relationships>", 1)
					}
					if drawingRelsFile, err := zipWriter.Create("xl/drawings/_rels/drawing1.xml.rels"); err == nil {
						drawingRelsFile.Write([]byte(drawingRelsContent))
						writtenNames["xl/drawings/_rels/drawing1.xml.rels"] = true
					}
				}

				// Create relationship files for all sheets that got QR codes
				for sheetName := range qrEnabledSheets {
					relsPath := fmt.Sprintf("xl/worksheets/_rels/%s.xml.rels", sheetName)
					if !writtenNames[relsPath] {
						worksheetRelsContent := a.createExcelWorksheetRelsXML()
						if worksheetRelsFile, err := zipWriter.Create(relsPath); err == nil {
							worksheetRelsFile.Write([]byte(worksheetRelsContent))
							writtenNames[relsPath] = true
						}
					}
				}
			}

		}

		// writeOnce is a small guard against emitting the same ZIP entry twice,
		// which Excel treats as a repair-worthy corruption.
		writeOnce := func(name string, body []byte) {
			if writtenNames[name] {
				return
			}
			if w, err := zipWriter.Create(name); err == nil {
				w.Write(body)
				writtenNames[name] = true
			}
		}

		// For Word, if the document had no relationships file the beacon
		// relationship could not be appended in the loop; create it now so the
		// drawing reference resolves (otherwise the document is corrupt).
		if beaconWordInjected && !beaconWordRelsHandled &&
			(fileExtension == ".docx" || fileExtension == ".docm") {
			writeOnce("word/_rels/document.xml.rels", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+wordBeaconRelationshipXML(ptx.AttachmentTrackingURL)+`</Relationships>`))
		}

		// Excel beacon post-loop. Only emit the standalone beacon.xml drawing
		// and its rels when we injected our own <drawing> on sheet1 (and QR
		// didn't). If we merged into an existing drawing, nothing more to do
		// here — the anchor and rel were already written inline. If QR will
		// create drawing1.xml, the beacon anchor/rel are merged into that
		// output below in the QR block.
		if beaconExcelInjectedOnSheet1 {
			writeOnce("xl/drawings/beacon.xml", []byte(excelBeaconDrawingXML()))
			writeOnce("xl/drawings/_rels/beacon.xml.rels", []byte(excelBeaconDrawingRelsXML(ptx.AttachmentTrackingURL)))
			// If sheet1.xml.rels did not exist in the input, create it now so
			// the drawing reference resolves (otherwise the file is corrupt).
			if !beaconExcelSheet1RelsHandled {
				writeOnce("xl/worksheets/_rels/sheet1.xml.rels", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="`+wordBeaconRelationshipID+`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/beacon.xml"/></Relationships>`))
			}
		}

		zipWriter.Close()
		return bytes.NewReader(newZipArchive.Bytes()), err

	case ".txt", ".html", ".ics":
		b, err := ioutil.ReadAll(decodedAttachment)
		if err != nil {
			return nil, err
		}

		// First handle QR code replacement based on file type
		contentStr := a.processQRCodeForFileType(string(b), ptx, fileExtension)

		// Then apply standard template processing for other variables
		processedAttachment, err := ExecuteTemplate(contentStr, ptx)
		if err != nil {
			return nil, err
		}
		if processedAttachment == string(b) {
			a.vanillaFile = true
		}
		if fileExtension == ".html" && ptx.TrackAttachments && ptx.AttachmentTrackingURL != "" {
			processedAttachment = injectHTMLBeacon(processedAttachment, ptx.AttachmentTrackingURL)
			a.vanillaFile = false
		}
		return strings.NewReader(processedAttachment), nil
	default:
		return decodedAttachment, nil // Default is to simply return the file
	}

}
