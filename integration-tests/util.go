package integration

import (
	"os"
	"testing"
)

// this function must be called for any test that requires docker, aborting the
// test if it returns false
func RequireDocker(t *testing.T) bool {
	if os.Getenv("IN_DOCKER") == "" {
		t.Skip("You must run these tests inside docker")
		return false
	}

	return true
}
