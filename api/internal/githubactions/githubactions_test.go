package githubactions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func repondant(status int, body string, seen *http.Request, seenBody *[]byte) *Client {
	c := New()
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if seen != nil {
			*seen = *r
			raw, _ := io.ReadAll(r.Body)
			*seenBody = raw
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: r}, nil
	})
	return c
}

func TestDispatchDemandeLeBonWorkflowSurLaBonneBranche(t *testing.T) {
	var req http.Request
	var body []byte
	c := repondant(http.StatusNoContent, "", &req, &body)
	err := c.Dispatch(context.Background(), Dispatch{
		Token: "jeton", Owner: "agence", Repo: "site", Workflow: "piilot-update.yml", Ref: "main",
		Inputs: map[string]string{"environment": "preprod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := req.URL.String(); got != "https://api.github.com/repos/agence/site/actions/workflows/piilot-update.yml/dispatches" {
		t.Errorf("adresse : %s", got)
	}
	if req.Header.Get("Authorization") != "Bearer jeton" {
		t.Errorf("en-tete : %s", req.Header.Get("Authorization"))
	}
	var sent struct {
		Ref    string            `json:"ref"`
		Inputs map[string]string `json:"inputs"`
	}
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Ref != "main" || sent.Inputs["environment"] != "preprod" {
		t.Errorf("corps : %+v", sent)
	}
}

func TestDispatchDistingueCeQuiSeRetente(t *testing.T) {
	cases := []struct {
		status    int
		body      string
		retryable bool
		message   string
	}{
		{http.StatusNotFound, `{"message":"Not Found"}`, false, "Not Found"},
		{http.StatusUnauthorized, `{"message":"Bad credentials"}`, false, "Bad credentials"},
		{http.StatusUnprocessableEntity, `{"message":"Unexpected inputs"}`, false, "Unexpected inputs"},
		{http.StatusTooManyRequests, `{}`, true, "Too Many Requests"},
		{http.StatusServiceUnavailable, `{}`, true, "Service Unavailable"},
		{http.StatusBadGateway, `pas du json`, false, "Bad Gateway"},
		{http.StatusGatewayTimeout, `{}`, false, "Gateway Timeout"},
	}
	for _, c := range cases {
		err := repondant(c.status, c.body, nil, nil).Dispatch(context.Background(), Dispatch{Token: "t", Owner: "o", Repo: "r", Workflow: "w.yml", Ref: "main"})
		var ghErr *Error
		if !errors.As(err, &ghErr) {
			t.Fatalf("%d : %v", c.status, err)
		}
		if ghErr.Retryable() != c.retryable || ghErr.Message != c.message {
			t.Errorf("%d : %+v", c.status, ghErr)
		}
		if ghErr.Refused() != (c.status < 500) {
			t.Errorf("%d : refus = %v", c.status, ghErr.Refused())
		}
		if strings.Contains(err.Error(), "Bearer") {
			t.Errorf("le jeton fuit : %v", err)
		}
	}
}
