// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/neko233-com/gitsss/modules/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageCaptchaSingleUseConcurrent(t *testing.T) {
	require.NoError(t, cache.Init())
	captcha := GetImageCaptcha()
	const id = "single-use-concurrency-test"
	require.NoError(t, captcha.Store.Put(captcha.CachePrefix+id, string([]byte{1, 2, 3, 4, 5, 6}), 60))
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			form := url.Values{captcha.FieldIdName: {id}, captcha.FieldCaptchaName: {"123456"}}
			req := httptest.NewRequest(http.MethodPost, "/user/login", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if VerifyImageCaptcha(req) {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	assert.EqualValues(t, 1, accepted.Load())
	called := false
	w := httptest.NewRecorder()
	ProtectImageCaptcha(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/captcha/"+id+".png?reload=1", nil))
	assert.False(t, called)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
