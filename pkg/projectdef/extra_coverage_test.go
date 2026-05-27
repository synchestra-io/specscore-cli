package projectdef

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// projectdef.go — lines 319-321: WriteSpecConfig os.WriteFile fails (stub-based)
// The existing TestWriteSpecConfig_NonWritableDir fails as root because
// root can write anywhere. Use the osWriteFileFn injectable stub instead.
// ---------------------------------------------------------------------------

func TestWriteSpecConfig_WriteFileError_Stub(t *testing.T) {
	orig := osWriteFileFn
	osWriteFileFn = func(name string, data []byte, perm os.FileMode) error {
		return fmt.Errorf("injected write error")
	}
	t.Cleanup(func() { osWriteFileFn = orig })

	err := WriteSpecConfig(t.TempDir(), SpecConfig{})
	if err == nil {
		t.Fatal("expected write error from injected failure")
	}
	if !strings.Contains(err.Error(), "writing spec config") {
		t.Errorf("expected wrapped write error; got %v", err)
	}
}
