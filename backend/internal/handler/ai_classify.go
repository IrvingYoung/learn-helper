package handler

// aiToolCall is the minimal shape needed by the classifier and dispatcher.
type aiToolCall struct {
	Name  string
	ID    string
	Input string // raw JSON
}

// classifyToolCalls splits a slice of tool calls into read / write / ask_user
// / load_skill batches. Read tools: lookup_page, read_page, search_pages,
// list_backlinks, list_links, list_children, find_broken_links, websearch,
// webfetch. Write tools: create_page, update_page, patch_page, delete_page,
// link_pages, move_page. ask_user: ask_user. load_skill: load_skill
// (progressive-disclosure skill loader, no side effects). Unknown names are
// routed to writeBatch (will fail validation at execution).
func classifyToolCalls(calls []aiToolCall) (reads, writes, asks, loadSkills []aiToolCall) {
	readSet := map[string]bool{
		"lookup_page": true, "read_page": true, "search_pages": true,
		"list_backlinks": true, "list_links": true, "list_children": true,
		"find_broken_links": true,
		"websearch":         true, "webfetch": true,
	}
	askSet := map[string]bool{"ask_user": true}
	loadSkillSet := map[string]bool{"load_skill": true}

	for _, c := range calls {
		switch {
		case askSet[c.Name]:
			asks = append(asks, c)
		case loadSkillSet[c.Name]:
			loadSkills = append(loadSkills, c)
		case readSet[c.Name]:
			reads = append(reads, c)
		default:
			writes = append(writes, c)
		}
	}
	return
}
