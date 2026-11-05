package main

import (
	_ "embed"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
)

//go:embed assets/icon.png
var iconPNG []byte

var appIcon, _ = ui.DecodeBitmap(iconPNG)

func (s *menubloom) view(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyW) {
		s.win.Hide()
	}
	s.sk = paletteNamed(s.cfg.Palette).skin(mygo.Theme.IsDark())
	c.SetTheme(s.sk.Theme)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		s.sidebar(c)
		ui.Scroll(c.Key(s.page)).Grow(1).Padding(26, 30, 30, 30).Gap(16).Children(func() {
			switch s.page {
			case "appearance":
				s.appearancePage(c)
			case "menu":
				s.menuPage(c)
			case "newfile":
				s.newFilePage(c)
			case "openin":
				s.openInPage(c)
			case "folders":
				s.foldersPage(c)
			case "actions":
				s.actionsPage(c)
			default:
				s.generalPage(c)
			}
		})
	})
}

func (s *menubloom) sidebar(c *ui.Context) {
	sk := s.sk
	ui.Column(c).Width(214).Background(sk.Sidebar).Padding(18, 12).Gap(2).Children(func() {
		ui.Row(c).Gap(10).Padding(2, 8, 16, 8).AlignItems(ui.Center).Children(func() {
			ui.Image(c, appIcon).Size(34, 34)
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "Menubloom").Bold().FontSize(14)
				ui.Text(c, "v"+mygo.App.Version()).FontSize(11).TextColor(sk.TextMuted)
			})
		})
		for _, p := range panes {
			on := s.page == p.ID
			b := ui.ButtonBase(c.Key(p.ID)).Padding(5, 8).Radius(9).Gap(10).Justify(ui.Start).Label(T(p.Title))
			if on {
				b.Background(sk.Card).Shadow(0, 1, 3, 0, sk.Shadow)
			} else if b.Hovered() {
				b.Background(sk.Card.Alpha(0.5))
			}
			if b.Pressed() {
				b.Scale(0.97, 0.97)
			}
			b.Children(func() {
				badge(c, p.Glyph, p.Hue, p.C, p.L, 22)
				t := ui.Text(c, T(p.Title)).FontSize(13)
				if on {
					t.Bold()
				}
			})
			if b.Clicked() {
				s.page = p.ID
			}
		}
	})
}

// header is a page's title and what it's for.
func header(c *ui.Context, title, detail string) {
	ui.Column(c).Gap(4).Padding(0, 2, 4, 2).Children(func() {
		ui.Text(c, title).FontSize(24).Bold()
		if detail != "" {
			ui.Text(c, detail).FontSize(12.5).TextColor(c.Theme().TextMuted)
		}
	})
}

func label(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(12).Bold().TextColor(c.Theme().TextMuted).Padding(6, 4, 0, 4)
}

func muted(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(12).TextColor(c.Theme().TextMuted)
}

// card holds rows on a raised surface, divided by hairlines.
func (s *menubloom) card(c *ui.Context, key string, fn func()) {
	ui.Column(c.Key(key)).Padding(2, 16).Radius(12).Background(s.sk.Card).
		Shadow(0, 1, 3, 0, s.sk.Shadow).Gap(1).Dividers(1, s.sk.Divider).Children(fn)
}

// row is a setting: its title and description, and its controls at the end.
func row(c *ui.Context, key, title, detail string, controls func()) {
	ui.Row(c.Key(key)).Padding(11, 0).Gap(14).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, title)
			if detail != "" {
				muted(c, detail)
			}
		})
		if controls != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(controls)
		}
	})
}

// step is a row of the setup checklist, marked done or not.
func step(c *ui.Context, key string, done bool, title, detail string, controls func()) {
	ui.Row(c.Key(key)).Gap(12).AlignItems(ui.Center).Children(func() {
		t := c.Theme()
		mark := ui.Column(c).Size(20, 20).Radius(10).Center()
		if done {
			mark.Background(t.Success).Children(func() {
				ui.Text(c, "✓").FontSize(12).Bold().TextColor(ui.Hex("#ffffff"))
			})
		} else {
			mark.Border(1.5, t.Border)
		}
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Children(func() {
			row(c, key, title, detail, controls)
		})
	})
}

// selectT is a Select of options shown translated, storing the English.
func selectT(c *ui.Context, value *string, options []string) bool {
	shown := make([]string, len(options))
	for i, o := range options {
		shown[i] = T(o)
	}
	cur := shown[max(0, slices.Index(options, *value))]
	if ui.Select(c, &cur, shown).Changed() {
		if i := slices.Index(shown, cur); i >= 0 {
			*value = options[i]
			return true
		}
	}
	return false
}

func open(args ...string) { go exec.Command("/usr/bin/open", args...).Run() }

func (s *menubloom) generalPage(c *ui.Context) {
	header(c, T("General"), T("Everything Finder's right-click is missing. Three steps to set it up:"))
	s.card(c, "setup", func() {
		extDetail := map[string]string{
			"on":      T("On. Right-click in any Finder window; the first right-click after Finder starts may come before it has loaded."),
			"off":     T("Off: turn it on to see Menubloom's items in Finder."),
			"missing": T("Not installed: install the packaged app (scripts/build.sh --install)."),
			"":        T("Checking…"),
		}[s.extStatus]
		step(c, "ext", s.extStatus == "on", T("Finder extension"), extDetail, func() {
			if s.extStatus != "on" {
				if ui.PrimaryButton(c, T("Turn On")).Disabled(s.extStatus == "missing").Clicked() {
					s.enableExtension()
				}
			}
			if ui.Button(c, T("Restart Finder")).Clicked() {
				go exec.Command("/usr/bin/killall", "Finder").Run()
			}
		})
		step(c, "login", s.login, T("Open at login"), T("Makes the first right-click of the day as fast as the rest, and keeps Cut & Paste ready."), func() {
			if ui.Switch(c, &s.login).Label(T("Open at login")).Changed() {
				if err := mygo.App.SetOpenAtLogin(s.login); err != nil {
					s.notify(T("Couldn't change Open at login"), err.Error())
					s.login = mygo.App.OpenAtLogin()
				}
			}
		})
		step(c, "cloud", true, T("iCloud Drive, Dropbox and OneDrive"),
			T("Finder shows no extension menus in cloud folders. There, use right-click ▸ Services ▸ Menubloom: …; if they're missing, turn them on in Keyboard Shortcuts ▸ Services ▸ Files and Folders."), func() {
				if ui.Button(c, T("Keyboard Shortcuts")).Clicked() {
					open("x-apple.systempreferences:com.apple.Keyboard-Settings.extension")
				}
			})
	})
	label(c, T("Preferences"))
	s.card(c, "prefs", func() {
		row(c, "lang", T("Language"), T("Of Settings and the right-click menu."), func() {
			if selectT(c, &s.cfg.Language, Languages) {
				setLanguage(s.cfg.Language, mygo.App.PreferredLanguages())
				s.save()
				if s.tray != nil {
					s.tray.SetMenu(s.trayMenu())
				}
			}
		})
		row(c, "toolbar", T("Toolbar button"), T("Add Menubloom to Finder's toolbar with View ▸ Customize Toolbar…, for its menu in any window."), nil)
		row(c, "updates", T("Updates"), T("Checks once a day and asks before installing."), func() {
			auto := updater.AutomaticChecks()
			if ui.Switch(c, &auto).Label(T("Check automatically")).Changed() {
				updater.SetAutomaticChecks(auto)
			}
			if ui.Button(c, T("Check Now")).Clicked() {
				updater.CheckForUpdates()
			}
		})
		row(c, "data", T("Data folder"), s.dir, func() {
			if ui.Button(c, T("Show")).Clicked() {
				open(s.dir)
			}
		})
	})
}

func (s *menubloom) appearancePage(c *ui.Context) {
	header(c, T("Appearance"), T("How Settings looks. The right-click menu is Finder's own."))
	s.card(c, "mode", func() {
		row(c, "appearance", T("Appearance"), "", func() {
			i := max(0, slices.Index(Appearances, s.cfg.Appearance))
			if ui.Segmented(c, &i, T("System"), T("Light"), T("Dark")).Label(T("Appearance")).Changed() {
				s.cfg.Appearance = Appearances[i]
				applyAppearance(s.cfg.Appearance)
				s.save()
			}
		})
	})
	label(c, T("Theme"))
	dark := mygo.Theme.IsDark()
	current := paletteNamed(s.cfg.Palette).Name
	ui.Row(c).Wrap().Gap(14).Children(func() {
		for _, p := range Palettes {
			if s.swatch(c, p, current == p.Name, dark) {
				s.cfg.Palette = p.Name
				s.save()
			}
		}
	})
}

// swatch previews a palette as a tiny window: its tint, a card with type
// and its accent, and its accent's shades as dots.
func (s *menubloom) swatch(c *ui.Context, p Palette, on, dark bool) bool {
	sk := p.skin(dark)
	b := ui.ButtonBase(c.Key(p.Name)).Column().Gap(7).Width(132).Label(p.Name)
	if b.Pressed() {
		b.Scale(0.97, 0.97)
	}
	b.Children(func() {
		box := ui.Column(c).Width(132).Height(84).Radius(14).Padding(10).Gap(10).
			LinearGradient(ui.LinearGradient{From: sk.Background, To: sk.Sidebar, Angle: 160, Oklab: true})
		if on {
			box.Border(2, s.sk.Accent)
		} else {
			box.Border(1, sk.Border.Alpha(0.6))
		}
		box.Children(func() {
			ui.Row(c).Padding(5, 8).Radius(8).Background(sk.Card).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Aa").FontSize(14).TextColor(sk.Text)
				ui.Spacer(c)
				ui.Column(c).Size(18, 8).Radius(4).Background(sk.Accent)
			})
			ui.Row(c).Gap(4).Children(func() {
				for _, l := range []float32{0.3, 0.45, 0.56, 0.68, 0.8} {
					ui.Column(c).Size(9, 9).Radius(4.5).Background(ui.Oklch(l, max(p.Accent, 0.02), p.Hue))
				}
			})
		})
		t := ui.Text(c, p.Name).FontSize(12).TextAlign(ui.Center).Width(132)
		if on {
			t.Bold()
		}
	})
	return b.Clicked()
}

func (s *menubloom) menuPage(c *ui.Context) {
	header(c, T("Menu Items"), T("What the right-click menu shows, in this order. Changes apply at the next right-click."))
	order := s.cfg.Ordered()
	s.card(c, "groups", func() {
		for i, id := range order {
			g := Groups[slices.IndexFunc(Groups, func(x struct{ ID, Title, Detail string }) bool { return x.ID == id })]
			on := s.cfg.On(id)
			row(c, id, T(g.Title), T(g.Detail), func() {
				if hasSubmenu(id) {
					inl := s.cfg.Inline(id)
					if ui.Checkbox(c, &inl, T("Inline")).Tooltip(T("Show its items at the top level instead of in a submenu")).Changed() {
						s.cfg.SetInline(id, inl)
						s.save()
					}
				}
				if iconButton(c, glyphUp, T("Move up"), i == 0) {
					s.cfg.Move(id, -1)
					s.save()
				}
				if iconButton(c, glyphDown, T("Move down"), i == len(order)-1) {
					s.cfg.Move(id, 1)
					s.save()
				}
				if ui.Switch(c, &on).Label(T(g.Title)).Changed() {
					s.cfg.SetOn(id, on)
					s.save()
				}
			})
		}
	})
}

func hasSubmenu(group string) bool {
	switch group {
	case GroupCopyPath, GroupCutPaste, GroupActions, GroupHidden:
		return false
	}
	return true
}

func iconButton(c *ui.Context, g *ui.SVG, name string, disabled bool) bool {
	return ui.Button(c, "").Label(name).Tooltip(name).Disabled(disabled).Padding(4, 6).Children(func() {
		ui.Icon(c, g).FontSize(13)
	}).Clicked()
}

func (s *menubloom) newFilePage(c *ui.Context) {
	header(c, T("New File"), T("Every file in the Templates folder is a kind of New File, as Windows' New menu. Add your own: a Keynote theme, a license, a project's README."))
	s.card(c, "templates", func() {
		for _, t := range s.templates {
			row(c, t, strings.TrimSuffix(t, filepath.Ext(t)), t, nil)
		}
		if len(s.templates) == 0 {
			row(c, "empty", T("The Templates folder is empty."), "", nil)
		}
	})
	ui.Row(c).Children(func() {
		if ui.PrimaryButton(c, T("Open Templates Folder")).Clicked() {
			open(s.templatesDir())
		}
	})
}

func (s *menubloom) openInPage(c *ui.Context) {
	header(c, T("Open In"), T("Apps that open the selection, or the folder you're in. A terminal opens the folder of a file."))
	s.card(c, "apps", func() {
		for i := range s.cfg.Apps {
			a := &s.cfg.Apps[i]
			known := slices.ContainsFunc(knownApps, func(k App) bool { return k.Path == a.Path })
			row(c, a.Path, a.Name, a.Path, func() {
				if ui.Checkbox(c, &a.Terminal, T("Terminal")).Changed() {
					s.save()
				}
				if !known && ui.Button(c, T("Remove")).Clicked() {
					s.cfg.Apps = slices.Delete(s.cfg.Apps, i, i+1)
					s.save()
				}
				on := !a.Off
				if ui.Switch(c, &on).Label(a.Name).Changed() {
					a.Off = !on
					s.save()
				}
			})
		}
	})
	ui.Row(c).Children(func() {
		if ui.Button(c, T("Add App…")).Clicked() {
			paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
				Parent: s.win, Title: T("Add an App"), DefaultPath: "/Applications",
				Filters: []mygo.FileFilter{{Name: "Apps", Extensions: []string{"app"}}}, Multiple: true,
			})
			if err == nil {
				for _, p := range paths {
					if !slices.ContainsFunc(s.cfg.Apps, func(a App) bool { return a.Path == p }) {
						s.cfg.Apps = append(s.cfg.Apps, App{Name: strings.TrimSuffix(filepath.Base(p), ".app"), Path: p})
					}
				}
				s.save()
			}
		}
	})
}

func (s *menubloom) foldersPage(c *ui.Context) {
	header(c, T("Folders"), T("Where Copy To and Move To send the selection. Other Folder… asks each time."))
	s.card(c, "folders", func() {
		for i, f := range s.cfg.Folders {
			row(c, f, filepath.Base(f), f, func() {
				if ui.Button(c, T("Remove")).Clicked() {
					s.cfg.Folders = slices.Delete(s.cfg.Folders, i, i+1)
					s.save()
				}
			})
		}
		if len(s.cfg.Folders) == 0 {
			row(c, "empty", T("No folders yet."), "", nil)
		}
	})
	ui.Row(c).Children(func() {
		if ui.Button(c, T("Add Folder…")).Clicked() {
			dirs, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: s.win, Title: T("Add a Folder"), Directory: true, Multiple: true, CreateDirectories: true})
			if err == nil {
				for _, d := range dirs {
					if !slices.Contains(s.cfg.Folders, d) {
						s.cfg.Folders = append(s.cfg.Folders, d)
					}
				}
				s.save()
			}
		}
	})
}

func (s *menubloom) actionsPage(c *ui.Context) {
	header(c, T("Custom Actions"), T(`Shell commands of your own in the menu. They run in zsh, in the selection's folder, with the selected paths as "$@".`))
	for i := range s.cfg.Actions {
		a := &s.cfg.Actions[i]
		if _, ok := s.extsText[a.ID]; !ok {
			s.extsText[a.ID] = strings.Join(a.Exts, ", ")
		}
		s.card(c, a.ID, func() {
			ui.Column(c).Padding(12, 0).Gap(10).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					badge(c, glyphActions, 55, 0.17, 0.7, 26)
					if ui.TextInput(c, &a.Name).Grow(1).Label(T("Name")).Changed() {
						s.save()
					}
					if ui.Button(c, T("Delete")).Clicked() {
						s.cfg.Actions = slices.Delete(s.cfg.Actions, i, i+1)
						s.save()
					}
				})
				ui.Form(c, func() {
					ui.Field(c, T("Command"), func() {
						if ui.TextInput(c, &a.Command).Placeholder(`e.g. code "$@"`).Font("Menlo").Changed() {
							s.save()
						}
					})
					ui.Field(c, T("Shows on"), func() {
						if selectT(c, &a.Scope, Scopes) {
							s.save()
						}
					})
					ui.Field(c, T("Selection"), func() {
						if selectT(c, &a.Count, Counts) {
							s.save()
						}
					})
					ui.Field(c, T("Extensions"), func() {
						text := s.extsText[a.ID]
						if ui.TextInput(c, &text).Placeholder(T("any; or e.g. png, jpg")).Changed() {
							s.extsText[a.ID] = text
							a.Exts = parseExts(text)
							s.save()
						}
					})
					ui.Field(c, T("Only in"), func() {
						if ui.TextInput(c, &a.Under).Placeholder(T("any folder; or e.g. ~/code, ~/work/*")).Changed() {
							s.save()
						}
					})
					ui.Field(c, T("Shortcut"), func() {
						if ui.TextInput(c, &a.Shortcut).Placeholder(T("none; or e.g. Cmd+Shift+G")).Changed() {
							s.save()
						}
					}).Description(T("Runs it on Finder's selection from anywhere; asks once to control Finder.")).Error(s.keysErr[a.ID])
					ui.Field(c, T("Output"), func() {
						if ui.Checkbox(c, &a.Terminal, T("Run in a Terminal window")).Changed() {
							s.save()
						}
					}).Description(T("Off: runs in the background, and what it prints shows as a notification."))
				})
			})
		})
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		if ui.PrimaryButton(c, T("Add Action")).Clicked() {
			s.cfg.Actions = append(s.cfg.Actions, Action{ID: newID(), Name: T("New Action"), Command: `echo "$@"`, Scope: Scopes[0]})
			s.save()
		}
		for _, ex := range examples {
			if ui.Button(c, "+ "+T(ex.Name)).Clicked() {
				ex.ID, ex.Name = newID(), T(ex.Name)
				s.cfg.Actions = append(s.cfg.Actions, ex)
				s.save()
			}
		}
	})
}

// examples are custom actions one click away.
var examples = []Action{
	{Name: "Make Executable", Command: `chmod +x "$@"`, Scope: Scopes[1]},
	{Name: "Count Lines", Command: `wc -l "$@"`, Scope: Scopes[1]},
	{Name: "Git Status", Command: `git status --short --branch`, Scope: Scopes[3]},
	{Name: "Disk Usage", Command: `du -sh "$@"`, Scope: Scopes[0], Terminal: true},
}

func parseExts(text string) []string {
	var exts []string
	for _, f := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' }) {
		if f = strings.ToLower(strings.TrimPrefix(f, ".")); f != "" {
			exts = append(exts, f)
		}
	}
	return exts
}
