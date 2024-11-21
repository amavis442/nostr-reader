package http

/*
 * Based on https://github.com/sbabashahi/urlPreviewGo/blob/master/main.go
 */
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	parse "net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// HTMLMeta data to response
type HTMLMeta struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Images      []string `json:"images"`
	SiteName    string   `json:"siteName"`
	Icon        string   `json:"icon"`
	Videos      []string `json:"videos"`
	MediaType   string   `json:"mediaType"`
	ContentType string   `json:"contentType"`
	Url         string   `json:"url"`
	Favicons    []string `json:"favicons"`
}

// HandleURL check it and validations
func HandleURL(url string) (string, error) {
	var msg string
	if url == "" {
		msg = "you missed to set url query param"
		return msg, errors.New(msg)
	}
	u, err := parse.Parse(url)
	if err != nil {
		return err.Error(), err
	}
	if u.Scheme == "" {
		url = fmt.Sprintf("%s%s", "http://", url)
	} else if !strings.HasPrefix(u.Scheme, "http") {
		msg = "URL schema must be http or https"
		return msg, errors.New(msg)
	}
	_, err = parse.ParseRequestURI(url)
	if err != nil {
		return err.Error(), err
	}
	return url, nil
}

// Message function structure response
func Message(data interface{}, message string, status bool) map[string]interface{} {
	now := time.Now()
	return map[string]interface{}{"data": data, "status": status, "message": message, "current_time": now.Unix()}
}

// Respond function send response as json
func Respond(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// URLPreview function for main page
func URLPreview(ctx context.Context, url string) (map[string]interface{}, error) {
	const ConnectMaxWaitTime = 1 * time.Second
	//const RequestMaxWaitTime = 5 * time.Second

	if url == "" {
		return map[string]interface{}{"url": url, "data": nil, "status": "error", "respcode": fmt.Sprint(408), "error": "cannot do request: No url present"}, errors.New("cannot do request: No url present")
	}

	meta := HTMLMeta{}

	client := http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: ConnectMaxWaitTime,
			}).DialContext,
		},
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	// handle the error if there is one
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)

	if resp != nil {
		defer resp.Body.Close()
	}

	if e, ok := err.(net.Error); ok && e.Timeout() {
		return map[string]interface{}{"url": url, "data": nil, "status": "error", "respcode": fmt.Sprint(408), "error": "preview do request timeout: " + err.Error()}, errors.New("do request timeout: " + err.Error())
	} else if err != nil {
		return map[string]interface{}{"url": url, "data": nil, "status": "error", "respcode": fmt.Sprint(400), "error": "preview cannot do request: " + err.Error()}, errors.New("cannot do request: " + err.Error())
	}

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{"url": url, "data": nil, "status": "error", "respcode": fmt.Sprint(resp.StatusCode), "error": "request not succesfull"}, errors.New("request not succesfull " + strconv.Itoa(resp.StatusCode))
	}

	contentType := resp.Header.Get("Content-type")

	if contentType == "image/jpeg" || contentType == "image/jpg" || contentType == "image/gif" || contentType == "image/webp" || contentType == "image/png" {
		meta.MediaType = "image"
	} else {
		// do this now so it won't be forgotten
		meta = Extract(resp.Body)
	}
	meta.Url = url
	meta.ContentType = contentType

	defer resp.Body.Close()

	data := map[string]interface{}{"url": url, "data": meta, "status": "ok", "respcode": fmt.Sprint(resp.StatusCode), "error": ""}
	return data, nil
}

// Extract html meta tags
func Extract(resp io.Reader) (hm HTMLMeta) {
	z := html.NewTokenizer(resp)

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.Data == "meta" {
				title, ok := extractMetaProperty(t, "og:title")
				if ok {
					hm.Title = title
				}

				desc, ok := extractMetaProperty(t, "og:description")
				if ok {
					hm.Description = desc
				}

				image, ok := extractMetaProperty(t, "og:image")
				if ok {
					hm.Images = append(hm.Images, image)
				}

				siteName, ok := extractMetaProperty(t, "og:site_name")
				if ok {
					hm.SiteName = siteName
				}

				mediaType, ok := extractMetaProperty(t, "og:type")
				if ok {
					hm.MediaType = mediaType
				}
			}
			if t.Data == "link" {
				icon, ok := extractIcon(t, "shortcut icon")
				if ok {
					hm.Favicons = append(hm.Favicons, icon)
				}
			}
		}
	}
}

func extractMetaProperty(t html.Token, prop string) (content string, ok bool) {
	for _, attr := range t.Attr {
		if attr.Key == "property" && attr.Val == prop {
			ok = true
		}

		if attr.Key == "content" {
			content = attr.Val
		}
	}

	return
}

func extractIcon(t html.Token, prop string) (content string, ok bool) {
	for _, attr := range t.Attr {
		if attr.Key == "rel" && attr.Val == prop {
			ok = true
		}

		if attr.Key == "href" {
			content = attr.Val
		}
	}

	return
}
