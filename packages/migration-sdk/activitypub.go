// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ActivityPub represents an ActivityPub object
type ActivityPub map[string]interface{}

// GetPerson returns the Person actor for a user.
func (c *ActivityPubService) GetPerson(ctx context.Context, userID int64) (ActivityPub, *Response, error) {
	result := make(ActivityPub)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/activitypub/user-id/%d", userID),
		jsonHeader, nil, &result)
	return result, resp, err
}

// SendInbox sends an ActivityPub message to a user's inbox.
func (c *ActivityPubService) SendInbox(ctx context.Context, userID int64, activity ActivityPub) (*Response, error) {
	body, err := json.Marshal(activity)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "POST",
		fmt.Sprintf("/activitypub/user-id/%d/inbox", userID),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// GetPersonResponse returns the raw ActivityPub Person response.
func (c *ActivityPubService) GetPersonResponse(ctx context.Context, userID int64) ([]byte, *Response, error) {
	return c.getResponse(ctx, "GET",
		fmt.Sprintf("/activitypub/user-id/%d", userID),
		jsonHeader, nil)
}
