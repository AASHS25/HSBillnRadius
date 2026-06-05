// Package radius holds RADIUS-related value types shared across the billing
// (provisioning) side and the radius-service. Protocol handling lives in the
// radius transport/service; this package stays dependency-free.
package radius

// Attr is a single RADIUS attribute-value pair with its operator, mirroring the
// FreeRADIUS rad*reply/check column layout.
type Attr struct {
	Attribute string
	Op        string
	Value     string
}

// UserGroup maps a username to a group with a match priority.
type UserGroup struct {
	Username  string
	Groupname string
	Priority  int32
}

// Common attribute names used when provisioning Mikrotik NAS replies.
const (
	AttrCleartextPassword = "Cleartext-Password"
	AttrMikrotikRateLimit = "Mikrotik-Rate-Limit"
	AttrFramedPool        = "Framed-Pool"
	AttrMikrotikAddrList  = "Mikrotik-Address-List"
)
