package whatsapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestExtractContextInfo_NoContextInfo(t *testing.T) {
	msg := &waE2E.Message{
		Conversation: proto.String("plain text"),
	}
	result := extractContextInfo(msg)
	assert.Empty(t, result.MentionedJIDs)
	assert.Empty(t, result.QuotedMessageID)
	assert.Empty(t, result.QuotedSender)
	assert.Empty(t, result.QuotedContent)
	assert.Empty(t, result.QuotedMediaType)
}

func TestExtractContextInfo_NilMessage(t *testing.T) {
	result := extractContextInfo(nil)
	assert.Empty(t, result.MentionedJIDs)
	assert.Empty(t, result.QuotedMessageID)
	assert.Empty(t, result.QuotedSender)
	assert.Empty(t, result.QuotedContent)
	assert.Empty(t, result.QuotedMediaType)
}

func TestExtractContextInfo_QuotedTextMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("I agree"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:    proto.String("original-msg-id"),
				Participant: proto.String("bob@s.whatsapp.net"),
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String("What do you think?"),
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "original-msg-id", result.QuotedMessageID)
	assert.Equal(t, "bob@s.whatsapp.net", result.QuotedSender)
	assert.Equal(t, "What do you think?", result.QuotedContent)
	assert.Empty(t, result.QuotedMediaType)
}

func TestExtractContextInfo_QuotedImageMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("Nice photo!"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:    proto.String("img-msg-id"),
				Participant: proto.String("alice@s.whatsapp.net"),
				QuotedMessage: &waE2E.Message{
					ImageMessage: &waE2E.ImageMessage{
						Caption: proto.String("sunset"),
					},
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "img-msg-id", result.QuotedMessageID)
	assert.Equal(t, "sunset", result.QuotedContent)
	assert.Equal(t, "image", result.QuotedMediaType)
}

func TestExtractContextInfo_QuotedVideoMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("LOL"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("vid-msg-id"),
				QuotedMessage: &waE2E.Message{
					VideoMessage: &waE2E.VideoMessage{
						Caption: proto.String("funny clip"),
					},
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "vid-msg-id", result.QuotedMessageID)
	assert.Equal(t, "funny clip", result.QuotedContent)
	assert.Equal(t, "video", result.QuotedMediaType)
}

func TestExtractContextInfo_QuotedAudioMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("heard it"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("aud-msg-id"),
				QuotedMessage: &waE2E.Message{
					AudioMessage: &waE2E.AudioMessage{},
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "aud-msg-id", result.QuotedMessageID)
	assert.Equal(t, "audio", result.QuotedMediaType)
	assert.Empty(t, result.QuotedContent)
}

func TestExtractContextInfo_QuotedDocumentMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("thanks for the doc"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("doc-msg-id"),
				QuotedMessage: &waE2E.Message{
					DocumentMessage: &waE2E.DocumentMessage{
						Caption: proto.String("quarterly report"),
					},
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "doc-msg-id", result.QuotedMessageID)
	assert.Equal(t, "document", result.QuotedMediaType)
	assert.Equal(t, "quarterly report", result.QuotedContent)
}

func TestExtractContextInfo_QuotedStickerMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("haha"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("stk-msg-id"),
				QuotedMessage: &waE2E.Message{
					StickerMessage: &waE2E.StickerMessage{},
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "sticker", result.QuotedMediaType)
	assert.Empty(t, result.QuotedContent)
}

func TestExtractContextInfo_MentionsPreserved(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("Hey @alice"),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: []string{"alice@s.whatsapp.net"},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Contains(t, result.MentionedJIDs, "alice@s.whatsapp.net")
	assert.Empty(t, result.QuotedMessageID)
}

func TestExtractContextInfo_MentionsAndQuotedTogether(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("@alice I agree with Bob"),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: []string{"alice@s.whatsapp.net"},
				StanzaID:     proto.String("bob-msg"),
				Participant:  proto.String("bob@s.whatsapp.net"),
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String("Let's do it"),
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Contains(t, result.MentionedJIDs, "alice@s.whatsapp.net")
	assert.Equal(t, "bob-msg", result.QuotedMessageID)
	assert.Equal(t, "bob@s.whatsapp.net", result.QuotedSender)
	assert.Equal(t, "Let's do it", result.QuotedContent)
	assert.Empty(t, result.QuotedMediaType)
}

func TestExtractContextInfo_NoParticipant(t *testing.T) {
	// Self-reply in 1-1 chat: WhatsApp omits Participant
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("replying to myself"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("self-msg-id"),
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String("my earlier message"),
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "self-msg-id", result.QuotedMessageID)
	assert.Empty(t, result.QuotedSender)
	assert.Equal(t, "my earlier message", result.QuotedContent)
}

func TestExtractContextInfo_ExpiredQuotedMessage(t *testing.T) {
	// StanzaID present but QuotedMessage nil (expired or stripped)
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("replying to deleted message"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:    proto.String("deleted-msg-id"),
				Participant: proto.String("bob@s.whatsapp.net"),
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "deleted-msg-id", result.QuotedMessageID)
	assert.Equal(t, "bob@s.whatsapp.net", result.QuotedSender)
	assert.Empty(t, result.QuotedContent)
	assert.Empty(t, result.QuotedMediaType)
}

func TestExtractContextInfo_ImageMessageWithContextInfo(t *testing.T) {
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption: proto.String("replying with a photo"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:    proto.String("orig-id"),
				Participant: proto.String("sender@s.whatsapp.net"),
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String("send me a pic"),
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "orig-id", result.QuotedMessageID)
	assert.Equal(t, "send me a pic", result.QuotedContent)
}
