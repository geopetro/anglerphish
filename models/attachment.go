package models

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
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

// createExcelDrawingXML creates the Excel drawing XML content
func (a *Attachment) createExcelDrawingXML(imageRelId string, ptx PhishingTemplateContext) string {
	qrSizeEMU := a.calculateQRSizeEMU(ptx)
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <xdr:twoCellAnchor editAs="oneCell">
    <xdr:from>
      <xdr:col>2</xdr:col>
      <xdr:colOff>0</xdr:colOff>
      <xdr:row>2</xdr:row>
      <xdr:rowOff>0</xdr:rowOff>
    </xdr:from>
    <xdr:to>
      <xdr:col>4</xdr:col>
      <xdr:colOff>0</xdr:colOff>
      <xdr:row>8</xdr:row>
      <xdr:rowOff>0</xdr:rowOff>
    </xdr:to>
    <xdr:pic>
      <xdr:nvPicPr>
        <xdr:cNvPr id="1" name="QR Code"/>
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
  </xdr:twoCellAnchor>
</xdr:wsDr>`
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
			return nil, err
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
				} else if strings.Contains(contentStr, "{{.QR}}") && ptx.QRImageData != nil && !qrImageEmbedded {
					// Handle Word documents
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
			_, err = newZipFile.Write([]byte(tFile))
			if err != nil {
				zipWriter.Close()
				return nil, err
			}
		}

		// If QR code was embedded, add the image file and required Excel drawing files
		if qrImageEmbedded && ptx.QRImageData != nil && qrImageName != "" {
			mediaPath := a.getMediaPath(fileExtension, qrImageName)
			qrImageFile, err := zipWriter.Create(mediaPath)
			if err == nil {
				_, err = qrImageFile.Write(ptx.QRImageData)
				if err != nil {
					// Log error but continue - document will still work without QR image
				}
			}

			// For Excel, we need to create additional drawing files
			if fileExtension == ".xlsx" || fileExtension == ".xlsm" {
				// Create xl/drawings/drawing1.xml
				drawingContent := a.createExcelDrawingXML("rId999", ptx)
				drawingFile, err := zipWriter.Create("xl/drawings/drawing1.xml")
				if err == nil {
					_, err = drawingFile.Write([]byte(drawingContent))
				}

				// Create xl/drawings/_rels/drawing1.xml.rels
				drawingRelsContent := a.createExcelDrawingRelsXML(qrImageName)
				drawingRelsFile, err := zipWriter.Create("xl/drawings/_rels/drawing1.xml.rels")
				if err == nil {
					_, err = drawingRelsFile.Write([]byte(drawingRelsContent))
				}

				// Create relationship files for all sheets that got QR codes
				for sheetName := range qrEnabledSheets {
					relsPath := fmt.Sprintf("xl/worksheets/_rels/%s.xml.rels", sheetName)
					worksheetRelsContent := a.createExcelWorksheetRelsXML()
					worksheetRelsFile, err := zipWriter.Create(relsPath)
					if err == nil {
						_, err = worksheetRelsFile.Write([]byte(worksheetRelsContent))
					}
				}
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
		return strings.NewReader(processedAttachment), nil
	default:
		return decodedAttachment, nil // Default is to simply return the file
	}

}
