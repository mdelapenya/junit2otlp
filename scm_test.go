package main

import (
	"testing"
)

func TestGetScm(t *testing.T) {
	t.Run("This project uses Git", func(t *testing.T) {
		scm := GetScm()
		switch scm.(type) {
		case *GitScm:
			// NOOP
		default:
			t.Error()
		}
	})
}
