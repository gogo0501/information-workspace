// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
	"miniflux.app/v2/internal/workspace"
)

var errInvalidWorkspaceSourceID = errors.New("invalid workspace source ID")

func (h *handler) createWorkspaceSourceHandler(w http.ResponseWriter, r *http.Request) {
	var input workspace.SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}
	source, err := workspace.NewSourceService(h.store).Subscribe(r.Context(), request.UserID(r), input)
	if err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}
	response.JSONCreated(w, r, source)
}

func (h *handler) deleteWorkspaceSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	sourceID := request.RouteInt64Param(r, "sourceID")
	if sourceID == 0 {
		response.JSONBadRequest(w, r, errInvalidWorkspaceSourceID)
		return
	}
	if err := workspace.NewSourceService(h.store).Unsubscribe(request.UserID(r), sourceID); err != nil {
		response.JSONServerError(w, r, err)
		return
	}
	response.NoContent(w, r)
}
