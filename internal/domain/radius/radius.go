// Package radius holds RADIUS-related value types shared across the billing
// (provisioning) side and the radius-service. Protocol handling lives in the
// radius transport/service; this package stays dependency-free.
package radius

import (
	"crypto/sha1" //nolint:gosec // used only to derive a stable session id
	"encoding/hex"
	"errors"
	"time"
)

// Domain errors.
var (
	ErrNasNotFound = errors.New("nas not found")
	ErrNasExists   = errors.New("nas already exists")
	ErrUserUnknown = errors.New("radius user not found")
)

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

// Nas is a network access server (a router) identified by source IP. Its shared
// secret is used to validate and encode RADIUS packets; it must never be logged.
type Nas struct {
	ID        int64
	TenantID  int64
	Name      string // nasname (IP/identifier)
	Shortname string
	Secret    string
}

// PostAuth is an authentication audit record written to radpostauth.
type PostAuth struct {
	TenantID int64
	Username string
	Pass     string // "" — never store cleartext passwords here
	Reply    string // "Access-Accept" | "Access-Reject"
	NASIP    string
}

// AcctStatus is the Acct-Status-Type of an accounting request.
type AcctStatus string

const (
	AcctStart   AcctStatus = "Start"
	AcctInterim AcctStatus = "Interim-Update"
	AcctStop    AcctStatus = "Stop"
)

// AcctEvent is a parsed accounting request handed to the batch writer.
type AcctEvent struct {
	TenantID       int64
	Status         AcctStatus
	SessionID      string
	UniqueID       string
	Username       string
	NASIP          string
	NASPort        string
	StartTime      time.Time
	SessionTime    int64
	InputOctets    int64
	OutputOctets   int64
	FramedIP       string
	CallingStation string
	CalledStation  string
	TerminateCause string
}

// ActiveSession is an open accounting session (no stop time yet).
type ActiveSession struct {
	SessionID      string
	NASIP          string
	FramedIP       string
	CallingStation string
}

// ComputeUniqueID derives the FreeRADIUS-style acctuniqueid from the session
// identity so Start/Interim/Stop for the same session collapse to one row.
func ComputeUniqueID(sessionID, nasIP, username string) string {
	sum := sha1.Sum([]byte(sessionID + "," + nasIP + "," + username)) //nolint:gosec // stable id, not security
	return hex.EncodeToString(sum[:])
}

// Common attribute names used when provisioning Mikrotik NAS replies.
const (
	AttrCleartextPassword = "Cleartext-Password"
	AttrMikrotikRateLimit = "Mikrotik-Rate-Limit"
	AttrFramedPool        = "Framed-Pool"
	AttrMikrotikAddrList  = "Mikrotik-Address-List"
)
