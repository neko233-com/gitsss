// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"context"
	"log"
	"net/http"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestPullReview(t *testing.T) {
	log.Println("== TestPullReview ==")
	c := newTestClient()

	repoName := "Reviews"
	repo, pull, submitter, reviewer, success := preparePullReviewTest(t, c, repoName)
	if !success {
		return
	}

	// CreatePullReview
	r1, _, err := c.PullRequests.CreatePullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.CreatePullReviewOptions{
		State: gitea.ReviewStateComment,
		Body:  "I'll have a look at it later",
	})
	assert.NoError(t, err)
	if assert.NotNil(t, r1) {
		assert.EqualValues(t, gitea.ReviewStateComment, r1.State)
		assert.EqualValues(t, 1, r1.Reviewer.ID)
	}

	c.SetSudo(submitter.UserName)
	_, _, err = c.PullRequests.CreatePullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.CreatePullReviewOptions{
		State: gitea.ReviewStateApproved,
		Body:  "lgtm it myself",
	})
	assert.Error(t, err)
	r2, _, err := c.PullRequests.CreatePullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.CreatePullReviewOptions{
		State: gitea.ReviewStateComment,
		Body:  "no seriously please have a look at it",
	})
	assert.NoError(t, err)
	assert.NotNil(t, r2)

	c.SetSudo(reviewer.UserName)
	r3, _, err := c.PullRequests.CreatePullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.CreatePullReviewOptions{
		State: gitea.ReviewStateApproved,
		Body:  "lgtm",
		Comments: []gitea.CreatePullReviewComment{
			{
				Path:       "WOW-file",
				Body:       "no better name - really?",
				NewLineNum: 1,
			},
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, r3)
	r3Comments, _, err := c.PullRequests.ListPullReviewComments(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r3.ID)
	assert.NoError(t, err)
	if assert.Len(t, r3Comments, 1) {
		assert.EqualValues(t, r3.ID, r3Comments[0].ReviewID)
		assert.EqualValues(t, "no better name - really?", r3Comments[0].Body)
	}

	// ListPullReviews
	c.SetSudo("")
	rl, _, err := c.PullRequests.ListPullReviews(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.ListPullReviewsOptions{})
	assert.NoError(t, err)
	assert.Len(t, rl, 3)
	for i := range rl {
		assert.EqualValues(t, pull.HTMLURL, rl[i].HTMLPullURL)
		if rl[i].CodeCommentsCount > 0 {
			assert.EqualValues(t, reviewer.ID, rl[i].Reviewer.ID)
		}
	}

	// GetPullReview
	rNew, _, err := c.PullRequests.GetPullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r3.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, r3, rNew)

	// DeletePullReview
	c.SetSudo(submitter.UserName)
	_, err = c.PullRequests.DeletePullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r2.ID)
	assert.NoError(t, err)
	_, err = c.PullRequests.DeletePullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r3.ID)
	assert.Error(t, err)

	// SubmitPullReview
	c.SetSudo("")
	r4, resp, err := c.PullRequests.CreatePullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.CreatePullReviewOptions{
		Body: "...",
		Comments: []gitea.CreatePullReviewComment{
			{
				Path:       "WOW-file",
				Body:       "its ok",
				NewLineNum: 1,
			},
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	r5, _, err := c.PullRequests.CreatePullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.CreatePullReviewOptions{
		Body: "...",
		Comments: []gitea.CreatePullReviewComment{
			{
				Path:       "WOW-file",
				Body:       "hehe and here it is",
				NewLineNum: 3,
			},
		},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, r4.ID, r5.ID)

	r, _, err := c.PullRequests.SubmitPullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r4.ID, gitea.SubmitPullReviewOptions{
		State: gitea.ReviewStateRequestChanges,
		Body:  "one nit",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, r4.ID, r.ID)
	assert.EqualValues(t, gitea.ReviewStateRequestChanges, r.State)

	// ListPullReviewComments
	rcl, _, err := c.PullRequests.ListPullReviewComments(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, r.CodeCommentsCount, len(rcl))
	for _, rc := range rcl {
		assert.EqualValues(t, pull.HTMLURL, rc.HTMLPullURL)
		if rc.LineNum == 3 {
			assert.EqualValues(t, "hehe and here it is", rc.Body)
		} else {
			assert.EqualValues(t, 1, rc.LineNum)
			assert.EqualValues(t, "its ok", rc.Body)
		}
	}

	r, _, err = c.PullRequests.GetPullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r.ID)
	assert.NoError(t, err)
	assert.False(t, r.Dismissed)

	// DismissPullReview
	resp, err = c.PullRequests.DismissPullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r.ID, gitea.DismissPullReviewOptions{Message: "stale"})
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.EqualValues(t, 200, resp.StatusCode)
	}
	r, _, _ = c.PullRequests.GetPullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r.ID)
	assert.True(t, r.Dismissed)

	// UnDismissPullReview
	resp, err = c.PullRequests.UnDismissPullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r.ID)
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.EqualValues(t, 200, resp.StatusCode)
	}
	r, _, _ = c.PullRequests.GetPullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, r.ID)
	assert.False(t, r.Dismissed)

	rl, _, err = c.PullRequests.ListPullReviews(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.ListPullReviewsOptions{})
	assert.NoError(t, err)
	assert.Len(t, rl, 3)

	c.SetSudo(submitter.UserName)
	resp, err = c.PullRequests.CreateReviewRequests(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.PullReviewRequestOptions{Reviewers: []string{reviewer.UserName}})
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	rl, _, _ = c.PullRequests.ListPullReviews(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.ListPullReviewsOptions{})
	if assert.Len(t, rl, 4) {
		assert.EqualValues(t, gitea.ReviewStateRequestReview, rl[3].State)
	}

	c.SetSudo(reviewer.UserName)
	resp, err = c.PullRequests.DeleteReviewRequests(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.PullReviewRequestOptions{Reviewers: []string{reviewer.UserName}})
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	rl, _, _ = c.PullRequests.ListPullReviews(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.ListPullReviewsOptions{})
	assert.Len(t, rl, 3)

	c.SetSudo("")
	_, err = c.Admin.DeleteUser(t.Context(), reviewer.UserName)
	assert.NoError(t, err)
	_, err = c.Admin.DeleteUser(t.Context(), submitter.UserName)
	assert.NoError(t, err)
}

func TestMainNightlyPullReviewCommentReply(t *testing.T) {
	log.Println("== TestMainNightlyPullReviewCommentReply ==")
	c := newTestClient()

	repo, pull, submitter, reviewer, success := preparePullReviewTest(t, c, "ReviewsMainNightly")
	if !success {
		return
	}
	cleanupCtx := context.Background()

	t.Cleanup(func() {
		c.SetSudo("")
		_, err := c.Admin.DeleteUser(cleanupCtx, reviewer.UserName)
		assert.NoError(t, err)
		_, err = c.Admin.DeleteUser(cleanupCtx, submitter.UserName)
		assert.NoError(t, err)
	})

	c.SetSudo(reviewer.UserName)
	review, _, err := c.PullRequests.CreatePullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.CreatePullReviewOptions{
		State: gitea.ReviewStateApproved,
		Body:  "lgtm",
		Comments: []gitea.CreatePullReviewComment{{
			Path:       "WOW-file",
			Body:       "no better name - really?",
			NewLineNum: 1,
		}},
	})
	assert.NoError(t, err)
	if !assert.NotNil(t, review) {
		return
	}

	comments, _, err := c.PullRequests.ListPullReviewComments(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, review.ID)
	assert.NoError(t, err)
	if !assert.Len(t, comments, 1) {
		return
	}

	reply, resp, err := c.PullRequests.CreatePullReviewCommentReply(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, comments[0].ID, gitea.CreatePullReviewCommentReplyOptions{
		Body: "reply to the original review comment",
	})
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	}
	if assert.NotNil(t, reply) {
		assert.EqualValues(t, review.ID, reply.ReviewID)
		assert.EqualValues(t, "reply to the original review comment", reply.Body)
	}

	comments, _, err = c.PullRequests.ListPullReviewComments(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, review.ID)
	assert.NoError(t, err)
	if assert.Len(t, comments, 2) {
		assert.Condition(t, func() bool {
			for _, comment := range comments {
				if comment.Body == "reply to the original review comment" {
					return true
				}
			}
			return false
		})
	}
}

func TestMainNightlyPullReviewCommentResolve(t *testing.T) {
	log.Println("== TestMainNightlyPullReviewCommentResolve ==")
	c := newTestClient()

	repo, pull, submitter, reviewer, success := preparePullReviewTest(t, c, "ReviewsResolveMainNightly")
	if !success {
		return
	}
	cleanupCtx := context.Background()

	t.Cleanup(func() {
		c.SetSudo("")
		_, err := c.Admin.DeleteUser(cleanupCtx, reviewer.UserName)
		assert.NoError(t, err)
		_, err = c.Admin.DeleteUser(cleanupCtx, submitter.UserName)
		assert.NoError(t, err)
	})

	comment := createMainNightlyReviewComment(t, c, repo, pull, reviewer)
	if comment == nil {
		return
	}
	c.SetSudo("")
	owner := mustCurrentUsername(t, c)

	resp, err := c.PullRequests.ResolvePullReviewComment(t.Context(), repo.Owner.UserName, repo.Name, comment.ID)
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	}

	resolved := mustFindPullReviewComment(t, c, repo, pull, comment.ReviewID, comment.ID)
	if assert.NotNil(t, resolved.Resolver) {
		assert.EqualValues(t, owner, resolved.Resolver.UserName)
	}

	resp, err = c.PullRequests.UnresolvePullReviewComment(t.Context(), repo.Owner.UserName, repo.Name, comment.ID)
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	}

	unresolved := mustFindPullReviewComment(t, c, repo, pull, comment.ReviewID, comment.ID)
	assert.Nil(t, unresolved.Resolver)
}

func createMainNightlyReviewComment(t *testing.T, c *gitea.Client, repo *gitea.Repository, pull *gitea.PullRequest, reviewer *gitea.User) *gitea.PullReviewComment {
	t.Helper()

	c.SetSudo(reviewer.UserName)
	review, _, err := c.PullRequests.CreatePullReview(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, gitea.CreatePullReviewOptions{
		State: gitea.ReviewStateApproved,
		Body:  "lgtm",
		Comments: []gitea.CreatePullReviewComment{{
			Path:       "WOW-file",
			Body:       "no better name - really?",
			NewLineNum: 1,
		}},
	})
	assert.NoError(t, err)
	if !assert.NotNil(t, review) {
		return nil
	}

	comments, _, err := c.PullRequests.ListPullReviewComments(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, review.ID)
	assert.NoError(t, err)
	if !assert.Len(t, comments, 1) {
		return nil
	}

	return comments[0]
}

func mustFindPullReviewComment(t *testing.T, c *gitea.Client, repo *gitea.Repository, pull *gitea.PullRequest, reviewID, commentID int64) *gitea.PullReviewComment {
	t.Helper()

	comments, _, err := c.PullRequests.ListPullReviewComments(t.Context(), repo.Owner.UserName, repo.Name, pull.Index, reviewID)
	assert.NoError(t, err)
	for _, comment := range comments {
		if comment.ID == commentID {
			return comment
		}
	}
	t.Fatalf("comment %d not found in review %d", commentID, reviewID)
	return nil
}

func preparePullReviewTest(t *testing.T, c *gitea.Client, repoName string) (*gitea.Repository, *gitea.PullRequest, *gitea.User, *gitea.User, bool) {
	repo, err := createTestRepo(t, repoName, c)
	if !assert.NoError(t, err) {
		return nil, nil, nil, nil, false
	}

	pullSubmitter := createTestUser(t, "pull_submitter", c)
	write := gitea.AccessModeWrite
	_, err = c.Repositories.AddCollaborator(t.Context(), repo.Owner.UserName, repo.Name, pullSubmitter.UserName, gitea.AddCollaboratorOption{
		Permission: &write,
	})
	assert.NoError(t, err)

	c.SetSudo("pull_submitter")

	newFile, _, err := c.Repositories.CreateFile(t.Context(), repo.Owner.UserName, repo.Name, "WOW-file", gitea.CreateFileOptions{
		Content: "QSBuZXcgRmlsZQoKYW5kIHNvbWUgbGluZXMK",
		FileOptions: gitea.FileOptions{
			Message:       "creat a new file",
			BranchName:    "main",
			NewBranchName: "new_file",
		},
	})

	if !assert.NoError(t, err) || !assert.NotNil(t, newFile) {
		return nil, nil, nil, nil, false
	}

	pull, _, err := c.PullRequests.CreatePullRequest(t.Context(), repo.Owner.UserName, repoName, gitea.CreatePullRequestOption{
		Base:  "main",
		Head:  "new_file",
		Title: "Creat a NewFile",
	})
	assert.NoError(t, err)
	assert.NotNil(t, pull)

	c.SetSudo("")

	reviewer := createTestUser(t, "pull_reviewer", c)
	admin := gitea.AccessModeAdmin
	_, err = c.Repositories.AddCollaborator(t.Context(), repo.Owner.UserName, repo.Name, pullSubmitter.UserName, gitea.AddCollaboratorOption{
		Permission: &admin,
	})
	assert.NoError(t, err)

	return repo, pull, pullSubmitter, reviewer, pull.Poster.ID == pullSubmitter.ID
}
