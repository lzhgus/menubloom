package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestParseRequest(t *testing.T) {
	const tok = "secret-token-secret-token-secret"
	// As the extension encodes them: only unreserved characters stay.
	raw := "menubloom://run?t=" + tok + "&id=copy%3Apath&target=/Users/a/My%20Docs&p=/Users/a/My%20Docs/a%2Bb%26c%3D.txt&p=/x"
	r, err := parseRequest(raw, tok)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "copy:path" || r.Target != "/Users/a/My Docs" || !slices.Equal(r.Paths, []string{"/Users/a/My Docs/a+b&c=.txt", "/x"}) {
		t.Fatalf("got %+v", r)
	}
	for _, bad := range []string{
		"menubloom://run?t=wrong&id=cut&p=/x",             // a web page can't know the token
		"menubloom://run?id=cut&p=/x",                     // nor leave it out
		"menubloom://run?t=" + tok + "&id=cut&p=relative", // paths are absolute
		"other://run?t=" + tok + "&id=cut",
	} {
		if _, err := parseRequest(bad, tok); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	if _, err := parseRequest(raw, ""); err == nil {
		t.Error("accepted a request with no token configured")
	}
}

func TestUniquePath(t *testing.T) {
	d := t.TempDir()
	if got := uniquePath(d, "Untitled.txt"); got != filepath.Join(d, "Untitled.txt") {
		t.Fatal(got)
	}
	touch(t, filepath.Join(d, "Untitled.txt"))
	touch(t, filepath.Join(d, "Untitled 2.txt"))
	if got := uniquePath(d, "Untitled.txt"); got != filepath.Join(d, "Untitled 3.txt") {
		t.Fatal(got)
	}
	touch(t, filepath.Join(d, ".env"))
	if got := uniquePath(d, ".env"); got != filepath.Join(d, ".env 2") {
		t.Fatal(got)
	}
}

func TestCopyAs(t *testing.T) {
	home, _ := os.UserHomeDir()
	p := []string{home + "/a b/it's.tar.gz", "/tmp/x"}
	for kind, want := range map[string]string{
		"path":   p[0] + "\n/tmp/x",
		"name":   "it's.tar.gz\nx",
		"stem":   "it's.tar\nx",
		"tilde":  "~/a b/it's.tar.gz\n/tmp/x",
		"quoted": `'` + home + `/a b/it'\''s.tar.gz'` + "\n'/tmp/x'",
		"url":    "file://" + home + "/a%20b/it%27s.tar.gz\nfile:///tmp/x",
	} {
		if got := copyAs(kind, p); got != want {
			t.Errorf("%s: got %q, want %q", kind, got, want)
		}
	}
}

func TestTransfer(t *testing.T) {
	d := t.TempDir()
	src, dst := filepath.Join(d, "src"), filepath.Join(d, "dst")
	os.MkdirAll(filepath.Join(src, "folder"), 0o755)
	os.MkdirAll(dst, 0o755)
	touch(t, filepath.Join(src, "a.txt"))
	touch(t, filepath.Join(src, "folder", "in.txt"))
	touch(t, filepath.Join(dst, "a.txt"))

	done, err := transfer([]string{filepath.Join(src, "a.txt"), filepath.Join(src, "folder")}, dst, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(done, []string{filepath.Join(dst, "a 2.txt"), filepath.Join(dst, "folder")}) {
		t.Fatalf("copied to %v", done)
	}
	exists(t, filepath.Join(src, "a.txt"), true) // a copy leaves the original
	exists(t, filepath.Join(dst, "folder", "in.txt"), true)

	if _, err := transfer([]string{filepath.Join(src, "folder")}, filepath.Join(src, "folder"), true); err == nil {
		t.Error("moved a folder into itself")
	}
	if _, err := transfer([]string{src}, filepath.Join(src, "folder"), true); err == nil {
		t.Error("moved a folder into its subfolder")
	}
	done, err = transfer([]string{filepath.Join(src, "a.txt")}, dst, true)
	if err != nil || !slices.Equal(done, []string{filepath.Join(dst, "a 3.txt")}) {
		t.Fatalf("moved to %v, %v", done, err)
	}
	exists(t, filepath.Join(src, "a.txt"), false)
	if _, err := transfer([]string{filepath.Join(dst, "a.txt")}, filepath.Join(d, "missing"), true); err == nil {
		t.Error("moved into a folder that doesn't exist")
	}
}

func TestTemplates(t *testing.T) {
	d := t.TempDir()
	tmpl := filepath.Join(d, "Templates")
	if err := seedTemplates(tmpl); err != nil {
		t.Fatal(err)
	}
	names := listTemplates(tmpl)
	if !slices.Contains(names, "Word Document.docx") || !slices.Contains(names, "Text Document.txt") {
		t.Fatalf("templates %v", names)
	}
	os.Remove(filepath.Join(tmpl, "CSV.csv"))
	if err := seedTemplates(tmpl); err != nil || slices.Contains(listTemplates(tmpl), "CSV.csv") {
		t.Fatal("seeding again brought back a template the user deleted")
	}

	if err := newFile(tmpl, "Word Document.docx", filepath.Join(d, "Untitled.docx")); err != nil {
		t.Fatal(err)
	}
	// The Office files must be real documents: textutil reads them as Word does.
	for _, name := range []string{"Untitled.docx"} {
		if out, err := exec.Command("textutil", "-info", filepath.Join(d, name)).CombinedOutput(); err != nil {
			t.Errorf("%s isn't a document: %s", name, out)
		}
	}
	if err := newFile(tmpl, "../config.json", filepath.Join(d, "x.json")); err == nil {
		t.Error("made a file from outside the Templates folder")
	}
	if fi, _ := os.Stat(filepath.Join(tmpl, "Shell Script.sh")); fi == nil || fi.Mode()&0o100 == 0 {
		t.Error("the shell script template isn't executable")
	}

	// A new file is dated now, not when the template was made, or Finder
	// sorts it among old files where the user doesn't see it.
	old := time.Now().Add(-48 * time.Hour)
	pkg := filepath.Join(tmpl, "Deck.key")
	os.MkdirAll(filepath.Join(pkg, "Data"), 0o755)
	for _, name := range []string{"Shell Script.sh", "Deck.key"} {
		os.Chtimes(filepath.Join(tmpl, name), old, old)
		p := uniquePath(d, "Untitled"+filepath.Ext(name))
		if err := newFile(tmpl, name, p); err != nil {
			t.Fatal(name, err)
		}
		fi, _ := os.Stat(p)
		born := time.Unix(fi.Sys().(*syscall.Stat_t).Birthtimespec.Unix())
		if time.Since(fi.ModTime()) > time.Minute || name != "Deck.key" && time.Since(born) > time.Minute {
			t.Errorf("%s is dated %v, born %v, as its template", filepath.Base(p), fi.ModTime(), born)
		}
		if name == "Shell Script.sh" && fi.Mode()&0o100 == 0 {
			t.Error("the new shell script isn't executable")
		}
		if name == "Deck.key" && !isDir(filepath.Join(p, "Data")) {
			t.Error("the package template wasn't copied whole")
		}
	}
	if err := newFile(tmpl, "Shell Script.sh", filepath.Join(d, "Untitled.sh")); err == nil {
		t.Error("overwrote a file that's there")
	}
}

func TestRunActionPassesPathsAsArguments(t *testing.T) {
	d := t.TempDir()
	evil := filepath.Join(d, `$(touch pwned); it's "x".txt`)
	touch(t, evil)
	out, err := runAction(Action{Name: "t", Command: `printf '%s|' "$@"; pwd`}, d, []string{evil})
	if err != nil {
		t.Fatal(err)
	}
	if want := evil + "|" + d; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
	exists(t, filepath.Join(d, "pwned"), false)
	if _, err := runAction(Action{Name: "t", Command: "exit 3"}, d, nil); err == nil {
		t.Error("a failing command reported no error")
	}
}

func TestBuildMenu(t *testing.T) {
	c := Config{Apps: []App{{Name: "Term", Path: "/T.app", Terminal: true}, {Name: "Off", Path: "/O.app", Off: true}}, Folders: []string{"/tmp"}}
	ids := func(m []Entry) []string {
		var out []string
		var walk func([]Entry)
		walk = func(es []Entry) {
			for _, e := range es {
				if e.ID != "" {
					out = append(out, e.ID)
				}
				walk(e.Children)
			}
		}
		walk(m)
		return out
	}
	got := ids(buildMenu(c, State{Templates: []string{"Text.txt"}}))
	for _, want := range []string{"new:Text.txt", "copy:path", "open:/T.app", "cut", "copyto:/tmp", "moveto:", "img:png", "hidden"} {
		if !slices.Contains(got, want) {
			t.Errorf("menu lacks %s: %v", want, got)
		}
	}
	if slices.Contains(got, "open:/O.app") || slices.Contains(got, "paste") {
		t.Errorf("menu has an app turned off or Paste with nothing cut: %v", got)
	}
	got = ids(buildMenu(c, State{Cut: []string{"/a", "/b"}}))
	if !slices.Contains(got, "paste") || slices.Contains(got, "new:Text.txt") {
		t.Errorf("after Cut: %v", got)
	}
	c.SetOn(GroupCutPaste, false)
	c.SetOn(GroupCopyPath, false)
	got = ids(buildMenu(c, State{Cut: []string{"/a"}}))
	if slices.Contains(got, "paste") || slices.Contains(got, "cut") || slices.Contains(got, "copy:path") {
		t.Errorf("groups turned off still show: %v", got)
	}
	c.SetOn(GroupCopyPath, true)
	if !c.On(GroupCopyPath) || len(c.Off) != 1 {
		t.Errorf("SetOn: %v", c.Off)
	}
	// The extension decodes this shape (extension/FinderSync.swift).
	b, _ := json.Marshal(buildMenu(c, State{}))
	if strings.Contains(string(b), `"id":""`) || !strings.Contains(string(b), `"separator":true`) {
		t.Errorf("menu JSON: %s", b)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	d := t.TempDir()
	c, err := loadConfig(d)
	if err != nil || len(c.Folders) != 3 {
		t.Fatalf("defaults %+v, %v", c, err)
	}
	c.Actions = append(c.Actions, Action{ID: "x", Name: "n", Command: "c", Scope: Scopes[2], Exts: []string{"md"}})
	if err := saveConfig(d, c); err != nil {
		t.Fatal(err)
	}
	c2, err := loadConfig(d)
	if err != nil || len(c2.Actions) != 1 || c2.Actions[0].Exts[0] != "md" {
		t.Fatalf("loaded %+v, %v", c2, err)
	}
	tok, err := loadToken(d)
	tok2, _ := loadToken(d)
	if err != nil || len(tok) != 64 || tok != tok2 {
		t.Fatalf("token %q then %q, %v", tok, tok2, err)
	}
	if fi, _ := os.Stat(filepath.Join(d, "token")); fi.Mode().Perm() != 0o600 {
		t.Errorf("token file mode %v", fi.Mode())
	}
	if got := parseExts(".PNG, jpg  heic,"); !slices.Equal(got, []string{"png", "jpg", "heic"}) {
		t.Errorf("parseExts: %v", got)
	}
}

func TestPickApps(t *testing.T) {
	apps := []App{
		{Name: "Off term", Path: "/A.app", Terminal: true, Off: true},
		{Name: "Code", Path: "/C.app"},
		{Name: "Term", Path: "/T.app", Terminal: true},
	}
	for arg, want := range map[string]string{"terminal": "Term", "editor": "Code", "/A.app": "Off term", "/missing.app": ""} {
		got := pickApps(apps, arg)
		if (want == "") != (len(got) == 0) || len(got) > 0 && got[0].Name != want {
			t.Errorf("%s: got %v, want %s", arg, got, want)
		}
	}
	r := serviceRequest("paste", []string{"/a/b/c.txt", "/a/b/d"})
	if r.Target != "/a/b" || r.ID != "paste" || len(r.Paths) != 2 {
		t.Errorf("service request %+v", r)
	}
}

func TestTrayIcon(t *testing.T) {
	if b := trayIcon(); len(b) < 100 || string(b[1:4]) != "PNG" {
		t.Fatal("not a PNG")
	}
}

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(t *testing.T, p string, want bool) {
	t.Helper()
	_, err := os.Stat(p)
	if (err == nil) != want {
		t.Errorf("%s exists: %v, want %v", p, err == nil, want)
	}
}

func TestCopyMore(t *testing.T) {
	if got := copyAs("md", []string{"/a b/c.md"}); got != "[c.md](file:///a%20b/c.md)" {
		t.Errorf("markdown link %q", got)
	}
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "a.txt"), []byte("héllo"), 0o644)
	os.WriteFile(filepath.Join(d, "b.bin"), []byte{0, 1, 2}, 0o644)
	if got, err := contents([]string{filepath.Join(d, "a.txt")}); err != nil || got != "héllo" {
		t.Errorf("contents %q, %v", got, err)
	}
	if _, err := contents([]string{filepath.Join(d, "b.bin")}); err == nil {
		t.Error("copied a binary file's contents")
	}
	os.MkdirAll(filepath.Join(d, "src", "deep"), 0o755)
	touch(t, filepath.Join(d, "src", "deep", "x.go"))
	touch(t, filepath.Join(d, ".hidden"))
	want := filepath.Base(d) + "/\n├── a.txt\n├── b.bin\n└── src/\n    └── deep/\n        └── x.go"
	if got := tree([]string{d}); got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
}

func TestFileActions(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "run.sh")
	os.WriteFile(f, []byte("#!/bin/sh\n"), 0o640)
	if err := makeExecutable([]string{f}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(f); fi.Mode().Perm() != 0o750 {
		t.Errorf("mode %v, want rwxr-x---", fi.Mode().Perm())
	}
	links, err := symlink([]string{f})
	if err != nil || len(links) != 1 {
		t.Fatal(links, err)
	}
	if to, _ := os.Readlink(links[0]); to != f || filepath.Base(links[0]) != "run.sh symlink" {
		t.Errorf("symlink %s → %s", links[0], to)
	}
	aliases, err := makeAlias([]string{f})
	if err != nil || len(aliases) != 1 || filepath.Base(aliases[0]) != "run.sh alias" {
		t.Fatal(aliases, err)
	}
	if b, _ := os.ReadFile(aliases[0]); !strings.HasPrefix(string(b), "book") {
		t.Errorf("not a bookmark file: %q", b[:min(len(b), 8)])
	}
	if err := unquarantine([]string{f}); err != nil {
		t.Errorf("unquarantine of a file without quarantine: %v", err)
	}

	box := filepath.Join(d, "box")
	os.MkdirAll(filepath.Join(box, "inner"), 0o755)
	touch(t, filepath.Join(box, "run.sh")) // its name is taken in the parent
	touch(t, filepath.Join(box, ".DS_Store"))
	moved, err := dissolve([]string{box})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(moved, []string{filepath.Join(d, "inner"), filepath.Join(d, "run 2.sh")}) {
		t.Errorf("dissolved into %v", moved)
	}
	exists(t, box, false)
}

func TestImages(t *testing.T) {
	d := t.TempDir()
	src := filepath.Join(d, "icon.png")
	b, _ := os.ReadFile("assets/icon.png")
	os.WriteFile(src, b, 0o644)
	done, err := resizeImage(256, []string{src})
	if err != nil || len(done) != 1 || filepath.Base(done[0]) != "icon-256.png" {
		t.Fatal(done, err)
	}
	out, _ := exec.Command("sips", "-g", "pixelWidth", done[0]).Output()
	if !strings.Contains(string(out), "pixelWidth: 256") {
		t.Errorf("resized: %s", out)
	}
	if done, err := convertImage("icns", []string{src}); err != nil || filepath.Ext(done[0]) != ".icns" {
		t.Errorf("icns: %v, %v", done, err)
	}
}

func TestTerminalScriptQuotes(t *testing.T) {
	d := t.TempDir()
	evil := filepath.Join(d, `it's $(touch pwned) "x"`)
	script := terminalScript(Action{Command: `printf '%s|' "$@"; pwd`}, d, []string{evil, "/b"})
	out, err := exec.Command("/bin/zsh", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if want := evil + "|/b|" + d + "\n"; string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
	exists(t, filepath.Join(d, "pwned"), false)
}

// Every string the app translates has its Chinese; this reads the sources,
// as the strings are spread over them.
func TestEveryStringIsTranslated(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	re := regexp.MustCompile("T\\(\"((?:[^\"\\\\]|\\\\.)*)\"\\)|T\\(`([^`]*)`\\)")
	var dynamic []string
	for _, g := range Groups {
		dynamic = append(dynamic, g.Title, g.Detail)
	}
	for _, p := range panes {
		dynamic = append(dynamic, p.Title)
	}
	for _, e := range examples {
		dynamic = append(dynamic, e.Name)
	}
	dynamic = append(append(append(dynamic, Scopes...), Counts...), Appearances...)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, _ := os.ReadFile(f)
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			s := m[2]
			if m[1] != "" {
				s, _ = strconv.Unquote(`"` + m[1] + `"`)
			}
			dynamic = append(dynamic, s)
		}
	}
	for _, s := range dynamic {
		if _, ok := zh[s]; !ok {
			t.Errorf("no Chinese for %q", s)
		}
	}
	setLanguage(Languages[2], nil)
	defer setLanguage(Languages[1], nil)
	if T("Copy Path") != "拷贝路径" || T("not a string of the app") != "not a string of the app" {
		t.Error("T doesn't translate")
	}
	setLanguage("System", []string{"zh-Hans-US", "en-US"})
	if lang != "zh" {
		t.Error("the system's Chinese isn't followed")
	}
	setLanguage("System", []string{"en-GB", "zh-Hans"})
	if lang != "en" {
		t.Error("English first should win")
	}
}

func TestOrderAndInline(t *testing.T) {
	var c Config
	o := c.Ordered()
	if len(o) != len(Groups) || o[0] != Groups[0].ID {
		t.Fatalf("default order %v", o)
	}
	at := slices.Index(o, GroupHidden)
	c.Move(GroupHidden, -1)
	c.Move(o[0], -1) // the first stays first
	if got := c.Ordered(); slices.Index(got, GroupHidden) != at-1 || got[0] != o[0] {
		t.Errorf("after moves %v", got)
	}
	c.Order = []string{GroupCopyPath, "gone", GroupCopyPath} // a group of an older version, twice
	if got := c.Ordered(); got[0] != GroupCopyPath || len(got) != len(Groups) || slices.Contains(got, "gone") {
		t.Errorf("order with unknown groups %v", got)
	}
	ids := func(m []Entry) (top []string) {
		for _, e := range m {
			top = append(top, e.ID+e.Title)
		}
		return top
	}
	c = Config{Apps: []App{{Name: "T", Path: "/T.app"}}}
	if m := ids(buildMenu(c, State{})); !slices.Contains(m, "Open In") || slices.Contains(m, "open:/T.appT") {
		t.Errorf("not inline: %v", m)
	}
	c.SetInline(GroupOpenIn, true)
	m := buildMenu(c, State{})
	i := slices.IndexFunc(m, func(e Entry) bool { return e.ID == "open:/T.app" })
	if i < 0 || !slices.Equal(m[i].Contexts, anywhere) {
		t.Errorf("inline item lost its submenu's contexts: %+v", m)
	}
	c.SetInline(GroupCopyInfo, true)
	for _, e := range buildMenu(c, State{}) {
		if e.ID == "hash:md5" && (e.Kinds != "files" || !slices.Equal(e.Contexts, onItems)) {
			t.Errorf("an inline item's own conditions were replaced: %+v", e)
		}
	}
}

func TestActionRules(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := expandGlobs(" ~/code/, /work/*,, "); !slices.Equal(got, []string{home + "/code", home + "/code/*", "/work/*"}) {
		t.Errorf("globs %v", got)
	}
	e := actionEntry(Action{ID: "x", Scope: Scopes[1], Count: Counts[1], Under: "~/code"})
	if e.Count != "one" || e.Kinds != "files" || len(e.Under) != 2 {
		t.Errorf("entry %+v", e)
	}
	if e := actionEntry(Action{ID: "y", Count: Counts[2]}); e.Count != "many" || e.Under != nil {
		t.Errorf("entry %+v", e)
	}
}

func TestInfo(t *testing.T) {
	for n, want := range map[int64]string{999: "999 bytes", 1000: "1 KB", 1234567: "1.2 MB", 123456789: "123 MB", 45600000000: "45.6 GB"} {
		if got := humanBytes(n); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", n, got, want)
		}
	}
	if groupDigits(1234567) != "1,234,567" || groupDigits(12) != "12" {
		t.Error("groupDigits")
	}
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "a"), make([]byte, 1500), 0o644)
	os.MkdirAll(filepath.Join(d, "sub"), 0o755)
	os.WriteFile(filepath.Join(d, "sub", "b"), make([]byte, 500), 0o644)
	if got, err := info("size", d); err != nil || got != "2 KB (2,000 bytes)" {
		t.Errorf("folder size %q, %v", got, err)
	}
	if got, err := info("modified", filepath.Join(d, "a")); err != nil || len(got) != len(time.DateTime) {
		t.Errorf("modified %q, %v", got, err)
	}
	if got, err := info("created", filepath.Join(d, "a")); err != nil || !strings.HasPrefix(got, time.Now().Format("2006-")) {
		t.Errorf("created %q, %v", got, err)
	}
	if got, err := info("dimensions", "assets/icon.png"); err != nil || got != "1024 × 1024" {
		t.Errorf("dimensions %q, %v", got, err)
	}
	if _, err := info("dimensions", filepath.Join(d, "a")); err == nil {
		t.Error("dimensions of a file that isn't an image")
	}
}

func TestParseSelection(t *testing.T) {
	target, paths, err := parseSelection("/Users/a/Docs/\n/Users/a/Docs/x.txt\n/Users/a/Docs/sub/\n")
	if err != nil || target != "/Users/a/Docs" || !slices.Equal(paths, []string{"/Users/a/Docs/x.txt", "/Users/a/Docs/sub"}) {
		t.Errorf("got %q %v %v", target, paths, err)
	}
	if target, _, _ := parseSelection("/\n"); target != "/" {
		t.Errorf("root became %q", target)
	}
	if _, _, err := parseSelection(""); err == nil {
		t.Error("no window, no error")
	}
}

func TestParseCLI(t *testing.T) {
	r, ok := parseCLI([]string{"run", "copy:path", "a.txt", "/abs/b"}, "/work")
	if !ok || r.ID != "copy:path" || r.Target != "/work" || !slices.Equal(r.Paths, []string{"/work/a.txt", "/abs/b"}) {
		t.Errorf("got %+v %v", r, ok)
	}
	if r, ok := parseCLI([]string{"run", "hidden"}, "/w"); !ok || r.Target != "/w" || r.Paths != nil {
		t.Errorf("no paths: %+v", r)
	}
	for _, args := range [][]string{nil, {"run"}, {"-psn_0_1"}, {"open", "copy:path"}} {
		if _, ok := parseCLI(args, "/"); ok {
			t.Errorf("accepted %v", args)
		}
	}
}

// Every built-in item a shortcut can run is one the menu has, and only
// those that are there, and set, are bound.
func TestShortcuts(t *testing.T) {
	c := Config{
		Apps:    []App{{Name: "Terminal", Path: "/System/Applications/Utilities/Terminal.app"}, {Name: "Off", Path: "/x/Off.app", Off: true}},
		Actions: []Action{{ID: "a1", Name: "Mine", Shortcut: " Cmd+Alt+M "}, {ID: "a2", Name: "None"}},
		Keys: map[string]string{
			"copy:path": "Cmd+Shift+C", "new:Gone.txt": "Cmd+Shift+G", "cut": "  ",
			"open:/x/Off.app": "Cmd+Shift+O",
		},
	}
	templates := []string{"Markdown.md"}
	menu := map[string]bool{}
	var walk func([]Entry)
	walk = func(es []Entry) {
		for _, e := range es {
			menu[e.ID] = true
			walk(e.Children)
		}
	}
	walk(buildMenu(c, State{Templates: templates, Cut: []string{"/tmp/x"}}))
	for _, b := range bindable(c, templates) {
		if !menu[b.ID] {
			t.Errorf("%s (%s) isn't in the menu", b.ID, b.Title)
		}
	}
	want := map[string]string{"copy:path": "Cmd+Shift+C", "act:a1": "Cmd+Alt+M"}
	if got := shortcuts(c, templates); !reflect.DeepEqual(got, want) {
		t.Errorf("shortcuts = %v, want %v", got, want)
	}
}
