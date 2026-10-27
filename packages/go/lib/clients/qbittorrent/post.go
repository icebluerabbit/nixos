package qbittorrent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxPostBody bounds how much of a response body is kept, whether it is the
// answer or an error page.
const maxPostBody = 512

// post issues one POST and returns the trimmed response body.
//
// httpx.Client.Do is not used: the WebUI API predates JSON bodies and answers
// in plain text, so there is nothing to decode, and the request body is form-
// or multipart-encoded rather than JSON. what names the operation in errors.
func (c *Client) post(
	ctx context.Context, path, contentType string, body io.Reader, what string,
) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.http.BaseURL()+path, body)
	if err != nil {
		return "", fmt.Errorf("qbittorrent: %s request: %w", what, err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.http.HTTP().Do(req)
	if err != nil {
		return "", fmt.Errorf("qbittorrent: %s: %w", what, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxPostBody))
	answer := strings.TrimSpace(string(raw))

	// Any 2xx: these endpoints are not consistent about which one they use.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return answer, fmt.Errorf("qbittorrent: %s: %s: %s", what, resp.Status, answer)
	}
	return answer, nil
}

// postForm issues one form-encoded POST.
func (c *Client) postForm(ctx context.Context, path string, form url.Values, what string) (string, error) {
	return c.post(ctx, path, "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()), what)
}
