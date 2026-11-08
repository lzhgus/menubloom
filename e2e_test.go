//go:build e2e

// End-to-end checks of the installed app, clicked as a user clicks: every
// action through the menubloom:// URL the extension sends, then the real
// Finder menu, right-clicked and read through Accessibility.
//
//	scripts/build.sh --install && go test -tags e2e -run E2E -v -count=1 .
//
// The terminal running it needs Accessibility (System Settings ▸ Privacy).
// It takes over the app for a minute or two: it quits it, swaps in a test
// config (every group on, English, a custom action, TextEdit in Open In),
// and puts back your config and clipboard after. Leave the mouse alone
// while TestE2EFinder runs. MENUBLOOM_E2E_INTRUSIVE=1 also checks Open in
// Terminal and Show Hidden Files, which restarts Finder twice.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const bundleID = "io.github.lzhgus.menubloom"

var (
	appDir string // the app's data folder
	token  string
	root   string // the fixtures, under the home folder for "~" paths
	home   string
)

func TestMain(m *testing.M) { os.Exit(runE2E(m)) }

func runE2E(m *testing.M) int {
	home, _ = os.UserHomeDir()
	appDir = dataDir()
	cfgPath := filepath.Join(appDir, "config.json")
	saved, err := os.ReadFile(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Println("e2e:", err)
		return 1
	}
	clip := pbpaste()
	if !quitApp() {
		fmt.Println("e2e: couldn't quit the app")
		return 1
	}
	defer func() {
		quitApp()
		if saved != nil {
			os.WriteFile(cfgPath, saved, 0o600)
		} else {
			os.Remove(cfgPath)
		}
		startApp()
		pbcopy(clip)
	}()

	c, err := loadConfig(appDir)
	if err != nil {
		fmt.Println("e2e:", err)
		return 1
	}
	c.Off, c.Order, c.Inlined = nil, nil, nil
	c.Language, c.AskedLogin = Languages[1], true
	c.Keys = map[string]string{"copy:path": e2eKey}
	c.Actions = append(c.Actions, Action{ID: "e2e", Name: "E2E Action", Command: `printf '%s\n' "$@" > e2e-action.txt`, Scope: Scopes[0]})
	if i := slices.IndexFunc(c.Apps, func(a App) bool { return a.Name == "TextEdit" }); i >= 0 {
		c.Apps[i].Off = false
	} else {
		c.Apps = append(c.Apps, App{Name: "TextEdit", Path: "/System/Applications/TextEdit.app"})
	}
	if err := saveConfig(appDir, c); err != nil {
		fmt.Println("e2e:", err)
		return 1
	}
	startApp()
	if !poll(func() bool { return strings.Contains(menuJSON(), `"act:e2e"`) }) {
		fmt.Println("e2e: the app didn't write the test menu; is /Applications/Menubloom.app installed?")
		return 1
	}
	b, _ := os.ReadFile(filepath.Join(appDir, "token"))
	token = strings.TrimSpace(string(b))

	root, _ = os.MkdirTemp(filepath.Join(home, "Library", "Caches"), "menubloom-e2e-")
	defer os.RemoveAll(root)
	code := m.Run()
	closeFinderWindows(root)
	return code
}

// The app.

func running() bool { return exec.Command("/usr/bin/pgrep", "-x", "Menubloom").Run() == nil }

// quitApp quits the app, so that it reads the config again when started.
func quitApp() bool {
	osa(`if application id "` + bundleID + `" is running then tell application id "` + bundleID + `" to quit`)
	if !poll(func() bool { return !running() }) {
		exec.Command("/usr/bin/pkill", "-x", "Menubloom").Run()
	}
	return poll(func() bool { return !running() })
}

// startApp launches the app as a click in the menu does, which opens no
// window; the request itself, without the token, is refused.
func startApp() {
	exec.Command("/usr/bin/open", "-g", "menubloom://run?t=-").Run()
	poll(running)
}

func menuJSON() string {
	b, _ := os.ReadFile(filepath.Join(appDir, "menu.json"))
	return string(b)
}

// send clicks an item as the extension does: target is the folder Finder
// shows, paths the selection.
func send(t *testing.T, id, target string, paths ...string) {
	t.Helper()
	sendToken(t, token, id, target, paths...)
}

func sendToken(t *testing.T, tok, id, target string, paths ...string) {
	t.Helper()
	q := url.Values{"t": {tok}, "id": {id}}
	if target != "" {
		q.Set("target", target)
	}
	for _, p := range paths {
		q.Add("p", p)
	}
	if out, err := exec.Command("/usr/bin/open", "-g", "menubloom://run?"+q.Encode()).CombinedOutput(); err != nil {
		t.Fatalf("open %s: %s", id, out)
	}
}

// Waiting and checking.

func poll(f func() bool) bool {
	for end := time.Now().Add(15 * time.Second); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
		if f() {
			return true
		}
	}
	return false
}

func waitFor(t *testing.T, what string, f func() bool) {
	t.Helper()
	if !poll(f) {
		t.Fatalf("timed out waiting for %s", what)
	}
}

func there(p string) bool { _, err := os.Lstat(p); return err == nil }

func read(p string) string { b, _ := os.ReadFile(p); return string(b) }

func write(t *testing.T, p, s string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func writePNG(t *testing.T, p string, w, h int) string {
	t.Helper()
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)))
	return write(t, p, b.String())
}

// fixture is a new, empty folder for one case.
func fixture(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp(root, "case-")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func fresh(t *testing.T, p string) {
	t.Helper()
	if fi, err := os.Stat(p); err != nil || time.Since(fi.ModTime()) > time.Minute {
		t.Errorf("%s isn't dated now: %v", filepath.Base(p), fi.ModTime())
	}
}

// The clipboard.

func pbpaste() string { out, _ := exec.Command("/usr/bin/pbpaste").Output(); return string(out) }

func pbcopy(s string) {
	cmd := exec.Command("/usr/bin/pbcopy")
	cmd.Stdin = strings.NewReader(s)
	cmd.Run()
}

// copied runs an item that copies, and returns what it copied.
func copied(t *testing.T, click func()) string {
	t.Helper()
	const sentinel = "menubloom-e2e-sentinel"
	pbcopy(sentinel)
	click()
	var got string
	waitFor(t, "the clipboard", func() bool { got = pbpaste(); return got != sentinel })
	return got
}

// answerNewFile clicks a button of New File's panel, after typing name
// into it unless name is "".
func answerNewFile(t *testing.T, button, name string) {
	t.Helper()
	mustOSA(t, `on run argv
tell application "System Events" to tell process "Menubloom"
	repeat 75 times
		if exists window "New File" then exit repeat
		delay 0.2
	end repeat
	set g to splitter group 1 of window "New File"
	if (item 2 of argv) is not "" then set value of text field 1 of g to (item 2 of argv)
	click button (item 1 of argv) of g
end tell
end run`, button, name)
}

// AppleScript.

func osa(script string, args ...string) (string, error) {
	cmd := exec.Command("/usr/bin/osascript", append([]string{"-"}, args...)...)
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func mustOSA(t *testing.T, script string, args ...string) string {
	t.Helper()
	out, err := osa(script, args...)
	if err != nil {
		t.Fatalf("osascript: %s", out)
	}
	return out
}

func finderWindows() string {
	out, _ := osa(`tell application "Finder" to get name of every Finder window`)
	return out
}

func closeFinderWindows(under string) {
	osa(`on run argv
tell application "Finder"
	repeat with w in (get every Finder window)
		try
			if POSIX path of (target of w as alias) starts with (item 1 of argv) then close w
		end try
	end repeat
end tell
end run`, under)
}

// TestE2EActions runs every action through the URL the extension sends.
func TestE2EActions(t *testing.T) {
	t.Cleanup(func() { closeFinderWindows(root) })

	t.Run("a request without the token does nothing", func(t *testing.T) {
		d := fixture(t)
		sendToken(t, "not-the-token", "new:Markdown.md", d)
		sendToken(t, "", "new:Markdown.md", d)
		time.Sleep(2 * time.Second)
		if out, _ := osa(`tell application "System Events" to exists window "New File" of process "Menubloom"`); out != "false" {
			t.Fatal("asked for a file's name for a request without the token")
		}
	})

	t.Run("new file from every template, beside the selected file", func(t *testing.T) {
		d := fixture(t)
		page := write(t, filepath.Join(d, "page.html"), "<p>x</p>")
		tdir := filepath.Join(appDir, "Templates")
		templates := listTemplates(tdir)
		if len(templates) == 0 {
			t.Fatal("no templates")
		}
		for _, tm := range templates {
			send(t, "new:"+tm, d, page)
			answerNewFile(t, "Create", "")
			p := filepath.Join(d, "Untitled"+filepath.Ext(tm))
			src := filepath.Join(tdir, tm)
			waitFor(t, filepath.Base(p)+" as a copy of "+tm, func() bool { return there(p) && (isDir(src) || read(p) == read(src)) })
			fresh(t, p)
		}
	})

	t.Run("new file into the one selected folder, with a free name", func(t *testing.T) {
		d := fixture(t)
		sub := filepath.Join(d, "sub")
		os.Mkdir(sub, 0o755)
		for range 2 {
			send(t, "new:Markdown.md", d, sub)
			answerNewFile(t, "Create", "")
		}
		waitFor(t, "sub/Untitled 2.md", func() bool { return there(filepath.Join(sub, "Untitled 2.md")) })
		if !there(filepath.Join(sub, "Untitled.md")) || there(filepath.Join(d, "Untitled.md")) {
			t.Fatal("not made in the selected folder")
		}
	})

	t.Run("new file by extension, as the services ask", func(t *testing.T) {
		d := fixture(t)
		send(t, "new:.txt", d)
		answerNewFile(t, "Create", "")
		waitFor(t, "Untitled.txt", func() bool { return there(filepath.Join(d, "Untitled.txt")) })
	})

	t.Run("new file with the name typed, which keeps the extension", func(t *testing.T) {
		d := fixture(t)
		send(t, "new:Markdown.md", d)
		answerNewFile(t, "Create", "notes")
		waitFor(t, "notes.md", func() bool { return there(filepath.Join(d, "notes.md")) })
	})

	t.Run("new file canceled makes nothing", func(t *testing.T) {
		d := fixture(t)
		send(t, "new:Markdown.md", d)
		answerNewFile(t, "Cancel", "")
		time.Sleep(time.Second)
		if des, _ := os.ReadDir(d); len(des) > 0 {
			t.Fatalf("made %s", des[0].Name())
		}
	})

	t.Run("copy as", func(t *testing.T) {
		d := fixture(t)
		p := write(t, filepath.Join(d, "a b.txt"), "hello\n")
		q := write(t, filepath.Join(d, "c.txt"), "x")
		u := "file://" + strings.ReplaceAll(p, " ", "%20")
		for id, want := range map[string]string{
			"copy:path":     p,
			"copy:name":     "a b.txt",
			"copy:stem":     "a b",
			"copy:tilde":    "~" + strings.TrimPrefix(p, home),
			"copy:quoted":   "'" + p + "'",
			"copy:url":      u,
			"copy:md":       "[a b.txt](" + u + ")",
			"copy:contents": "hello\n",
			"hash:md5":      "b1946ac92492d2347c6235b4d2611184",
			"hash:sha1":     "f572d396fae9206628714fb2ce00f72e94f2258f",
			"hash:sha256":   "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		} {
			if got := copied(t, func() { send(t, id, d, p) }); got != want {
				t.Errorf("%s copied %q, want %q", id, got, want)
			}
		}
		if got := copied(t, func() { send(t, "copy:path", d, p, q) }); got != p+"\n"+q {
			t.Errorf("two paths copied %q", got)
		}
		if got := copied(t, func() { send(t, "copy:path", d) }); got != d {
			t.Errorf("the folder's path, with nothing selected, copied %q", got)
		}
	})

	t.Run("copy folder tree", func(t *testing.T) {
		d := fixture(t)
		proj := filepath.Join(d, "proj")
		write(t, filepath.Join(proj, "README"), "")
		write(t, filepath.Join(proj, "src", "main.go"), "")
		write(t, filepath.Join(proj, ".git", "HEAD"), "")
		want := "proj/\n├── README\n└── src/\n    └── main.go"
		if got := copied(t, func() { send(t, "copy:tree", d, proj) }); got != want {
			t.Errorf("tree:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("info", func(t *testing.T) {
		d := fixture(t)
		p := write(t, filepath.Join(d, "a.txt"), "hello\n")
		pic := writePNG(t, filepath.Join(d, "pic.png"), 64, 32)
		date := regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d$`)
		if got := copied(t, func() { send(t, "info:size", d, p) }); got != "6 bytes (6 bytes)" {
			t.Errorf("size %q", got)
		}
		for _, id := range []string{"info:created", "info:modified"} {
			if got := copied(t, func() { send(t, id, d, p) }); !date.MatchString(got) {
				t.Errorf("%s %q", id, got)
			}
		}
		if got := copied(t, func() { send(t, "info:dimensions", d, pic) }); got != "64 × 32" {
			t.Errorf("dimensions %q", got)
		}
	})

	t.Run("cut, then paste in another folder", func(t *testing.T) {
		d := fixture(t)
		p := write(t, filepath.Join(d, "a.txt"), "x")
		other := filepath.Join(d, "other")
		os.Mkdir(other, 0o755)
		send(t, "cut", d, p)
		waitFor(t, "Paste in the menu", func() bool { return strings.Contains(menuJSON(), `"paste"`) })
		send(t, "paste", other)
		waitFor(t, "the move", func() bool { return there(filepath.Join(other, "a.txt")) && !there(p) })
		waitFor(t, "Paste to leave the menu", func() bool { return !strings.Contains(menuJSON(), `"paste"`) })
	})

	t.Run("copy to and move to, renaming on conflicts", func(t *testing.T) {
		d := fixture(t)
		p := write(t, filepath.Join(d, "a.txt"), "x")
		dst := filepath.Join(d, "dst")
		os.Mkdir(dst, 0o755)
		send(t, "copyto:"+dst, d, p)
		waitFor(t, "the copy", func() bool { return there(filepath.Join(dst, "a.txt")) })
		send(t, "copyto:"+dst, d, p)
		waitFor(t, "the second copy", func() bool { return there(filepath.Join(dst, "a 2.txt")) })
		send(t, "moveto:"+dst, d, p)
		waitFor(t, "the move", func() bool { return there(filepath.Join(dst, "a 3.txt")) && !there(p) })
	})

	t.Run("file actions", func(t *testing.T) {
		d := fixture(t)
		p := write(t, filepath.Join(d, "run.sh"), "#!/bin/sh\n")

		send(t, "file:alias", d, p)
		alias := filepath.Join(d, "run.sh alias")
		waitFor(t, "the alias", func() bool { return there(alias) })
		if !strings.HasPrefix(read(alias), "book") {
			t.Error("the alias isn't a bookmark file")
		}

		send(t, "file:symlink", d, p)
		link := filepath.Join(d, "run.sh symlink")
		waitFor(t, "the symlink", func() bool { return there(link) })
		if to, _ := os.Readlink(link); to != p {
			t.Errorf("the symlink points to %q", to)
		}

		send(t, "file:exec", d, p)
		waitFor(t, "chmod +x", func() bool { fi, _ := os.Stat(p); return fi.Mode()&0o111 == 0o111 })

		exec.Command("/usr/bin/xattr", "-w", "com.apple.quarantine", "0081;00000000;Safari;", p).Run()
		send(t, "file:unquarantine", d, p)
		waitFor(t, "the quarantine to go", func() bool {
			return exec.Command("/usr/bin/xattr", "-p", "com.apple.quarantine", p).Run() != nil
		})

		box := filepath.Join(d, "box")
		write(t, filepath.Join(box, "one.txt"), "1")
		write(t, filepath.Join(box, "run.sh"), "clash")
		send(t, "file:dissolve", d, box)
		waitFor(t, "the folder to dissolve", func() bool { return !there(box) })
		if !there(filepath.Join(d, "one.txt")) || read(filepath.Join(d, "run 2.sh")) != "clash" {
			t.Error("dissolve lost or overwrote a file")
		}
	})

	t.Run("delete immediately asks first", func(t *testing.T) {
		d := fixture(t)
		p := write(t, filepath.Join(d, "doomed.txt"), "x")
		answer := `on run argv
tell application "System Events" to tell process "Menubloom"
	repeat 75 times
		if exists button (item 1 of argv) of window 1 then exit repeat
		delay 0.2
	end repeat
	click button (item 1 of argv) of window 1
end tell
end run`
		send(t, "file:delete", d, p)
		mustOSA(t, answer, "Cancel")
		time.Sleep(time.Second)
		if !there(p) {
			t.Fatal("deleted after Cancel")
		}
		send(t, "file:delete", d, p)
		mustOSA(t, answer, "Delete")
		waitFor(t, "the delete", func() bool { return !there(p) })
	})

	t.Run("convert and resize images", func(t *testing.T) {
		d := fixture(t)
		pic := writePNG(t, filepath.Join(d, "pic.png"), 600, 300)
		for format, ext := range map[string]string{"png": "pic 2.png", "jpeg": "pic.jpg", "heic": "pic.heic", "tiff": "pic.tiff"} {
			send(t, "img:"+format, d, pic)
			out := filepath.Join(d, ext)
			waitFor(t, ext, func() bool { return there(out) })
			waitFor(t, ext+" as "+format, func() bool {
				b, _ := exec.Command("/usr/bin/sips", "-g", "format", out).Output()
				return strings.Contains(string(b), "format: "+format)
			})
		}
		send(t, "img:icns", d, pic)
		icns := filepath.Join(d, "pic.icns")
		waitFor(t, "pic.icns", func() bool { return strings.HasPrefix(read(icns), "icns") })
		send(t, "resize:256", d, pic)
		small := filepath.Join(d, "pic-256.png")
		waitFor(t, "pic-256.png at 256 × 128", func() bool { got, _ := info("dimensions", small); return got == "256 × 128" })
	})

	t.Run("custom action gets the paths as arguments", func(t *testing.T) {
		d := fixture(t)
		p := write(t, filepath.Join(d, "it's $(x).txt"), "")
		send(t, "act:e2e", d, p)
		out := filepath.Join(d, "e2e-action.txt")
		waitFor(t, "the action's output", func() bool { return read(out) == p+"\n" })
	})

	t.Run("edit templates opens the Templates folder", func(t *testing.T) {
		send(t, "templates", "")
		waitFor(t, "a Templates window", func() bool { return strings.Contains(finderWindows(), "Templates") })
		closeFinderWindows(filepath.Join(appDir, "Templates"))
	})

	t.Run("open in TextEdit", func(t *testing.T) {
		d := fixture(t)
		p := write(t, filepath.Join(d, "e2e-note.txt"), "hi")
		send(t, "open:/System/Applications/TextEdit.app", d, p)
		// Through System Events, which the Finder checks need anyway: telling
		// TextEdit itself would ask for one more Automation permission.
		waitFor(t, "TextEdit to open it", func() bool {
			out, _ := osa(`tell application "System Events" to get name of windows of process "TextEdit"`)
			return strings.Contains(out, "e2e-note.txt")
		})
		osa(`tell application "System Events" to click button 1 of window "e2e-note.txt" of process "TextEdit"`)
	})

	if os.Getenv("MENUBLOOM_E2E_INTRUSIVE") == "" {
		return
	}

	t.Run("open in Terminal opens the file's folder", func(t *testing.T) {
		d := fixture(t)
		p := write(t, filepath.Join(d, "a.txt"), "")
		send(t, "open:/System/Applications/Utilities/Terminal.app", d, p)
		waitFor(t, "a Terminal window", func() bool {
			out, _ := osa(`tell application "System Events" to get name of windows of process "Terminal"`)
			return strings.Contains(out, filepath.Base(d))
		})
		osa(`on run argv
tell application "System Events" to click button 1 of (first window of process "Terminal" whose name starts with (item 1 of argv))
end run`, filepath.Base(d))
	})

	t.Run("show hidden files, then hide them again", func(t *testing.T) {
		was := showsHidden()
		if exec.Command("/usr/bin/defaults", "read", "com.apple.finder", "AppleShowAllFiles").Run() != nil {
			t.Cleanup(func() { exec.Command("/usr/bin/defaults", "delete", "com.apple.finder", "AppleShowAllFiles").Run() })
		}
		send(t, "hidden", "")
		waitFor(t, "the switch", func() bool { return showsHidden() != was })
		send(t, "hidden", "")
		waitFor(t, "the switch back", func() bool { return showsHidden() == was })
	})
}

// Finder, through Accessibility.

const outline = `outline 1 of scroll area 1 of splitter group 1 of splitter group 1 of window 1`

// rightClick right-clicks a row of the front Finder window by its name, or
// the empty space below the rows for "", as a mouse does.
func rightClick(t *testing.T, name string) {
	t.Helper()
	xy := mustOSA(t, `on run argv
tell application "Finder" to activate
delay 0.3
tell application "System Events" to tell process "Finder"
	set o to `+outline+`
	if (item 1 of argv) is "" then
		set {px, py} to position of o
		set {w, h} to size of o
		return ((px + 60) as integer as text) & " " & ((py + h - 30) as integer as text)
	end if
	repeat with r in rows of o
		set f to text field 1 of UI element 1 of r
		if value of f is (item 1 of argv) then
			set {px, py} to position of f
			set {w, h} to size of f
			return ((px + 10) as integer as text) & " " & ((py + h / 2) as integer as text)
		end if
	end repeat
	error "no row " & (item 1 of argv)
end tell
end run`, name)
	cmd := exec.Command("/usr/bin/osascript", append([]string{"-l", "JavaScript", "-"}, strings.Fields(xy)...)...)
	cmd.Stdin = strings.NewReader(`ObjC.import('CoreGraphics');
function run(argv) {
	const p = {x: Number(argv[0]), y: Number(argv[1])};
	for (const type of [3, 4]) { // right mouse down, up
		$.CGEventPost(0, $.CGEventCreateMouseEvent(null, type, p, 1));
		delay(0.05);
	}
}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("right-click: %s", out)
	}
	time.Sleep(800 * time.Millisecond)
}

// e2eKey is Copy Path's shortcut in the test config, one no app uses.
const (
	e2eKey          = "Cmd+Ctrl+Alt+Shift+P"
	cmdCtrlAltShift = 0x100000 | 0x40000 | 0x80000 | 0x20000 // CGEventFlags
)

// pressKey types a key with modifiers, as the keyboard does, so global
// shortcuts see it.
func pressKey(t *testing.T, code, flags int) {
	t.Helper()
	cmd := exec.Command("/usr/bin/osascript", "-l", "JavaScript", "-", strconv.Itoa(code), strconv.Itoa(flags))
	cmd.Stdin = strings.NewReader(`ObjC.import('CoreGraphics');
function run(argv) {
	for (const down of [true, false]) {
		const e = $.CGEventCreateKeyboardEvent(null, Number(argv[0]), down);
		$.CGEventSetFlags(e, Number(argv[1]));
		$.CGEventPost(0, e);
		delay(0.05);
	}
}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("key: %s", out)
	}
}

// menu returns the titles of the open context menu, or of its submenu
// path, and closes it.
func menu(t *testing.T, path ...string) []string {
	t.Helper()
	out := mustOSA(t, `on run argv
tell application "System Events" to tell process "Finder"
	set m to menu 1 of `+outline+`
	repeat with nm in argv
		set mi to menu item (nm as text) of m
		click mi
		delay 0.4
		set m to menu 1 of mi
	end repeat
	set titles to {}
	repeat with i in menu items of m
		set end of titles to (title of i as text)
	end repeat
	repeat (count of argv) + 1 times
		key code 53
	end repeat
	set AppleScript's text item delimiters to linefeed
	return titles as text
end tell
end run`, path...)
	return strings.Split(out, "\n")
}

// choose clicks an item of the open context menu, through its submenus.
func choose(t *testing.T, path ...string) {
	t.Helper()
	mustOSA(t, `on run argv
tell application "System Events" to tell process "Finder"
	set m to menu 1 of `+outline+`
	repeat with i from 1 to count of argv
		set mi to menu item (item i of argv) of m
		click mi
		if i < (count of argv) then
			delay 0.4
			set m to menu 1 of mi
		end if
	end repeat
end tell
end run`, path...)
}

func selectedRows(t *testing.T) string {
	return mustOSA(t, `tell application "System Events" to tell process "Finder"
	set names to {}
	repeat with r in (rows of `+outline+` whose selected is true)
		set end of names to value of text field 1 of UI element 1 of r
	end repeat
	return names as text
end tell`)
}

func hasAll(t *testing.T, where string, titles []string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !slices.Contains(titles, w) {
			t.Errorf("%s: no %q in %q", where, w, titles)
		}
	}
}

func hasNone(t *testing.T, where string, titles []string, unwanted ...string) {
	t.Helper()
	for _, w := range unwanted {
		if slices.Contains(titles, w) {
			t.Errorf("%s: %q shouldn't be there", where, w)
		}
	}
}

// TestE2EFinder right-clicks in a real Finder window: the extension draws
// the menu from menu.json, and its clicks reach the app.
func TestE2EFinder(t *testing.T) {
	d := fixture(t)
	page := write(t, filepath.Join(d, "page.html"), "<p>x</p>")
	writePNG(t, filepath.Join(d, "pic.png"), 64, 32)
	os.Mkdir(filepath.Join(d, "sub"), 0o755)
	mustOSA(t, `on run argv
tell application "Finder"
	activate
	set w to make new Finder window to (POSIX file (item 1 of argv) as alias)
	set current view of w to list view
	set bounds of w to {200, 150, 1100, 750}
end tell
end run`, d)
	t.Cleanup(func() { closeFinderWindows(d) })
	waitFor(t, "the Finder window", func() bool {
		out, _ := osa(`tell application "System Events" to tell process "Finder" to count rows of ` + outline)
		return out == "3"
	})

	rightClick(t, "page.html")
	titles := menu(t)
	hasAll(t, "a file", titles, "New File", "Copy Path", "Copy As", "Info", "Open In", "Cut", "Copy To", "Move To", "File Actions", "E2E Action")
	hasNone(t, "a file", titles, "Convert Image", "Resize Image", "Show Hidden Files", "Hide Hidden Files")

	rightClick(t, "page.html")
	hasAll(t, "New File", menu(t, "New File"), "Markdown", "Text Document", "Edit Templates…")

	rightClick(t, "page.html")
	if info := menu(t, "Info"); len(info) == 0 || info[0] != "Size:  8 bytes" {
		t.Errorf("Info shows %q, want the size first", info)
	}

	rightClick(t, "pic.png")
	hasAll(t, "an image", menu(t), "Convert Image", "Resize Image")

	rightClick(t, "")
	titles = menu(t)
	hasAll(t, "the background", titles, "New File", "Copy Path", "Open In")
	if !slices.Contains(titles, "Show Hidden Files") && !slices.Contains(titles, "Hide Hidden Files") {
		t.Errorf("the background: no Show/Hide Hidden Files in %q", titles)
	}
	hasNone(t, "the background", titles, "Cut", "Copy To", "Move To", "E2E Action")

	t.Run("new file beside the selected file, selected and dated now", func(t *testing.T) {
		rightClick(t, "page.html")
		choose(t, "New File", "Markdown")
		answerNewFile(t, "Create", "")
		p := filepath.Join(d, "Untitled.md")
		waitFor(t, "Untitled.md", func() bool { return there(p) })
		fresh(t, p)
		waitFor(t, "Finder to select it", func() bool { return selectedRows(t) == "Untitled.md" })
	})

	t.Run("new file on the background", func(t *testing.T) {
		rightClick(t, "")
		choose(t, "New File", "Text Document")
		answerNewFile(t, "Create", "")
		waitFor(t, "Untitled.txt", func() bool { return there(filepath.Join(d, "Untitled.txt")) })
	})

	t.Run("copy path", func(t *testing.T) {
		got := copied(t, func() { rightClick(t, "page.html"); choose(t, "Copy Path") })
		if got != page {
			t.Errorf("copied %q, want %q", got, page)
		}
	})

	t.Run("a shortcut copies the selected path", func(t *testing.T) {
		mustOSA(t, `on run argv
tell application "Finder"
	activate
	select (POSIX file (item 1 of argv) as alias)
end tell
end run`, page)
		waitFor(t, "the selection", func() bool { return selectedRows(t) == "page.html" })
		got := copied(t, func() { pressKey(t, 35, cmdCtrlAltShift) }) // P
		if got != page {
			t.Errorf("copied %q, want %q", got, page)
		}
	})

	t.Run("cut, then paste into a folder", func(t *testing.T) {
		rightClick(t, "page.html")
		choose(t, "Cut")
		waitFor(t, "Paste in the menu", func() bool { return strings.Contains(menuJSON(), `"paste"`) })
		rightClick(t, "sub")
		choose(t, "Paste Item Here")
		waitFor(t, "the move", func() bool { return there(filepath.Join(d, "sub", "page.html")) && !there(page) })
	})
}
