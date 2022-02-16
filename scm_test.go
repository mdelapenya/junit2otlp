package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetScm(t *testing.T) {
	t.Run("This project uses Git", func(t *testing.T) {
		workingDir, err := os.Getwd()
		if err != nil {
			workingDir = "."
		}

		scm := GetScm(workingDir)
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
