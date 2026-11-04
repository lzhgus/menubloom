package main

import (
	"errors"
	"os/exec"
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

// registerShortcuts binds the custom actions' shortcuts, each running its
// action on Finder's selection. It returns why those that failed did.
func (s *menubloom) registerShortcuts() map[string]string {
	mygo.GlobalShortcut.UnregisterAll()
	failed := map[string]string{}
	for _, a := range s.cfg.Actions {
		if a.Shortcut == "" {
			continue
		}
		id := "act:" + a.ID
		if err := mygo.GlobalShortcut.Register(a.Shortcut, func() { s.runOnFinder(id) }); err != nil {
			failed[a.ID] = err.Error()
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
