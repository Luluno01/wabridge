package action

import "context"

// Backend abstracts actions requiring a live WhatsApp connection.
// DirectBackend implements this for standalone mode (calling whatsmeow directly).
// APIClient implements this for bridge mode (proxying over HTTP).
type Backend interface {
	SendMessage(ctx context.Context, recipient, text string) error
	SendFile(ctx context.Context, recipient, filePath string) error
	SendAudioMessage(ctx context.Context, recipient, filePath string) error
	DownloadMedia(ctx context.Context, messageID, chatJID string) (string, error)
	RequestHistorySync(ctx context.Context) error
}
