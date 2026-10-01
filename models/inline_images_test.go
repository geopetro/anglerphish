package models

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"

	"github.com/gophish/gomail"
	"github.com/jordan-wright/email"
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestEmbedRemoteImagesRewritesRemoteImage(c *check.C) {
	imgData := []byte("fake-image-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(imgData)
	}))
	defer srv.Close()

	html := `<html><body><img src="` + srv.URL + `/logo.png"></body></html>`
	out, images := embedRemoteImages(html, "phish.example.com", srv.Client())

	c.Assert(len(images), check.Equals, 1)
	c.Assert(images[0].contentType, check.Equals, "image/png")
	c.Assert(string(images[0].data), check.Equals, string(imgData))
	c.Assert(strings.Contains(out, "cid:"+images[0].cid), check.Equals, true)
	c.Assert(strings.Contains(out, srv.URL), check.Equals, false)
}

func (s *ModelsSuite) TestEmbedRemoteImagesSkipsTrackingHost(c *check.C) {
	html := `<html><body><img alt='' style='display:none' src='http://phish.example.com/track?rid=abc'/></body></html>`
	out, images := embedRemoteImages(html, "phish.example.com", http.DefaultClient)

	c.Assert(len(images), check.Equals, 0)
	c.Assert(out, check.Equals, html)
}

func (s *ModelsSuite) TestEmbedRemoteImagesLeavesFailedFetchRemote(c *check.C) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	html := `<html><body><img src="` + srv.URL + `/logo.png"></body></html>`
	out, images := embedRemoteImages(html, "phish.example.com", srv.Client())

	c.Assert(len(images), check.Equals, 0)
	c.Assert(out, check.Equals, html)
}

func (s *ModelsSuite) TestEmbedRemoteImagesDeduplicatesURLs(c *check.C) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "image/gif")
		w.Write([]byte("gif"))
	}))
	defer srv.Close()

	u := srv.URL + "/a.gif"
	html := `<html><body><img src="` + u + `"><img src="` + u + `"></body></html>`
	out, images := embedRemoteImages(html, "phish.example.com", srv.Client())

	c.Assert(len(images), check.Equals, 1)
	c.Assert(atomic.LoadInt32(&hits), check.Equals, int32(1))
	c.Assert(strings.Count(out, "cid:"+images[0].cid), check.Equals, 2)
}

func (s *ModelsSuite) TestEmbedRemoteImagesIgnoresDataURIs(c *check.C) {
	html := `<html><body><img src="data:image/png;base64,AAAA"></body></html>`
	out, images := embedRemoteImages(html, "phish.example.com", http.DefaultClient)

	c.Assert(len(images), check.Equals, 0)
	c.Assert(out, check.Equals, html)
}

func (s *ModelsSuite) TestEmbedRemoteImagesSkipsNonImageContentType(c *check.C) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>nope</html>"))
	}))
	defer srv.Close()

	html := `<html><body><img src="` + srv.URL + `/evil"></body></html>`
	out, images := embedRemoteImages(html, "phish.example.com", srv.Client())

	c.Assert(len(images), check.Equals, 0)
	c.Assert(out, check.Equals, html)
}

func (s *ModelsSuite) TestMailLogGenerateEmbedsRemoteImages(c *check.C) {
	imgData := []byte("fake-image-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(imgData)
	}))
	defer srv.Close()

	template := Template{
		Name:              "EmbedTemplate",
		UserId:            1,
		Text:              "text",
		HTML:              `<html><body><img src="` + srv.URL + `/logo.png"><p>{{.RId}}</p></body></html>`,
		Subject:           "subject",
		EmbedRemoteImages: true,
	}
	c.Assert(PostTemplate(&template), check.Equals, nil)

	campaign := s.createCampaignDependencies(c)
	campaign.URL = "http://phish.example.com"
	campaign.Template = template
	c.Assert(PostCampaign(&campaign, campaign.UserId), check.Equals, nil)

	got := s.emailFromFirstMailLog(campaign, c)
	c.Assert(strings.Contains(string(got.HTML), "cid:inline-image-1"), check.Equals, true)
	c.Assert(strings.Contains(string(got.HTML), srv.URL), check.Equals, false)
}

func (s *ModelsSuite) TestMailLogGenerateLeavesRemoteImagesWhenDisabled(c *check.C) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("fake-image-bytes"))
	}))
	defer srv.Close()

	template := Template{
		Name:              "NoEmbedTemplate",
		UserId:            1,
		Text:              "text",
		HTML:              `<html><body><img src="` + srv.URL + `/logo.png"><p>{{.RId}}</p></body></html>`,
		Subject:           "subject",
		EmbedRemoteImages: false,
	}
	c.Assert(PostTemplate(&template), check.Equals, nil)

	campaign := s.createCampaignDependencies(c)
	campaign.URL = "http://phish.example.com"
	campaign.Template = template
	c.Assert(PostCampaign(&campaign, campaign.UserId), check.Equals, nil)

	got := s.emailFromFirstMailLog(campaign, c)
	c.Assert(strings.Contains(string(got.HTML), srv.URL), check.Equals, true)
	c.Assert(strings.Contains(string(got.HTML), "cid:"), check.Equals, false)
}

func (s *ModelsSuite) TestEmailRequestGenerateEmbedsRemoteImages(c *check.C) {
	imgData := []byte("fake-image-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(imgData)
	}))
	defer srv.Close()

	req := &EmailRequest{
		SMTP: SMTP{FromAddress: "from@example.com"},
		URL:  "http://phish.example.com",
		Template: Template{
			Name:              "Embed Test Template",
			Subject:           "subject",
			Text:              "text",
			HTML:              `<html><body><img src="` + srv.URL + `/logo.png"></body></html>`,
			EmbedRemoteImages: true,
		},
		BaseRecipient: BaseRecipient{FirstName: "First", LastName: "Last", Email: "firstlast@example.com"},
		FromAddress:   "from@example.com",
		RId:           PreviewPrefix + "-foobar",
	}

	msg := gomail.NewMessage()
	c.Assert(req.Generate(msg), check.Equals, nil)

	buf := &bytes.Buffer{}
	_, err := msg.WriteTo(buf)
	c.Assert(err, check.Equals, nil)
	got, err := email.NewEmailFromReader(buf)
	c.Assert(err, check.Equals, nil)

	c.Assert(strings.Contains(string(got.HTML), "cid:inline-image-1"), check.Equals, true)
	c.Assert(strings.Contains(string(got.HTML), srv.URL), check.Equals, false)
}

func (s *ModelsSuite) TestEmbedInlineImageSetsMatchingContentID(c *check.C) {
	msg := gomail.NewMessage()
	msg.SetHeader("From", "a@example.com")
	msg.SetHeader("To", "b@example.com")
	img := inlineImage{cid: "inline-image-1", contentType: "image/png", data: []byte("abc")}
	msg.SetBody("text/html", `<img src="cid:`+img.cid+`">`)

	embedInlineImage(msg, img)

	var buf bytes.Buffer
	_, err := msg.WriteTo(&buf)
	c.Assert(err, check.Equals, nil)
	out := buf.String()
	c.Assert(strings.Contains(out, "Content-ID: <inline-image-1>"), check.Equals, true)
	c.Assert(strings.Contains(out, "Content-Disposition: inline"), check.Equals, true)
	c.Assert(strings.Contains(out, base64.StdEncoding.EncodeToString(img.data)), check.Equals, true)
}
