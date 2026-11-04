package main

import "strings"

// Languages Settings offers: the system's choice, or one of the app's.
var Languages = []string{"System", "English", "简体中文"}

// lang is the language the app shows: "en" or "zh". Set on the main
// thread, before the menu or the window are built.
var lang = "en"

// setLanguage picks the language of a setting, the system's preferred
// languages deciding for "System".
func setLanguage(setting string, preferred []string) {
	switch setting {
	case Languages[1]:
		lang = "en"
	case Languages[2]:
		lang = "zh"
	default:
		lang = "en"
		for _, l := range preferred {
			if strings.HasPrefix(l, "zh") {
				lang = "zh"
				break
			}
			if strings.HasPrefix(l, "en") {
				break
			}
		}
	}
}

// T translates an English string of the app.
// ponytail: one table keyed by the English text; gettext catalogs if more languages come.
func T(s string) string {
	if lang == "zh" {
		if t, ok := zh[s]; ok {
			return t
		}
	}
	return s
}

var zh = map[string]string{
	"Text Document":  "文本文档",
	"Rich Text":      "RTF 文稿",
	"Shell Script":   "Shell 脚本",
	"Word Document":  "Word 文稿",
	"Excel Workbook": "Excel 工作簿",
	"%d items":       "%d 个项目",
	"Action failed":  "动作失败",
	"Add Action":     "添加动作",
	"Add App…":       "添加 App…",
	"Add Folder…":    "添加文件夹…",
	"Add Menubloom to Finder's toolbar with View ▸ Customize Toolbar…, for its menu in any window.": "用“显示 ▸ 自定工具栏…”把 Menubloom 加到访达工具栏，在任何窗口都能打开它的菜单。",
	"Add a Folder": "添加文件夹",
	"Add an App":   "添加 App",
	"Any number":   "任意数量",
	"Appearance":   "外观",
	"Apps that open the selection, or the folder you're in. A terminal opens the folder of a file.": "用来打开所选项目或当前文件夹的 App。终端会打开文件所在的文件夹。",
	"Cancel":          "取消",
	"Checking…":       "正在检查…",
	"Choose a Folder": "选择文件夹",
	"Command":         "命令",
	"Contents":        "文件内容",
	"Convert Image":   "转换图片",
	"Convert to PNG, JPEG, HEIC, TIFF or ICNS; resize": "转换为 PNG、JPEG、HEIC、TIFF 或 ICNS；调整尺寸",
	"Copied":                              "已拷贝",
	"Copy As":                             "拷贝为",
	"Copy Here":                           "拷贝到这里",
	"Copy Path":                           "拷贝路径",
	"Copy To":                             "拷贝到",
	"Copy the full path of the selection": "拷贝所选项目的完整路径",
	"Copy the selection to a favorite folder":             "把所选项目拷贝到常用文件夹",
	"Couldn't change Open at login":                       "无法更改登录时打开",
	"Couldn't change hidden files":                        "无法切换隐藏文件",
	"Couldn't convert":                                    "无法转换",
	"Couldn't copy":                                       "无法拷贝",
	"Couldn't create the file":                            "无法创建文件",
	"Couldn't finish":                                     "无法完成",
	"Couldn't hash":                                       "无法计算哈希",
	"Couldn't move":                                       "无法移动",
	"Couldn't open":                                       "无法打开",
	"Couldn't open at login":                              "无法设置登录时打开",
	"Couldn't read Finder's selection":                    "无法读取访达中的所选项目",
	"Couldn't resize":                                     "无法调整尺寸",
	"Couldn't save settings":                              "无法保存设置",
	"Count Lines":                                         "统计行数",
	"Create a file from a template in the current folder": "在当前文件夹用模板新建文件",
	"Created":        "创建时间",
	"Custom Actions": "自定义动作",
	"Cut":            "剪切",
	"Cut & Paste":    "剪切与粘贴",
	"Cut files, then move them with Paste in another folder": "剪切文件，再在别的文件夹用“粘贴”移动过去",
	"Dark":                   "深色",
	"Data folder":            "数据文件夹",
	"Delete":                 "删除",
	"Delete %s immediately?": "要立即删除%s吗？",
	"Delete Immediately…":    "立即删除…",
	"Dimensions":             "尺寸",
	"Disk Usage":             "占用空间",
	"Dissolve Folder":        "解散文件夹",
	"Edit Templates…":        "编辑模板…",
	"Every file in the Templates folder is a kind of New File, as Windows' New menu. Add your own: a Keynote theme, a license, a project's README.": "模板文件夹里的每个文件都是一种“新建文件”，就像 Windows 的“新建”菜单。放进你自己的：Keynote 主题、许可证、项目的 README。",
	"Everything Finder's right-click is missing. Three steps to set it up:":                                                                         "访达右键缺的，这里都有。三步完成设置：",
	"Extensions":        "扩展名",
	"File Actions":      "文件操作",
	"File URL":          "文件 URL",
	"Files":             "文件",
	"Files and folders": "文件和文件夹",
	"Finder extension":  "访达扩展",
	"Finder shows no extension menus in cloud folders. There, use right-click ▸ Services ▸ Menubloom: …; if they're missing, turn them on in Keyboard Shortcuts ▸ Services ▸ Files and Folders.": "访达在云盘文件夹里不显示扩展菜单。在那里请用 右键 ▸ 服务 ▸ Menubloom：…；如果没有，到“键盘快捷键 ▸ 服务 ▸ 文件和文件夹”里打开。",
	"Folder Tree":       "目录树",
	"Folder background": "文件夹空白处",
	"Folders":           "文件夹",
	"General":           "通用",
	"Git Status":        "Git 状态",
	"Hide Hidden Files": "隐藏隐藏文件",
	"How Settings looks. The right-click menu is Finder's own.": "设置窗口的样子。右键菜单的样式由访达决定。",
	"ICNS (app icon)": "ICNS（App 图标）",
	"Images":          "图片",
	"Info":            "简介",
	"Inline":          "展开",
	"It won't go to the Trash, and this can't be undone.": "它不会进入废纸篓，而且无法撤销。",
	"Keyboard Shortcuts": "键盘快捷键",
	"Language":           "语言",
	"Light":              "浅色",
	"Make Alias":         "制作替身",
	"Make Executable":    "添加执行权限",
	"Make Symlink":       "制作软链接",
	"Make alias or symlink, make executable, remove quarantine, dissolve folder, delete immediately": "制作替身或软链接、添加执行权限、移除隔离属性、解散文件夹、立即删除",
	"Makes the first right-click of the day as fast as the rest, and keeps Cut & Paste ready.":       "让每天第一次右键也一样快，并保留剪切的内容。",
	"Markdown Link": "Markdown 链接",
	"Menu Items":    "菜单项目",
	"Modified":      "修改时间",
	"Move Here":     "移到这里",
	"Move To":       "移动到",
	"Move down":     "下移",
	"Move the selection to a favorite folder": "把所选项目移动到常用文件夹",
	"Move up":                "上移",
	"Name":                   "名称",
	"Name Without Extension": "不含扩展名的名称",
	"Name, ~ path, quoted path, file URL, Markdown link, contents, folder tree, MD5, SHA-1, SHA-256": "名称、~ 路径、带引号的路径、文件 URL、Markdown 链接、文件内容、目录树、MD5、SHA-1、SHA-256",
	"New Action":          "新动作",
	"New File":            "新建文件",
	"No app to open with": "没有可用来打开的 App",
	"No folders yet.":     "还没有文件夹。",
	"Not Now":             "以后再说",
	"Not installed: install the packaged app (scripts/build.sh --install).":                                          "未安装：请安装打包好的 App（scripts/build.sh --install）。",
	"Of Settings and the right-click menu.":                                                                          "设置窗口和右键菜单的语言。",
	"Off: runs in the background, and what it prints shows as a notification.":                                       "关闭时在后台运行，输出显示为通知。",
	"Off: turn it on to see Menubloom's items in Finder.":                                                            "已关闭：打开后才能在访达里看到 Menubloom 的项目。",
	"On. Right-click in any Finder window; the first right-click after Finder starts may come before it has loaded.": "已开启。在任何访达窗口右键即可；访达刚启动时的第一次右键可能还没加载好。",
	"One item":                 "一个项目",
	"Only in":                  "仅限文件夹",
	"Open In":                  "打开方式",
	"Open Menubloom at login?": "要在登录时打开 Menubloom 吗？",
	"Open Templates Folder":    "打开模板文件夹",
	"Open at Login":            "登录时打开",
	"Open at login":            "登录时打开",
	"Open the selection or folder in a terminal or editor": "在终端或编辑器中打开所选项目或文件夹",
	"Other Folder…":               "其他文件夹…",
	"Output":                      "输出",
	"Paste %d Items Here":         "将 %d 个项目粘贴到这里",
	"Paste Item Here":             "将项目粘贴到这里",
	"Preferences":                 "偏好设置",
	"Quoted Path (for the shell)": "带引号的路径（用于 Shell）",
	"Remove":                      "移除",
	"Remove Quarantine":           "移除隔离属性",
	"Resize Image":                "调整图片尺寸",
	"Restart Finder":              "重新启动访达",
	"Run in a Terminal window":    "在终端窗口中运行",
	"Runs it on Finder's selection from anywhere; asks once to control Finder.": "在任何地方按下都会对访达中的所选项目运行；首次会请求控制访达的权限。",
	"Selection":     "所选数量",
	"Settings…":     "设置…",
	"Several items": "多个项目",
	"Shell commands of your own in the menu. They run in zsh, in the selection's folder, with the selected paths as \"$@\".": "把你自己的 Shell 命令放进菜单。它们在 zsh 中、所选项目所在的文件夹里运行，所选路径就是 \"$@\"。",
	"Shortcut":          "快捷键",
	"Show":              "显示",
	"Show Hidden Files": "显示隐藏文件",
	"Show its items at the top level instead of in a submenu": "把子菜单里的项目直接放到顶层",
	"Shows on": "显示于",
	"Size":     "大小",
	"Size, created and modified dates, image dimensions; click one to copy it":                                                                             "大小、创建和修改时间、图片尺寸；点一下即可拷贝",
	"Menubloom runs the actions of its menu. Opening it at login makes the first right-click of the day as fast as the rest, and keeps Cut & Paste ready.": "Menubloom 负责执行菜单里的动作。登录时打开它，每天第一次右键也一样快，剪切的内容也会一直保留。",
	"System":                         "跟随系统",
	"Terminal":                       "终端",
	"The Templates folder is empty.": "模板文件夹是空的。",
	"Theme":                          "主题",
	"Toggle hidden files in Finder":  "在访达中显示或隐藏隐藏文件",
	"Toolbar button":                 "工具栏按钮",
	"Turn On":                        "打开",
	"What the right-click menu shows, in this order. Changes apply at the next right-click.": "右键菜单显示哪些项目、按什么顺序。改动在下一次右键时生效。",
	"Where Copy To and Move To send the selection. Other Folder… asks each time.":            "“拷贝到”和“移动到”的目标文件夹。“其他文件夹…”每次都会询问。",
	"Your own shell commands":              "你自己的 Shell 命令",
	"any folder; or e.g. ~/code, ~/work/*": "任意文件夹；或例如 ~/code, ~/work/*",
	"any; or e.g. png, jpg":                "任意；或例如 png, jpg",
	"iCloud Drive, Dropbox and OneDrive":   "iCloud 云盘、Dropbox 和 OneDrive",
	"none; or e.g. Cmd+Shift+G":            "无；或例如 Cmd+Shift+G",
	"~ Path":                               "~ 路径",
}
