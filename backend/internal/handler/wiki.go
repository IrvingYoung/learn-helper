package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

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
	ID            int64                    `json:"id"`
	Title         string                   `json:"title"`
	Slug          string                   `json:"slug"`
	PageType      string                   `json:"page_type"`
	ContentStatus string                   `json:"content_status"`
	ParentID      sql.NullInt64            `json:"parent_id"`
	SortOrder     int64                    `json:"sort_order"`
	Children      []WikiTreeNode           `json:"children,omitempty"`
}

type WikiPageResponse struct {
	ID            int64         `json:"id"`
	Title         string        `json:"title"`
	Slug          string        `json:"slug"`
	PageType      string        `json:"page_type"`
	Content       string        `json:"content"`
	Tags          string        `json:"tags"`
	ParentID      sql.NullInt64 `json:"parent_id"`
	ContentStatus string        `json:"content_status"`
	SortOrder     int64         `json:"sort_order"`
	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at"`
}

func (h *WikiHandler) GetWikiTree(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pages, err := h.queries.GetWikiPageTree(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch wiki tree", http.StatusInternalServerError)
		return
	}

	nodeMap := make(map[int64]*WikiTreeNode)
	var roots []WikiTreeNode

	for i := range pages {
		p := pages[i]
		node := &WikiTreeNode{
			ID:            p.ID,
			Title:         p.Title,
			Slug:          p.Slug,
			PageType:      p.PageType,
			ContentStatus: p.ContentStatus,
			ParentID:      p.ParentID,
			SortOrder:     p.SortOrder,
			Children:      []WikiTreeNode{},
		}
		nodeMap[p.ID] = node
	}

	for i := range pages {
		p := pages[i]
		node := nodeMap[p.ID]
		if !p.ParentID.Valid || p.ParentID.Int64 == 0 {
			roots = append(roots, *node)
		} else if parent, ok := nodeMap[p.ParentID.Int64]; ok {
			parent.Children = append(parent.Children, *node)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tree": roots})
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

	resp := WikiPageResponse{
		ID:            page.ID,
		Title:         page.Title,
		Slug:          page.Slug,
		PageType:      page.PageType,
		Content:       page.Content,
		Tags:          page.Tags.String,
		ParentID:      page.ParentID,
		ContentStatus: page.ContentStatus,
		SortOrder:     page.SortOrder,
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
	err = h.queries.DeleteWikiPage(ctx, id)
	if err != nil {
		http.Error(w, "Failed to delete page", http.StatusInternalServerError)
		return
	}

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

	resp := WikiPageResponse{
		ID:            page.ID,
		Title:         page.Title,
		Slug:          page.Slug,
		PageType:      page.PageType,
		Content:       page.Content,
		Tags:          page.Tags.String,
		ParentID:      page.ParentID,
		ContentStatus: page.ContentStatus,
		SortOrder:     page.SortOrder,
		CreatedAt:     page.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     page.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
