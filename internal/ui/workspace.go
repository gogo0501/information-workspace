// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"net/http"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
	"miniflux.app/v2/internal/ui/view"
	"miniflux.app/v2/internal/workspace"
)

func (h *handler) showWorkspacePage(w http.ResponseWriter, r *http.Request) {
	user, err := h.store.UserByID(request.UserID(r))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	query := request.QueryStringParam(r, "q", "")
	documents, err := workspace.NewService(h.store).Search(user.ID, query, user.EntriesPerPage)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	answer := ""
	if r.URL.Query().Get("ask") != "" {
		result, askErr := workspace.NewService(h.store).Ask(user.ID, r.URL.Query().Get("ask"))
		if askErr != nil {
			response.HTMLServerError(w, r, askErr)
			return
		}
		answer = result.Answer
	}

	v := view.New(h.tpl, r)
	v.Set("menu", "workspace")
	v.Set("user", user)
	v.Set("workspaceQuery", query)
	v.Set("workspaceAsk", r.URL.Query().Get("ask"))
	v.Set("workspaceAnswer", answer)
	v.Set("workspaceDocuments", documents)
	navMetadata, _ := h.store.GetNavMetadata(user.ID)
	v.Set("countUnread", navMetadata.CountUnread)
	v.Set("countErrorFeeds", navMetadata.CountErrorFeeds)
	v.Set("hasSaveEntry", navMetadata.HasSaveEntry)
	response.HTML(w, r, v.Render("workspace"))
}
