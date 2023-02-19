package services

import (
	"bytes"
	"encoding/json"

	"net/http"
	"time"

	"github.com/pkg/errors"
)

type HTTPLBUpdaterClient interface {
	Update(url string, params *UpdateParams) error
	Delete(url string, params *DeleteParams) error
}

type httpLBUpdaterClient struct {
	httpClient *http.Client
}

type UpstreamServer struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type UpdateParams struct {
	BackendName     string           `json:"backendName"`
	LBPort          int              `json:"lbPort"`
	LBProtocol      string           `json:"lbProtocol"`
	UpstreamServers []UpstreamServer `json:"upstreamServers"`
	// fields below are not implemented in CRD yet
	ProxyTimeoutSeconds        int `json:"proxyTimeoutSeconds"`
	ProxyConnectTimeoutSeconds int `json:"proxyConnectTimeoutSeconds"`
}

type DeleteParams struct {
	BackendName string `json:"backendName"`
}

func NewHTTPLBUpdaterClient() HTTPLBUpdaterClient {
	cl := &httpLBUpdaterClient{}

	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 50
	t.MaxConnsPerHost = 50
	t.MaxIdleConnsPerHost = 50

	httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: t,
	}

	cl.httpClient = httpClient

	return cl
}

func (cl *httpLBUpdaterClient) Update(url string, params *UpdateParams) error {
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(params)
	if err != nil {
		return errors.Wrapf(err, "failed to encode update request")
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return errors.Wrapf(err, "failed to create update request")
	}

	resp, err := cl.httpClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed to make update request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 {
		return nil
	}

	jsonErr := &JSONErr{}

	err = json.NewDecoder(resp.Body).Decode(jsonErr)
	if err != nil {
		return errors.Wrapf(err, "failed to decode response")
	}

	return errors.New(jsonErr.Err)
}

func (cl *httpLBUpdaterClient) Delete(url string, params *DeleteParams) error {
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(params)
	if err != nil {
		return errors.Wrapf(err, "failed to encode delete request")
	}

	req, err := http.NewRequest(http.MethodDelete, url, &buf)
	if err != nil {
		return errors.Wrapf(err, "failed to create delete request")
	}

	resp, err := cl.httpClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed to make delete request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 {
		return nil
	}

	jsonErr := &JSONErr{}

	err = json.NewDecoder(resp.Body).Decode(jsonErr)
	if err != nil {
		return errors.Wrapf(err, "failed to decode response")
	}

	return errors.New(jsonErr.Err)
}

type JSONErr struct {
	Err string `json:"err"`
}
