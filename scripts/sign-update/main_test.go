package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseNotes(t *testing.T) {
	log := "# Changelog\n\n## 0.5.2\n\n- New.\n\n## [0.5.1] - 2026-11-04\n\n- Old.\n"
	if got := releaseNotes(log, "0.5.2"); got != "- New." {
		t.Errorf("0.5.2: %q", got)
	}
	if got := releaseNotes(log, "0.5.1"); got != "- Old." {
		t.Errorf("0.5.1: %q", got)
	}
	if got := releaseNotes(log, "0.5"); got != "" {
		t.Errorf("0.5: %q", got)
	}
}

// The archive holds the one app, as the updater wants it.
func TestArchive(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "A.app", "Contents", "MacOS", "A")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("bin"), 0o755)
	os.Symlink("MacOS/A", filepath.Join(dir, "A.app", "Contents", "link"))
	var b bytes.Buffer
	if err := writeArchive(&b, dir, "A.app"); err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(&b)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	got := map[string]*tar.Header{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(h.Name, "A.app/") || h.Uid != 0 || h.Uname != "" {
			t.Errorf("entry %q, owner %d %q", h.Name, h.Uid, h.Uname)
		}
		got[h.Name] = h
	}
	if h := got["A.app/Contents/MacOS/A"]; h == nil || h.Mode&0o111 == 0 || h.Size != 3 {
		t.Errorf("executable: %+v", h)
	}
	if h := got["A.app/Contents/link"]; h == nil || h.Typeflag != tar.TypeSymlink || h.Linkname != "MacOS/A" {
		t.Errorf("link: %+v", h)
	}
}
