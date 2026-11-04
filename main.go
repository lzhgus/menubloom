// Menubloom adds what Finder's right-click lacks: New File from
// templates, Copy Path, Open in a terminal or editor, Cut & Paste, Copy To
// and Move To, image conversion, hidden files, and your own shell actions.
//
// The Finder Sync extension (extension/) draws the menu this app writes to
// menu.json, and sends each click back as a menubloom://run URL; this app
// runs the actions and shows Settings from its menu bar icon.
//
//	go run .   (the menu needs the packaged app: scripts/build.sh)
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const extensionID = "io.github.lzhgus.menubloom.finder"

// menubloom is the app's state, read and changed on the main thread.
type menubloom struct {
	dir, token string
	cfg        Config
	cut        []string
	hidden     bool
	templates  []string

	win      *mygo.Window
	tray     *mygo.Tray
	quitting bool
	gotURL   bool // launched by a click in the menu: no window then

	// Settings.
	page      string
	extStatus string // "on", "off" or "missing"
	login     bool
	extsText  map[string]string
	keysErr   map[string]string // why shortcuts failed, by action
	keys      string            // the shortcuts registered, to notice changes
	sk        Skin              // the palette, as the view draws it
}

func main() {
	mygo.App.SetName("Menubloom")
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	s := &menubloom{dir: dataDir(), page: "general", extsText: map[string]string{}}
	if err := s.load(); err != nil {
		log.Fatal(err)
	}
	// The command line, for scripts and Shortcuts: a running app gets the
	// arguments of later instances, without the program's name.
	mygo.App.OnSecondInstance(func(args []string, wd string) {
		if r, ok := parseCLI(args, wd); ok {
			s.handle(r)
		} else {
			s.showSettings()
		}
	})
	if wd, _ := os.Getwd(); len(os.Args) > 1 {
		if r, ok := parseCLI(os.Args[1:], wd); ok {
			s.gotURL = true
			mygo.App.WhenReady(func() { s.handle(r) })
		}
	}
	mygo.App.OnOpenURL(func(raw string) {
		s.gotURL = true
		mygo.App.WhenReady(func() { s.openURL(raw) })
	})
	mygo.App.OnWindowAllClosed(func() {}) // live in the menu bar
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { s.quitting = true })
	mygo.App.OnActivate(func(bool) { s.showSettings() }) // opened again from Finder
	mygo.App.WhenReady(s.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (s *menubloom) templatesDir() string { return filepath.Join(s.dir, "Templates") }

func (s *menubloom) load() error {
	var err error
	if s.cfg, err = loadConfig(s.dir); err != nil {
		return err
	}
	if s.token, err = loadToken(s.dir); err != nil {
		return err
	}
	setLanguage(s.cfg.Language, mygo.App.PreferredLanguages())
	if err := seedTemplates(s.templatesDir()); err != nil {
		return err
	}
	applyAppearance(s.cfg.Appearance)
	s.cfg.Apps = detectApps(s.cfg.Apps)
	s.templates = listTemplates(s.templatesDir())
	s.hidden = showsHidden()
	s.refreshMenu()
	return nil
}

// refreshMenu writes the menu the extension shows from now on.
func (s *menubloom) refreshMenu() {
	m := buildMenu(s.cfg, State{Templates: s.templates, Cut: s.cut, ShowsHidden: s.hidden})
	if err := writeMenu(s.dir, s.token, m); err != nil {
		log.Println("menu:", err)
	}
}

func (s *menubloom) save() {
	if err := saveConfig(s.dir, s.cfg); err != nil {
		s.notify(T("Couldn't save settings"), err.Error())
	}
	s.refreshMenu()
	s.bindKeys()
}

// bindKeys registers the custom actions' shortcuts when they changed.
func (s *menubloom) bindKeys() {
	var keys []string
	for _, a := range s.cfg.Actions {
		keys = append(keys, a.ID+"="+a.Shortcut)
	}
	if k := strings.Join(keys, ","); k != s.keys {
		s.keys = k
		s.keysErr = s.registerShortcuts()
	}
}

func (s *menubloom) ready() {
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	s.login = mygo.App.OpenAtLogin()
	s.win = mygo.NewWindow(mygo.WindowOptions{
		Title:   "Menubloom",
		Width:   880,
		Height:  640,
		Hidden:  true,
		Content: ui.View(s.view),
	})
	s.win.OnClose(func(e *mygo.CloseEvent) {
		if !s.quitting {
			e.PreventDefault()
			s.win.Hide()
		}
	})
	var err error
	s.tray, err = mygo.NewTray(mygo.TrayOptions{
		ID:             "menubloom",
		Icon:           trayIcon(),
		IconIsTemplate: true,
		ToolTip:        "Menubloom",
		Menu:           s.trayMenu(),
	})
	if err != nil {
		log.Println("tray:", err)
	}
	registerServices(s.handle)
	s.bindKeys()
	go s.watchTemplates()
	// A launch by a click in the menu runs its action and stays out of the
	// way; any other launch, but at login, opens Settings.
	time.AfterFunc(700*time.Millisecond, func() {
		mygo.RunOnMain(func() {
			if !s.gotURL && !mygo.App.WasOpenedAtLogin() {
				s.showSettings()
			}
			s.askOpenAtLogin()
		})
	})
}

// trayMenu is the menu bar icon's menu, in the app's language.
func (s *menubloom) trayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: T("Settings…"), Accelerator: "Cmd+,", Click: func(*mygo.MenuItem, *mygo.Window) { s.showSettings() }},
		{Label: T("Restart Finder"), Click: func(*mygo.MenuItem, *mygo.Window) { go exec.Command("/usr/bin/killall", "Finder").Run() }},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
}

// askOpenAtLogin asks once whether to open at login, which keeps the first
// click of a session from waiting for the app to start.
func (s *menubloom) askOpenAtLogin() {
	if s.cfg.AskedLogin || s.login {
		return
	}
	s.cfg.AskedLogin = true
	s.save()
	mygo.App.Focus()
	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Title:   "Menubloom",
		Message: T("Open Menubloom at login?"),
		Detail:  T("Menubloom runs the actions of its menu. Opening it at login makes the first right-click of the day as fast as the rest, and keeps Cut & Paste ready."),
		Buttons: []string{T("Open at Login"), T("Not Now")},
	})
	if err == nil && res.Button == 0 {
		if err := mygo.App.SetOpenAtLogin(true); err != nil {
			s.notify(T("Couldn't open at login"), err.Error())
		}
		s.login = mygo.App.OpenAtLogin()
		s.win.Invalidate()
	}
}

func (s *menubloom) showSettings() {
	if s.win == nil {
		return
	}
	go s.checkExtension()
	s.win.Show()
	s.win.Focus()
	mygo.App.Focus()
}

// watchTemplates notices files added to or removed from the Templates
// folder, by its modification time.
// ponytail: polls a stat every 2s; an FSEvents watch if it ever matters.
func (s *menubloom) watchTemplates() {
	var last time.Time
	for range time.Tick(2 * time.Second) {
		fi, err := os.Stat(s.templatesDir())
		if err != nil || fi.ModTime().Equal(last) {
			continue
		}
		first := last.IsZero()
		last = fi.ModTime()
		if first {
			continue
		}
		names := listTemplates(s.templatesDir())
		mygo.RunOnMain(func() {
			s.templates = names
			s.refreshMenu()
			s.win.Invalidate()
		})
	}
}

// checkExtension asks PlugInKit whether the Finder extension is enabled.
func (s *menubloom) checkExtension() {
	out, _ := exec.Command("/usr/bin/pluginkit", "-m", "-i", extensionID).Output()
	status := "missing"
	switch line := strings.TrimSpace(string(out)); {
	case strings.HasPrefix(line, "+"):
		status = "on"
	case line != "":
		status = "off"
	}
	s.win.Update(func() { s.extStatus = status })
}

func (s *menubloom) enableExtension() {
	go func() {
		exec.Command("/usr/bin/pluginkit", "-e", "use", "-i", extensionID).Run()
		s.checkExtension()
	}()
}

func (s *menubloom) notify(title, body string) {
	n := mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body, Silent: true})
	if err := n.Show(); err != nil {
		log.Printf("%s: %s", title, body)
	}
}

// openURL runs a click of the extension's menu.
func (s *menubloom) openURL(raw string) {
	req, err := parseRequest(raw, s.token)
	if err != nil {
		log.Println(err)
		return
	}
	s.handle(req)
}

// handle runs a request: quick ones here on the main thread, the rest on
// goroutines, which report back with notifications.
func (s *menubloom) handle(r Request) {
	kind, arg, _ := strings.Cut(r.ID, ":")
	items := r.items()
	switch kind {
	case "copy":
		switch arg {
		case "contents", "tree":
			go func() {
				text, err := "", error(nil)
				if arg == "tree" {
					text = tree(items)
				} else {
					text, err = contents(items)
				}
				mygo.RunOnMain(func() {
					if err != nil {
						s.notify(T("Couldn't copy"), err.Error())
						return
					}
					mygo.Clipboard.WriteText(text)
				})
			}()
		default:
			mygo.Clipboard.WriteText(copyAs(arg, items))
		}
	case "file":
		s.fileAction(arg, items)
	case "info":
		if len(items) != 1 {
			return
		}
		go func() {
			text, err := info(arg, items[0])
			mygo.RunOnMain(func() {
				if err != nil {
					s.notify(T("Couldn't copy"), err.Error())
					return
				}
				mygo.Clipboard.WriteText(text)
				s.notify(T("Copied"), text)
			})
		}()
	case "resize":
		size, err := strconv.Atoi(arg)
		if err != nil || size <= 0 {
			return
		}
		go func() {
			done, err := resizeImage(size, items)
			if len(done) > 0 {
				reveal(done...)
			}
			if err != nil {
				s.notify(T("Couldn't resize"), err.Error())
			}
		}()
	case "hash":
		go func() {
			sum, err := hashFiles(arg, items)
			mygo.RunOnMain(func() {
				if err != nil {
					s.notify(T("Couldn't hash"), err.Error())
					return
				}
				mygo.Clipboard.WriteText(sum)
				s.notify(T("Copied")+" "+strings.ToUpper(arg), sum)
			})
		}()
	case "new":
		if strings.HasPrefix(arg, ".") { // the services': the first template of the extension
			if i := slices.IndexFunc(s.templates, func(t string) bool { return strings.EqualFold(filepath.Ext(t), arg) }); i >= 0 {
				arg = s.templates[i]
			}
		}
		dir := r.folder()
		go func() {
			p, err := newFile(s.templatesDir(), arg, dir)
			if err != nil {
				s.notify(T("Couldn't create the file"), err.Error())
				return
			}
			reveal(p)
		}()
	case "templates":
		go exec.Command("/usr/bin/open", s.templatesDir()).Run()
	case "cut":
		s.cut = items
		s.refreshMenu()
	case "paste":
		cut, dir := s.cut, r.folder()
		s.cut = nil
		s.refreshMenu()
		go s.transfer(cut, dir, true)
	case "copyto", "moveto":
		dir := arg
		if dir == "" {
			mygo.App.Focus()
			dirs, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
				Title:             T("Choose a Folder"),
				ButtonLabel:       map[string]string{"copyto": T("Copy Here"), "moveto": T("Move Here")}[kind],
				Directory:         true,
				CreateDirectories: true,
			})
			if err != nil || len(dirs) == 0 {
				return
			}
			dir = dirs[0]
		}
		go s.transfer(items, dir, kind == "moveto")
	case "img":
		go func() {
			done, err := convertImage(arg, items)
			if len(done) > 0 {
				reveal(done...)
			}
			if err != nil {
				s.notify(T("Couldn't convert"), err.Error())
			}
		}()
	case "open":
		apps := pickApps(s.cfg.Apps, arg)
		if len(apps) == 0 {
			s.notify(T("No app to open with"), "Turn one on in Menubloom ▸ Settings ▸ Open In.")
		}
		for _, a := range apps {
			go func() {
				if err := openIn(a, items); err != nil {
					s.notify(T("Couldn't open"), err.Error())
				}
			}()
		}
	case "act":
		for _, a := range s.cfg.Actions {
			if a.ID != arg {
				continue
			}
			dir := r.Target
			if dir == "" && len(items) > 0 {
				dir = filepath.Dir(items[0])
			}
			if a.Terminal {
				go func() {
					if err := runInTerminal(a, dir, items); err != nil {
						s.notify(T("Action failed"), err.Error())
					}
				}()
				continue
			}
			go func() {
				out, err := runAction(a, dir, items)
				switch {
				case err != nil:
					s.notify(T("Action failed"), err.Error())
				case out != "":
					s.notify(a.Name, out)
				}
			}()
		}
	case "hidden":
		on := !s.hidden
		go func() {
			if err := setShowsHidden(on); err != nil {
				s.notify(T("Couldn't change hidden files"), err.Error())
				return
			}
			mygo.RunOnMain(func() {
				s.hidden = on
				s.refreshMenu()
			})
		}()
	default:
		log.Println("unknown action:", r.ID)
	}
}

// fileAction runs an item of File Actions; Delete Immediately asks first.
func (s *menubloom) fileAction(kind string, items []string) {
	report := func(done []string, err error) {
		if len(done) > 0 {
			reveal(done...)
		}
		if err != nil {
			s.notify(T("Couldn't finish"), err.Error())
		}
	}
	background := func(f func() ([]string, error)) { go func() { report(f()) }() }
	switch kind {
	case "alias":
		report(makeAlias(items)) // NSURL: here, on the main thread
	case "symlink":
		background(func() ([]string, error) { return symlink(items) })
	case "exec":
		background(func() ([]string, error) { return nil, makeExecutable(items) })
	case "unquarantine":
		background(func() ([]string, error) { return nil, unquarantine(items) })
	case "dissolve":
		background(func() ([]string, error) { return dissolve(items) })
	case "delete":
		what := fmt.Sprintf(T("%d items"), len(items))
		if len(items) == 1 {
			what = "“" + filepath.Base(items[0]) + "”"
		}
		mygo.App.Focus()
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Type:               mygo.MessageWarning,
			Title:              "Menubloom",
			Message:            fmt.Sprintf(T("Delete %s immediately?"), what),
			Detail:             T("It won't go to the Trash, and this can't be undone."),
			Buttons:            []string{T("Delete"), T("Cancel")},
			DefaultButton:      1,
			CancelButton:       1,
			DestructiveButtons: []int{0},
		})
		if err != nil || res.Button != 0 {
			return
		}
		background(func() ([]string, error) {
			for _, p := range items {
				if err := os.RemoveAll(p); err != nil {
					return nil, err
				}
			}
			return nil, nil
		})
	}
}

func (s *menubloom) transfer(paths []string, dir string, move bool) {
	done, err := transfer(paths, dir, move)
	if len(done) > 0 {
		reveal(done...)
	}
	if err != nil {
		title := T("Couldn't copy")
		if move {
			title = T("Couldn't move")
		}
		s.notify(title, err.Error())
	}
}
