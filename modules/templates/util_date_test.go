// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/neko233-com/gitsss/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestDateTime(t *testing.T) {
	testTz, _ := time.LoadLocation("America/New_York")
	defer test.MockVariableValue(&setting.DefaultUILocation, testTz)()
	defer test.MockVariableValue(&setting.IsProd, true)()
	defer test.MockVariableValue(&setting.IsInTesting, false)()

	du := NewDateUtils()

	refTimeStr := "2018-01-01T00:00:00Z"
	refTime, _ := time.Parse(time.RFC3339, refTimeStr)
	refTimeStamp := timeutil.TimeStamp(refTime.Unix())

	assert.EqualValues(t, "-", du.AbsoluteShort(nil))
	assert.EqualValues(t, "-", du.AbsoluteShort(0))
	assert.EqualValues(t, "-", du.AbsoluteShort(time.Time{}))
	assert.EqualValues(t, "-", du.AbsoluteShort(timeutil.TimeStamp(0)))

	actual := du.AbsoluteShort(refTime)
	assert.EqualValues(t, `<relative-time weekday="" year="numeric" threshold="P0Y" month="short" day="numeric" prefix="" datetime="2018-01-01T00:00:00Z">2018-01-01</relative-time>`, actual)

	actual = du.AbsoluteShort(refTimeStamp)
	assert.EqualValues(t, `<relative-time weekday="" year="numeric" threshold="P0Y" month="short" day="numeric" prefix="" datetime="2017-12-31T19:00:00-05:00">2017-12-31</relative-time>`, actual)

	actual = du.FullTime(refTimeStamp)
	assert.EqualValues(t, `<relative-time weekday="" year="numeric" format="datetime" month="short" day="numeric" hour="numeric" minute="numeric" second="numeric" data-tooltip-content data-tooltip-interactive="true" datetime="2017-12-31T19:00:00-05:00">2017-12-31 19:00:00 -05:00</relative-time>`, actual)
}

func TestTimeSince(t *testing.T) {
	testTz, _ := time.LoadLocation("America/New_York")
	defer test.MockVariableValue(&setting.DefaultUILocation, testTz)()
	defer test.MockVariableValue(&setting.IsProd, true)()
	defer test.MockVariableValue(&setting.IsInTesting, false)()

	du := NewDateUtils()
	assert.EqualValues(t, "-", du.TimeSince(nil))

	refTimeStr := "2018-01-01T00:00:00Z"
	refTime, _ := time.Parse(time.RFC3339, refTimeStr)

	actual := du.TimeSince(refTime)
	assert.EqualValues(t, `<relative-time prefix="" tense="past" datetime="2018-01-01T00:00:00Z" data-tooltip-content data-tooltip-interactive="true">2018-01-01 00:00:00 +00:00</relative-time>`, actual)

	actual = timeSinceTo(&refTime, time.Time{})
	assert.EqualValues(t, `<relative-time prefix="" tense="future" datetime="2018-01-01T00:00:00Z" data-tooltip-content data-tooltip-interactive="true">2018-01-01 00:00:00 +00:00</relative-time>`, actual)

	actual = du.TimeSince(timeutil.TimeStampNano(refTime.UnixNano()))
	assert.EqualValues(t, `<relative-time prefix="" tense="past" datetime="2017-12-31T19:00:00-05:00" data-tooltip-content data-tooltip-interactive="true">2017-12-31 19:00:00 -05:00</relative-time>`, actual)
}

func TestActivityTime(t *testing.T) {
	du := NewDateUtils()
	for _, value := range []any{nil, 0, time.Time{}, timeutil.TimeStamp(0)} {
		assert.EqualValues(t, "-", du.ActivityTime(value))
	}
	ref := time.Date(2026, time.October, 16, 9, 8, 7, 0, time.FixedZone("local", 8*60*60))
	assert.EqualValues(t, `<relative-time format="activity" datetime="2026-10-16T09:08:07+08:00" data-tooltip-content data-tooltip-interactive="true">2026-10-16 09:08:07 Fri +08:00</relative-time>`, du.ActivityTime(ref))
	assert.Equal(t, du.ActivityTime(ref), du.ActivityTime(&ref))
}
