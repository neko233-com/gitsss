// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestNotifications(t *testing.T) {
	log.Println("== TestNotifications ==")

	// init user2
	c := newTestClient()

	user1, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)
	user2 := createTestUser(t, "notify2", c)

	// create 2 repos
	repoA, err := createTestRepo(t, "TestNotifications_A", c)
	assert.NoError(t, err)

	c.SetSudo(user2.UserName)
	repoB, err := createTestRepo(t, "TestNotifications_B", c)
	assert.NoError(t, err)
	_, err = c.Repositories.WatchRepo(t.Context(), user1.UserName, repoA.Name)
	c.SetSudo("")
	assert.NoError(t, err)

	c.SetSudo(user2.UserName)
	notifications, _, err := c.Notifications.MarkRead(t.Context(), gitea.MarkNotificationOptions{})
	assert.NoError(t, err)
	assert.Len(t, notifications, 0)
	count, _, err := c.Notifications.Check(t.Context())
	assert.EqualValues(t, 0, count)
	assert.NoError(t, err)
	c.SetSudo("")
	_, _, err = c.Issues.CreateIssue(t.Context(), repoA.Owner.UserName, repoA.Name, gitea.CreateIssueOption{Title: "A Issue", Closed: false})
	assert.NoError(t, err)
	issue, _, err := c.Issues.CreateIssue(t.Context(), repoB.Owner.UserName, repoB.Name, gitea.CreateIssueOption{Title: "B Issue", Closed: false})
	assert.NoError(t, err)
	time.Sleep(time.Second * 5)

	// Check of user2
	c.SetSudo(user2.UserName)
	count, _, err = c.Notifications.Check(t.Context())
	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)

	// List
	nList, _, err := c.Notifications.List(t.Context(), gitea.ListNotificationOptions{})
	assert.NoError(t, err)
	assert.Len(t, nList, 2)
	for _, n := range nList {
		assert.EqualValues(t, true, n.Unread)
		assert.EqualValues(t, "Issue", n.Subject.Type)
		assert.EqualValues(t, gitea.NotificationSubjectOpen, nList[0].Subject.State)
		assert.EqualValues(t, gitea.NotificationSubjectOpen, nList[1].Subject.State)
		switch n.Subject.Title {
		case "A Issue":
			assert.EqualValues(t, repoA.Name, n.Repository.Name)
		case "B Issue":
			assert.EqualValues(t, repoB.Name, n.Repository.Name)
		default:
			assert.Error(t, fmt.Errorf("List returned a Issue witch should not"))
		}
	}

	// ListByRepo
	nList, _, err = c.Notifications.ListByRepo(t.Context(), repoA.Owner.UserName, repoA.Name, gitea.ListNotificationOptions{})
	assert.NoError(t, err)
	assert.Len(t, nList, 1)
	assert.EqualValues(t, "A Issue", nList[0].Subject.Title)
	// MarkReadByRepo
	notifications, _, err = c.Notifications.MarkReadByRepo(t.Context(), repoA.Owner.UserName, repoA.Name, gitea.MarkNotificationOptions{})
	assert.NoError(t, err)
	assert.Len(t, notifications, 1)

	// GetByID
	n, _, err := c.Notifications.GetByID(t.Context(), nList[0].ID)
	assert.NoError(t, err)
	assert.EqualValues(t, false, n.Unread)
	assert.EqualValues(t, "A Issue", n.Subject.Title)

	// MarkRead
	notifications, _, err = c.Notifications.MarkRead(t.Context(), gitea.MarkNotificationOptions{})
	assert.NoError(t, err)
	assert.Len(t, notifications, 1)
	nList, _, err = c.Notifications.List(t.Context(), gitea.ListNotificationOptions{})
	assert.NoError(t, err)
	assert.Len(t, nList, 0)

	// MarkReadByID
	iState := gitea.StateClosed
	c.SetSudo("")
	_, _, err = c.Issues.EditIssue(t.Context(), repoB.Owner.UserName, repoB.Name, issue.Index, gitea.EditIssueOption{State: &iState})
	assert.NoError(t, err)
	time.Sleep(time.Second * 5)

	c.SetSudo(user2.UserName)
	nList, _, err = c.Notifications.List(t.Context(), gitea.ListNotificationOptions{})
	assert.NoError(t, err)
	count, _, err = c.Notifications.Check(t.Context())
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)
	if assert.Len(t, nList, 1) {
		assert.EqualValues(t, gitea.NotificationSubjectClosed, nList[0].Subject.State)
		notification, _, err := c.Notifications.MarkReadByID(t.Context(), nList[0].ID)
		assert.NoError(t, err)
		assert.EqualValues(t, notification.ID, nList[0].ID)
	}

	c.SetSudo("")
	notifications, _, err = c.Notifications.MarkRead(t.Context(), gitea.MarkNotificationOptions{})
	assert.NoError(t, err)
	assert.Len(t, notifications, 2)
	nList, _, err = c.Notifications.List(t.Context(), gitea.ListNotificationOptions{Status: []gitea.NotificationStatus{gitea.NotificationStatusRead}})
	assert.NoError(t, err)
	if assert.Len(t, nList, 2) {
		notification, _, err := c.Notifications.MarkReadByID(t.Context(), nList[0].ID, gitea.NotificationStatusPinned)
		assert.EqualValues(t, notification.ID, nList[0].ID)
		assert.NoError(t, err)

		notification, _, err = c.Notifications.MarkReadByID(t.Context(), nList[1].ID, gitea.NotificationStatusUnread)
		assert.EqualValues(t, notification.ID, nList[1].ID)
		assert.NoError(t, err)
	}
	nList, _, err = c.Notifications.List(t.Context(), gitea.ListNotificationOptions{Status: []gitea.NotificationStatus{gitea.NotificationStatusPinned, gitea.NotificationStatusUnread}})
	assert.NoError(t, err)
	if assert.Len(t, nList, 2) {
		assert.EqualValues(t, gitea.NotificationSubjectOpen, nList[0].Subject.State)
		assert.EqualValues(t, gitea.NotificationSubjectOpen, nList[1].Subject.State)
	}
}
