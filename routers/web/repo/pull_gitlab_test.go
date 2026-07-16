package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitLabDefaultMergeRequestTemplateCandidates(t *testing.T) {
	assert.Contains(t, pullRequestTemplateCandidates, ".gitlab/merge_request_templates/Default.md")
	assert.Contains(t, pullRequestTemplateCandidates, ".gitlab/MERGE_REQUEST_TEMPLATE.md")
}
