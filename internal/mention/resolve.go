package mention

import (
	"encoding/json"
	"strings"
)

// Resolve replaces @<number> patterns in content with display names
// using the mentioned JIDs list and a lookup function.
// The lookupFn takes a full JID and returns a display name (or empty string).
func Resolve(content, mentionedJIDsJSON string, lookupFn func(jid string) string) string {
	if mentionedJIDsJSON == "" {
		return content
	}

	var jids []string
	if err := json.Unmarshal([]byte(mentionedJIDsJSON), &jids); err != nil {
		return content
	}

	for _, jid := range jids {
		name := lookupFn(jid)
		if name == "" {
			continue
		}

		// Extract user part: "1234567890@s.whatsapp.net" -> "1234567890"
		// or "555@lid" -> "555"
		user := jid
		if idx := strings.Index(jid, "@"); idx > 0 {
			user = jid[:idx]
		}

		content = strings.ReplaceAll(content, "@"+user, "@"+name)
	}

	return content
}
