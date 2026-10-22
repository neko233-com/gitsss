// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {contentRangeHeader, planChunks} from './org-share.ts';

test('planChunks splits a file and formats inclusive ranges', () => {
  expect(planChunks(12, 5)).toEqual([
    {start: 0, end: 5},
    {start: 5, end: 10},
    {start: 10, end: 12},
  ]);
  expect(contentRangeHeader(0, 5, 12)).toBe('bytes 0-4/12');
  expect(planChunks(0, 5)).toEqual([]);
});
