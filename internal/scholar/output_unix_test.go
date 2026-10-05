//go:build unix

package scholar

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWriteHonorsUmask(t *testing.T) {
	if os.Getenv("HUGO_SCHOLAR_TEST_OUTPUT_UMASK") != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestWriteHonorsUmask$")
		command.Env = append(os.Environ(), "HUGO_SCHOLAR_TEST_OUTPUT_UMASK=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("output permission subprocess: %v\n%s", err, output)
		}
		return
	}
	// Umask is process-wide; isolate it from the rest of the test suite.
	old := syscall.Umask(0077)
	defer syscall.Umask(old)
	options, output := generationFixture(t)
	for _, path := range []string{output, filepath.Join(options.ContentDir, "reading.md"), filepath.Join(options.ContentDir, "bibliography", "paper.md")} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("new output does not honor private umask: %s, %v, %v", path, info, err)
		}
	}
	reading := filepath.Join(options.ContentDir, "reading.md")
	if err := os.Chmod(reading, 0640); err != nil {
		t.Fatal(err)
	}
	changeGenerationInputs(t, options)
	data, err := PrepareWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(output, data); err != nil {
		t.Fatal(err)
	}
	for path, mode := range map[string]os.FileMode{reading: 0640, filepath.Join(options.ContentDir, "bibliography", "new.md"): 0600} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("incorrect output permissions: %s, want %o, got %v, %v", path, mode, info, err)
		}
	}
}
