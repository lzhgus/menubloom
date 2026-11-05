package main

import (
	"fmt"
	"log"
	"net/url"
	"path/filepath"

	"github.com/ebitengine/purego/objc"
)

// The services of mygo.json's NSServices: Finder lists them under Services
// in the right-click menu, also in iCloud Drive, Dropbox and OneDrive
// folders, where it loads no Finder Sync extension. Each names its action
// in NSUserData, as the extension's ids do.

var (
	selUTF8        = objc.RegisterName("UTF8String")
	selItems       = objc.RegisterName("pasteboardItems")
	selCount       = objc.RegisterName("count")
	selObjectAt    = objc.RegisterName("objectAtIndex:")
	selStringFor   = objc.RegisterName("stringForType:")
	selURLWith     = objc.RegisterName("URLWithString:")
	selFilePathURL = objc.RegisterName("filePathURL")
	selPath        = objc.RegisterName("path")
	selStringWith  = objc.RegisterName("stringWithUTF8String:")
	selShared      = objc.RegisterName("sharedApplication")
	selSetProvider = objc.RegisterName("setServicesProvider:")
	selNew         = objc.RegisterName("new")
	selRunService  = objc.RegisterName("runService:userData:error:")
)

func goString(s objc.ID) string {
	if s == 0 {
		return ""
	}
	return objc.Send[string](s, selUTF8)
}

// pasteboardPaths returns the files on a services pasteboard. Finder may
// pass file reference URLs (file:///.file/id=…), which NSURL resolves.
func pasteboardPaths(pboard objc.ID) []string {
	typ := objc.ID(objc.GetClass("NSString")).Send(selStringWith, "public.file-url")
	nsurl := objc.ID(objc.GetClass("NSURL"))
	items := pboard.Send(selItems)
	n := objc.Send[uint](items, selCount)
	var paths []string
	for i := uint(0); i < n; i++ {
		s := items.Send(selObjectAt, i).Send(selStringFor, typ)
		if s == 0 {
			continue
		}
		if p := goString(nsurl.Send(selURLWith, s).Send(selFilePathURL).Send(selPath)); p != "" {
			paths = append(paths, p)
		} else if u, err := url.Parse(goString(s)); err == nil && u.Scheme == "file" {
			paths = append(paths, u.Path)
		}
	}
	return paths
}

// serviceRequest turns a service's files into the request the extension
// would send: the folder is the selected one, or the selection's.
func serviceRequest(id string, paths []string) Request {
	r := Request{ID: id, Paths: paths}
	if len(paths) > 0 {
		r.Target = filepath.Dir(paths[0])
	}
	return r
}

// registerServices makes the app the provider of its services. Call it on
// the main thread once the app is ready; AppKit calls the method there.
func registerServices(handle func(Request)) {
	class, err := objc.RegisterClass("MBServicesProvider", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{
		Cmd: selRunService,
		Fn: func(_ objc.ID, _ objc.SEL, pboard, userData objc.ID, _ *objc.ID) {
			paths := pasteboardPaths(pboard)
			if len(paths) == 0 {
				return
			}
			handle(serviceRequest(goString(userData), paths))
		},
	}})
	if err != nil {
		log.Println("services:", err)
		return
	}
	app := objc.ID(objc.GetClass("NSApplication")).Send(selShared)
	app.Send(selSetProvider, objc.ID(class).Send(selNew))
}

var (
	selFileURLWithPath = objc.RegisterName("fileURLWithPath:")
	selBookmarkData    = objc.RegisterName("bookmarkDataWithOptions:includingResourceValuesForKeys:relativeToURL:error:")
	selWriteBookmark   = objc.RegisterName("writeBookmarkData:toURL:options:error:")
)

// NSURLBookmarkCreationSuitableForBookmarkFile: the bookmark of an alias.
const bookmarkSuitableForFile = 1 << 10

// makeAlias makes a Finder alias of each item beside it, "<name> alias" as
// Finder names them: a bookmark file, which follows the item when it moves,
// unlike a symbolic link.
func makeAlias(paths []string) ([]string, error) {
	nsurl := objc.ID(objc.GetClass("NSURL"))
	nsstring := objc.ID(objc.GetClass("NSString"))
	fileURL := func(p string) objc.ID { return nsurl.Send(selFileURLWithPath, nsstring.Send(selStringWith, p)) }
	var done []string
	for _, p := range paths {
		dst := uniquePath(filepath.Dir(p), filepath.Base(p)+" alias")
		data := fileURL(p).Send(selBookmarkData, uint(bookmarkSuitableForFile), objc.ID(0), objc.ID(0), objc.ID(0))
		if data == 0 || !objc.Send[bool](nsurl, selWriteBookmark, data, fileURL(dst), uint(0), objc.ID(0)) {
			return done, fmt.Errorf("couldn't make an alias of %s", filepath.Base(p))
		}
		done = append(done, dst)
	}
	return done, nil
}

var (
	selAEManager    = objc.RegisterName("sharedAppleEventManager")
	selCurrentEvent = objc.RegisterName("currentAppleEvent")
	selEventID      = objc.RegisterName("eventID")
)

// launchedPlainly reports whether the Apple Event being handled, the one
// that launched the app, is a plain open ('oapp'): a launch to open a
// URL, by a click in the menu, comes with none, and its URL only later.
func launchedPlainly() bool {
	const openApplication = 'o'<<24 | 'a'<<16 | 'p'<<8 | 'p'
	ev := objc.ID(objc.GetClass("NSAppleEventManager")).Send(selAEManager).Send(selCurrentEvent)
	return ev != 0 && objc.Send[uint32](ev, selEventID) == openApplication
}
