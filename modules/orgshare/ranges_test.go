// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddSpanMergesAndRejectsOverlap(t *testing.T) {
	spans, err := AddSpan(nil, 0, 4)
	require.NoError(t, err)
	spans, err = AddSpan(spans, 8, 10)
	require.NoError(t, err)
	spans, err = AddSpan(spans, 4, 8)
	require.NoError(t, err)
	assert.Equal(t, []Span{{Start: 0, End: 10}}, spans)
	assert.Equal(t, int64(10), CoveredBytes(spans))
	assert.Empty(t, Missing(spans, 10))

	again, err := AddSpan(spans, 2, 4)
	require.NoError(t, err)
	assert.Equal(t, spans, again)

	_, err = AddSpan(spans, 9, 12)
	require.Error(t, err)
}

func TestPlanChunksAndMissing(t *testing.T) {
	assert.Equal(t, []Span{{Start: 0, End: 5}, {Start: 5, End: 10}, {Start: 10, End: 12}}, PlanChunks(12, 5))
	assert.Equal(t, []Span{{Start: 4, End: 8}}, Missing([]Span{{Start: 0, End: 4}}, 8))
	assert.Equal(t, 4, NormalizeParallel(0))
	assert.Equal(t, MaxParallel, NormalizeParallel(100))
	assert.Equal(t, DefaultChunkBytes, NormalizeChunk(0))
	assert.Equal(t, MaxChunkBytes, NormalizeChunk(1<<40))
}
