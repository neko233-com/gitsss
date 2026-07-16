// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "github.com/neko233-com/gitsss/models/actions"
	"github.com/neko233-com/gitsss/modules/timeutil"
	"github.com/neko233-com/gitsss/modules/translation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToViewModel(t *testing.T) {
	task := &actions_model.ActionTask{
		Status: actions_model.StatusSuccess,
		Steps: []*actions_model.ActionTaskStep{
			{Name: "Run step-name", Index: 0, Status: actions_model.StatusSuccess, LogLength: 1, Started: timeutil.TimeStamp(1), Stopped: timeutil.TimeStamp(5)},
		},
		Stopped: timeutil.TimeStamp(20),
	}

	viewJobSteps, _, err := convertToViewModel(t.Context(), translation.MockLocale{}, nil, task)
	require.NoError(t, err)

	expectedViewJobs := []*ViewJobStep{
		{
			Summary:  "Set up job",
			Duration: "0s",
			Status:   "success",
		},
		{
			Summary:  "Run step-name",
			Duration: "4s",
			Status:   "success",
		},
		{
			Summary:  "Complete job",
			Duration: "15s",
			Status:   "success",
		},
	}
	assert.Equal(t, expectedViewJobs, viewJobSteps)
}
