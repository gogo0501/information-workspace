// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api // import "miniflux.app/v2/internal/api"

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
	"miniflux.app/v2/internal/workspace"
)

type workspaceAgentRequest struct {
	Question string `json:"question"`
}

func (h *handler) workspaceService() *workspace.Service {
	return workspace.NewService(h.store)
}

func (h *handler) workspaceEntriesHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := h.workspaceService().Search(request.UserID(r), query, limit)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	response.JSON(w, r, entries)
}

func (h *handler) workspaceAgentHandler(w http.ResponseWriter, r *http.Request) {
	var input workspaceAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}
	answer, err := workspace.NewRouterChain(h.store).Run(r.Context(), request.UserID(r), input.Question)
	if err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}
	response.JSON(w, r, answer)
}

func (h *handler) workspaceEventsHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		response.JSONServerError(w, r, http.ErrNotSupported)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			entries, err := h.workspaceService().Search(request.UserID(r), "", 1)
			if err != nil {
				return
			}
			payload, _ := json.Marshal(entries)
			w.Write([]byte("event: workspace\ndata: "))
			w.Write(payload)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}
