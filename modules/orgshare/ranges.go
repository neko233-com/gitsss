// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"errors"
	"fmt"
	"sort"
)

// Span is a half-open byte interval [Start, End).
type Span struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// AddSpan merges a chunk into the covered set.
// A chunk that is already covered is ignored so a retry can be repeated.
// A partial overlap is rejected so two writers cannot tear the same bytes.
func AddSpan(spans []Span, start, end int64) ([]Span, error) {
	if start < 0 || end <= start {
		return nil, errors.New("chunk range is empty")
	}
	if covered(spans, start, end) {
		return cloneSpans(spans), nil
	}
	for _, span := range spans {
		if start < span.End && end > span.Start {
			return nil, fmt.Errorf("chunk %d-%d overlaps stored bytes %d-%d", start, end, span.Start, span.End)
		}
	}
	next := append(cloneSpans(spans), Span{Start: start, End: end})
	sort.Slice(next, func(i, j int) bool { return next[i].Start < next[j].Start })
	merged := []Span{next[0]}
	for _, span := range next[1:] {
		last := &merged[len(merged)-1]
		if span.Start <= last.End {
			if span.End > last.End {
				last.End = span.End
			}
			continue
		}
		merged = append(merged, span)
	}
	return merged, nil
}

// CoveredBytes is the number of bytes contained in spans.
func CoveredBytes(spans []Span) int64 {
	var total int64
	for _, span := range spans {
		total += span.End - span.Start
	}
	return total
}

// Missing returns the gaps before size. The result is empty when the file is complete.
func Missing(spans []Span, size int64) []Span {
	if size <= 0 {
		return nil
	}
	var missing []Span
	cursor := int64(0)
	for _, span := range spans {
		if span.Start > cursor {
			missing = append(missing, Span{Start: cursor, End: span.Start})
		}
		if span.End > cursor {
			cursor = span.End
		}
	}
	if cursor < size {
		missing = append(missing, Span{Start: cursor, End: size})
	}
	return missing
}

// PlanChunks splits size into chunks of chunk bytes. The last chunk is shorter.
func PlanChunks(size, chunk int64) []Span {
	if size <= 0 {
		return nil
	}
	if chunk <= 0 {
		chunk = DefaultChunkBytes
	}
	spans := make([]Span, 0, (size+chunk-1)/chunk)
	for start := int64(0); start < size; start += chunk {
		spans = append(spans, Span{Start: start, End: min(start+chunk, size)})
	}
	return spans
}

// NormalizeParallel clamps a worker count.
func NormalizeParallel(n int) int {
	if n <= 0 {
		return DefaultParallel
	}
	if n > MaxParallel {
		return MaxParallel
	}
	return n
}

// NormalizeChunk clamps one request body. Zero uses the default.
func NormalizeChunk(n int64) int64 {
	if n <= 0 {
		return DefaultChunkBytes
	}
	if n > MaxChunkBytes {
		return MaxChunkBytes
	}
	return n
}

func covered(spans []Span, start, end int64) bool {
	for _, span := range spans {
		if span.Start <= start && span.End >= end {
			return true
		}
	}
	return false
}

func cloneSpans(spans []Span) []Span {
	if len(spans) == 0 {
		return nil
	}
	out := make([]Span, len(spans))
	copy(out, spans)
	return out
}
