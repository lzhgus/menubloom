package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The groups of built-in items, which Settings turns on and off.
const (
	GroupNewFile  = "newfile"
	GroupCopyPath = "copypath"
	GroupCopyInfo = "copyinfo"
	GroupOpenIn   = "openin"
	GroupCutPaste = "cutpaste"
	GroupCopyTo   = "copyto"
	GroupMoveTo   = "moveto"
	GroupImage    = "image"
	GroupFiles    = "files"
	GroupInfo     = "info"
	GroupHidden   = "hidden"
	GroupActions  = "actions"
)

// Groups lists them in the order Settings shows them.
var Groups = []struct{ ID, Title, Detail string }{
	{GroupNewFile, "New File", "Create a file from a template in the current folder"},
	{GroupCopyPath, "Copy Path", "Copy the full path of the selection"},
	{GroupCopyInfo, "Copy As", "Name, ~ path, quoted path, file URL, Markdown link, contents, folder tree, MD5, SHA-1, SHA-256"},
	{GroupInfo, "Info", "Size, created and modified dates, image dimensions; click one to copy it"},
	{GroupOpenIn, "Open In", "Open the selection or folder in a terminal or editor"},
	{GroupCutPaste, "Cut & Paste", "Cut files, then move them with Paste in another folder"},
	{GroupCopyTo, "Copy To", "Copy the selection to a favorite folder"},
	{GroupMoveTo, "Move To", "Move the selection to a favorite folder"},
	{GroupFiles, "File Actions", "Make alias or symlink, make executable, remove quarantine, dissolve folder, delete immediately"},
	{GroupImage, "Images", "Convert to PNG, JPEG, HEIC, TIFF or ICNS; resize"},
	{GroupHidden, "Show Hidden Files", "Toggle hidden files in Finder"},
	{GroupActions, "Custom Actions", "Your own shell commands"},
}

// App is an app of Open In.
type App struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Terminal bool   `json:"terminal,omitempty"` // opens folders: a file opens its folder
	Off      bool   `json:"off,omitempty"`
}

// Action is a custom action: a shell command run with the selected paths
// as its arguments ("$@"), never pasted into the command's text.
type Action struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Scope   string   `json:"scope"` // Scopes
	Exts    []string `json:"exts,omitempty"`
	// Terminal runs it in a Terminal window instead of in the background.
	Terminal bool   `json:"terminal,omitempty"`
	Count    string `json:"count,omitempty"`    // Counts
	Under    string `json:"under,omitempty"`    // folders, as globs: "~/code, ~/work/*"
	Shortcut string `json:"shortcut,omitempty"` // a global shortcut, e.g. "Cmd+Shift+G"
}

// Counts are how many selected items a custom action needs.
var Counts = []string{"Any number", "One item", "Several items"}

// Scopes of custom actions: where they show.
var Scopes = []string{"Files and folders", "Files", "Folders", "Folder background"}

// Config is what Settings changes, in config.json.
type Config struct {
	Off        []string `json:"off,omitempty"` // groups turned off
	Apps       []App    `json:"apps"`
	Folders    []string `json:"folders"`
	Actions    []Action `json:"actions"`
	Order      []string `json:"order,omitempty"`      // groups, in the user's order
	Inlined    []string `json:"inline,omitempty"`     // groups whose submenus show at the top level
	Language   string   `json:"language,omitempty"`   // Languages; empty is the system's
	Appearance string   `json:"appearance,omitempty"` // Appearances; empty is the system's
	Palette    string   `json:"palette,omitempty"`    // a Palette's name
	// AskedLogin: the app asked once whether to open at login.
	AskedLogin bool `json:"askedLogin,omitempty"`
}

// Ordered returns the groups in the user's order, then those added since.
func (c *Config) Ordered() []string {
	var out []string
	for _, g := range c.Order {
		if slices.ContainsFunc(Groups, func(x struct{ ID, Title, Detail string }) bool { return x.ID == g }) && !slices.Contains(out, g) {
			out = append(out, g)
		}
	}
	for _, g := range Groups {
		if !slices.Contains(out, g.ID) {
			out = append(out, g.ID)
		}
	}
	return out
}

// Move moves a group up (-1) or down (+1).
func (c *Config) Move(group string, by int) {
	o := c.Ordered()
	i := slices.Index(o, group)
	if j := i + by; i >= 0 && j >= 0 && j < len(o) {
		o[i], o[j] = o[j], o[i]
	}
	c.Order = o
}

func (c *Config) Inline(group string) bool { return slices.Contains(c.Inlined, group) }

func (c *Config) SetInline(group string, on bool) {
	c.Inlined = slices.DeleteFunc(c.Inlined, func(g string) bool { return g == group })
	if on {
		c.Inlined = append(c.Inlined, group)
	}
}

func (c *Config) On(group string) bool { return !slices.Contains(c.Off, group) }

func (c *Config) SetOn(group string, on bool) {
	c.Off = slices.DeleteFunc(c.Off, func(g string) bool { return g == group })
	if !on {
		c.Off = append(c.Off, group)
	}
}

// dataDir is where the app keeps its files, the one the Finder extension's
// sandbox may read. It is fixed, not App.Path's, so that development builds
// and the extension agree on it.
func dataDir() string {
	if d := os.Getenv("MENUBLOOM_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Menubloom")
}

func loadConfig(dir string) (Config, error) {
	var c Config
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if errors.Is(err, fs.ErrNotExist) {
		home, _ := os.UserHomeDir()
		for _, d := range []string{"Desktop", "Documents", "Downloads"} {
			c.Folders = append(c.Folders, filepath.Join(home, d))
		}
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

func saveConfig(dir string, c Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "config.json"), b)
}

// writeFile replaces a file whole, so that the extension never reads half
// of one.
func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// loadToken returns the secret that the extension's requests carry, made
// once: any web page can open a menubloom:// URL, only the extension can
// read the token.
func loadToken(dir string) (string, error) {
	path := filepath.Join(dir, "token")
	if b, err := os.ReadFile(path); err == nil && len(b) >= 32 {
		return strings.TrimSpace(string(b)), nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	t := hex.EncodeToString(b)
	return t, writeFile(path, []byte(t))
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// knownApps are the terminals and editors that Open In offers when they are
// installed.
var knownApps = []App{
	{Name: "Terminal", Path: "/System/Applications/Utilities/Terminal.app", Terminal: true},
	{Name: "iTerm", Path: "/Applications/iTerm.app", Terminal: true},
	{Name: "Ghostty", Path: "/Applications/Ghostty.app", Terminal: true},
	{Name: "Warp", Path: "/Applications/Warp.app", Terminal: true},
	{Name: "WezTerm", Path: "/Applications/WezTerm.app", Terminal: true},
	{Name: "kitty", Path: "/Applications/kitty.app", Terminal: true},
	{Name: "Alacritty", Path: "/Applications/Alacritty.app", Terminal: true},
	{Name: "Visual Studio Code", Path: "/Applications/Visual Studio Code.app"},
	{Name: "Cursor", Path: "/Applications/Cursor.app"},
	{Name: "Windsurf", Path: "/Applications/Windsurf.app"},
	{Name: "Zed", Path: "/Applications/Zed.app"},
	{Name: "Sublime Text", Path: "/Applications/Sublime Text.app"},
	{Name: "Nova", Path: "/Applications/Nova.app"},
	{Name: "BBEdit", Path: "/Applications/BBEdit.app"},
	{Name: "Xcode", Path: "/Applications/Xcode.app", Off: true},
	{Name: "TextEdit", Path: "/System/Applications/TextEdit.app", Off: true},
}

// detectApps adds the known apps installed since the list was made.
func detectApps(apps []App) []App {
	for _, k := range knownApps {
		if slices.ContainsFunc(apps, func(a App) bool { return a.Path == k.Path }) {
			continue
		}
		if _, err := os.Stat(k.Path); err == nil {
			apps = append(apps, k)
		}
	}
	return apps
}
