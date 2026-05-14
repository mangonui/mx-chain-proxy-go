package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveLogColorsShouldNotPanic(t *testing.T) {
	require.NotPanics(t, removeLogColors)
}
