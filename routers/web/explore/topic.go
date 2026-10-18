// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"net/http"

	"github.com/neko233-com/gitsss/models/db"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	api "github.com/neko233-com/gitsss/modules/structs"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/convert"
)

// TopicSearch search for creating topic
func TopicSearch(ctx *context.Context) {
	opts := &repo_model.FindTopicOptions{
		Keyword:  ctx.FormString("q"),
		Page:     ctx.FormInt("page"),
		PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
	}
	opts.VisibleRepoCond = repo_model.SearchRepositoryCondition(repo_model.SearchRepoOptions{Actor: ctx.Doer, Private: ctx.Doer != nil})

	topics, total, err := db.FindAndCount[repo_model.Topic](ctx, opts)
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}
	if err := repo_model.LoadVisibleTopicCounts(ctx, topics, opts.VisibleRepoCond); err != nil {
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}

	topicResponses := make([]*api.TopicResponse, len(topics))
	for i, topic := range topics {
		topicResponses[i] = convert.ToTopicResponse(topic)
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, map[string]any{
		"topics": topicResponses,
	})
}
