package issue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitLabIssueTemplateDirectoryCandidate(t *testing.T) {
	assert.Contains(t, templateDirCandidates, ".gitlab/issue_templates")
}
