package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPLBUpdaterUpdate(t *testing.T) {
	u := NewHTTPLBUpdaterClient()

	params := &UpdateParams{
		BackendName: "namespace-a_myservice",
		LBPort:      9000,
		LBProtocol:  "tcp",
		UpstreamServers: []UpstreamServer{
			{
				Host: "192.168.101.2",
				Port: 5014,
			},
			{
				Host: "192.168.101.3",
				Port: 5014,
			},
		},
		ProxyTimeoutSeconds:        6,
		ProxyConnectTimeoutSeconds: 3,
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := &UpdateParams{}

		err := json.NewDecoder(r.Body).Decode(p)
		assert.NoError(t, err)

		assert.Equal(t, p, params)

		w.Write([]byte(`{}`))
	}))

	err := u.Update(context.Background(), s.URL, params)
	assert.NoError(t, err)
}

func TestHTTPLBUpdaterDelete(t *testing.T) {
	u := NewHTTPLBUpdaterClient()

	params := &DeleteParams{
		BackendName: "namespace-a_myservice",
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := &DeleteParams{}

		err := json.NewDecoder(r.Body).Decode(p)
		assert.NoError(t, err)

		assert.Equal(t, p, params)

		w.Write([]byte(`{}`))
	}))

	err := u.Delete(context.Background(), s.URL, params)
	assert.NoError(t, err)
}
