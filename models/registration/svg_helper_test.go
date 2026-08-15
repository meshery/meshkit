package registration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openFDCount reports the number of open file descriptors for this process.
// Only meaningful on Linux, where /proc/self/fd is available - this is what
// CI runs on (see .github/workflows/ci.yml).
func openFDCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	return len(entries)
}

func TestWriteAndReplaceSVGWithFileSystemPathClosesFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fd-count check relies on /proc/self/fd, only available on linux")
	}

	tmp := t.TempDir()
	before := openFDCount(t)

	for i := 0; i < 50; i++ {
		dirname := filepath.Join("model", "component")
		WriteAndReplaceSVGWithFileSystemPath("<svg>color</svg>", "<svg>white</svg>", "<svg>complete</svg>", tmp, dirname, "icon", false)
	}

	after := openFDCount(t)
	assert.Less(t, after-before, 10, "open file descriptors grew by %d after 50 calls (3 unclosed *os.File per call would leak 150)", after-before)
}
