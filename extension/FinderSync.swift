// Menubloom's Finder Sync extension. It is deliberately dumb: the Go app
// writes the whole menu to menu.json, this draws it for the right-click,
// and a click sends the item's id and Finder's selection back to the app as
// a menubloom://run URL carrying the app's secret token. Every action runs
// in the app, which is not sandboxed; this extension is, as Apple requires.
import Cocoa
import FinderSync

struct Entry: Decodable {
    var id: String?
    var title: String?
    var symbol: String?      // an SF Symbol name
    var app: String?         // an app's path, whose icon the item shows
    var separator: Bool?
    var contexts: [String]?  // items, container, sidebar, toolbar; nil: all
    var kinds: String?       // "files" or "folders": every selected item is one
    var exts: [String]?      // every selected item has one of these extensions
    var count: String?       // "one" or "many" selected items
    var under: [String]?     // globs, one of which the folder's path matches
    var info: String?        // "size", "created" or "modified" of the one selected item
    var children: [Entry]?
}

struct Model: Decodable {
    var token: String
    var items: [Entry]
}

// The real home: NSHomeDirectory() is the sandbox's container.
let realHome: String = {
    if let pw = getpwuid(getuid()), let dir = pw.pointee.pw_dir { return String(cString: dir) }
    return NSHomeDirectory()
}()

let modelPath = realHome + "/Library/Application Support/Menubloom/menu.json"

// MenuBuilder turns the app's entries into the menu for one context and
// selection; ids collects the id of each clickable item, by tag.
struct MenuBuilder {
    let context: String
    let selection: [URL]
    let folder: String?      // the folder Finder shows
    let target: AnyObject?
    let action: Selector?
    var ids: [String] = []

    func shows(_ e: Entry) -> Bool {
        if let cs = e.contexts, !cs.contains(context) { return false }
        if let k = e.kinds {
            let wantDir = k == "folders"
            for u in selection where isDir(u) != wantDir { return false }
        }
        if let exts = e.exts {
            for u in selection where !exts.contains(u.pathExtension.lowercased()) { return false }
        }
        if e.count == "one" && selection.count != 1 { return false }
        if e.count == "many" && selection.count < 2 { return false }
        if let globs = e.under {
            guard let path = folder ?? selection.first?.deletingLastPathComponent().path,
                  globs.contains(where: { fnmatch($0, path, 0) == 0 }) else { return false }
        }
        return true
    }

    // infoText is the value an info item shows, from the file's metadata,
    // which the sandbox lets the extension read; a folder's size needs its
    // contents, so the app works it out when clicked.
    func infoText(_ kind: String) -> String? {
        guard selection.count == 1,
              let a = try? FileManager.default.attributesOfItem(atPath: selection[0].path) else { return nil }
        switch kind {
        case "size":
            guard a[.type] as? FileAttributeType != .typeDirectory, let n = a[.size] as? Int64 else { return nil }
            return ByteCountFormatter.string(fromByteCount: n, countStyle: .file)
        case "created", "modified":
            guard let d = a[kind == "created" ? .creationDate : .modificationDate] as? Date else { return nil }
            return DateFormatter.localizedString(from: d, dateStyle: .medium, timeStyle: .short)
        default:
            return nil
        }
    }

    mutating func add(_ entries: [Entry], to menu: NSMenu) {
        for e in entries where shows(e) {
            if e.separator == true {
                if let last = menu.items.last, !last.isSeparatorItem { menu.addItem(.separator()) }
                continue
            }
            var title = e.title ?? ""
            if let kind = e.info, let value = infoText(kind) { title += ":  " + value }
            let item = NSMenuItem(title: title, action: nil, keyEquivalent: "")
            if let s = e.symbol {
                item.image = NSImage(systemSymbolName: s, accessibilityDescription: nil)
            }
            if let a = e.app {
                item.image = smallIcon(a)
            }
            if let children = e.children {
                let sub = NSMenu(title: item.title)
                add(children, to: sub)
                if sub.items.isEmpty { continue }
                item.submenu = sub
            } else if let id = e.id {
                item.action = action
                item.target = target
                item.tag = ids.count
                ids.append(id)
            }
            menu.addItem(item)
        }
        while menu.items.last?.isSeparatorItem == true { menu.removeItem(at: menu.items.count - 1) }
        while menu.items.first?.isSeparatorItem == true { menu.removeItem(at: 0) }
    }
}

func isDir(_ u: URL) -> Bool {
    var dir: ObjCBool = false
    return FileManager.default.fileExists(atPath: u.path, isDirectory: &dir) && dir.boolValue
        && u.pathExtension.lowercased() != "app"
}

// smallIcon is a file's icon as one 16pt bitmap at 2x. Finder gets a copy
// of the menu, and NSWorkspace's icon carries every size up to 1024px:
// hundreds of MB to copy for a few apps, which held the menu back.
var icons: [String: NSImage] = [:]

func smallIcon(_ path: String) -> NSImage {
    if let i = icons[path] { return i }
    let size = NSSize(width: 16, height: 16)
    let image = NSImage(size: size)
    if let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: 32, pixelsHigh: 32, bitsPerSample: 8,
                                  samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB,
                                  bytesPerRow: 0, bitsPerPixel: 0) {
        // Sized before the context, whose units it sets: points, not pixels.
        rep.size = size
        guard let ctx = NSGraphicsContext(bitmapImageRep: rep) else { return image }
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = ctx
        NSWorkspace.shared.icon(forFile: path).draw(in: NSRect(origin: .zero, size: size))
        NSGraphicsContext.restoreGraphicsState()
        image.addRepresentation(rep)
    }
    icons[path] = image
    return image
}

// warm loads the menu's images before the first right-click asks for them.
func warm(_ entries: [Entry]) {
    for e in entries {
        if let a = e.app { _ = smallIcon(a) }
        if let s = e.symbol { _ = NSImage(systemSymbolName: s, accessibilityDescription: nil) }
        warm(e.children ?? [])
    }
}

func loadModel() -> Model? {
    guard let data = FileManager.default.contents(atPath: modelPath) else { return nil }
    return try? JSONDecoder().decode(Model.self, from: data)
}

@objc(MBFinderSync)
class MBFinderSync: FIFinderSync {
    // The ids of the last menu's items, by tag: Finder copies menus, so
    // only an item's tag survives to the action.
    var ids: [String] = []

    override init() {
        super.init()
        FIFinderSyncController.default().directoryURLs = [URL(fileURLWithPath: "/")]
        DispatchQueue.main.async {
            if let model = loadModel() { warm(model.items) }
        }
    }

    override var toolbarItemName: String { "Menubloom" }
    override var toolbarItemToolTip: String { "Menubloom: new files, paths, terminals and more" }
    override var toolbarItemImage: NSImage {
        NSImage(systemSymbolName: "cursorarrow.click.2", accessibilityDescription: "Menubloom") ?? NSImage()
    }

    override func menu(for kind: FIMenuKind) -> NSMenu {
        let menu = NSMenu(title: "")
        ids = []
        guard let model = loadModel() else { return menu }
        let context: String
        switch kind {
        case .contextualMenuForItems: context = "items"
        case .contextualMenuForContainer: context = "container"
        case .contextualMenuForSidebar: context = "sidebar"
        case .toolbarItemMenu: context = "toolbar"
        @unknown default: return menu
        }
        let c = FIFinderSyncController.default()
        var selection: [URL] = []
        if context == "items" {
            selection = c.selectedItemURLs() ?? []
        } else if context == "sidebar", let t = c.targetedURL() {
            selection = [t]
        }
        var b = MenuBuilder(context: context, selection: selection, folder: c.targetedURL()?.path, target: self, action: #selector(run(_:)))
        b.add(model.items, to: menu)
        ids = b.ids
        return menu
    }

    @objc func run(_ sender: NSMenuItem) {
        guard sender.tag >= 0, sender.tag < ids.count, let model = loadModel() else { return }
        let c = FIFinderSyncController.default()
        var query = [("t", model.token), ("id", ids[sender.tag])]
        if let t = c.targetedURL() { query.append(("target", t.path)) }
        for u in c.selectedItemURLs() ?? [] { query.append(("p", u.path)) }
        // Only unreserved characters stay as they are, so that "+", "&" and
        // "=" in a file's name reach the app intact.
        let unreserved = CharacterSet(charactersIn: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~/")
        let q = query.map { k, v in k + "=" + (v.addingPercentEncoding(withAllowedCharacters: unreserved) ?? "") }
        guard let url = URL(string: "menubloom://run?" + q.joined(separator: "&")) else { return }
        let config = NSWorkspace.OpenConfiguration()
        config.activates = false
        NSWorkspace.shared.open(url, configuration: config) { _, err in
            if let err = err { NSLog("Menubloom: %@", err.localizedDescription) }
        }
    }
}
