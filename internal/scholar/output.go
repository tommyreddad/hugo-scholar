package scholar

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type stagedFile struct {
	path      string
	temporary string
	directory string
	backup    string
	backedUp  bool
	installed bool
}

// Stage every write before replacing anything. Keep originals until all writes
// and deletions succeed so a handled commit failure can restore the old output.
// This does not promise crash-atomicity across multiple filesystem paths.
func writeFiles(files map[string][]byte) error {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	staged := make([]stagedFile, 0, len(paths))
	committed := false
	defer func() {
		for _, file := range staged {
			if file.temporary != "" {
				_ = os.Remove(file.temporary)
			}
			if file.directory != "" {
				_ = os.Remove(file.directory)
			}
			// Retain a backup if rollback failed, rather than destroy recovery data.
			if file.backup != "" && (committed || !file.backedUp) {
				_ = os.Remove(file.backup)
			}
		}
	}()
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if info != nil && !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("output %s is not a regular file", path)
		}
		staged = append(staged, stagedFile{path: path})
		file := &staged[len(staged)-1]
		if content := files[path]; content != nil {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			directory, err := os.MkdirTemp(filepath.Dir(path), ".hugo-scholar-*")
			if err != nil {
				return err
			}
			file.directory = directory
			file.temporary = filepath.Join(directory, "output")
			temporary, err := os.OpenFile(file.temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			_, writeErr := temporary.Write(content)
			var chmodErr error
			if info != nil && info.Mode().IsRegular() {
				chmodErr = temporary.Chmod(info.Mode().Perm())
			}
			closeErr := temporary.Close()
			if err := errors.Join(writeErr, chmodErr, closeErr); err != nil {
				return err
			}
		}
		if info != nil {
			backup, err := os.CreateTemp(filepath.Dir(path), ".hugo-scholar-backup-*")
			if err != nil {
				return err
			}
			file.backup = backup.Name()
			if err := backup.Close(); err != nil {
				return err
			}
		}
	}
	rollback := func(cause error) error {
		errorsToReturn := []error{cause}
		for index := len(staged) - 1; index >= 0; index-- {
			file := &staged[index]
			if file.installed {
				if err := os.Remove(file.path); err != nil && !os.IsNotExist(err) {
					errorsToReturn = append(errorsToReturn, fmt.Errorf("remove new output %s: %w", file.path, err))
				}
			}
			if file.backedUp {
				if err := os.Rename(file.backup, file.path); err != nil {
					errorsToReturn = append(errorsToReturn, fmt.Errorf("restore %s from %s: %w", file.path, file.backup, err))
				} else {
					file.backedUp = false
				}
			}
		}
		return errors.Join(errorsToReturn...)
	}
	for index := range staged {
		file := &staged[index]
		if file.backup != "" {
			if err := os.Rename(file.path, file.backup); err != nil {
				return rollback(err)
			}
			file.backedUp = true
		}
		if file.temporary != "" {
			if err := os.Rename(file.temporary, file.path); err != nil {
				return rollback(err)
			}
			file.installed = true
		}
	}
	committed = true
	return nil
}
