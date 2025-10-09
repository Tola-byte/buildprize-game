package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type TestClient struct {
	baseURL string
	client  *http.Client
}

func NewTestClient(baseURL string) *TestClient {
	return &TestClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (tc *TestClient) Get(path string) (*http.Response, error) {
	return tc.client.Get(tc.baseURL + path)
}

func (tc *TestClient) Post(path string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	
	return tc.client.Post(
		tc.baseURL+path,
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
}

func (tc *TestClient) GetJSON(path string, target interface{}) error {
	resp, err := tc.Get(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	
	return json.Unmarshal(body, target)
}

func (tc *TestClient) PostJSON(path string, body interface{}, target interface{}) error {
	resp, err := tc.Post(path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	
	if target != nil {
		return json.Unmarshal(respBody, target)
	}
	
	return nil
}
