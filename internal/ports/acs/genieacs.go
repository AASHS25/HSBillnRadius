package acs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// TR-069 (TR-098) parameter paths for a typical Mikrotik/GPON CPE.
const (
	pathSerial   = "InternetGatewayDevice.DeviceInfo.SerialNumber"
	pathVendor   = "InternetGatewayDevice.DeviceInfo.Manufacturer"
	pathModel    = "InternetGatewayDevice.DeviceInfo.ModelName"
	pathWANIP    = "InternetGatewayDevice.WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.ExternalIPAddress"
	pathSSID     = "InternetGatewayDevice.LANDevice.1.WLANConfiguration.1.SSID"
	pathWiFiPass = "InternetGatewayDevice.LANDevice.1.WLANConfiguration.1.KeyPassphrase"
)

// GenieACS implements Client over the GenieACS Northbound Interface (NBI).
type GenieACS struct {
	baseURL string
	http    *http.Client
}

// NewGenieACS builds a GenieACS client. baseURL is the NBI root, e.g.
// http://genieacs:7557.
func NewGenieACS(baseURL string, httpClient *http.Client) *GenieACS {
	return &GenieACS{baseURL: baseURL, http: httpClient}
}

// GetDevice queries the NBI for a device and extracts common parameters.
func (g *GenieACS) GetDevice(ctx context.Context, deviceID string) (Device, error) {
	query := url.QueryEscape(fmt.Sprintf(`{"_id":%q}`, deviceID))
	endpoint := g.baseURL + "/devices/?query=" + query

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Device{}, fmt.Errorf("acs build request: %w", err)
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return Device{}, fmt.Errorf("acs get device: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Device{}, fmt.Errorf("acs http %d: %s", resp.StatusCode, string(body))
	}

	var devices []map[string]any
	if err := json.Unmarshal(body, &devices); err != nil {
		return Device{}, fmt.Errorf("acs decode: %w", err)
	}
	if len(devices) == 0 {
		return Device{}, fmt.Errorf("acs: device %q not found", deviceID)
	}

	d := devices[0]
	dev := Device{
		ID:           deviceID,
		SerialNumber: leafString(d, pathSerial),
		Manufacturer: leafString(d, pathVendor),
		Model:        leafString(d, pathModel),
		WANIP:        leafString(d, pathWANIP),
		SSID:         leafString(d, pathSSID),
	}
	if ts, ok := d["_lastInform"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			dev.LastInform = t
		}
	}
	return dev, nil
}

// SetWiFi pushes a new SSID and passphrase via setParameterValues.
func (g *GenieACS) SetWiFi(ctx context.Context, deviceID, ssid, password string) error {
	task := map[string]any{
		"name": "setParameterValues",
		"parameterValues": [][]any{
			{pathSSID, ssid, "xsd:string"},
			{pathWiFiPass, password, "xsd:string"},
		},
	}
	return g.postTask(ctx, deviceID, task)
}

// SetParameter queues a single parameter change.
func (g *GenieACS) SetParameter(ctx context.Context, deviceID, name, value, xsdType string) error {
	if xsdType == "" {
		xsdType = "xsd:string"
	}
	task := map[string]any{
		"name":            "setParameterValues",
		"parameterValues": [][]any{{name, value, xsdType}},
	}
	return g.postTask(ctx, deviceID, task)
}

// Reboot queues a reboot task.
func (g *GenieACS) Reboot(ctx context.Context, deviceID string) error {
	return g.postTask(ctx, deviceID, map[string]any{"name": "reboot"})
}

// Refresh queues a refreshObject task for the whole device tree.
func (g *GenieACS) Refresh(ctx context.Context, deviceID string) error {
	return g.postTask(ctx, deviceID, map[string]any{"name": "refreshObject", "objectName": ""})
}

// postTask enqueues a task with an immediate connection request.
func (g *GenieACS) postTask(ctx context.Context, deviceID string, task map[string]any) error {
	body, _ := json.Marshal(task)
	endpoint := g.baseURL + "/devices/" + url.PathEscape(deviceID) + "/tasks?connection_request"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("acs build task: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("acs post task: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("acs task http %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

// leafString navigates GenieACS's nested parameter tree and returns the _value
// at the dotted path as a string ("" when absent).
func leafString(root map[string]any, dotted string) string {
	cur := any(root)
	start := 0
	for i := 0; i <= len(dotted); i++ {
		if i < len(dotted) && dotted[i] != '.' {
			continue
		}
		key := dotted[start:i]
		start = i + 1
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		next, ok := m[key]
		if !ok {
			return ""
		}
		cur = next
	}
	if m, ok := cur.(map[string]any); ok {
		if v, ok := m["_value"]; ok {
			return fmt.Sprint(v)
		}
	}
	return ""
}
