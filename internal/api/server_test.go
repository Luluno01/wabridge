package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackend implements action.Backend for testing.
type mockBackend struct {
	sendMessageCalled    bool
	sendFileCalled       bool
	sendAudioCalled      bool
	downloadMediaCalled  bool
	historySyncCalled    bool
	lastRecipient        string
	lastText             string
	lastFilePath         string
	lastMessageID        string
	lastChatJID          string
	downloadMediaPath    string
	err                  error // error to return from all methods
}

func (m *mockBackend) SendMessage(_ context.Context, recipient, text string) error {
	m.sendMessageCalled = true
	m.lastRecipient = recipient
	m.lastText = text
	return m.err
}

func (m *mockBackend) SendFile(_ context.Context, recipient, filePath string) error {
	m.sendFileCalled = true
	m.lastRecipient = recipient
	m.lastFilePath = filePath
	return m.err
}

func (m *mockBackend) SendAudioMessage(_ context.Context, recipient, filePath string) error {
	m.sendAudioCalled = true
	m.lastRecipient = recipient
	m.lastFilePath = filePath
	return m.err
}

func (m *mockBackend) DownloadMedia(_ context.Context, messageID, chatJID string) (string, error) {
	m.downloadMediaCalled = true
	m.lastMessageID = messageID
	m.lastChatJID = chatJID
	return m.downloadMediaPath, m.err
}

func (m *mockBackend) RequestHistorySync(_ context.Context, chatJID string) error {
	m.historySyncCalled = true
	m.lastChatJID = chatJID
	return m.err
}

func newTestServer(backend *mockBackend) *APIServer {
	return NewAPIServer(backend, ":0")
}

func doRequest(t *testing.T, s *APIServer, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	handler := s.Handler()

	var reqBody *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = &bytes.Buffer{}
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func parseResponse(t *testing.T, rr *httptest.ResponseRecorder) apiResponse {
	t.Helper()
	var resp apiResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err, "response body: %s", rr.Body.String())
	return resp
}

// --- Health -----------------------------------------------------------------

func TestHealthEndpoint(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "GET", "/health", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseResponse(t, rr)
	assert.True(t, resp.Success)
	assert.Equal(t, "ok", resp.Message)
}

// --- Send Message -----------------------------------------------------------

func TestSendMessage_Success(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/send", map[string]string{
		"recipient": "1234567890@s.whatsapp.net",
		"message":   "hello",
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseResponse(t, rr)
	assert.True(t, resp.Success)
	assert.Equal(t, "sent", resp.Message)
	assert.True(t, backend.sendMessageCalled)
	assert.Equal(t, "1234567890@s.whatsapp.net", backend.lastRecipient)
	assert.Equal(t, "hello", backend.lastText)
}

func TestSendMessage_MissingFields(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/send", map[string]string{
		"recipient": "1234567890@s.whatsapp.net",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	resp := parseResponse(t, rr)
	assert.False(t, resp.Success)
	assert.False(t, backend.sendMessageCalled)
}

func TestSendMessage_BackendError(t *testing.T) {
	backend := &mockBackend{err: fmt.Errorf("connection lost")}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/send", map[string]string{
		"recipient": "1234567890@s.whatsapp.net",
		"message":   "hello",
	})

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	resp := parseResponse(t, rr)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "connection lost")
}

func TestSendMessage_InvalidJSON(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	req := httptest.NewRequest("POST", "/api/send", bytes.NewBufferString("{invalid"))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	resp := parseResponse(t, rr)
	assert.False(t, resp.Success)
}

// --- Send File --------------------------------------------------------------

func TestSendFile_Success(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/send-file", map[string]string{
		"recipient": "1234567890@s.whatsapp.net",
		"file_path": "/tmp/photo.jpg",
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseResponse(t, rr)
	assert.True(t, resp.Success)
	assert.Equal(t, "file sent", resp.Message)
	assert.True(t, backend.sendFileCalled)
	assert.Equal(t, "/tmp/photo.jpg", backend.lastFilePath)
}

func TestSendFile_MissingFields(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/send-file", map[string]string{
		"recipient": "1234567890@s.whatsapp.net",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.False(t, backend.sendFileCalled)
}

// --- Send Audio -------------------------------------------------------------

func TestSendAudio_Success(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/send-audio", map[string]string{
		"recipient": "1234567890@s.whatsapp.net",
		"file_path": "/tmp/voice.ogg",
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseResponse(t, rr)
	assert.True(t, resp.Success)
	assert.Equal(t, "audio sent", resp.Message)
	assert.True(t, backend.sendAudioCalled)
	assert.Equal(t, "/tmp/voice.ogg", backend.lastFilePath)
}

func TestSendAudio_MissingFields(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/send-audio", map[string]string{
		"file_path": "/tmp/voice.ogg",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.False(t, backend.sendAudioCalled)
}

// --- Download Media ---------------------------------------------------------

func TestDownload_Success(t *testing.T) {
	backend := &mockBackend{downloadMediaPath: "/media/chat/photo.jpg"}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/download", map[string]string{
		"message_id": "msg-123",
		"chat_jid":   "1234567890@s.whatsapp.net",
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseResponse(t, rr)
	assert.True(t, resp.Success)
	assert.True(t, backend.downloadMediaCalled)
	assert.Equal(t, "msg-123", backend.lastMessageID)
	assert.Equal(t, "1234567890@s.whatsapp.net", backend.lastChatJID)

	// Check the data field contains the path
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "/media/chat/photo.jpg", data["path"])
}

func TestDownload_MissingFields(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/download", map[string]string{
		"message_id": "msg-123",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.False(t, backend.downloadMediaCalled)
}

func TestDownload_BackendError(t *testing.T) {
	backend := &mockBackend{err: fmt.Errorf("media expired")}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/download", map[string]string{
		"message_id": "msg-123",
		"chat_jid":   "1234567890@s.whatsapp.net",
	})

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	resp := parseResponse(t, rr)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "media expired")
}

// --- Sync History -----------------------------------------------------------

func TestSyncHistory_Success(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/sync-history", map[string]string{
		"chat_jid": "group@g.us",
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseResponse(t, rr)
	assert.True(t, resp.Success)
	assert.Equal(t, "history sync requested", resp.Message)
	assert.True(t, backend.historySyncCalled)
	assert.Equal(t, "group@g.us", backend.lastChatJID)
}

func TestSyncHistory_MissingChatJID(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/sync-history", map[string]string{})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	resp := parseResponse(t, rr)
	assert.False(t, resp.Success)
	assert.False(t, backend.historySyncCalled)
}

func TestSyncHistory_NilBody(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/sync-history", nil)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	resp := parseResponse(t, rr)
	assert.False(t, resp.Success)
	assert.False(t, backend.historySyncCalled)
}

func TestSyncHistory_BackendError(t *testing.T) {
	backend := &mockBackend{err: fmt.Errorf("client not ready")}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "POST", "/api/sync-history", map[string]string{
		"chat_jid": "group@g.us",
	})

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	resp := parseResponse(t, rr)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "client not ready")
}

// --- Content-Type -----------------------------------------------------------

func TestResponseContentType(t *testing.T) {
	backend := &mockBackend{}
	srv := newTestServer(backend)

	rr := doRequest(t, srv, "GET", "/health", nil)

	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}
