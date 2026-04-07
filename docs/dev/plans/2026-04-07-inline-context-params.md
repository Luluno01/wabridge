# Inline Context Params Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `context_before` / `context_after` parameters to `list_messages` so callers can fetch surrounding messages at time-window edges without separate round trips.

**Architecture:** Extend `ListMessagesOpts` and `MessageResult` in the store layer, add context-fetching logic after the main query in `ListMessages()`, wire up two new MCP parameters with validation.

**Tech Stack:** Go, GORM (SQLite), mcp-go, testify

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/store/messages.go` | Modify | Add `IsContext` to `MessageResult`, add `ContextBefore`/`ContextAfter` to `ListMessagesOpts`, implement edge queries |
| `internal/store/store_test.go` | Modify | Add test cases for context edge extension |
| `internal/mcp/tools.go` | Modify | Add `context_before`/`context_after` params, validation, clamping |
| `docs/ops/MCP_TOOLS.md` | Modify | Document new params and `is_context` field |
| `docs/dev/backlogs/index.md` | Modify | Strike through completed item |

---

### Task 1: Add `IsContext` field to `MessageResult`

**Files:**
- Modify: `internal/store/messages.go:22-26`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write failing test — `IsContext` field exists and is `false` by default**

Add to `store_test.go` after the `TestListMessages` function (after line 248):

```go
func TestListMessages_ContextEdges(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.StoreMessage(&Message{
			ID:        fmt.Sprintf("msg%d", i),
			ChatJID:   "chat@g.us",
			Sender:    "alice@s.whatsapp.net",
			Content:   fmt.Sprintf("Message %d", i),
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}

	// Without context params, IsContext should be false on all results
	results, err := s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", Limit: 10})
	require.NoError(t, err)
	for _, r := range results {
		assert.False(t, r.IsContext)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/store/ -run TestListMessages_ContextEdges -v`
Expected: compile error — `MessageResult` has no field `IsContext`

- [ ] **Step 3: Add `IsContext` field to `MessageResult`**

In `internal/store/messages.go`, change the `MessageResult` struct (lines 22-26) to:

```go
type MessageResult struct {
	Message
	ChatName   string `json:"chat_name"`
	SenderName string `json:"sender_name"`
	IsContext  bool   `json:"is_context,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/store/ -run TestListMessages_ContextEdges -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/messages.go internal/store/store_test.go
git commit -m "feat: add IsContext field to MessageResult"
```

---

### Task 2: Add `ContextBefore` / `ContextAfter` to `ListMessagesOpts`

**Files:**
- Modify: `internal/store/messages.go:11-20`

- [ ] **Step 1: Extend the opts struct**

In `internal/store/messages.go`, change `ListMessagesOpts` (lines 11-20) to:

```go
type ListMessagesOpts struct {
	ChatJID       string
	Sender        string
	After         *time.Time
	Before        *time.Time
	Search        string
	Limit         int
	Page          int
	Latest        bool // if true, return most recent messages first
	ContextBefore int  // messages before the After boundary (edge extension)
	ContextAfter  int  // messages after the Before boundary (edge extension)
}
```

- [ ] **Step 2: Verify existing tests still pass**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/store/ -v`
Expected: all PASS (new fields default to 0, no behavior change)

- [ ] **Step 3: Commit**

```bash
git add internal/store/messages.go
git commit -m "feat: add ContextBefore/ContextAfter to ListMessagesOpts"
```

---

### Task 3: Implement context edge queries in `ListMessages`

**Files:**
- Modify: `internal/store/messages.go:76-115`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write failing test — context_before extends the window**

Add to the existing `TestListMessages_ContextEdges` function, after the "without context params" block:

```go
	// context_before: 2 messages before the After boundary
	after := base.Add(5 * time.Minute) // msg5 is first match
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:       "chat@g.us",
		After:         &after,
		ContextBefore: 2,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 7) // 2 context (msg3,msg4) + 5 matched (msg5..msg9)
	assert.True(t, results[0].IsContext)
	assert.Equal(t, "msg3", results[0].ID)
	assert.True(t, results[1].IsContext)
	assert.Equal(t, "msg4", results[1].ID)
	assert.False(t, results[2].IsContext)
	assert.Equal(t, "msg5", results[2].ID)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/store/ -run TestListMessages_ContextEdges -v`
Expected: FAIL — `assert.Len` expects 7, gets 5

- [ ] **Step 3: Implement context_before logic**

In `internal/store/messages.go`, replace the `ListMessages` function (lines 76-115) with:

```go
func (s *Store) ListMessages(opts ListMessagesOpts) ([]MessageResult, error) {
	var results []MessageResult

	query := messageJoins(s.db.Table("messages").Select(messageSelect))

	if opts.ChatJID != "" {
		query = query.Where("messages.chat_jid = ?", opts.ChatJID)
	}
	if opts.Sender != "" {
		query = query.Where("messages.sender = ?", opts.Sender)
	}
	if opts.After != nil {
		query = query.Where("messages.timestamp >= ?", *opts.After)
	}
	if opts.Before != nil {
		query = query.Where("messages.timestamp <= ?", *opts.Before)
	}
	if opts.Search != "" {
		like := "%" + opts.Search + "%"
		query = query.Where("messages.content LIKE ?", like)
	}

	if opts.Latest {
		query = query.Order("messages.timestamp DESC")
	} else {
		query = query.Order("messages.timestamp ASC")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	query = query.Limit(limit)

	if opts.Page > 1 {
		query = query.Offset((opts.Page - 1) * limit)
	}

	if err := query.Scan(&results).Error; err != nil {
		return nil, err
	}

	// Context before: fetch messages just before the After boundary
	if opts.ContextBefore > 0 && opts.After != nil && opts.ChatJID != "" {
		cb := opts.ContextBefore
		if cb > 20 {
			cb = 20
		}
		var before []MessageResult
		q := messageJoins(s.db.Table("messages").Select(messageSelect)).
			Where("messages.chat_jid = ? AND messages.timestamp < ?", opts.ChatJID, *opts.After).
			Order("messages.timestamp DESC").
			Limit(cb)
		if err := q.Scan(&before).Error; err != nil {
			return nil, fmt.Errorf("context before: %w", err)
		}
		for i, j := 0, len(before)-1; i < j; i, j = i+1, j-1 {
			before[i], before[j] = before[j], before[i]
		}
		for i := range before {
			before[i].IsContext = true
		}
		results = append(before, results...)
	}

	// Context after: fetch messages just after the Before boundary
	if opts.ContextAfter > 0 && opts.Before != nil && opts.ChatJID != "" {
		ca := opts.ContextAfter
		if ca > 20 {
			ca = 20
		}
		var after []MessageResult
		q := messageJoins(s.db.Table("messages").Select(messageSelect)).
			Where("messages.chat_jid = ? AND messages.timestamp > ?", opts.ChatJID, *opts.Before).
			Order("messages.timestamp ASC").
			Limit(ca)
		if err := q.Scan(&after).Error; err != nil {
			return nil, fmt.Errorf("context after: %w", err)
		}
		for i := range after {
			after[i].IsContext = true
		}
		results = append(results, after...)
	}

	return results, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/store/ -run TestListMessages_ContextEdges -v`
Expected: PASS

- [ ] **Step 5: Write failing test — context_after extends the window**

Add to `TestListMessages_ContextEdges`:

```go
	// context_after: 2 messages after the Before boundary
	before := base.Add(4 * time.Minute) // msg4 is last match
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:      "chat@g.us",
		Before:       &before,
		ContextAfter: 2,
		Limit:        10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 7) // 5 matched (msg0..msg4) + 2 context (msg5,msg6)
	assert.False(t, results[4].IsContext)
	assert.Equal(t, "msg4", results[4].ID)
	assert.True(t, results[5].IsContext)
	assert.Equal(t, "msg5", results[5].ID)
	assert.True(t, results[6].IsContext)
	assert.Equal(t, "msg6", results[6].ID)
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/store/ -run TestListMessages_ContextEdges -v`
Expected: PASS (implementation already handles both directions)

- [ ] **Step 7: Write failing test — both context_before and context_after**

Add to `TestListMessages_ContextEdges`:

```go
	// Both context_before and context_after
	after2 := base.Add(3 * time.Minute) // msg3 is first match
	before2 := base.Add(6 * time.Minute) // msg6 is last match
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:       "chat@g.us",
		After:         &after2,
		Before:        &before2,
		ContextBefore: 2,
		ContextAfter:  2,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 8) // 2 ctx (msg1,msg2) + 4 match (msg3..msg6) + 2 ctx (msg7,msg8)
	assert.True(t, results[0].IsContext)
	assert.Equal(t, "msg1", results[0].ID)
	assert.True(t, results[1].IsContext)
	assert.Equal(t, "msg2", results[1].ID)
	assert.False(t, results[2].IsContext)
	assert.Equal(t, "msg6", results[5].ID)
	assert.True(t, results[6].IsContext)
	assert.Equal(t, "msg7", results[6].ID)
	assert.True(t, results[7].IsContext)
	assert.Equal(t, "msg8", results[7].ID)
```

- [ ] **Step 8: Run test to verify it passes**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/store/ -run TestListMessages_ContextEdges -v`
Expected: PASS

- [ ] **Step 9: Write failing test — context without time window is no-op**

Add to `TestListMessages_ContextEdges`:

```go
	// context_before without After is a no-op
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:       "chat@g.us",
		ContextBefore: 3,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 10)
	for _, r := range results {
		assert.False(t, r.IsContext)
	}
```

- [ ] **Step 10: Run test to verify it passes**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/store/ -run TestListMessages_ContextEdges -v`
Expected: PASS

- [ ] **Step 11: Write failing test — fewer available than requested**

Add to `TestListMessages_ContextEdges`:

```go
	// Fewer context messages available than requested
	after3 := base.Add(1 * time.Minute) // msg1 is first match — only msg0 before it
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:       "chat@g.us",
		After:         &after3,
		ContextBefore: 5,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 10) // 1 context (msg0) + 9 matched (msg1..msg9)
	assert.True(t, results[0].IsContext)
	assert.Equal(t, "msg0", results[0].ID)
	assert.False(t, results[1].IsContext)
```

- [ ] **Step 12: Run test to verify it passes**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/store/ -run TestListMessages_ContextEdges -v`
Expected: PASS

- [ ] **Step 13: Run all store tests to verify no regressions**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/store/ -v`
Expected: all PASS

- [ ] **Step 14: Commit**

```bash
git add internal/store/messages.go internal/store/store_test.go
git commit -m "feat: implement context edge queries in ListMessages"
```

---

### Task 4: Wire up MCP parameters with validation

**Files:**
- Modify: `internal/mcp/tools.go:174-222`

- [ ] **Step 1: Add `context_before` and `context_after` params to tool registration**

In `internal/mcp/tools.go`, in the `registerListMessages` function, add two params after the `latest` line (line 185):

```go
		mcplib.WithNumber("context_before", mcplib.Description("Messages to include before the time window (requires chat_jid and after)")),
		mcplib.WithNumber("context_after", mcplib.Description("Messages to include after the time window (requires chat_jid and before)")),
```

- [ ] **Step 2: Extract and validate context params in the handler**

In the same function's handler (after opts is built, around line 195), add:

```go
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
```

- [ ] **Step 3: Verify the project compiles**

Run: `cd /home/untitled/personal/wabridge && go build ./...`
Expected: clean build

- [ ] **Step 4: Run all tests**

Run: `cd /home/untitled/personal/wabridge && go test ./... 2>&1 | tail -20`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/tools.go
git commit -m "feat: wire context_before/context_after MCP params with validation"
```

---

### Task 5: Update documentation

**Files:**
- Modify: `docs/ops/MCP_TOOLS.md:81-97`
- Modify: `docs/dev/backlogs/index.md:16`

- [ ] **Step 1: Add params to MCP_TOOLS.md list_messages table**

In `docs/ops/MCP_TOOLS.md`, add two rows to the `list_messages` parameter table (after the `latest` row, line 95) and update the returns description:

Add these rows to the table:

```
| `context_before` | number | no | Messages to include before the `after` boundary (max 20, requires `chat_jid`) |
| `context_after` | number | no | Messages to include after the `before` boundary (max 20, requires `chat_jid`) |
```

Update the returns description (line 97) to:

```
Returns: array of message objects with `chat_name` and `sender_name` resolved. When `context_before` or `context_after` is used, edge messages include `"is_context": true`. Reply messages include `quoted_message_id`, `quoted_sender`, `quoted_content`, and optionally `quoted_media_type` — see [SCHEMA.md](../dev/SCHEMA.md) for details.
```

- [ ] **Step 2: Strike through the backlog item**

In `docs/dev/backlogs/index.md`, change line 16 from:

```
| [Inline context params](2026-04-07-inline-context-params.md) | `context_before` / `context_after` on `list_messages` to return surrounding messages inline, avoiding per-message `get_message_context` round trips |
```

to:

```
| ~~[Inline context params](2026-04-07-inline-context-params.md)~~ | Done — `context_before` / `context_after` on `list_messages` return edge messages marked with `is_context: true` |
```

- [ ] **Step 3: Commit**

```bash
git add docs/ops/MCP_TOOLS.md docs/dev/backlogs/index.md
git commit -m "docs: document inline context params and mark backlog done"
```
