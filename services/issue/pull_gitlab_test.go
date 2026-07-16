package issue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCodeOwnerFileGitLab(t *testing.T) {
	assert.True(t, IsCodeOwnerFile(".gitlab/CODEOWNERS"))
	assert.False(t, IsCodeOwnerFile(".gitlab/codeowners"))
}
