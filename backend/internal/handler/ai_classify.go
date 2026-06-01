package handler

// aiToolCall is the minimal shape needed by the classifier and dispatcher.
type aiToolCall struct {
	Name  string
	ID    string
	Input string // raw JSON
}

// classifyToolCalls splits a slice of tool calls into read / write / ask_user batches.
// Read tools: lookup_page, read_page, search_pages, websearch, webfetch.
// Write tools: create_page, update_page, patch_page, delete_page, link_pages, move_page.
// ask_user: ask_user.
// Unknown names are routed to writeBatch (will fail validation at execution).
func classifyToolCalls(calls []aiToolCall) (reads, writes, asks []aiToolCall) {
	readSet := map[string]bool{
		"lookup_page": true, "read_page": true, "search_pages": true,
		"websearch": true, "webfetch": true,
	}
	askSet := map[string]bool{"ask_user": true}

	for _, c := range calls {
		switch {
		case askSet[c.Name]:
			asks = append(asks, c)
		case readSet[c.Name]:
			reads = append(reads, c)
		default:
			writes = append(writes, c)
		}
	}
	return
}
