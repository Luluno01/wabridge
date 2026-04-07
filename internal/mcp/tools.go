package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"wabridge/internal/mention"
	"wabridge/internal/store"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// jsonResult marshals v as indented JSON and returns it as a tool result.
func jsonResult(v interface{}) (*mcplib.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return mcplib.NewToolResultText(string(data)), nil
}

// resolveMessages applies mention resolution to message results in-place.
// When raw is true, messages are returned as-is without resolution.
func (s *Server) resolveMessages(messages []store.MessageResult, raw bool) {
	if raw {
		return
	}
	for i := range messages {
		jids := ""
		if messages[i].MentionedJIDs != nil {
			jids = *messages[i].MentionedJIDs
		}
		messages[i].Content = mention.Resolve(
			messages[i].Content,
			jids,
			s.store.GetContactName,
		)
	}
}

// registerTools registers MCP tools on the server.
// Query tools are always registered. Action tools are gated by feature config.
func (s *Server) registerTools() {
	// Query tools — always registered
	s.registerSearchContacts()
	s.registerListChats()
	s.registerGetChat()
	s.registerGetDirectChatByContact()
	s.registerGetContactChats()
	s.registerListMessages()
	s.registerGetLastInteraction()
	s.registerGetMessageContext()

	// Action tools — conditional
	if s.features.Send {
		s.registerSendMessage()
		s.registerSendFile()
		s.registerSendAudioMessage()
	}
	if s.features.Download {
		s.registerDownloadMedia()
	}
	if s.features.HistorySync {
		s.registerRequestHistorySync()
	}
}

// --- Query tools ---

func (s *Server) registerSearchContacts() {
	tool := mcplib.NewTool("search_contacts",
		mcplib.WithDescription("Search contacts by name, phone number, or JID"),
		mcplib.WithString("query", mcplib.Required(), mcplib.Description("Search query string")),
		mcplib.WithNumber("limit", mcplib.Description("Maximum number of results (default 20)")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return nil, err
		}
		limit := req.GetInt("limit", 20)

		contacts, err := s.store.SearchContacts(query, limit)
		if err != nil {
			return nil, fmt.Errorf("search contacts: %w", err)
		}
		return jsonResult(contacts)
	})
}

func (s *Server) registerListChats() {
	tool := mcplib.NewTool("list_chats",
		mcplib.WithDescription("List chats, optionally filtered by name or JID"),
		mcplib.WithString("filter", mcplib.Description("Filter chats by name or JID")),
		mcplib.WithNumber("limit", mcplib.Description("Maximum number of results")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		filter := req.GetString("filter", "")
		limit := req.GetInt("limit", 0)

		chats, err := s.store.ListChats(filter, limit)
		if err != nil {
			return nil, fmt.Errorf("list chats: %w", err)
		}
		return jsonResult(chats)
	})
}

func (s *Server) registerGetChat() {
	tool := mcplib.NewTool("get_chat",
		mcplib.WithDescription("Get a specific chat by JID"),
		mcplib.WithString("jid", mcplib.Required(), mcplib.Description("Chat JID")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		jid, err := req.RequireString("jid")
		if err != nil {
			return nil, err
		}

		chat, err := s.store.GetChat(jid)
		if err != nil {
			return nil, fmt.Errorf("get chat: %w", err)
		}
		return jsonResult(chat)
	})
}

func (s *Server) registerGetDirectChatByContact() {
	tool := mcplib.NewTool("get_direct_chat_by_contact",
		mcplib.WithDescription("Find a direct (non-group) chat by phone number"),
		mcplib.WithString("phone", mcplib.Required(), mcplib.Description("Phone number to search for")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		phone, err := req.RequireString("phone")
		if err != nil {
			return nil, err
		}

		chat, err := s.store.GetDirectChatByPhone(phone)
		if err != nil {
			return nil, fmt.Errorf("get direct chat: %w", err)
		}
		if chat == nil {
			return mcplib.NewToolResultText("no direct chat found for phone: " + phone), nil
		}

		return jsonResult(chat)
	})
}

func (s *Server) registerGetContactChats() {
	tool := mcplib.NewTool("get_contact_chats",
		mcplib.WithDescription("List chats that a contact has participated in"),
		mcplib.WithString("jid", mcplib.Required(), mcplib.Description("Contact JID")),
		mcplib.WithNumber("limit", mcplib.Description("Maximum number of results (default 20)")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		jid, err := req.RequireString("jid")
		if err != nil {
			return nil, err
		}
		limit := req.GetInt("limit", 20)

		chats, err := s.store.GetContactChats(jid, limit)
		if err != nil {
			return nil, fmt.Errorf("get contact chats: %w", err)
		}
		return jsonResult(chats)
	})
}

func (s *Server) registerListMessages() {
	tool := mcplib.NewTool("list_messages",
		mcplib.WithDescription("List messages with filtering options"),
		mcplib.WithString("chat_jid", mcplib.Description("Filter by chat JID")),
		mcplib.WithString("sender", mcplib.Description("Filter by sender JID (exact match)")),
		mcplib.WithString("after", mcplib.Description("Only messages after this time (RFC3339)")),
		mcplib.WithString("before", mcplib.Description("Only messages before this time (RFC3339)")),
		mcplib.WithString("search", mcplib.Description("Search message content")),
		mcplib.WithNumber("limit", mcplib.Description("Maximum in-window messages returned, excluding context rows (default 50)")),
		mcplib.WithNumber("page", mcplib.Description("Page number for pagination")),
		mcplib.WithBoolean("raw", mcplib.Description("If true, skip mention resolution")),
		mcplib.WithBoolean("latest", mcplib.Description("If true, return most recent messages first (default false)")),
		mcplib.WithNumber("context_before", mcplib.Description("Messages to include before the time window (requires chat_jid; ignored without after)")),
		mcplib.WithNumber("context_after", mcplib.Description("Messages to include after the time window (requires chat_jid; ignored without before)")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		opts := store.ListMessagesOpts{
			ChatJID: req.GetString("chat_jid", ""),
			Sender:  req.GetString("sender", ""),
			Search:  req.GetString("search", ""),
			Limit:   req.GetInt("limit", 50),
			Page:    req.GetInt("page", 0),
			Latest:  req.GetBool("latest", false),
		}

		contextBefore := req.GetInt("context_before", 0)
		contextAfter := req.GetInt("context_after", 0)
		if (contextBefore > 0 || contextAfter > 0) && opts.ChatJID == "" {
			return nil, fmt.Errorf("context_before/context_after require chat_jid")
		}
		if contextBefore > 20 {
			contextBefore = 20
		}
		if contextAfter > 20 {
			contextAfter = 20
		}
		opts.ContextBefore = contextBefore
		opts.ContextAfter = contextAfter

		if afterStr := req.GetString("after", ""); afterStr != "" {
			t, err := time.Parse(time.RFC3339, afterStr)
			if err != nil {
				return nil, fmt.Errorf("invalid 'after' time format: %w", err)
			}
			opts.After = &t
		}
		if beforeStr := req.GetString("before", ""); beforeStr != "" {
			t, err := time.Parse(time.RFC3339, beforeStr)
			if err != nil {
				return nil, fmt.Errorf("invalid 'before' time format: %w", err)
			}
			opts.Before = &t
		}

		messages, err := s.store.ListMessages(opts)
		if err != nil {
			return nil, fmt.Errorf("list messages: %w", err)
		}

		raw := req.GetBool("raw", false)
		s.resolveMessages(messages, raw)

		return jsonResult(messages)
	})
}

func (s *Server) registerGetLastInteraction() {
	tool := mcplib.NewTool("get_last_interaction",
		mcplib.WithDescription("Get the last message sent by a contact"),
		mcplib.WithString("jid", mcplib.Required(), mcplib.Description("Contact JID")),
		mcplib.WithBoolean("raw", mcplib.Description("If true, skip mention resolution")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		jid, err := req.RequireString("jid")
		if err != nil {
			return nil, err
		}

		messages, err := s.store.ListMessages(store.ListMessagesOpts{
			Sender: jid,
			Limit:  1,
			Latest: true,
		})
		if err != nil {
			return nil, fmt.Errorf("get last interaction: %w", err)
		}
		if len(messages) == 0 {
			return mcplib.NewToolResultText("no messages found for: " + jid), nil
		}

		raw := req.GetBool("raw", false)
		s.resolveMessages(messages, raw)

		return jsonResult(messages[0])
	})
}

func (s *Server) registerGetMessageContext() {
	tool := mcplib.NewTool("get_message_context",
		mcplib.WithDescription("Get messages surrounding a specific message"),
		mcplib.WithString("message_id", mcplib.Required(), mcplib.Description("Message ID")),
		mcplib.WithString("chat_jid", mcplib.Required(), mcplib.Description("Chat JID the message belongs to")),
		mcplib.WithNumber("before", mcplib.Description("Number of messages before (default 10)")),
		mcplib.WithNumber("after", mcplib.Description("Number of messages after (default 10)")),
		mcplib.WithBoolean("raw", mcplib.Description("If true, skip mention resolution")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		messageID, err := req.RequireString("message_id")
		if err != nil {
			return nil, err
		}
		chatJID, err := req.RequireString("chat_jid")
		if err != nil {
			return nil, err
		}
		beforeCount := req.GetInt("before", 10)
		afterCount := req.GetInt("after", 10)

		messages, err := s.store.GetMessageContext(messageID, chatJID, beforeCount, afterCount)
		if err != nil {
			return nil, fmt.Errorf("get message context: %w", err)
		}

		raw := req.GetBool("raw", false)
		s.resolveMessages(messages, raw)

		return jsonResult(messages)
	})
}

// --- Action tools ---

func (s *Server) registerSendMessage() {
	tool := mcplib.NewTool("send_message",
		mcplib.WithDescription("Send a text message to a recipient"),
		mcplib.WithString("recipient", mcplib.Required(), mcplib.Description("Recipient JID or phone number")),
		mcplib.WithString("message", mcplib.Required(), mcplib.Description("Message text to send")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		recipient, err := req.RequireString("recipient")
		if err != nil {
			return nil, err
		}
		message, err := req.RequireString("message")
		if err != nil {
			return nil, err
		}

		if err := s.backend.SendMessage(ctx, recipient, message); err != nil {
			return nil, fmt.Errorf("send message: %w", err)
		}
		return mcplib.NewToolResultText("message sent to " + recipient), nil
	})
}

func (s *Server) registerSendFile() {
	tool := mcplib.NewTool("send_file",
		mcplib.WithDescription("Send a file to a recipient"),
		mcplib.WithString("recipient", mcplib.Required(), mcplib.Description("Recipient JID or phone number")),
		mcplib.WithString("file_path", mcplib.Required(), mcplib.Description("Path to the file to send")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		recipient, err := req.RequireString("recipient")
		if err != nil {
			return nil, err
		}
		filePath, err := req.RequireString("file_path")
		if err != nil {
			return nil, err
		}

		if err := s.backend.SendFile(ctx, recipient, filePath); err != nil {
			return nil, fmt.Errorf("send file: %w", err)
		}
		return mcplib.NewToolResultText("file sent to " + recipient), nil
	})
}

func (s *Server) registerSendAudioMessage() {
	tool := mcplib.NewTool("send_audio_message",
		mcplib.WithDescription("Send an audio file as a voice message"),
		mcplib.WithString("recipient", mcplib.Required(), mcplib.Description("Recipient JID or phone number")),
		mcplib.WithString("file_path", mcplib.Required(), mcplib.Description("Path to the Ogg Opus audio file")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		recipient, err := req.RequireString("recipient")
		if err != nil {
			return nil, err
		}
		filePath, err := req.RequireString("file_path")
		if err != nil {
			return nil, err
		}

		if err := s.backend.SendAudioMessage(ctx, recipient, filePath); err != nil {
			return nil, fmt.Errorf("send audio message: %w", err)
		}
		return mcplib.NewToolResultText("audio message sent to " + recipient), nil
	})
}

func (s *Server) registerDownloadMedia() {
	tool := mcplib.NewTool("download_media",
		mcplib.WithDescription("Download media from a message"),
		mcplib.WithString("message_id", mcplib.Required(), mcplib.Description("Message ID")),
		mcplib.WithString("chat_jid", mcplib.Required(), mcplib.Description("Chat JID the message belongs to")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		messageID, err := req.RequireString("message_id")
		if err != nil {
			return nil, err
		}
		chatJID, err := req.RequireString("chat_jid")
		if err != nil {
			return nil, err
		}

		path, err := s.backend.DownloadMedia(ctx, messageID, chatJID)
		if err != nil {
			return nil, fmt.Errorf("download media: %w", err)
		}
		return mcplib.NewToolResultText("media downloaded to: " + path), nil
	})
}

func (s *Server) registerRequestHistorySync() {
	tool := mcplib.NewTool("request_history_sync",
		mcplib.WithDescription("Request additional message history from the primary device for a specific chat"),
		mcplib.WithString("chat_jid", mcplib.Required(), mcplib.Description("JID of the chat to request older history for")),
	)
	s.mcp.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		chatJID, err := req.RequireString("chat_jid")
		if err != nil {
			return nil, err
		}
		if err := s.backend.RequestHistorySync(ctx, chatJID); err != nil {
			return nil, fmt.Errorf("request history sync: %w", err)
		}
		return mcplib.NewToolResultText("history sync requested for " + chatJID), nil
	})
}
