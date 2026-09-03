package mcp

import (
	"encoding/json"
	"errors"
)

// Reserved _meta keys of the modern era. A stateless protocol has nowhere else
// to put the things a handshake used to establish once, so they travel on every
// message.
const (
	metaProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	metaClientCapabilities = "io.modelcontextprotocol/clientCapabilities"
	metaServerInfo         = "io.modelcontextprotocol/serverInfo"
)

// ErrCodeUnsupportedProtocolVersion is the modern era's answer to a request in
// a revision the server does not implement. It carries the list the client
// should retry from, and a dual-era client treats receiving it as proof that
// the server is modern — so it must never be sent by a server pinned to the
// legacy era, which has to look legacy instead.
const ErrCodeUnsupportedProtocolVersion = -32022

// methodCancelled is the cancellation notification, spelled as the protocol
// spells it rather than as a US dictionary would.
const methodCancelled = "notifications/cancelled" //nolint:misspell // the spec's own spelling

// methodDiscover is the modern era's one mandatory method. It is also the probe
// a dual-era client opens with, so the answer to it decides which era the
// client will use for everything after.
const methodDiscover = "server/discover"

// errMissingCapabilities is returned for a request that declares a protocol
// version but not the capabilities that go with it. The spec makes both
// required, and a server that guessed at the missing half would be relying on
// exactly the connection state this era removed.
var errMissingCapabilities = errors.New("_meta is missing " + metaClientCapabilities)

// modernMeta is the protocol half of a modern request's params.
type modernMeta struct {
	ProtocolVersion string
	HasCapabilities bool
}

// DiscoverResult answers server/discover: the versions to speak, what the
// server can do, and who it is — everything a client would otherwise learn from
// a handshake plus three probing list calls.
type DiscoverResult struct {
	SupportedVersions []string           `json:"supportedVersions"`
	Capabilities      ServerCapabilities `json:"capabilities"`
	Instructions      string             `json:"instructions,omitempty"`
}

// UnsupportedVersionData is the body of an UnsupportedProtocolVersionError:
// what was asked for, and what the client can retry with.
type UnsupportedVersionData struct {
	Supported []string `json:"supported"`
	Requested string   `json:"requested"`
}

// readModernMeta pulls the protocol fields out of a request's params.
//
// It reports whether the request is modern at all — a legacy request has no
// _meta and is answered under the handshake rules instead — separately from
// whether it is well formed, because those two failures have different answers:
// the first is another era, the second is a malformed request.
func readModernMeta(params json.RawMessage) (meta modernMeta, isModern bool, err error) {
	if len(params) == 0 {
		return modernMeta{}, false, nil
	}

	var envelope struct {
		Meta map[string]json.RawMessage `json:"_meta"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		return modernMeta{}, false, nil
	}

	raw, ok := envelope.Meta[metaProtocolVersion]
	if !ok {
		return modernMeta{}, false, nil
	}

	var version string
	if err := json.Unmarshal(raw, &version); err != nil {
		return modernMeta{}, true, err
	}

	_, hasCapabilities := envelope.Meta[metaClientCapabilities]
	if !hasCapabilities {
		return modernMeta{ProtocolVersion: version}, true, errMissingCapabilities
	}

	return modernMeta{ProtocolVersion: version, HasCapabilities: true}, true, nil
}

// decorateModernResult adds the two fields this era asks of every result:
// resultType, which tells a client how to read the rest, and the server
// identity, which a stateless protocol gives no other place to state.
//
// It works on the marshaled form rather than on each result type, so the typed
// results stay the plain data structures both eras share.
func decorateModernResult(result interface{}, info ServerInfo) (interface{}, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		// A result that is not a JSON object has no place to carry the fields;
		// it is sent as it is rather than being silently reshaped.
		return result, nil //nolint:nilerr // the value is still a valid result
	}

	fields["resultType"] = json.RawMessage(`"complete"`)

	identity, err := json.Marshal(map[string]ServerInfo{metaServerInfo: info})
	if err != nil {
		return nil, err
	}
	fields["_meta"] = identity

	return fields, nil
}
