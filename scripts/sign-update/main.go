// Sign-update makes the update of a release from the finished app, the one
// with the Finder extension that mygo build has no place for: the archive
// of Menubloom.app and update-darwin-universal.json, the manifest that
// installed apps read, in the format of mygo's updater.
//
//	go run ./scripts/sign-update build/darwin-universal/Menubloom.app build
//
// The key is MYGO_UPDATER_PRIVATE_KEY, else the file of `mygo keygen -o`
// in ~/Library/Application Support/mygo/update-keys/menubloom. The notes
// are the version's section of CHANGELOG.md.
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const target = "darwin-universal"

type config struct {
	Slug    string `json:"slug"`
	Version string `json:"version"`
	Updates struct {
		PublicKey string `json:"publicKey"`
		GitHub    string `json:"github"`
	} `json:"updates"`
}

type manifest struct {
	Version   string `json:"version"`
	Notes     string `json:"notes,omitempty"`
	Date      string `json:"date,omitempty"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Signature string `json:"signature"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: sign-update <Menubloom.app> <out dir>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "sign-update:", err)
		os.Exit(1)
	}
}

func run(app, out string) error {
	var c config
	b, err := os.ReadFile("mygo.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return fmt.Errorf("mygo.json: %w", err)
	}
	key, err := signingKey(c.Updates.PublicKey)
	if err != nil {
		return err
	}
	changelog, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		return err
	}
	notes := releaseNotes(string(changelog), c.Version)
	if notes == "" {
		return fmt.Errorf("CHANGELOG.md has no \"## %s\" section", c.Version)
	}
	name := c.Slug + "-" + c.Version + "-" + target + ".tar.gz"
	archive := filepath.Join(out, name)
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	sum := sha256.New()
	err = writeArchive(io.MultiWriter(f, sum), filepath.Dir(app), filepath.Base(app))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	info, err := os.Stat(archive)
	if err != nil {
		return err
	}
	m := manifest{
		Version:   c.Version,
		Notes:     notes,
		Date:      time.Now().UTC().Format(time.RFC3339),
		URL:       "https://github.com/" + c.Updates.GitHub + "/releases/download/v" + c.Version + "/" + name,
		Size:      info.Size(),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, sum.Sum(nil))),
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	path := filepath.Join(out, "update-"+target+".json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Println(archive)
	fmt.Println(path)
	return nil
}

// signingKey reads the private key and checks it against mygo.json's.
func signingKey(public string) (ed25519.PrivateKey, error) {
	text := os.Getenv("MYGO_UPDATER_PRIVATE_KEY")
	if text == "" {
		dir, _ := os.UserConfigDir()
		b, err := os.ReadFile(filepath.Join(dir, "mygo", "update-keys", "menubloom", "mygo-update.key"))
		if err != nil {
			return nil, fmt.Errorf("no signing key: set MYGO_UPDATER_PRIVATE_KEY (%w)", err)
		}
		text = string(b)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid update private key")
	}
	key := ed25519.PrivateKey(raw)
	if base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)) != strings.TrimSpace(public) {
		return nil, errors.New("the signing key does not match updates.publicKey in mygo.json")
	}
	return key, nil
}

// writeArchive writes entry of dir as a tar.gz, as mygo's updater
// unpacks it: owners cleared, directories, files and symbolic links only.
func writeArchive(w io.Writer, dir, entry string) error {
	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(gz)
	err = filepath.WalkDir(filepath.Join(dir, entry), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&fs.ModeSymlink != 0 {
			if link, err = os.Readlink(p); err != nil {
				return err
			}
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
		}
		hdr.Uid, hdr.Gid, hdr.Uname, hdr.Gname = 0, 0, "", ""
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// releaseNotes is the section of changelog under "## <version>".
func releaseNotes(changelog, version string) string {
	var notes []string
	in := false
	for _, line := range strings.Split(changelog, "\n") {
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			if in {
				break
			}
			fields := strings.Fields(rest)
			in = len(fields) > 0 && strings.Trim(fields[0], "[]v") == version
			continue
		}
		if in {
			notes = append(notes, line)
		}
	}
	return strings.TrimSpace(strings.Join(notes, "\n"))
}
