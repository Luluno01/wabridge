package whatsapp

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "github.com/mattn/go-sqlite3"

	appstore "wabridge/internal/store"
)

// Client wraps a whatsmeow client with application-level state.
type Client struct {
	WAClient *whatsmeow.Client
	Store    *appstore.Store
	Log      waLog.Logger

	mu            sync.Mutex
	syncSettled   chan struct{}
	lastSyncEvent int64
}

// NewClient creates a new WhatsApp client backed by the given session database
// and application store. The logLevel controls whatsmeow log verbosity
// (e.g. "INFO", "WARN", "DEBUG").
func NewClient(sessionDBPath string, appStore *appstore.Store, logLevel string) (*Client, error) {
	dbLog := waLog.Stdout("Database", logLevel, true)

	container, err := sqlstore.New(context.Background(), "sqlite3",
		"file:"+sessionDBPath+"?_foreign_keys=on", dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to open session store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	clientLog := waLog.Stdout("Client", logLevel, true)
	waClient := whatsmeow.NewClient(deviceStore, clientLog)

	return &Client{
		WAClient:    waClient,
		Store:       appStore,
		Log:         clientLog,
		syncSettled: make(chan struct{}),
	}, nil
}

// Connect establishes a connection to WhatsApp. If no session exists, it
// initiates QR code pairing by printing QR codes to stderr. Otherwise it
// reconnects with the existing session.
func (c *Client) Connect(ctx context.Context) error {
	if c.WAClient.Store.ID == nil {
		// No session — need QR code pairing
		qrChan, err := c.WAClient.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("failed to get QR channel: %w", err)
		}

		if err := c.WAClient.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		for evt := range qrChan {
			switch evt.Event {
			case "code":
				fmt.Println("Scan this QR code with WhatsApp:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stderr)
			case "success":
				c.Log.Infof("Login successful")
				return nil
			case "timeout":
				return fmt.Errorf("QR code timed out")
			}
		}
		return nil
	}

	// Existing session — just connect
	return c.WAClient.Connect()
}

// Disconnect cleanly disconnects from WhatsApp.
func (c *Client) Disconnect() {
	c.WAClient.Disconnect()
}

// IsConnected returns true if the client has an active connection to WhatsApp.
func (c *Client) IsConnected() bool {
	return c.WAClient.IsConnected()
}
