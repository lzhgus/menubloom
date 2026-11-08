package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// Request is a click in the extension's menu.
type Request struct {
	ID     string
	Target string   // the folder Finder shows
	Paths  []string // the selected items
}

// parseRequest checks a menubloom://run URL against the token.
func parseRequest(raw, token string) (Request, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Request{}, err
	}
	if u.Scheme != "menubloom" || u.Host != "run" {
		return Request{}, fmt.Errorf("not a request: %s", raw)
	}
	q := u.Query()
	if token == "" || subtle.ConstantTimeCompare([]byte(q.Get("t")), []byte(token)) != 1 {
		return Request{}, errors.New("request without the extension's token")
	}
	r := Request{ID: q.Get("id"), Target: q.Get("target"), Paths: q["p"]}
	for _, p := range append([]string{r.Target}, r.Paths...) {
		if p != "" && !filepath.IsAbs(p) {
			return Request{}, fmt.Errorf("not an absolute path: %q", p)
		}
	}
	return r, nil
}

// items returns what an action works on: the selection, or the folder.
func (r Request) items() []string {
	if len(r.Paths) > 0 {
		return r.Paths
	}
	if r.Target != "" {
		return []string{r.Target}
	}
	return nil
}

// folder returns the folder that new items go to: the one selected, when
// a single folder is, else the folder Finder shows.
func (r Request) folder() string {
	if len(r.Paths) == 1 && isDir(r.Paths[0]) {
		return r.Paths[0]
	}
	return r.Target
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir() && filepath.Ext(p) != ".app"
}

// uniquePath returns dir/name, or "name 2.ext", "name 3.ext"… when taken,
// as Finder names copies.
func uniquePath(dir, name string) string {
	p := filepath.Join(dir, name)
	ext := filepath.Ext(name)
	if isDir(filepath.Join(dir, name)) || strings.HasPrefix(name, ".") {
		ext = ""
	}
	stem := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
			return p
		}
		p = filepath.Join(dir, fmt.Sprintf("%s %d%s", stem, i, ext))
	}
}

// copyAs formats the paths as Copy As's kinds do, one per line.
func copyAs(kind string, paths []string) string {
	home, _ := os.UserHomeDir()
	out := make([]string, len(paths))
	for i, p := range paths {
		switch kind {
		case "name":
			p = filepath.Base(p)
		case "stem":
			p = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		case "tilde":
			if home != "" && (p == home || strings.HasPrefix(p, home+"/")) {
				p = "~" + p[len(home):]
			}
		case "quoted":
			p = shellQuote(p)
		case "md":
			p = markdownLink(p)
		case "url":
			p = (&url.URL{Scheme: "file", Path: p}).String()
		}
		out[i] = p
	}
	return strings.Join(out, "\n")
}

func hashFiles(kind string, paths []string) (string, error) {
	var lines []string
	for _, p := range paths {
		var h hash.Hash
		switch kind {
		case "md5":
			h = md5.New()
		case "sha1":
			h = sha1.New()
		default:
			h = sha256.New()
		}
		f, err := os.Open(p)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}
		sum := hex.EncodeToString(h.Sum(nil))
		if len(paths) > 1 {
			sum += "  " + filepath.Base(p)
		}
		lines = append(lines, sum)
	}
	return strings.Join(lines, "\n"), nil
}

// newFile makes the file dst from a template. It is dated now, not when
// the template was: Finder sorts it with today's files, where the user
// looks for it.
func newFile(templates, template, dst string) error {
	if template != filepath.Base(template) || strings.HasPrefix(template, ".") {
		return fmt.Errorf("not a template: %q", template)
	}
	src := filepath.Join(templates, template)
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	if fi.IsDir() { // a package, as Pages or Keynote save
		if err := copyItem(src, dst); err != nil {
			return err
		}
		now := time.Now()
		return os.Chtimes(dst, now, now)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fi.Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dst)
	}
	return err
}

// copyItem copies a file or folder with its metadata, as Finder does.
func copyItem(src, dst string) error {
	if out, err := exec.Command("/usr/bin/ditto", src, dst).CombinedOutput(); err != nil {
		return fmt.Errorf("copy %s: %s", filepath.Base(src), strings.TrimSpace(string(out)))
	}
	return nil
}

// transfer copies or moves items into dir, renaming those whose name is
// taken there. It returns the new paths, of the items done before an error.
func transfer(paths []string, dir string, move bool) ([]string, error) {
	if !isDir(dir) {
		return nil, fmt.Errorf("not a folder: %s", dir)
	}
	var done []string
	for _, src := range paths {
		if move && filepath.Dir(src) == filepath.Clean(dir) {
			continue // already there
		}
		if rel, err := filepath.Rel(src, dir); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
			return done, fmt.Errorf("can't put %s inside itself", filepath.Base(src))
		}
		dst := uniquePath(dir, filepath.Base(src))
		var err error
		switch {
		case !move:
			err = copyItem(src, dst)
		default:
			err = os.Rename(src, dst)
			if errors.Is(err, syscall.EXDEV) { // another volume: copy, then delete
				if err = copyItem(src, dst); err == nil {
					err = os.RemoveAll(src)
				}
			}
		}
		if err != nil {
			return done, err
		}
		done = append(done, dst)
	}
	return done, nil
}

// convertImage writes a copy of each image in another format beside it.
func convertImage(format string, paths []string) ([]string, error) {
	ext := map[string]string{"png": "png", "jpeg": "jpg", "heic": "heic", "tiff": "tiff", "icns": "icns"}[format]
	if ext == "" {
		return nil, fmt.Errorf("unknown format %q", format)
	}
	var done []string
	for _, src := range paths {
		dst := uniquePath(filepath.Dir(src), strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))+"."+ext)
		if format == "icns" {
			if err := makeICNS(src, dst); err != nil {
				return done, fmt.Errorf("convert %s: %v", filepath.Base(src), err)
			}
			done = append(done, dst)
			continue
		}
		if out, err := exec.Command("/usr/bin/sips", "-s", "format", format, src, "--out", dst).CombinedOutput(); err != nil {
			return done, fmt.Errorf("convert %s: %s", filepath.Base(src), strings.TrimSpace(string(out)))
		}
		done = append(done, dst)
	}
	return done, nil
}

// pickApps returns the app of Open In at a path, or for the services'
// "terminal" and "editor", the first of that kind that is on.
func pickApps(apps []App, arg string) []App {
	for _, a := range apps {
		if a.Path == arg || !a.Off && (arg == "terminal" && a.Terminal || arg == "editor" && !a.Terminal) {
			return []App{a}
		}
	}
	return nil
}

// makeICNS makes an app icon of every size from 16 to 1024 pixels, as
// iconutil packs an .iconset: sips writes ICNS of one size of 512 at most.
func makeICNS(src, dst string) error {
	dir, err := os.MkdirTemp("", "menubloom-*.iconset")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	for _, n := range []int{16, 32, 128, 256, 512} {
		for scale, suffix := range map[int]string{1: "", 2: "@2x"} {
			px := fmt.Sprint(n * scale)
			out := filepath.Join(dir, fmt.Sprintf("icon_%dx%d%s.png", n, n, suffix))
			if b, err := exec.Command("/usr/bin/sips", "-s", "format", "png", "-z", px, px, src, "--out", out).CombinedOutput(); err != nil {
				return errors.New(strings.TrimSpace(string(b)))
			}
		}
	}
	if b, err := exec.Command("/usr/bin/iconutil", "-c", "icns", "-o", dst, dir).CombinedOutput(); err != nil {
		return errors.New(strings.TrimSpace(string(b)))
	}
	return nil
}

// openIn opens items with an app; a terminal opens the folder of a file.
func openIn(a App, paths []string) error {
	if a.Terminal {
		seen := map[string]bool{}
		var dirs []string
		for _, p := range paths {
			if !isDir(p) {
				p = filepath.Dir(p)
			}
			if !seen[p] {
				seen[p] = true
				dirs = append(dirs, p)
			}
		}
		paths = dirs
	}
	out, err := exec.Command("/usr/bin/open", append([]string{"-a", a.Path}, paths...)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("open in %s: %s", a.Name, strings.TrimSpace(string(out)))
	}
	return nil
}

// runAction runs a custom action in the folder of its items, with the
// paths as "$@" of a login shell, so that the user's PATH applies.
func runAction(a Action, dir string, paths []string) (string, error) {
	cmd := exec.Command("/bin/zsh", append([]string{"-lc", a.Command, "menubloom"}, paths...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return "", fmt.Errorf("%s: %s", a.Name, text)
	}
	return text, nil
}

// showsHidden reports whether Finder shows hidden files.
func showsHidden() bool {
	out, _ := exec.Command("/usr/bin/defaults", "read", "com.apple.finder", "AppleShowAllFiles").Output()
	switch strings.ToLower(strings.TrimSpace(string(out))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

func setShowsHidden(on bool) error {
	if err := exec.Command("/usr/bin/defaults", "write", "com.apple.finder", "AppleShowAllFiles", "-bool", fmt.Sprint(on)).Run(); err != nil {
		return err
	}
	return exec.Command("/usr/bin/killall", "Finder").Run()
}

// reveal selects the paths in Finder.
func reveal(paths ...string) {
	exec.Command("/usr/bin/open", append([]string{"-R"}, paths...)...).Run()
}

// markdownLink formats a path as a Markdown link to its file.
func markdownLink(p string) string {
	return "[" + filepath.Base(p) + "](" + (&url.URL{Scheme: "file", Path: p}).String() + ")"
}

const maxContents = 4 << 20

// contents returns the text of files, one after the other.
func contents(paths []string) (string, error) {
	var parts []string
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return "", err
		}
		if fi.Size() > maxContents {
			return "", fmt.Errorf("%s is over 4 MB", filepath.Base(p))
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
			return "", fmt.Errorf("%s isn't text", filepath.Base(p))
		}
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n"), nil
}

// tree draws folders as the tree command does, without hidden files.
// ponytail: stops at maxTree lines and 4 levels; enough to paste into a chat.
func tree(paths []string) string {
	const maxTree = 500
	var lines []string
	var walk func(dir, prefix string, depth int)
	walk = func(dir, prefix string, depth int) {
		des, _ := os.ReadDir(dir)
		des = slices.DeleteFunc(des, func(d os.DirEntry) bool { return strings.HasPrefix(d.Name(), ".") })
		for i, d := range des {
			if len(lines) >= maxTree {
				return
			}
			branch, next := "├── ", "│   "
			if i == len(des)-1 {
				branch, next = "└── ", "    "
			}
			name := d.Name()
			if d.IsDir() {
				name += "/"
			}
			lines = append(lines, prefix+branch+name)
			if d.IsDir() && depth < 4 {
				walk(filepath.Join(dir, d.Name()), prefix+next, depth+1)
			}
		}
	}
	for _, p := range paths {
		lines = append(lines, filepath.Base(p)+"/")
		walk(p, "", 1)
	}
	if len(lines) >= maxTree {
		lines = append(lines, "…")
	}
	return strings.Join(lines, "\n")
}

// symlink makes a symbolic link to each item beside it.
func symlink(paths []string) ([]string, error) {
	var done []string
	for _, p := range paths {
		dst := uniquePath(filepath.Dir(p), filepath.Base(p)+" symlink")
		if err := os.Symlink(p, dst); err != nil {
			return done, err
		}
		done = append(done, dst)
	}
	return done, nil
}

// makeExecutable adds execute permission wherever there is read permission,
// as chmod +x does.
func makeExecutable(paths []string) error {
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return err
		}
		if err := os.Chmod(p, fi.Mode()|(fi.Mode()&0o444)>>2); err != nil {
			return err
		}
	}
	return nil
}

func unquarantine(paths []string) error {
	for _, p := range paths {
		out, err := exec.Command("/usr/bin/xattr", "-dr", "com.apple.quarantine", p).CombinedOutput()
		if err != nil && !strings.Contains(string(out), "No such xattr") {
			return fmt.Errorf("%s: %s", filepath.Base(p), strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// dissolve moves what folders hold into their parents, then removes them.
func dissolve(paths []string) ([]string, error) {
	var done []string
	for _, dir := range paths {
		des, err := os.ReadDir(dir)
		if err != nil {
			return done, err
		}
		var kids []string
		for _, d := range des {
			if d.Name() != ".DS_Store" {
				kids = append(kids, filepath.Join(dir, d.Name()))
			}
		}
		moved, err := transfer(kids, filepath.Dir(dir), true)
		done = append(done, moved...)
		if err != nil {
			return done, err
		}
		os.Remove(filepath.Join(dir, ".DS_Store"))
		if err := os.Remove(dir); err != nil {
			return done, err
		}
	}
	return done, nil
}

// resizeImage writes a copy of each image no larger than size pixels on
// its longer side beside it.
func resizeImage(size int, paths []string) ([]string, error) {
	var done []string
	for _, src := range paths {
		ext := filepath.Ext(src)
		dst := uniquePath(filepath.Dir(src), fmt.Sprintf("%s-%d%s", strings.TrimSuffix(filepath.Base(src), ext), size, ext))
		if out, err := exec.Command("/usr/bin/sips", "-Z", fmt.Sprint(size), src, "--out", dst).CombinedOutput(); err != nil {
			return done, fmt.Errorf("resize %s: %s", filepath.Base(src), strings.TrimSpace(string(out)))
		}
		done = append(done, dst)
	}
	return done, nil
}

// shellQuote quotes a string for sh and zsh.
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// terminalScript is a .command file that runs an action in Terminal, with
// the paths as "$@", as runAction runs it.
func terminalScript(a Action, dir string, paths []string) string {
	q := make([]string, len(paths))
	for i, p := range paths {
		q[i] = shellQuote(p)
	}
	return "#!/bin/zsh -l\ncd " + shellQuote(dir) + " || exit\nset -- " + strings.Join(q, " ") + "\n" + a.Command + "\n"
}

func runInTerminal(a Action, dir string, paths []string) error {
	f, err := os.CreateTemp("", "menubloom-*.command")
	if err != nil {
		return err
	}
	_, err = f.WriteString(terminalScript(a, dir, paths))
	f.Close()
	if err == nil {
		err = os.Chmod(f.Name(), 0o700)
	}
	if err != nil {
		return err
	}
	return exec.Command("/usr/bin/open", "-a", "Terminal", f.Name()).Run()
}

// info formats what an Info item copies: an item's size (a folder's is
// what it holds), dates, or an image's dimensions.
func info(kind, p string) (string, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	switch kind {
	case "size":
		n := fi.Size()
		if fi.IsDir() {
			n = 0
			filepath.WalkDir(p, func(_ string, d os.DirEntry, err error) error {
				if err == nil && d.Type().IsRegular() {
					if i, err := d.Info(); err == nil {
						n += i.Size()
					}
				}
				return nil
			})
		}
		return humanBytes(n) + " (" + groupDigits(n) + " bytes)", nil
	case "created":
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return "", errors.New("no creation date")
		}
		return time.Unix(st.Birthtimespec.Unix()).Format(time.DateTime), nil
	case "modified":
		return fi.ModTime().Format(time.DateTime), nil
	case "dimensions":
		out, err := exec.Command("/usr/bin/sips", "-g", "pixelWidth", "-g", "pixelHeight", p).Output()
		if err != nil {
			return "", fmt.Errorf("%s isn't an image", filepath.Base(p))
		}
		var w, h string
		for _, l := range strings.Split(string(out), "\n") {
			if f := strings.Fields(l); len(f) == 2 {
				switch f[0] {
				case "pixelWidth:":
					w = f[1]
				case "pixelHeight:":
					h = f[1]
				}
			}
		}
		if _, err := strconv.Atoi(w); err != nil || h == "" || h == "<nil>" { // sips prints <nil> for what isn't an image
			return "", fmt.Errorf("%s isn't an image", filepath.Base(p))
		}
		return w + " × " + h, nil
	}
	return "", fmt.Errorf("unknown info %q", kind)
}

// humanBytes writes sizes in decimal units, as Finder does.
func humanBytes(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d bytes", n)
	}
	v, unit := float64(n), ""
	for _, u := range []string{"KB", "MB", "GB", "TB", "PB"} {
		v, unit = v/1000, u
		if v < 1000 {
			break
		}
	}
	s := fmt.Sprintf("%.0f", v)
	if v < 100 {
		s = strings.TrimSuffix(fmt.Sprintf("%.1f", v), ".0")
	}
	return s + " " + unit
}

func groupDigits(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// parseCLI reads the arguments "run <action id> [paths…]", the command
// line of scripts and Shortcuts, with paths relative to wd.
func parseCLI(args []string, wd string) (Request, bool) {
	if len(args) < 2 || args[0] != "run" || args[1] == "" {
		return Request{}, false
	}
	r := Request{ID: args[1]}
	for _, p := range args[2:] {
		if !filepath.IsAbs(p) {
			p = filepath.Join(wd, p)
		}
		r.Paths = append(r.Paths, filepath.Clean(p))
	}
	if len(r.Paths) > 0 {
		r.Target = filepath.Dir(r.Paths[0])
	} else {
		r.Target = wd
	}
	return r, true
}
