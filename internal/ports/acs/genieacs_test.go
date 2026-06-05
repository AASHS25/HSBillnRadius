package acs_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aashs25/hsbillnradius/internal/ports/acs"
)

const deviceJSON = `[{
  "_id": "DEVICE-1",
  "_lastInform": "2026-06-05T10:00:00Z",
  "InternetGatewayDevice": {
    "DeviceInfo": {
      "SerialNumber": {"_value": "SN123"},
      "Manufacturer": {"_value": "ZTE"},
      "ModelName": {"_value": "F670L"}
    },
    "LANDevice": {"1": {"WLANConfiguration": {"1": {"SSID": {"_value": "MyWiFi"}}}}},
    "WANDevice": {"1": {"WANConnectionDevice": {"1": {"WANIPConnection": {"1": {"ExternalIPAddress": {"_value": "100.64.0.5"}}}}}}}
  }
}]`

func TestGenieACS_GetDevice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "query=")
		_, _ = io.WriteString(w, deviceJSON)
	}))
	defer srv.Close()

	dev, err := acs.NewGenieACS(srv.URL, srv.Client()).GetDevice(context.Background(), "DEVICE-1")
	require.NoError(t, err)
	assert.Equal(t, "SN123", dev.SerialNumber)
	assert.Equal(t, "ZTE", dev.Manufacturer)
	assert.Equal(t, "F670L", dev.Model)
	assert.Equal(t, "MyWiFi", dev.SSID)
	assert.Equal(t, "100.64.0.5", dev.WANIP)
	assert.Equal(t, 2026, dev.LastInform.Year())
}

func TestGenieACS_SetWiFi(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := acs.NewGenieACS(srv.URL, srv.Client()).SetWiFi(context.Background(), "DEVICE-1", "NewSSID", "secret123")
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(gotPath, "/devices/DEVICE-1/tasks"), "path was %s", gotPath)
	assert.Equal(t, "setParameterValues", gotBody["name"])

	// The task must set both SSID and passphrase.
	pv, _ := json.Marshal(gotBody["parameterValues"])
	assert.Contains(t, string(pv), "SSID")
	assert.Contains(t, string(pv), "NewSSID")
	assert.Contains(t, string(pv), "KeyPassphrase")
	assert.Contains(t, string(pv), "secret123")
}

func TestGenieACS_Reboot(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	require.NoError(t, acs.NewGenieACS(srv.URL, srv.Client()).Reboot(context.Background(), "DEVICE-1"))
	assert.Equal(t, "reboot", gotBody["name"])
}
