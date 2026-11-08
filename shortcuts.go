package main

import (
	"errors"
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
)

// finderScript prints the folder of Finder's front window, then each
// selected item, one per line.
const finderScript = `tell application "Finder"
	try
		set out to POSIX path of (insertion location as alias)
	on error
		set out to POSIX path of (path to desktop folder)
	end try
	repeat with i in (get selection)
		set out to out & linefeed & POSIX path of (i as alias)
	end repeat
	return out
end tell`

// finderSelection asks Finder what is selected, which needs the user's
// permission to control Finder, asked once.
func finderSelection() (target string, paths []string, err error) {
	out, err := exec.Command("/usr/bin/osascript", "-e", finderScript).CombinedOutput()
	if err != nil {
		return "", nil, errors.New(strings.TrimSpace(string(out)))
	}
	return parseSelection(string(out))
}

func parseSelection(out string) (string, []string, error) {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	clean := func(p string) string {
		if len(p) > 1 {
			p = strings.TrimSuffix(p, "/")
		}
		return p
	}
	if len(lines) == 0 || lines[0] == "" {
		return "", nil, errors.New("no Finder window")
	}
	var paths []string
	for _, l := range lines[1:] {
		if l != "" {
			paths = append(paths, clean(l))
		}
	}
	return clean(lines[0]), paths, nil
}

// Bindable is a built-in item a shortcut can run: its menu id, and its
// path in the menu.
type Bindable struct{ ID, Title string }

// bindable lists the built-in items that make sense from the keyboard,
// as Settings shows them.
func bindable(c Config, templates []string) []Bindable {
	b := []Bindable{{"copy:path", T("Copy Path")}}
	for _, k := range [][2]string{
		{"copy:name", "Name"}, {"copy:stem", "Name Without Extension"}, {"copy:tilde", "~ Path"},
		{"copy:quoted", "Quoted Path (for the shell)"}, {"copy:url", "File URL"},
		{"copy:md", "Markdown Link"}, {"copy:contents", "Contents"},
	} {
		b = append(b, Bindable{k[0], T("Copy As") + " ▸ " + T(k[1])})
	}
	for _, t := range templates {
		b = append(b, Bindable{"new:" + t, T("New File") + " ▸ " + strings.TrimSuffix(t, filepath.Ext(t))})
	}
	for _, a := range c.Apps {
		if !a.Off {
			b = append(b, Bindable{"open:" + a.Path, T("Open In") + " ▸ " + a.Name})
		}
	}
	return append(b, Bindable{"cut", T("Cut")}, Bindable{"paste", T("Paste Item Here")},
		Bindable{"hidden", T("Show Hidden Files")})
}

// shortcuts maps the menu ids that have a shortcut to it. A built-in item
// gone from the menu, as a deleted template, keeps its key in the config
// but isn't bound.
func shortcuts(c Config, templates []string) map[string]string {
	m := map[string]string{}
	for _, b := range bindable(c, templates) {
		if k := strings.TrimSpace(c.Keys[b.ID]); k != "" {
			m[b.ID] = k
		}
	}
	for _, a := range c.Actions {
		if k := strings.TrimSpace(a.Shortcut); k != "" {
			m["act:"+a.ID] = k
		}
	}
	return m
}

// registerShortcuts binds the shortcuts, each running its item on Finder's
// selection. It returns why those that failed did, by menu id.
func (s *menubloom) registerShortcuts(keys map[string]string) map[string]string {
	mygo.GlobalShortcut.UnregisterAll()
	failed := map[string]string{}
	for _, id := range slices.Sorted(maps.Keys(keys)) {
		if mygo.GlobalShortcut.IsRegistered(keys[id]) {
			failed[id] = T("Another item has this shortcut.")
			continue
		}
		if err := mygo.GlobalShortcut.Register(keys[id], func() { s.runOnFinder(id) }); err != nil {
			failed[id] = err.Error()
		}
	}
	return failed
}

func (s *menubloom) runOnFinder(id string) {
	go func() {
		target, paths, err := finderSelection()
		mygo.RunOnMain(func() {
			if err != nil {
				s.notify(T("Couldn't read Finder's selection"), err.Error())
				return
			}
			s.handle(Request{ID: id, Target: target, Paths: paths})
		})
	}()
}
