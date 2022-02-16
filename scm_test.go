package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetScm(t *testing.T) {
	t.Run("This project uses Git", func(t *testing.T) {
		scm := GetScm(getDefaultwd())
		switch scm.(type) {
		case *GitScm:
			// NOOP
		default:
			t.Error()
		}
	})

	t.Run("This project does not use Git", func(t *testing.T) {
		scm := GetScm(t.TempDir())

		assert.Nil(t, scm, "The directory should not contain a .git directory")
	})
}

func TestGetTargetBranch(t *testing.T) {
	t.Run("For change-requests it must return target branch", func(t *testing.T) {
		ctx := &ScmContext{
			ChangeRequest: true,
			TargetBranch:  "target",
			Branch:        "branch",
		}

		assert.Equal(t, "target", ctx.GetTargetBranch())
	})

	t.Run("For branches it must return branch", func(t *testing.T) {
		ctx := &ScmContext{
			ChangeRequest: false,
			TargetBranch:  "target",
			Branch:        "branch",
		}

		assert.Equal(t, "branch", ctx.GetTargetBranch())
	})
}
