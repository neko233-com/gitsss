// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitAdvertisementStreamsWithoutChangingWireFormat(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := &gitAdvertisementWriter{ResponseWriter: recorder, service: ServiceTypeUploadPack}
	assert.False(t, writer.started)
	n, err := writer.Write([]byte("first"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	_, err = writer.Write([]byte("second"))
	assert.NoError(t, err)
	assert.NoError(t, writer.finish())
	assert.Equal(t, string(packetWrite("# service=git-upload-pack\n"))+"0000firstsecond", recorder.Body.String())
	assert.Equal(t, "application/x-git-upload-pack-advertisement", recorder.Header().Get("Content-Type"))
}

func TestGitAdvertisementProtocolVersionsAndChunkBoundaries(t *testing.T) {
	for _, payload := range []string{"0000", "000eversion 1\n0000", "000eversion 2\n0015agent=git/test\n0000"} {
		for chunk := 1; chunk <= len(payload); chunk++ {
			recorder := httptest.NewRecorder()
			writer := &gitAdvertisementWriter{ResponseWriter: recorder, service: ServiceTypeUploadPack}
			for start := 0; start < len(payload); start += chunk {
				data := []byte(payload[start:min(start+chunk, len(payload))])
				n, err := writer.Write(data)
				assert.NoError(t, err)
				assert.Equal(t, len(data), n)
			}
			assert.NoError(t, writer.finish())
			expected := payload
			if !strings.HasPrefix(payload, "000eversion 2\n") {
				expected = string(packetWrite("# service=git-upload-pack\n")) + "0000" + payload
			}
			assert.Equal(t, expected, recorder.Body.String())
			assert.LessOrEqual(t, cap(writer.prefix), 16)
		}
	}
}

func TestContainsParentDirectorySeparator(t *testing.T) {
	tests := []struct {
		v string
		b bool
	}{
		{
			v: `user2/repo1/info/refs`,
			b: false,
		},
		{
			v: `user2/repo1/HEAD`,
			b: false,
		},
		{
			v: `user2/repo1/some.../strange_file...mp3`,
			b: false,
		},
		{
			v: `user2/repo1/../../custom/conf/app.ini`,
			b: true,
		},
		{
			v: `user2/repo1/objects/info/..\..\..\..\custom\conf\app.ini`,
			b: true,
		},
	}

	for i := range tests {
		assert.Equal(t, tests[i].b, containsParentDirectorySeparator(tests[i].v))
	}
}
