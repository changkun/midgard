# 使用 midgard

[English](./usage.md) | 中文

在一台设备上复制，在任意另一台上粘贴。每一次复制都会进入你的历史记录，每台设备上都相同；任何复制内容或文件都可以变成可分享的链接。你的设备用它们共享的密钥加密复制内容：服务端负责转发，但无法读取。各部分的安装方法见[安装 midgard](./install.cn.md)。

## 在 Mac 上：Midgard 应用

Midgard 常驻菜单栏。像平常一样复制，内容就会到达你的其他设备；在其他设备上复制的内容也会出现在这台 Mac 的剪贴板上。菜单中有：

- **最近的复制内容**：选择一条即可把它放回剪贴板，这台 Mac 和所有设备上都会更新；
- **打开历史记录…**：来自所有设备的每一条复制内容，可搜索、查看、复制回剪贴板和删除；
- **将剪贴板分享为链接**，快捷键 **Ctrl+Option+S**：链接会放到你的剪贴板上；
- **暂停同步**：暂停期间 Midgard 不再读取或写入这台 Mac 的剪贴板，直到你恢复；
- **登录时启动**，以及 **设置…**：设置服务端、登录与加密。“**Pair Another Device**”会显示配对码，以及给手机扫描的二维码。

## 在 Linux 和 Windows 上：`mg daemon`

守护进程在你的桌面会话中运行，无需窗口即可完成同样的事：你的复制内容会到达其他设备，其他设备的内容会到达你的剪贴板；Linux 上按 **Ctrl+Mod4+S**（Windows 上按 **Ctrl+Shift+S**）可把剪贴板分享为链接。`mg history` 和网页可以查看历史记录。

## 在浏览器中

在任意电脑或手机上打开 `https://your.domain/midgard/` 并登录。第一次需要为浏览器配对：“**Pair this browser**”会要求输入配对码（见[为设备配对](#为设备配对)）。之后你可以查看剪贴板，向设备发送文本和图片；浏览历史记录；查看正在发往离线设备的内容以及你的设备；把文件或剪贴板分享为链接；以及签发应用令牌。

## 在 iPhone 上

**网页**，已加密。在 Mac 的 Midgard 应用中打开“Settings → Encryption → **Pair Another Device**”，用 iPhone 的相机扫描二维码：网页会在 Safari 中打开并完成配对，需要时会引导你登录。然后通过“分享 → 添加到主屏幕”，一点即可打开：把剪贴板复制到 iPhone，或把 iPhone 的剪贴板发送到你的设备。

**快捷指令**，未加密，按需开启。iPhone 不允许应用监视剪贴板，快捷指令也无法加密，因此两个快捷指令经由你允许它们进入的 Mac，以明文访问你的复制内容：

- **Get from Midgard**：把你最新的复制内容（文本或图片）放到 iPhone 的剪贴板上。
- **Send to Midgard**：把你从任意应用分享菜单分享给它的内容，否则把剪贴板上的内容，发送到你的设备。

在那台 Mac 上打开“Settings → Encryption → **Let iPhone Shortcuts in, unencrypted**”；或在运行 `mg daemon` 的机器上，在其 `config.yml` 中写入 `plain_bridge: true`。此后服务端能读取经由这里传输的内容，也可以借此发送它自己的内容。然后在 iPhone 上打开网页，在 **Your iPhone** 的“**Or Shortcuts, unencrypted**”中：为这台 iPhone 签发令牌，添加两个快捷指令，在快捷指令询问时粘贴令牌。把它们放到主屏幕，或放到“辅助功能”中的“轻点背面”，一点即可运行。

Android 上的 Tasker 也可以这样做：带上令牌，`GET` 和 `POST` `/midgard/api/v1/plain/clipboard`，发送时使用 `{"type": "text", "data": "...", "daemon_id": "pixel"}`，其中 `daemon_id` 是这条复制内容显示的来源设备名。

## 为设备配对

你的复制内容用设备共享的密钥加密。第一台连接的设备生成密钥；其他设备各配对一次，从已有密钥的设备获得它：

1. 在有密钥的设备上显示配对码：在 Midgard 应用中打开“Settings → Encryption → **Pair Another Device**”，同时会显示二维码；或运行 `mg pair`。
2. 在新设备上输入配对码：Midgard 应用会提示输入；`mg pair <配对码>`；网页的“**Pair this browser**”；或用手机扫描二维码。

配对码只能用一次，十分钟内有效。服务端只在新设备取走之前保存用配对码加密的密钥，而且无法解开它：它从不知道配对码。浏览器可以加入，但不显示配对码，因为它把密钥保存在无法被读出的地方。

```sh
$ mg pair                  # 在有密钥的机器上
7K2M-9QXR-A4BC-8DEF-GH1J-KMNP-QR
$ mg pair 7K2M-9QXR-A4BC-8DEF-GH1J-KMNP-QR   # 在新机器上
```

没有密钥的设备在配对之前不会同步任何内容；`mg` 会以退出码 6 结束并说明原因。工作原理及其边界见 [midgard 的工作方式](./architecture.cn.md#加密服务端能看到什么)。

## 你的历史记录

你的每台设备都保存来自所有设备的最近复制内容：最近 200 条、30 天内、不超过 64 MB，每台设备上的列表和顺序都相同。最新的一条就是你的剪贴板。服务端不保存任何内容；网页和 `mg` 会向你某台在线的设备查询。

```sh
$ mg history              # 最新的在前，附每条文本的开头
$ mg history show 41      # 输出第 41 条
$ mg history copy 41      # 让第 41 条重新成为所有设备上的剪贴板
$ mg history rm 41        # 从所有设备上删除
$ mg history clear        # 全部删除
```

同一内容在历史记录中只出现一次：再次复制它，或从历史记录中把它放回剪贴板，它会移到最前面。密码管理器标记为机密的内容既不会被同步，也不会被保存。从历史记录中删除一条不会改变任何人此刻剪贴板上的内容。

## 分享链接

分享是发布到链接上的复制内容或文件，任何人都能打开，直到过期或你撤销。这是服务端唯一保存的内容，因为链接必须在你的设备关机时依然可用。

```sh
$ mg share                          # 你的剪贴板，随机链接
https://changkun.de/midgard/s/fboVP8u4xNMHfvsv2EeLzL

$ mg share -f report.pdf            # 一个文件
$ mg share notes/today -f a.txt     # 指定名称：链接为 notes/today.txt
$ mg share --expires 24h            # 一天后失效

$ mg shares                         # 列出你的分享
$ mg shares rm fboVP8u4xNMHfvsv2EeLzL   # 撤销；其链接随即失效
```

链接会放到你的剪贴板上。名称先到先得；每个分享也保留其随机链接。网页上同样可以分享文件和剪贴板。

## 你的设备，以及正在路上的内容

复制内容会在服务端内存中等待，直到你的每台设备都收到：合上的笔记本再打开时，会收到这期间你复制的内容。

```sh
$ mg devices              # 你的设备，无论是否在线
$ mg devices forget <id>  # 不再使用的设备：复制内容不再等待它

$ mg queue                # 正在等待的内容，以及它们在等哪些设备
$ mg queue rm 57          # 撤回尚未到达任何设备的内容
```

30 天未使用的设备会自动不再计入。

## 应用令牌

无法用浏览器登录的客户端，例如快捷指令、没有显示器的机器或脚本，可以改用应用令牌。令牌只代表你本人，撤销它只会让使用它的客户端退出登录。在网页的 **App tokens** 中签发，或在服务端执行：

```sh
$ docker compose run --rm midgard token add phone --owner <主体 ID>
$ docker compose run --rm midgard token ls --owner <主体 ID>
$ docker compose run --rm midgard token rm phone --owner <主体 ID>
```

把它写入设备的 `config.yml`（`token: mgt_...`）代替 `mg login`，或作为 `Authorization: Bearer mgt_...` 发送。只有当你在服务端白名单上时它才有效。

## 供脚本与智能体使用的 `mg`

脚本或智能体也通过 `mg` 使用 midgard。在不是你的设备的机器上，先用 `mg pair <配对码>` 配对：它会像你的设备一样，用密钥加密和解开你的复制内容。

```sh
$ mg copy hello world          # 复制到你所有设备的剪贴板
$ git log -1 | mg copy         # 原样复制标准输入的内容
$ mg copy < shot.png           # 一张 PNG 图片
$ mg paste                     # 你的剪贴板：最新的一条
$ mg paste > shot.png          # 图片需输出到文件
```

任何输出结果的命令加上 `--json`，结果就会以 JSON 形式输出到标准输出，提示信息则输出到标准错误：

```sh
$ echo "the build is green" | mg copy --json
{"seq": 58, "type": "text", "size": 19}
```

每个命令都以下列退出码之一结束：

| 退出码 | 含义 |
|---|---|
| 0 | 成功 |
| 1 | 服务端或网络出错，或请求被拒绝 |
| 2 | 命令用法错误 |
| 3 | 本设备未登录：运行 `mg login`，或在 `config.yml` 中配置令牌 |
| 4 | 你没有设备在线可以应答（剪贴板和历史记录都在设备上） |
| 5 | 没有这条复制内容、分享、设备或令牌 |
| 6 | 本设备没有你的密钥：运行 `mg pair <配对码>` |

## 是否正常工作？

```sh
$ mg status
server status: OK
daemon status: OK
```

`mg login` 登录设备，`mg logout` 退出登录。在 Mac 上，应用的菜单会显示是否正在同步。

## 许可

版权所有 2020-2026 [欧长坤](https://changkun.de)。保留所有权利。
