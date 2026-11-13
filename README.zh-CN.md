<p align="center">
  <img src="assets/icon.png" width="128" height="128" alt="Menubloom 图标">
</p>

<h1 align="center">Menubloom</h1>

<p align="center">
  <b>补齐 Mac 右键菜单缺的那些功能。</b><br>
  <a href="https://menubloom.lzhgus.com/zh/">官网</a> ·
  <a href="#下载">下载</a> ·
  <a href="#加了什么">加了什么</a> ·
  <a href="#从源码构建">从源码构建</a><br>
  <a href="README.md">English</a> · <b>简体中文</b>
</p>

<p align="center">
  <a href="https://github.com/lzhgus/menubloom/releases/latest"><img src="https://img.shields.io/github/v/release/lzhgus/menubloom?label=download&logo=apple&color=f08a4b" alt="最新版本"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/lzhgus/menubloom?color=blue" alt="MIT 许可证"></a>
  <a href="https://awesomemygo.dev/apps/menubloom"><img src="https://awesomemygo.dev/badge.svg" alt="已收录于 Awesome MyGo"></a>
</p>

<p align="center">
  <img src="assets/screenshot-menu.png" width="460" alt="在访达窗口空白处右键：Menubloom 加上了新建文件、拷贝路径、拷贝为、打开方式、显示隐藏文件和一个自定义动作，新建文件子菜单已展开">
</p>

<p align="center">
  <img src="assets/promo/menubloom-promo-20s.gif" width="640" alt="Menubloom 20 秒演示：访达里右键、拷贝路径、新建 Untitled.md">
</p>

<p align="center">
  <a href="assets/promo/menubloom-promo-20s.mp4">观看 20 秒演示视频（MP4）</a>
</p>

## 为什么做

在 Windows 上，在文件夹里点右键，就能新建文本文档、复制文件路径、在这里打开终端。
到了 Mac 上，访达的右键这些都做不到：没有 **新建 ▸ 文本文档**，没有 **复制为路径**，
没有 **剪切** 和 **粘贴**，也没有 **在终端中打开**。只好先打开一个 App 存个空文件，
把文件拖进终端才拿到路径，或者记住 ⌘C 之后要按 ⌘⌥V 才是“移动”。

这是我们每天都会遇到的痛点，也是从 Windows 转到 Mac 的人最先问的问题。
Menubloom 把这些功能放回访达的右键菜单，再加上开发者和重度用户常用的那些。
打开一个开关就能用，不用逐个授权文件夹。

## 下载

**[下载最新版本](https://github.com/lzhgus/menubloom/releases/latest)**：
经过签名和公证的通用版 App（Apple 芯片和 Intel 都可以），支持 macOS 12 及以上。

1. 打开 DMG，把 **Menubloom** 拖到“应用程序”。
2. 打开它。设置里有一份简短的清单：打开访达扩展，需要的话再打开“登录时打开”。
3. 在任意访达窗口里点右键。

也可以用 [Homebrew](https://github.com/lzhgus/homebrew-tap) 安装：

```sh
brew install --cask lzhgus/tap/menubloom
```

## 加了什么

| 项目 | 出现在 | 做什么 |
|---|---|---|
| **新建文件 ▸** | 文件夹空白处、文件夹、工具栏 | 用模板（文本、Markdown、Word、Excel、RTF、HTML、JSON、CSV、Python、Shell）新建文件：先问名字（直接回车就是 `Untitled.<扩展名>`），再选中它。**放进模板文件夹的任何文件都会变成一种新建文件**，和 Windows 一样 |
| **拷贝路径** | 任何地方 | 所选项目或当前文件夹的完整路径，一行一个 |
| **拷贝为 ▸** | 任何地方 | 名称、不含扩展名的名称、`~` 路径、带引号的路径、`file://` URL、Markdown 链接；文本文件的内容；文件夹的目录树；MD5 / SHA-1 / SHA-256 |
| **简介 ▸** | 单个项目 | 菜单里直接显示大小、创建和修改时间，还有图片尺寸；点一下即可拷贝 |
| **打开方式 ▸** | 任何地方 | 终端、iTerm、Ghostty、Warp、VS Code、Cursor、Zed……（自动检测，也可以添加任意 App）；终端会打开文件所在的文件夹 |
| **剪切**，再 **粘贴到这里** | 先选项目，再到文件夹 | 像 Windows 一样移动文件：重名会自动编号，跨磁盘也可以 |
| **拷贝到 ▸ / 移动到 ▸** | 所选项目 | 送到常用文件夹，或者 **其他文件夹…** |
| **文件操作 ▸** | 所选项目 | 制作真正的访达替身（原文件移动后仍能找到）、制作软链接、添加执行权限、移除隔离属性、解散文件夹、立即删除（会先确认） |
| **转换图片 ▸ / 调整图片尺寸 ▸** | 图片 | 在原图旁边生成 PNG、JPEG、HEIC、TIFF 或 ICNS（含 16 到 1024 全部尺寸），或者长边为 2048、1024、512、256 px 的副本 |
| **显示 / 隐藏隐藏文件** | 文件夹空白处 | 切换访达是否显示隐藏文件 |
| **自定义动作** | 你决定 | 你自己的 Shell 命令，所选路径就是 `"$@"`；可以限定文件类型、选中数量和文件夹；在后台运行或在终端窗口里运行；还可以设全局快捷键 |

**快捷键。** 拷贝路径、拷贝为、新建文件、打开方式、剪切、粘贴和显示隐藏文件也可以设全局快捷键（设置 ▸ 快捷键，例如 `Cmd+Shift+C`），不用打开菜单就对访达里选中的项目执行。第一次使用时会请求一次控制访达的权限。

在 **设置**（菜单栏图标 ▸ 设置…）里，每一组都可以关闭、上下移动，或者 **展开**
到顶层而不放在子菜单里。设置和菜单支持英文和简体中文，有九种配色主题，浅色深色都有。


### iCloud 云盘、Dropbox 和 OneDrive

访达在云盘文件夹里不显示扩展菜单。在那里，Menubloom 最常用的功能在
**右键 ▸ 服务** 里：拷贝路径、拷贝名称、在终端中打开、在编辑器中打开、在此新建文本文件、
剪切、粘贴到这里、拷贝到…、移动到…。如果没看到，到 系统设置 ▸ 键盘 ▸ 键盘快捷键 ▸
服务 ▸ 文件和文件夹 里打开。

### 脚本和快捷指令

每个项目都能从命令行运行，所以脚本、快捷指令的“运行 Shell 脚本”、Raycast、Alfred 都能用：

```sh
SC="/Applications/Menubloom.app/Contents/MacOS/Menubloom"
"$SC" run copy:path ~/Desktop/a.txt     # 拷贝路径
"$SC" run new:.md ~/notes               # 在 ~/notes 里新建 Markdown 文件
"$SC" run img:png photo.heic            # 转换图片 ▸ PNG
"$SC" run open:terminal .               # 在终端里打开当前文件夹
```

全部 id 见 [English README](README.md#scripts-and-shortcuts)。

### 如果看不到菜单项

- **设置 ▸ 通用** 会显示访达扩展是否打开，并可以一键打开。也可以在 系统设置 ▸ 通用 ▸
  登录项与扩展 ▸ 访达 里打开。
- 访达刚启动时的第一次右键，扩展可能还没加载好，再右键一次就有了。
- 更新之后，可以用菜单栏图标菜单里的 **重新启动访达**。

## 和同类应用对比

还有两款应用也能扩展 Finder 右键菜单：[RClick](https://github.com/wflixu/RClick)
和 [SaneClick](https://saneclick.com)。下面关于它们的信息来自各自的 README 和官网
（截至 2026-11-13），不是上手实测；有不准确的地方欢迎指正。

| | Menubloom | RClick | SaneClick |
|---|---|---|---|
| 许可证 | MIT | GPL-3.0 | PolyForm Shield 1.0.0（源码公开） |
| 价格 | 免费 | 免费 | 免费试用 14 天，之后一次性 $14.99；Mac App Store 版免费，含 13 个动作 |
| macOS | 12 或更新 | 15.6 或更新 | 14 或更新 |
| 芯片 | Apple Silicon 和 Intel | — | 仅 Apple Silicon |
| 内置功能 | 新建文件、拷贝路径和多种拷贝格式、用其他应用打开、剪切粘贴、拷贝到 / 移动到、文件操作、图片转换 | 用任意应用打开、拷贝路径 / 文件名、按模板新建文件、直接删除、隐藏 / 显示、隔空投送、常用文件夹 | 62 个动作，包括图片识字（OCR）、PDF 合并拆分、图片转换和哈希 |
| 自定义动作 | Shell 命令，可设全局快捷键 | 用应用打开，可带自定义参数 | Bash、AppleScript 或 Automator 脚本 |

“—”表示我们在对方文档里没找到。想在右键里用 OCR 和 PDF 工具，可以选 SaneClick；
Menubloom 专注 Windows 用户最想念的那些文件基础操作，免费，旧款 Mac 也能用。

## 隐私

Menubloom 不收集任何数据。它唯一的联网是每天去 GitHub 检查一次新版本，
可以在「设置 ▸ 通用」里关掉。设置都保存在你的 Mac 上：
`~/Library/Application Support/Menubloom/`。

## 从源码构建

需要 macOS 12+、[Go](https://go.dev/dl/) 1.27.3+ 和 Xcode 命令行工具（`swiftc`）。

```sh
git clone https://github.com/lzhgus/menubloom.git
cd menubloom
scripts/build.sh --install     # 构建、安装到“应用程序”、打开扩展并启动
```

更多构建方式和工作原理见 [English README](README.md#build-from-source)。

## 许可证

MIT
