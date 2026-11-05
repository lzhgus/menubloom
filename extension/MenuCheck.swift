// A check of the extension's menu logic against the menu.json the app
// wrote, outside Finder: scripts/check-extension.sh builds and runs it.
import Cocoa

func titles(_ m: NSMenu, _ indent: String = "") -> [String] {
    m.items.flatMap { i -> [String] in
        let line = indent + (i.isSeparatorItem ? "—" : i.title)
        return [line] + (i.submenu.map { titles($0, indent + "  ") } ?? [])
    }
}

func expect(_ ok: Bool, _ what: String) {
    if !ok {
        print("FAIL:", what)
        exit(1)
    }
}

@main
struct MenuCheck {
    static func main() {
        guard let model = loadModel() else {
            print("FAIL: can't read", modelPath)
            exit(1)
        }
        let dir = URL(fileURLWithPath: CommandLine.arguments[1])
        let image = dir.appendingPathComponent("pic.png")
        let text = dir.appendingPathComponent("a.txt")
        for (context, selection) in [("container", [URL]()), ("items", [text]), ("items", [image]), ("items", [dir])] {
            var b = MenuBuilder(context: context, selection: selection, folder: dir.path, target: nil, action: nil)
            let menu = NSMenu()
            b.add(model.items, to: menu)
            let t = titles(menu)
            print("== \(context) \(selection.map(\.lastPathComponent))")
            print(t.joined(separator: "\n"))
            expect(t.first != "—" && t.last != "—", "no separator at the ends")
            expect(!zip(t, t.dropFirst()).contains { $0 == "—" && $1 == "—" }, "no double separators")
            switch (context, selection.first?.pathExtension) {
            case ("container", _):
                expect(t.contains("New File") && !t.contains("Cut") && !t.contains("Convert Image"), "background menu")
            case ("items", "png"):
                expect(t.contains("Convert Image") && t.contains("Resize Image") && t.contains("Cut"), "image menu")
            case ("items", "txt"):
                expect(!t.contains("Convert Image") && t.contains("  SHA-256") && t.contains("  Contents"), "file menu")
                expect(t.contains("  Make Executable") && !t.contains("  Dissolve Folder"), "file actions on a file")
                expect(t.contains { $0.hasPrefix("  Size:  ") } && t.contains { $0.hasPrefix("  Modified:  ") }, "info rows with values")
            default:
                expect(!t.contains("  SHA-256") && !t.contains("Convert Image") && t.contains("  Folder Tree"), "folder menu")
                expect(t.contains("  Dissolve Folder") && !t.contains("  Make Executable"), "file actions on a folder")
            }
        }
        // Rules: one item only, and only under a folder.
        let rules = [Entry(id: "a", title: "One", count: "one"), Entry(id: "b", title: "Many", count: "many"),
                     Entry(id: "c", title: "Here", under: [dir.path + "/*", dir.path]), Entry(id: "d", title: "Elsewhere", under: ["/nowhere/*"])]
        for (sel, want) in [([text], ["One", "Here"]), ([text, image], ["Many", "Here"])] {
            var b = MenuBuilder(context: "items", selection: sel, folder: dir.path, target: nil, action: nil)
            let m = NSMenu()
            b.add(rules, to: m)
            expect(titles(m) == want, "rules for \(sel.count) items: \(titles(m))")
        }
        // An app's icon fills its whole 32px bitmap, not a corner of it.
        let icon = smallIcon("/System/Applications/Utilities/Terminal.app")
        guard let rep = icon.representations.first as? NSBitmapImageRep else { expect(false, "icon is a bitmap"); return }
        expect(rep.pixelsWide == 32 && icon.size.width == 16, "icon is 16pt at 2x")
        expect((rep.colorAt(x: 24, y: 8)?.alphaComponent ?? 0) > 0, "icon fills its top right")
        print("OK")
    }
}
