package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// FonnteSender sends WhatsApp messages through fonnte.com's HTTP API:
// POST {APIURL} with header "Authorization: <device token>" and a
// form-encoded body of target/message. See https://fonnte.com/ docs.
type FonnteSender struct {
	Token  string
	APIURL string
	HTTP   *http.Client
}

type fonnteResponse struct {
	Status  bool   `json:"status"`
	Reason  string `json:"reason"`
	Detail  string `json:"detail"`
	ID      any    `json:"id"`
	Process string `json:"process"`
}

func (f FonnteSender) Send(ctx context.Context, phone, message string) (string, error) {
	form := url.Values{
		"target":  {phone},
		"message": {message},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.APIURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", f.Token)

	res, err := f.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var out fonnteResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("fonnte: could not decode response: %w", err)
	}
	if res.StatusCode != http.StatusOK || !out.Status {
		reason := out.Reason
		if reason == "" {
			reason = out.Detail
		}
		return "", fmt.Errorf("fonnte: send failed: %s", reason)
	}
	return fmt.Sprintf("%v", out.ID), nil
}
