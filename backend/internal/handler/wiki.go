package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"learn-helper/internal/model"
)

type WikiHandler struct {
	db      *sql.DB
	queries *model.Queries
}

func NewWikiHandler(db *sql.DB) *WikiHandler {
	return &WikiHandler{
		db:      db,
		queries: model.New(db),
	}
}

type WikiTreeNode struct {
	ID            int64          `json:"id"`
	Title         string         `json:"title"`
	Slug          string         `json:"slug"`
	PageType      string         `json:"page_type"`
	ContentStatus string         `json:"content_status"`
	ParentID      *int64         `json:"parent_id"`
	Path          string         `json:"path"`
	SortOrder     int64          `json:"sort_order"`
	Children      []*WikiTreeNode `json:"children,omitempty"`
}

type WikiPageResponse struct {
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	Slug          string  `json:"slug"`
	PageType      string  `json:"page_type"`
	Content       string  `json:"content"`
	Tags          string  `json:"tags"`
	ParentID      *int64  `json:"parent_id"`
	ContentStatus string  `json:"content_status"`
	SortOrder     int64   `json:"sort_order"`
	Links         []int64 `json:"links"`
	Backlinks     []int64 `json:"backlinks"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

func nullInt64ToPtr(n sql.NullInt64) *int64 {
	if n.Valid {
		return &n.Int64
	}
	return nil
}

// --- Link maintenance helpers ---

var wikiLinkPattern = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// updatePageLinks parses wiki links from content and updates the links/backlinks columns.
func (h *WikiHandler) updatePageLinks(pageID int64, content string) {
	matches := wikiLinkPattern.FindAllStringSubmatch(content, -1)

	newLinkIDs := make(map[int64]bool)
	for _, m := range matches {
		title := m[1]
		var targetID int64
		err := h.db.QueryRow("SELECT id FROM wiki_pages WHERE title = ?", title).Scan(&targetID)
		if err != nil || targetID == pageID {
			continue
		}
		newLinkIDs[targetID] = true
	}

	// Get old links
	var oldLinksJSON string
	h.db.QueryRow("SELECT COALESCE(links, '[]') FROM wiki_pages WHERE id = ?", pageID).Scan(&oldLinksJSON)
	var oldLinkIDs []int64
	json.Unmarshal([]byte(oldLinksJSON), &oldLinkIDs)
	oldLinkSet := make(map[int64]bool)
	for _, id := range oldLinkIDs {
		oldLinkSet[id] = true
	}

	// Build new links array and update
	var newLinks []int64
	for id := range newLinkIDs {
		newLinks = append(newLinks, id)
	}
	linksJSON, _ := json.Marshal(newLinks)
	h.db.Exec("UPDATE wiki_pages SET links = ? WHERE id = ?", string(linksJSON), pageID)

	// Add backlinks for new links
	for targetID := range newLinkIDs {
		if !oldLinkSet[targetID] {
			// Newly linked — add pageID to target's backlinks
			h.addBacklink(targetID, pageID)
		}
	}

	// Remove backlinks for old links that are no longer present
	for _, targetID := range oldLinkIDs {
		if !newLinkIDs[targetID] {
			// No longer linked — remove pageID from target's backlinks
			h.removeBacklinkEntry(targetID, pageID)
		}
	}
}

// addBacklink adds backlinkID to the backlinks array of pageID.
func (h *WikiHandler) addBacklink(pageID, backlinkID int64) {
	var backlinks string
	err := h.db.QueryRow("SELECT COALESCE(backlinks, '[]') FROM wiki_pages WHERE id = ?", pageID).Scan(&backlinks)
	if err != nil {
		return
	}
	var blIDs []int64
	json.Unmarshal([]byte(backlinks), &blIDs)
	for _, id := range blIDs {
		if id == backlinkID {
			return // already present
		}
	}
	blIDs = append(blIDs, backlinkID)
	blJSON, _ := json.Marshal(blIDs)
	h.db.Exec("UPDATE wiki_pages SET backlinks = ? WHERE id = ?", string(blJSON), pageID)
}

// removeBacklinkEntry removes backlinkID from the backlinks array of pageID.
func (h *WikiHandler) removeBacklinkEntry(pageID, backlinkID int64) {
	var backlinks string
	err := h.db.QueryRow("SELECT COALESCE(backlinks, '[]') FROM wiki_pages WHERE id = ?", pageID).Scan(&backlinks)
	if err != nil {
		return
	}
	var blIDs []int64
	json.Unmarshal([]byte(backlinks), &blIDs)
	filtered := make([]int64, 0, len(blIDs))
	for _, id := range blIDs {
		if id != backlinkID {
			filtered = append(filtered, id)
		}
	}
	blJSON, _ := json.Marshal(filtered)
	h.db.Exec("UPDATE wiki_pages SET backlinks = ? WHERE id = ?", string(blJSON), pageID)
}

// removeLinkEntry removes linkID from the links array of pageID.
func (h *WikiHandler) removeLinkEntry(pageID, linkID int64) {
	var links string
	err := h.db.QueryRow("SELECT COALESCE(links, '[]') FROM wiki_pages WHERE id = ?", pageID).Scan(&links)
	if err != nil {
		return
	}
	var lIDs []int64
	json.Unmarshal([]byte(links), &lIDs)
	filtered := make([]int64, 0, len(lIDs))
	for _, id := range lIDs {
		if id != linkID {
			filtered = append(filtered, id)
		}
	}
	lJSON, _ := json.Marshal(filtered)
	h.db.Exec("UPDATE wiki_pages SET links = ? WHERE id = ?", string(lJSON), pageID)
}

// cleanupLinksForPage removes all link/backlink references when a page is deleted.
func (h *WikiHandler) cleanupLinksForPage(pageID int64) {
	// Remove pageID from backlinks of all pages this page links to
	var links string
	err := h.db.QueryRow("SELECT COALESCE(links, '[]') FROM wiki_pages WHERE id = ?", pageID).Scan(&links)
	if err == nil {
		var linkIDs []int64
		json.Unmarshal([]byte(links), &linkIDs)
		for _, targetID := range linkIDs {
			h.removeBacklinkEntry(targetID, pageID)
		}
	}

	// Remove pageID from links of all pages that link to this page, and also update their content
	var backlinks string
	err = h.db.QueryRow("SELECT COALESCE(backlinks, '[]') FROM wiki_pages WHERE id = ?", pageID).Scan(&backlinks)
	if err == nil {
		var blIDs []int64
		json.Unmarshal([]byte(backlinks), &blIDs)
		// Get the title of the page being deleted
		var title string
		h.db.QueryRow("SELECT title FROM wiki_pages WHERE id = ?", pageID).Scan(&title)
		for _, sourceID := range blIDs {
			h.removeLinkEntry(sourceID, pageID)
			// Also remove [[title]] from content
			if title != "" {
				oldLink := fmt.Sprintf("[[%s]]", title)
				var content string
				h.db.QueryRow("SELECT content FROM wiki_pages WHERE id = ?", sourceID).Scan(&content)
				newContent := strings.ReplaceAll(content, oldLink, "")
				contentStatus := "published"
				if strings.TrimSpace(newContent) == "" {
					contentStatus = "empty"
				}
				h.db.Exec("UPDATE wiki_pages SET content = ?, content_status = ? WHERE id = ?", newContent, contentStatus, sourceID)
				// Re-parse links for this source page
				h.updatePageLinks(sourceID, newContent)
			}
		}
	}

	// Clear this page's own links and backlinks
	h.db.Exec("UPDATE wiki_pages SET links = '[]', backlinks = '[]' WHERE id = ?", pageID)
}

func (h *WikiHandler) GetWikiTree(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pages, err := h.queries.GetWikiPageTree(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch wiki tree", http.StatusInternalServerError)
		return
	}

	nodeMap := make(map[int64]*WikiTreeNode)
	var roots []*WikiTreeNode

	for i := range pages {
		p := pages[i]
		node := &WikiTreeNode{
			ID:            p.ID,
			Title:         p.Title,
			Slug:          p.Slug,
			PageType:      p.PageType,
			ContentStatus: p.ContentStatus,
			ParentID:      nullInt64ToPtr(p.ParentID),
			Path:          p.Path,
			SortOrder:     p.SortOrder,
			Children:      []*WikiTreeNode{},
		}
		nodeMap[p.ID] = node
	}

	for i := range pages {
		p := pages[i]
		if !p.ParentID.Valid || p.ParentID.Int64 == 0 {
			roots = append(roots, nodeMap[p.ID])
		} else if parent, ok := nodeMap[p.ParentID.Int64]; ok {
			parent.Children = append(parent.Children, nodeMap[p.ID])
		}
	}

	w.Header().Set("Content-Type", "application/json")
	resultSlice := make([]WikiTreeNode, len(roots))
	for i, r := range roots {
		resultSlice[i] = *r
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"tree": resultSlice})
}

func (h *WikiHandler) GetWikiPageByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var title, slug string
	err = h.db.QueryRow("SELECT title, slug FROM wiki_pages WHERE id = ?", id).Scan(&title, &slug)
	if err != nil {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"title": title, "slug": slug})
}

func (h *WikiHandler) GetWikiPageBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.Error(w, "Slug required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	page, err := h.queries.GetWikiPageBySlug(ctx, slug)
	if err == sql.ErrNoRows {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to fetch page", http.StatusInternalServerError)
		return
	}

	// Fetch links and backlinks
	var linksJSON, backlinksJSON string
	h.db.QueryRow("SELECT COALESCE(links, '[]'), COALESCE(backlinks, '[]') FROM wiki_pages WHERE id = ?", page.ID).Scan(&linksJSON, &backlinksJSON)
	var linkIDs, blIDs []int64
	json.Unmarshal([]byte(linksJSON), &linkIDs)
	json.Unmarshal([]byte(backlinksJSON), &blIDs)

	resp := WikiPageResponse{
		ID:            page.ID,
		Title:         page.Title,
		Slug:          page.Slug,
		PageType:      page.PageType,
		Content:       page.Content,
		Tags:          page.Tags.String,
		ParentID:      nullInt64ToPtr(page.ParentID),
		ContentStatus: page.ContentStatus,
		SortOrder:     page.SortOrder,
		Links:         linkIDs,
		Backlinks:     blIDs,
		CreatedAt:     page.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     page.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type CreateWikiPageRequest struct {
	Title         string `json:"title"`
	Slug          string `json:"slug"`
	PageType      string `json:"page_type"`
	Content       string `json:"content"`
	Tags          string `json:"tags"`
	ParentID      *int64 `json:"parent_id"`
	ContentStatus string `json:"content_status"`
	SortOrder     int64  `json:"sort_order"`
}

func (h *WikiHandler) CreateWikiPage(w http.ResponseWriter, r *http.Request) {
	var req CreateWikiPageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var parentID sql.NullInt64
	if req.ParentID != nil {
		parentID = sql.NullInt64{Int64: *req.ParentID, Valid: true}
	}

	if req.PageType == "" {
		req.PageType = "entity"
	}
	if req.ContentStatus == "" {
		if req.Content != "" {
			req.ContentStatus = "published"
		} else {
			req.ContentStatus = "empty"
		}
	}

	result, err := h.queries.CreateWikiPage(ctx, model.CreateWikiPageParams{
		Title:         req.Title,
		Slug:          req.Slug,
		PageType:      req.PageType,
		Content:       req.Content,
		Tags:          sql.NullString{String: req.Tags, Valid: req.Tags != ""},
		ParentID:      parentID,
		ContentStatus: req.ContentStatus,
		SortOrder:     req.SortOrder,
	})
	if err != nil {
		http.Error(w, "Failed to create page", http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()
	var pagePath string
	if id > 0 {
		if req.ParentID != nil && *req.ParentID > 0 {
			parentPath, err := h.queries.GetWikiPagePathByID(ctx, *req.ParentID)
			if err != nil {
				log.Printf("Failed to fetch parent path: %v", err)
			} else {
				pagePath = parentPath + fmt.Sprintf("%d/", id)
			}
		} else {
			pagePath = fmt.Sprintf("%d/", id)
		}
		if pagePath != "" {
			h.queries.UpdateWikiPagePath(ctx, model.UpdateWikiPagePathParams{
				Path: pagePath,
				ID:   id,
			})
		}
		// Update links/backlinks based on content
		h.updatePageLinks(id, req.Content)
	}

	// Log the create action with source="manual".
	_ = h.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
		Action:    "create",
		PageID:    sql.NullInt64{Int64: id, Valid: id > 0},
		PageTitle: req.Title,
		PagePath:  sql.NullString{String: pagePath, Valid: pagePath != ""},
		Source:    "manual",
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "slug": req.Slug})
}

type UpdateWikiPageRequest struct {
	Title         string `json:"title"`
	Slug          string `json:"slug"`
	PageType      string `json:"page_type"`
	Content       string `json:"content"`
	Tags          string `json:"tags"`
	ParentID      *int64 `json:"parent_id"`
	ContentStatus string `json:"content_status"`
	SortOrder     int64  `json:"sort_order"`
}

func (h *WikiHandler) UpdateWikiPage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid page ID", http.StatusBadRequest)
		return
	}

	var req UpdateWikiPageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var parentID sql.NullInt64
	if req.ParentID != nil {
		parentID = sql.NullInt64{Int64: *req.ParentID, Valid: true}
	}

	err = h.queries.UpdateWikiPage(ctx, model.UpdateWikiPageParams{
		Title:         req.Title,
		Slug:          req.Slug,
		PageType:      req.PageType,
		Content:       req.Content,
		Tags:          sql.NullString{String: req.Tags, Valid: req.Tags != ""},
		ParentID:      parentID,
		ContentStatus: req.ContentStatus,
		SortOrder:     req.SortOrder,
		ID:            id,
	})
	if err != nil {
		http.Error(w, "Failed to update page", http.StatusInternalServerError)
		return
	}

	// Update links/backlinks based on content
	h.updatePageLinks(id, req.Content)

	// Log the update action with source="manual".
	_ = h.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
		Action:    "update",
		PageID:    sql.NullInt64{Int64: id, Valid: true},
		PageTitle: req.Title,
		Source:    "manual",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *WikiHandler) DeleteWikiPage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid page ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Fetch page to get its path for children reparenting
	page, err := h.queries.GetWikiPageByID(ctx, id)
	if err == sql.ErrNoRows {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to fetch page", http.StatusInternalServerError)
		return
	}

	// Cleanup links/backlinks before deleting
	h.cleanupLinksForPage(id)

	// Reparent children to the deleted page's parent (only update parent_id, preserve content)
	var deletedParentID sql.NullInt64
	h.db.QueryRow("SELECT parent_id FROM wiki_pages WHERE id = ?", id).Scan(&deletedParentID)
	if _, err := h.db.Exec("UPDATE wiki_pages SET parent_id = ? WHERE parent_id = ?", deletedParentID, id); err != nil {
		log.Printf("WARN: failed to reparent children of page %d: %v", id, err)
	}

	// Delete the page
	err = h.queries.DeleteWikiPage(ctx, id)
	if err != nil {
		http.Error(w, "Failed to delete page", http.StatusInternalServerError)
		return
	}

	// Reparent children: remove the deleted node's path segment from descendants
	if page.Path != "" {
		h.queries.BatchUpdateWikiPagePath(ctx, model.BatchUpdateWikiPagePathParams{
			OldPrefix:  page.Path,
			NewPrefix:  "",
			LikePrefix: sql.NullString{String: page.Path, Valid: true},
		})
	}

	// Log the delete action with source="manual".
	// page_id is NULL (the row is gone) but page_title is preserved.
	_ = h.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
		Action:    "delete",
		PageID:    sql.NullInt64{}, // explicitly invalid
		PageTitle: page.Title,
		PagePath:  sql.NullString{String: page.Path, Valid: page.Path != ""},
		Source:    "manual",
	})

	w.WriteHeader(http.StatusNoContent)
}

func (h *WikiHandler) GetOverviewPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, err := h.queries.GetOverviewPage(ctx)
	if err == sql.ErrNoRows {
		http.Error(w, "Overview page not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to fetch overview", http.StatusInternalServerError)
		return
	}

	// Fetch links and backlinks
	var linksJSON, backlinksJSON string
	h.db.QueryRow("SELECT COALESCE(links, '[]'), COALESCE(backlinks, '[]') FROM wiki_pages WHERE id = ?", page.ID).Scan(&linksJSON, &backlinksJSON)
	var linkIDs, blIDs []int64
	json.Unmarshal([]byte(linksJSON), &linkIDs)
	json.Unmarshal([]byte(backlinksJSON), &blIDs)

	resp := WikiPageResponse{
		ID:            page.ID,
		Title:         page.Title,
		Slug:          page.Slug,
		PageType:      page.PageType,
		Content:       page.Content,
		Tags:          page.Tags.String,
		ParentID:      nullInt64ToPtr(page.ParentID),
		ContentStatus: page.ContentStatus,
		SortOrder:     page.SortOrder,
		Links:         linkIDs,
		Backlinks:     blIDs,
		CreatedAt:     page.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     page.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Structure operations (no confirmation needed) ---

func (h *WikiHandler) RenameWikiPage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid page ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		http.Error(w, "Title required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	page, err := h.queries.GetWikiPageByID(ctx, id)
	if err == sql.ErrNoRows {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to fetch page", http.StatusInternalServerError)
		return
	}

	// Save old title before rename
	oldTitle := page.Title

	// Generate new slug from title
	newSlug := slugify(req.Title)
	if newSlug == "" {
		newSlug = page.Slug
	}

	err = h.queries.UpdateWikiPage(ctx, model.UpdateWikiPageParams{
		Title:         req.Title,
		Slug:          newSlug,
		PageType:      page.PageType,
		Content:       page.Content,
		Tags:          page.Tags,
		ParentID:      page.ParentID,
		ContentStatus: page.ContentStatus,
		SortOrder:     page.SortOrder,
		ID:            id,
	})
	if err != nil {
		http.Error(w, "Failed to rename page", http.StatusInternalServerError)
		return
	}

	// Update all [[oldTitle]] references to [[newTitle]] in other pages' content
	if oldTitle != req.Title {
		oldLink := fmt.Sprintf("[[%s]]", oldTitle)
		newLink := fmt.Sprintf("[[%s]]", req.Title)
		h.db.Exec("UPDATE wiki_pages SET content = REPLACE(content, ?, ?) WHERE content LIKE ?",
			oldLink, newLink, "%"+oldLink+"%")

		// Re-parse links for all affected pages
		rows, err := h.db.Query("SELECT id, content FROM wiki_pages WHERE content LIKE ?", "%"+newLink+"%")
		if err == nil {
			for rows.Next() {
				var affectedID int64
				var affectedContent string
				if rows.Scan(&affectedID, &affectedContent) == nil {
					h.updatePageLinks(affectedID, affectedContent)
				}
			}
			rows.Close()
		}
	}

	// Log the rename action with source="manual".
	_ = h.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
		Action:    "rename",
		PageID:    sql.NullInt64{Int64: id, Valid: true},
		PageTitle: req.Title,
		Source:    "manual",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "title": req.Title, "slug": newSlug})
}

func (h *WikiHandler) MoveWikiPage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid page ID", http.StatusBadRequest)
		return
	}

	var req struct {
		ParentID  *int64 `json:"parent_id"`
		SortOrder *int64 `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	page, err := h.queries.GetWikiPageByID(ctx, id)
	if err == sql.ErrNoRows {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to fetch page", http.StatusInternalServerError)
		return
	}

	parentID := page.ParentID
	if req.ParentID != nil {
		parentID = sql.NullInt64{Int64: *req.ParentID, Valid: *req.ParentID > 0}
		// Prevent moving page under itself
		if parentID.Valid && parentID.Int64 == id {
			http.Error(w, "Cannot move page under itself", http.StatusBadRequest)
			return
		}
	}

	sortOrder := page.SortOrder
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	err = h.queries.UpdateWikiPage(ctx, model.UpdateWikiPageParams{
		Title:         page.Title,
		Slug:          page.Slug,
		PageType:      page.PageType,
		Content:       page.Content,
		Tags:          page.Tags,
		ParentID:      parentID,
		ContentStatus: page.ContentStatus,
		SortOrder:     sortOrder,
		ID:            id,
	})
	if err != nil {
		http.Error(w, "Failed to move page", http.StatusInternalServerError)
		return
	}

	// Migrate subtree paths
	if page.Path != "" {
		var newPath string
		if parentID.Valid && parentID.Int64 > 0 {
			parentPath, err := h.queries.GetWikiPagePathByID(ctx, parentID.Int64)
			if err != nil {
				log.Printf("Failed to fetch parent path: %v", err)
				http.Error(w, "Failed to get parent page path", http.StatusInternalServerError)
				return
			}
			newPath = parentPath + fmt.Sprintf("%d/", id)
		} else {
			newPath = fmt.Sprintf("%d/", id)
		}

		// Prevent moving page under its own descendant (cycle detection)
		if strings.HasPrefix(newPath, page.Path) {
			http.Error(w, "Cannot move page under its own descendant", http.StatusBadRequest)
			return
		}

		// Update the moved page's own path
		h.queries.UpdateWikiPagePath(ctx, model.UpdateWikiPagePathParams{
			Path: newPath,
			ID:   id,
		})

		// Batch update descendants
		h.queries.BatchUpdateWikiPagePath(ctx, model.BatchUpdateWikiPagePathParams{
			OldPrefix:  page.Path,
			NewPrefix:  newPath,
			LikePrefix: sql.NullString{String: page.Path, Valid: true},
		})
	}

	// Log the move action with source="manual".
	_ = h.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
		Action:    "move",
		PageID:    sql.NullInt64{Int64: id, Valid: true},
		PageTitle: page.Title,
		Source:    "manual",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *WikiHandler) CreateEmptyWikiPage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string `json:"title"`
		ParentID *int64 `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		http.Error(w, "Title required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	slug := slugify(req.Title)
	if slug == "" {
		slug = fmt.Sprintf("page-%d", time.Now().Unix())
	}

	var parentID sql.NullInt64
	if req.ParentID != nil && *req.ParentID > 0 {
		parentID = sql.NullInt64{Int64: *req.ParentID, Valid: true}
	}

	result, err := h.queries.CreateWikiPage(ctx, model.CreateWikiPageParams{
		Title:         req.Title,
		Slug:          slug,
		PageType:      "entity",
		Content:       "",
		ParentID:      parentID,
		ContentStatus: "empty",
		SortOrder:     0,
	})
	if err != nil {
		http.Error(w, "Failed to create page", http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()
	if id > 0 {
		var path string
		if req.ParentID != nil && *req.ParentID > 0 {
			parentPath, err := h.queries.GetWikiPagePathByID(ctx, *req.ParentID)
			if err != nil {
				log.Printf("Failed to fetch parent path: %v", err)
			} else {
				path = parentPath + fmt.Sprintf("%d/", id)
			}
		} else {
			path = fmt.Sprintf("%d/", id)
		}
		if path != "" {
			h.queries.UpdateWikiPagePath(ctx, model.UpdateWikiPagePathParams{
				Path: path,
				ID:   id,
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": id, "slug": slug, "title": req.Title})
}

// slugify converts a title to a URL-friendly slug.
func slugify(title string) string {
	s := strings.ToLower(title)
	// Replace spaces and common separators with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, "/", "-")
	// Remove non-alphanumeric characters (keep hyphens and CJK)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r >= 0x4e00 {
			b.WriteRune(r)
		}
	}
	result := b.String()
	// Collapse multiple hyphens
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}

// ConfirmPageContent marks a page's content as confirmed (published).
// PUT /api/wiki/{id}/confirm
func (h *WikiHandler) ConfirmPageContent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid page ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	page, err := h.queries.GetWikiPageByID(ctx, id)
	if err == sql.ErrNoRows {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to fetch page", http.StatusInternalServerError)
		return
	}

	// Update content_status to "published" while keeping everything else the same
	err = h.queries.UpdateWikiPage(ctx, model.UpdateWikiPageParams{
		Title:         page.Title,
		Slug:          page.Slug,
		PageType:      page.PageType,
		Content:       page.Content,
		Tags:          page.Tags,
		ParentID:      page.ParentID,
		ContentStatus: "published",
		SortOrder:     page.SortOrder,
		ID:            id,
	})
	if err != nil {
		http.Error(w, "Failed to confirm page content", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": id, "content_status": "published"})
}
