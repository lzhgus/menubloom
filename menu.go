package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Entry is an item of the menu the extension draws (extension/FinderSync.swift).
type Entry struct {
	ID        string   `json:"id,omitempty"`
	Title     string   `json:"title,omitempty"`
	Symbol    string   `json:"symbol,omitempty"`
	App       string   `json:"app,omitempty"`
	Separator bool     `json:"separator,omitempty"`
	Contexts  []string `json:"contexts,omitempty"`
	Kinds     string   `json:"kinds,omitempty"` // "files" or "folders": every selected item is one
	Exts      []string `json:"exts,omitempty"`
	Count     string   `json:"count,omitempty"` // "one" or "many" selected items
	Under     []string `json:"under,omitempty"` // globs one of which the folder's path matches
	Info      string   `json:"info,omitempty"`  // the extension adds the item's "size", "created" or "modified"
	Children  []Entry  `json:"children,omitempty"`
}

// Where Finder shows a menu: on selected items, on a folder's background,
// on the sidebar, and the toolbar button's.
var (
	onItems      = []string{"items", "sidebar"}
	onBackground = []string{"container", "toolbar"}
	anywhere     = []string{"items", "container", "sidebar", "toolbar"}
)

var imageExts = []string{"png", "jpg", "jpeg", "heic", "heif", "tif", "tiff", "gif", "bmp", "webp"}

// State is what the menu shows besides the configuration.
type State struct {
	Templates   []string // the file names in the Templates folder
	Cut         []string // paths waiting for Paste
	ShowsHidden bool     // Finder shows hidden files
}

var sep = Entry{Separator: true}

// sections put separators between groups of different kinds, wherever
// the user moves them.
var sections = map[string]int{
	GroupNewFile: 0, GroupCopyPath: 0, GroupCopyInfo: 0, GroupOpenIn: 0, GroupInfo: 0,
	GroupCutPaste: 1, GroupCopyTo: 1, GroupMoveTo: 1, GroupFiles: 1, GroupImage: 1,
	GroupActions: 2, GroupHidden: 3,
}

// buildMenu turns the configuration into the extension's menu, its groups
// in the user's order. Ids name what to do and are stable across edits:
// "new:<template>", "open:<app path>", "act:<action id>"…
func buildMenu(c Config, s State) []Entry {
	var m []Entry
	last := -1
	for _, g := range c.Ordered() {
		if !c.On(g) {
			continue
		}
		es := groupEntries(g, c, s)
		if len(es) == 0 {
			continue
		}
		if c.Inline(g) {
			es = inline(es)
		}
		if last >= 0 && sections[g] != last {
			m = append(m, sep)
		}
		last = sections[g]
		m = append(m, es...)
	}
	return m
}

// inline puts the items of submenus at the top level, between separators,
// each keeping the conditions of its submenu.
func inline(es []Entry) []Entry {
	var out []Entry
	for _, e := range es {
		if e.Children == nil {
			out = append(out, e)
			continue
		}
		out = append(out, sep)
		for _, k := range e.Children {
			if k.Contexts == nil {
				k.Contexts = e.Contexts
			}
			if k.Kinds == "" {
				k.Kinds = e.Kinds
			}
			if k.Exts == nil {
				k.Exts = e.Exts
			}
			if k.Count == "" {
				k.Count = e.Count
			}
			if k.Symbol == "" && k.App == "" && !k.Separator {
				k.Symbol = e.Symbol
			}
			out = append(out, k)
		}
		out = append(out, sep)
	}
	return out
}

func groupEntries(g string, c Config, s State) []Entry {
	switch g {
	case GroupNewFile:
		if len(s.Templates) == 0 {
			return nil
		}
		var kids []Entry
		for _, t := range s.Templates {
			kids = append(kids, Entry{ID: "new:" + t, Title: strings.TrimSuffix(t, filepath.Ext(t)), Symbol: symbolFor(t)})
		}
		kids = append(kids, sep, Entry{ID: "templates", Title: T("Edit Templates…"), Symbol: "folder.badge.gearshape"})
		return []Entry{{Title: T("New File"), Symbol: "doc.badge.plus", Contexts: anywhere, Children: kids}}
	case GroupCutPaste:
		var es []Entry
		if len(s.Cut) > 0 {
			title := T("Paste Item Here")
			if len(s.Cut) > 1 {
				title = fmt.Sprintf(T("Paste %d Items Here"), len(s.Cut))
			}
			es = append(es, Entry{ID: "paste", Title: title, Symbol: "doc.on.clipboard", Contexts: anywhere})
		}
		return append(es, Entry{ID: "cut", Title: T("Cut"), Symbol: "scissors", Contexts: []string{"items"}})
	case GroupCopyPath:
		return []Entry{{ID: "copy:path", Title: T("Copy Path"), Symbol: "link", Contexts: anywhere}}
	case GroupCopyInfo:
		return []Entry{{Title: T("Copy As"), Symbol: "doc.on.doc", Contexts: anywhere, Children: []Entry{
			{ID: "copy:name", Title: T("Name")},
			{ID: "copy:stem", Title: T("Name Without Extension")},
			{ID: "copy:tilde", Title: T("~ Path")},
			{ID: "copy:quoted", Title: T("Quoted Path (for the shell)")},
			{ID: "copy:url", Title: T("File URL")},
			{ID: "copy:md", Title: T("Markdown Link")},
			sep,
			{ID: "copy:contents", Title: T("Contents"), Kinds: "files", Contexts: onItems},
			{ID: "copy:tree", Title: T("Folder Tree"), Kinds: "folders"},
			{Separator: true, Kinds: "files"},
			{ID: "hash:md5", Title: "MD5", Kinds: "files", Contexts: onItems},
			{ID: "hash:sha1", Title: "SHA-1", Kinds: "files", Contexts: onItems},
			{ID: "hash:sha256", Title: "SHA-256", Kinds: "files", Contexts: onItems},
		}}}
	case GroupInfo:
		return []Entry{{Title: T("Info"), Symbol: "info.circle", Contexts: onItems, Count: "one", Children: []Entry{
			{ID: "info:size", Title: T("Size"), Info: "size"},
			{ID: "info:created", Title: T("Created"), Info: "created"},
			{ID: "info:modified", Title: T("Modified"), Info: "modified"},
			{ID: "info:dimensions", Title: T("Dimensions"), Kinds: "files", Exts: imageExts},
		}}}
	case GroupOpenIn:
		var kids []Entry
		for _, a := range c.Apps {
			if !a.Off {
				kids = append(kids, Entry{ID: "open:" + a.Path, Title: a.Name, App: a.Path})
			}
		}
		if len(kids) == 0 {
			return nil
		}
		return []Entry{{Title: T("Open In"), Symbol: "terminal", Contexts: anywhere, Children: kids}}
	case GroupCopyTo, GroupMoveTo:
		verb, title, symbol := "copy", T("Copy To"), "doc.on.doc.fill"
		if g == GroupMoveTo {
			verb, title, symbol = "move", T("Move To"), "folder"
		}
		var kids []Entry
		for _, f := range c.Folders {
			kids = append(kids, Entry{ID: verb + "to:" + f, Title: filepath.Base(f), App: f})
		}
		kids = append(kids, sep, Entry{ID: verb + "to:", Title: T("Other Folder…")})
		return []Entry{{Title: title, Symbol: symbol, Contexts: []string{"items"}, Children: kids}}
	case GroupFiles:
		return []Entry{{Title: T("File Actions"), Symbol: "wand.and.stars", Contexts: onItems, Children: []Entry{
			{ID: "file:alias", Title: T("Make Alias")},
			{ID: "file:symlink", Title: T("Make Symlink")},
			{ID: "file:exec", Title: T("Make Executable"), Kinds: "files"},
			{ID: "file:unquarantine", Title: T("Remove Quarantine")},
			{ID: "file:dissolve", Title: T("Dissolve Folder"), Kinds: "folders"},
			sep,
			{ID: "file:delete", Title: T("Delete Immediately…")},
		}}}
	case GroupImage:
		img := func(e Entry) Entry {
			e.Contexts, e.Kinds, e.Exts = []string{"items"}, "files", imageExts
			return e
		}
		return []Entry{img(Entry{Title: T("Convert Image"), Symbol: "photo", Children: []Entry{
			{ID: "img:png", Title: "PNG"}, {ID: "img:jpeg", Title: "JPEG"}, {ID: "img:heic", Title: "HEIC"}, {ID: "img:tiff", Title: "TIFF"}, {ID: "img:icns", Title: T("ICNS (app icon)")},
		}}), img(Entry{Title: T("Resize Image"), Symbol: "arrow.up.left.and.arrow.down.right", Children: []Entry{
			{ID: "resize:2048", Title: "2048 px"}, {ID: "resize:1024", Title: "1024 px"}, {ID: "resize:512", Title: "512 px"}, {ID: "resize:256", Title: "256 px"},
		}})}
	case GroupActions:
		var es []Entry
		for _, a := range c.Actions {
			es = append(es, actionEntry(a))
		}
		return es
	case GroupHidden:
		title := T("Show Hidden Files")
		if s.ShowsHidden {
			title = T("Hide Hidden Files")
		}
		return []Entry{{ID: "hidden", Title: title, Symbol: "eye", Contexts: onBackground}}
	}
	return nil
}

// actionEntry is a custom action's item, shown where its rules say.
func actionEntry(a Action) Entry {
	e := Entry{ID: "act:" + a.ID, Title: a.Name, Symbol: "bolt", Exts: a.Exts, Under: expandGlobs(a.Under)}
	switch a.Scope {
	case Scopes[1]:
		e.Contexts, e.Kinds = []string{"items"}, "files"
	case Scopes[2]:
		e.Contexts, e.Kinds = onItems, "folders"
	case Scopes[3]:
		e.Contexts = onBackground
	default:
		e.Contexts = onItems
	}
	if len(a.Exts) > 0 {
		e.Kinds = "files"
	}
	switch a.Count {
	case Counts[1]:
		e.Count = "one"
	case Counts[2]:
		e.Count = "many"
	}
	return e
}

// expandGlobs turns "~/code, ~/work/*" into absolute globs; a folder also
// matches what it holds.
func expandGlobs(text string) []string {
	home, _ := os.UserHomeDir()
	var globs []string
	for _, g := range strings.Split(text, ",") {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if g == "~" || strings.HasPrefix(g, "~/") {
			g = home + g[1:]
		}
		g = strings.TrimSuffix(g, "/")
		globs = append(globs, g)
		if !strings.HasSuffix(g, "*") {
			globs = append(globs, g+"/*")
		}
	}
	return globs
}

func symbolFor(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".txt", ".rtf":
		return "doc.text"
	case ".docx", ".doc", ".pages":
		return "doc.richtext"
	case ".xlsx", ".csv", ".numbers":
		return "tablecells"
	case ".pptx", ".key":
		return "rectangle.on.rectangle"
	case ".py", ".sh", ".js", ".ts", ".go", ".html", ".json", ".swift":
		return "chevron.left.forwardslash.chevron.right"
	}
	return "doc"
}

func writeMenu(dir, token string, items []Entry) error {
	b, err := json.Marshal(struct {
		Token string  `json:"token"`
		Items []Entry `json:"items"`
	}{token, items})
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "menu.json"), b)
}

// listTemplates returns the files of the Templates folder: any file
// dropped there becomes a kind of New File.
func listTemplates(dir string) []string {
	des, _ := os.ReadDir(dir)
	var names []string
	for _, de := range des {
		if !strings.HasPrefix(de.Name(), ".") && !strings.HasSuffix(de.Name(), ".tmp") {
			names = append(names, de.Name())
		}
	}
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return names
}

// seedTemplates fills a new Templates folder; one that exists is the user's.
func seedTemplates(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]string{
		"Text Document.txt": "",
		"Markdown.md":       "",
		"Rich Text.rtf":     "{\\rtf1\\ansi\\deff0 }\n",
		"HTML.html":         "<!doctype html>\n<html>\n<head>\n  <meta charset=\"utf-8\">\n  <title></title>\n</head>\n<body>\n\n</body>\n</html>\n",
		"JSON.json":         "{}\n",
		"CSV.csv":           "",
		"Python.py":         "#!/usr/bin/env python3\n",
		"Shell Script.sh":   "#!/bin/sh\n",
		"Word Document.docx": ooxml(map[string]string{
			"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
			"_rels/.rels":         rels("http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument", "word/document.xml"),
			"word/document.xml":   `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p/></w:body></w:document>`,
		}),
		"Excel Workbook.xlsx": ooxml(map[string]string{
			"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
			"_rels/.rels":                rels("http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument", "xl/workbook.xml"),
			"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
			"xl/_rels/workbook.xml.rels": rels("http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet", "worksheets/sheet1.xml"),
			"xl/worksheets/sheet1.xml":   `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`,
		}),
	}
	for name, body := range files {
		name = T(strings.TrimSuffix(name, filepath.Ext(name))) + filepath.Ext(name) // in the user's language
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".py") {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), mode); err != nil {
			return err
		}
	}
	return nil
}

func rels(typ, target string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="` + typ + `" Target="` + target + `"/></Relationships>`
}

// ooxml zips the parts of an empty Office document.
func ooxml(parts map[string]string) string {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	names := make([]string, 0, len(parts))
	for n := range parts {
		names = append(names, n)
	}
	slices.Sort(names) // [Content_Types].xml first, as Office expects
	for _, n := range names {
		w, _ := z.Create(n)
		w.Write([]byte(parts[n]))
	}
	z.Close()
	return buf.String()
}
