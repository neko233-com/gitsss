package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitLabDefaultIssueTemplateCandidates(t *testing.T) {
	assert.Contains(t, IssueTemplateCandidates, ".gitlab/ISSUE_TEMPLATE.md")
	assert.Contains(t, IssueTemplateCandidates, ".gitlab/issue_template.md")
}
