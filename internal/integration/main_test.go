package integration

import (
	"os"
	"testing"

	"github.com/mykso/myks/internal/testutil"
)

func TestMain(m *testing.M) {
	testutil.SetupTestLogging()
	os.Exit(m.Run())
}
