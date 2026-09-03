package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// modernParams is the metadata every modern request has to carry, since the
// era removed the handshake that used to state it once.
func modernParams(version string) string {
	return `{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + version +
		`","io.modelcontextprotocol/clientCapabilities":{}}}`
}

// converse runs lines through a server speaking the given revisions and returns
// what came back, so a test can read the wire rather than the implementation.
func converse(t *testing.T, spec string, lines ...string) []Response {
	t.Helper()

	versions, err := ParseVersionSet(spec)
	require.NoError(t, err)

	input := bytes.NewBufferString(strings.Join(lines, "\n") + "\n")
	output := &bytes.Buffer{}

	server := NewServerWithIO("wpexporter", "9.9.9", input, output)
	server.SetProtocols(versions)
	assert.Equal(t, versions.Modern(), server.Protocols().Modern())
	RegisterAllTools(server)

	require.NoError(t, server.Run())

	var responses []Response
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		if line == "" {
			continue
		}

		var resp Response
		require.NoError(t, json.Unmarshal([]byte(line), &resp))
		responses = append(responses, resp)
	}

	return responses
}

func resultOf(t *testing.T, resp Response) map[string]interface{} {
	t.Helper()

	require.Nil(t, resp.Error)
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok, "result is not an object")

	return result
}

// TestDiscoverAnswersWithoutAHandshake: server/discover is the modern era's one
// mandatory method and the probe a dual-era client opens with, so it has to
// answer identity, capabilities and versions in a single call.
func TestDiscoverAnswersWithoutAHandshake(t *testing.T) {
	responses := converse(t, ProtocolAll,
		`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":`+modernParams(Version20260728)+`}`)

	require.Len(t, responses, 1)
	result := resultOf(t, responses[0])

	assert.Equal(t, "complete", result["resultType"])
	assert.Equal(t, []interface{}{Version20260728}, result["supportedVersions"])
	assert.NotEmpty(t, result["instructions"])

	meta, ok := result["_meta"].(map[string]interface{})
	require.True(t, ok)
	info, ok := meta[metaServerInfo].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "wpexporter", info["name"])
	assert.Equal(t, "9.9.9", info["version"])
}

// TestUnsupportedVersionNamesWhatToRetryWith: the error is also how a dual-era
// client recognizes a modern server, so it must carry the list to retry from
// rather than a bare failure.
func TestUnsupportedVersionNamesWhatToRetryWith(t *testing.T) {
	responses := converse(t, ProtocolAll,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":`+modernParams("1900-01-01")+`}`)

	require.Len(t, responses, 1)
	require.NotNil(t, responses[0].Error)
	assert.Equal(t, ErrCodeUnsupportedProtocolVersion, responses[0].Error.Code)

	data, ok := responses[0].Error.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "1900-01-01", data["requested"])
	assert.Equal(t, []interface{}{Version20260728}, data["supported"])
}

// TestModernRequestMustCarryItsMetadata: a stateless era cannot infer the half
// a request left out, so an incomplete one is malformed rather than guessed at.
func TestModernRequestMustCarryItsMetadata(t *testing.T) {
	responses := converse(t, ProtocolAll,
		`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"`+Version20260728+`"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"server/discover","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":7,"io.modelcontextprotocol/clientCapabilities":{}}}}`)

	require.Len(t, responses, 3)
	for _, resp := range responses {
		require.NotNil(t, resp.Error)
		assert.Equal(t, ErrCodeInvalidParams, resp.Error.Code)
	}
	assert.Contains(t, responses[0].Error.Data, metaClientCapabilities)
	assert.Contains(t, responses[1].Error.Data, metaProtocolVersion)
}

// TestModernToolsCarryTheEnvelope: every modern result states its type and who
// answered, because nothing else in the exchange does.
func TestModernToolsCarryTheEnvelope(t *testing.T) {
	responses := converse(t, ProtocolAll,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":`+modernParams(Version20260728)+`}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping","params":`+modernParams(Version20260728)+`}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_formats","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"`+Version20260728+`","io.modelcontextprotocol/clientCapabilities":{}}}}`)

	require.Len(t, responses, 3)
	for _, resp := range responses {
		result := resultOf(t, resp)
		assert.Equal(t, "complete", result["resultType"])
		assert.Contains(t, result, "_meta")
	}
	assert.NotEmpty(t, resultOf(t, responses[0])["tools"])
	assert.NotEmpty(t, resultOf(t, responses[2])["content"])
}

// TestModernUnknownMethod keeps an unknown method an unknown method, rather
// than letting the era check swallow it.
func TestModernUnknownMethod(t *testing.T) {
	responses := converse(t, ProtocolAll,
		`{"jsonrpc":"2.0","id":1,"method":"resources/list","params":`+modernParams(Version20260728)+`}`)

	require.Len(t, responses, 1)
	require.NotNil(t, responses[0].Error)
	assert.Equal(t, ErrCodeMethodNotFound, responses[0].Error.Code)
}

// TestLegacyHandshakeIsUntouchedByTheNewEra: a client that opens with
// initialize still gets the revision it asked for, and results with no envelope
// around them — the era it is in has no resultType.
func TestLegacyHandshakeIsUntouchedByTheNewEra(t *testing.T) {
	responses := converse(t, ProtocolAll,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+Version20250618+`","capabilities":{}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`)

	require.Len(t, responses, 3, "a notification is never answered")

	handshake := resultOf(t, responses[0])
	assert.Equal(t, Version20250618, handshake["protocolVersion"])
	assert.NotEmpty(t, handshake["instructions"])

	tools := resultOf(t, responses[1])
	assert.NotContains(t, tools, "resultType")
	assert.NotEmpty(t, tools["tools"])
}

// TestLegacyOnlyServerLooksLegacy: a client probing with server/discover has to
// get an ordinary error, not the modern one — the modern error would tell it to
// keep negotiating a version this server will never answer.
func TestLegacyOnlyServerLooksLegacy(t *testing.T) {
	responses := converse(t, ProtocolLegacy,
		`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":`+modernParams(Version20260728)+`}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"`+Version20260728+`","capabilities":{}}}`)

	require.Len(t, responses, 2)
	require.NotNil(t, responses[0].Error)
	assert.Equal(t, ErrCodeMethodNotFound, responses[0].Error.Code)
	assert.NotEqual(t, ErrCodeUnsupportedProtocolVersion, responses[0].Error.Code)

	// The modern revision it asked for is not on offer, so it is answered with
	// the newest one that is.
	assert.Equal(t, Version20251125, resultOf(t, responses[1])["protocolVersion"])
}

// TestModernOnlyServerNamesWhatItSpeaks: a legacy client has no fall-forward
// mechanism, so the error it gets is the only diagnostic a user will see.
func TestModernOnlyServerNamesWhatItSpeaks(t *testing.T) {
	responses := converse(t, ProtocolModern,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+Version20241105+`","capabilities":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)

	require.Len(t, responses, 2)

	require.NotNil(t, responses[0].Error)
	assert.Contains(t, responses[0].Error.Message, Version20260728)
	data, ok := responses[0].Error.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, []interface{}{Version20260728}, data["supportedVersions"])

	// With no legacy era to fall back to, a request without metadata is
	// malformed rather than a method that does not exist.
	require.NotNil(t, responses[1].Error)
	assert.Equal(t, ErrCodeInvalidParams, responses[1].Error.Code)
}

// TestNotificationsAreNeverAnswered covers both spellings of the initialized
// notification and the cancellation a client may send at any time.
func TestNotificationsAreNeverAnswered(t *testing.T) {
	responses := converse(t, ProtocolAll,
		`{"jsonrpc":"2.0","method":"initialized"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"`+methodCancelled+`","params":{"requestId":1}}`)

	assert.Empty(t, responses)
}

// TestDecorateModernResult covers the two results that cannot take the
// envelope: one that is not a JSON object, and one that is not JSON at all.
func TestDecorateModernResult(t *testing.T) {
	info := ServerInfo{Name: "wpexporter", Version: "9.9.9"}

	plain, err := decorateModernResult("not an object", info)
	require.NoError(t, err)
	assert.Equal(t, "not an object", plain)

	_, err = decorateModernResult(func() {}, info)
	assert.Error(t, err)
}

// TestModernResultThatCannotBeEncoded: a result the envelope cannot be put
// around is an internal error, not a silently dropped response.
func TestModernResultThatCannotBeEncoded(t *testing.T) {
	output := &bytes.Buffer{}
	server := NewServerWithIO("wpexporter", "9.9.9", bytes.NewBufferString(""), output)

	server.sendModernResult(json.RawMessage("1"), func() {})

	var resp Response
	require.NoError(t, json.Unmarshal(output.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeInternal, resp.Error.Code)
}

// TestLegacyInitializeRejectsMalformedParams: a handshake that cannot be read
// is answered as malformed rather than silently negotiated as if it were empty.
func TestLegacyInitializeRejectsMalformedParams(t *testing.T) {
	responses := converse(t, ProtocolAll,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":42}}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize"}`)

	require.Len(t, responses, 2)

	require.NotNil(t, responses[0].Error)
	assert.Equal(t, ErrCodeInvalidParams, responses[0].Error.Code)

	// A handshake with no params at all is still a handshake: it gets the
	// newest revision on offer.
	assert.Equal(t, Version20251125, resultOf(t, responses[1])["protocolVersion"])
}
