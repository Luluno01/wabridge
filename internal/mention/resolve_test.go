package mention

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func mockLookup(jid string) string {
	names := map[string]string{
		"1234567890@s.whatsapp.net": "Alice Smith",
		"9876543210@s.whatsapp.net": "Bob Jones",
		"555@lid":                   "Charlie",
	}
	return names[jid]
}

func TestResolve_SingleMention(t *testing.T) {
	content := "Hey @1234567890 check this out"
	jids := `["1234567890@s.whatsapp.net"]`

	result := Resolve(content, jids, mockLookup)
	assert.Equal(t, "Hey @Alice Smith check this out", result)
}

func TestResolve_MultipleMentions(t *testing.T) {
	content := "@1234567890 and @9876543210 should see this"
	jids := `["1234567890@s.whatsapp.net", "9876543210@s.whatsapp.net"]`

	result := Resolve(content, jids, mockLookup)
	assert.Equal(t, "@Alice Smith and @Bob Jones should see this", result)
}

func TestResolve_NoMentions(t *testing.T) {
	content := "Just a regular message"
	result := Resolve(content, "", mockLookup)
	assert.Equal(t, "Just a regular message", result)
}

func TestResolve_NilJIDs(t *testing.T) {
	content := "Message with @1234567890"
	result := Resolve(content, "", mockLookup)
	assert.Equal(t, "Message with @1234567890", result)
}

func TestResolve_UnknownContact(t *testing.T) {
	content := "Hey @9999999999"
	jids := `["9999999999@s.whatsapp.net"]`

	result := Resolve(content, jids, mockLookup)
	assert.Equal(t, "Hey @9999999999", result)
}

func TestResolve_LIDMention(t *testing.T) {
	content := "Hey @555"
	jids := `["555@lid"]`

	result := Resolve(content, jids, mockLookup)
	assert.Equal(t, "Hey @Charlie", result)
}

func TestResolve_MalformedJSON(t *testing.T) {
	content := "Hey @1234567890"
	result := Resolve(content, "not-json", mockLookup)
	assert.Equal(t, "Hey @1234567890", result)
}
