package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"wabridge/internal/store"
	"wabridge/internal/whatsapp"
)

// ActionBackend abstracts actions requiring a live WhatsApp connection.
// DirectBackend implements this for standalone mode (calling whatsmeow directly).
// A future REST-backed implementation can proxy these calls to a bridge server.
type ActionBackend interface {
	SendMessage(ctx context.Context, recipient, text string) error
	SendFile(ctx context.Context, recipient, filePath string) error
	SendAudioMessage(ctx context.Context, recipient, filePath string) error
	DownloadMedia(ctx context.Context, messageID, chatJID string) (string, error)
	RequestHistorySync(ctx context.Context) error
}

// DirectBackend implements ActionBackend by calling whatsmeow directly.
// It is used in standalone mode where the MCP server and WhatsApp client
// run in the same process.
type DirectBackend struct {
	Client   *whatsapp.Client
	Store    *store.Store
	MediaDir string // Base directory for downloaded media
}

// NewDirectBackend creates a new DirectBackend.
func NewDirectBackend(client *whatsapp.Client, appStore *store.Store, mediaDir string) *DirectBackend {
	return &DirectBackend{
		Client:   client,
		Store:    appStore,
		MediaDir: mediaDir,
	}
}

// SendMessage sends a plain text message to the given recipient.
// The recipient can be a full JID (e.g. "1234567890@s.whatsapp.net") or
// a bare phone number (which will be treated as an individual chat).
func (b *DirectBackend) SendMessage(ctx context.Context, recipient, text string) error {
	jid, err := parseRecipient(recipient)
	if err != nil {
		return err
	}

	msg := &waE2E.Message{
		Conversation: proto.String(text),
	}

	_, err = b.Client.WAClient.SendMessage(ctx, jid, msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return nil
}

// SendFile sends a file as a media message. The media type is detected from
// the file extension: images, videos, audio, or documents. An optional caption
// is not supported through this method; use SendMessage for text.
func (b *DirectBackend) SendFile(ctx context.Context, recipient, filePath string) error {
	jid, err := parseRecipient(recipient)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" {
		ext = ext[1:] // strip leading dot
	}
	mediaType, mimeType := detectMediaType(ext)

	resp, err := b.Client.WAClient.Upload(ctx, data, mediaType)
	if err != nil {
		return fmt.Errorf("failed to upload media: %w", err)
	}

	msg := buildMediaMessage(mediaType, mimeType, filePath, &resp, uint64(len(data)))

	_, err = b.Client.WAClient.SendMessage(ctx, jid, msg)
	if err != nil {
		return fmt.Errorf("failed to send media message: %w", err)
	}
	return nil
}

// SendAudioMessage sends an Ogg Opus audio file as a push-to-talk (PTT) voice
// message. It analyzes the file for duration and generates a waveform.
func (b *DirectBackend) SendAudioMessage(ctx context.Context, recipient, filePath string) error {
	jid, err := parseRecipient(recipient)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}

	// Analyze Ogg Opus for duration
	duration, err := whatsapp.AnalyzeOggOpus(data)
	if err != nil {
		return fmt.Errorf("failed to analyze Ogg Opus file: %w", err)
	}
	waveform := whatsapp.PlaceholderWaveform(duration)

	// Upload as audio
	resp, err := b.Client.WAClient.Upload(ctx, data, whatsmeow.MediaAudio)
	if err != nil {
		return fmt.Errorf("failed to upload audio: %w", err)
	}

	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			Mimetype:      proto.String("audio/ogg; codecs=opus"),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
			Seconds:       proto.Uint32(duration),
			PTT:           proto.Bool(true),
			Waveform:      waveform,
		},
	}

	_, err = b.Client.WAClient.SendMessage(ctx, jid, msg)
	if err != nil {
		return fmt.Errorf("failed to send audio message: %w", err)
	}
	return nil
}

// DownloadMedia downloads media for a stored message identified by messageID
// and chatJID. It returns the absolute path to the downloaded file.
func (b *DirectBackend) DownloadMedia(ctx context.Context, messageID, chatJID string) (string, error) {
	msg, err := b.Store.GetMessage(messageID, chatJID)
	if err != nil {
		return "", fmt.Errorf("message not found: %w", err)
	}

	// Build a per-chat output directory
	sanitizedChat := strings.ReplaceAll(chatJID, ":", "_")
	outputDir := filepath.Join(b.MediaDir, sanitizedChat)

	return b.Client.DownloadMedia(ctx, msg, outputDir)
}

// RequestHistorySync requests additional history from the user's primary
// device. The response arrives asynchronously as HistorySync events.
// This method wraps BuildHistorySyncRequest with panic recovery since
// it can panic if internal client state is incomplete.
func (b *DirectBackend) RequestHistorySync(ctx context.Context) error {
	var historyMsg *waE2E.Message
	var panicErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = fmt.Errorf("panic in BuildHistorySyncRequest: %v", r)
			}
		}()
		historyMsg = b.Client.WAClient.BuildHistorySyncRequest(nil, 100)
	}()

	if panicErr != nil {
		return panicErr
	}
	if historyMsg == nil {
		return fmt.Errorf("failed to build history sync request: client state may not be ready")
	}

	_, err := b.Client.WAClient.SendPeerMessage(ctx, historyMsg)
	if err != nil {
		return fmt.Errorf("failed to send history sync request: %w", err)
	}
	return nil
}

// parseRecipient parses a recipient string into a JID. If it contains "@",
// it is parsed as a full JID. Otherwise it is treated as a phone number
// for a personal chat on s.whatsapp.net.
func parseRecipient(recipient string) (types.JID, error) {
	if strings.Contains(recipient, "@") {
		jid, err := types.ParseJID(recipient)
		if err != nil {
			return types.JID{}, fmt.Errorf("invalid JID %q: %w", recipient, err)
		}
		return jid, nil
	}

	return types.JID{
		User:   recipient,
		Server: "s.whatsapp.net",
	}, nil
}

// detectMediaType returns the whatsmeow media type and MIME type for a given
// file extension (without leading dot).
func detectMediaType(ext string) (whatsmeow.MediaType, string) {
	switch ext {
	// Image types
	case "jpg", "jpeg":
		return whatsmeow.MediaImage, "image/jpeg"
	case "png":
		return whatsmeow.MediaImage, "image/png"
	case "gif":
		return whatsmeow.MediaImage, "image/gif"
	case "webp":
		return whatsmeow.MediaImage, "image/webp"

	// Video types
	case "mp4":
		return whatsmeow.MediaVideo, "video/mp4"
	case "avi":
		return whatsmeow.MediaVideo, "video/avi"
	case "mov":
		return whatsmeow.MediaVideo, "video/quicktime"
	case "mkv":
		return whatsmeow.MediaVideo, "video/x-matroska"

	// Audio types
	case "ogg":
		return whatsmeow.MediaAudio, "audio/ogg; codecs=opus"
	case "mp3":
		return whatsmeow.MediaAudio, "audio/mpeg"
	case "wav":
		return whatsmeow.MediaAudio, "audio/wav"
	case "m4a":
		return whatsmeow.MediaAudio, "audio/mp4"

	// Everything else is a document
	default:
		return whatsmeow.MediaDocument, "application/octet-stream"
	}
}

// buildMediaMessage constructs the appropriate waE2E.Message for the given
// media type, populated with upload response metadata.
func buildMediaMessage(mediaType whatsmeow.MediaType, mimeType, filePath string, resp *whatsmeow.UploadResponse, fileSize uint64) *waE2E.Message {
	switch mediaType {
	case whatsmeow.MediaImage:
		return &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
			},
		}
	case whatsmeow.MediaVideo:
		return &waE2E.Message{
			VideoMessage: &waE2E.VideoMessage{
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
			},
		}
	case whatsmeow.MediaAudio:
		return &waE2E.Message{
			AudioMessage: &waE2E.AudioMessage{
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
			},
		}
	default:
		// Document (catch-all)
		filename := filepath.Base(filePath)
		return &waE2E.Message{
			DocumentMessage: &waE2E.DocumentMessage{
				Title:         proto.String(filename),
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
			},
		}
	}
}
