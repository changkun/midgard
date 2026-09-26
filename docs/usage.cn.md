# Midgard 使用指南

[English](./usage.md) | 中文

Midgard 命令行指令 `mg` 提供了各种丰富的指令可以与 midgard 服务端和守护进程进行交互。

## 状态检查

可以通过下列命令检查服务端和守护进程的运行状态：

```sh
$ mg status
server status: OK
daemon status: OK
```

## 网页

打开 `https://<你的域名>/midgard/` 并登录。网页上可以查看剪贴板并将文本发送到你的设备、
浏览历史记录、将文件或剪贴板分享为链接并撤销分享，以及签发应用令牌。在没有运行守护进程的手机上同样可用。

## 历史记录

你的每台设备都保存来自所有设备的最近复制内容：最近 200 条、30 天内、不超过 64 MB，每台设备上都相同。最新的一条就是你的剪贴板。服务端不保存任何内容，`mg history` 会向你某台在线的设备查询。

```sh
$ mg history              # 最新的在前
$ mg history copy 41      # 让第 41 条重新成为所有设备上的剪贴板
$ mg history rm 41
$ mg history clear
```

密码管理器标记为机密的内容不会被保存，也不会离开复制它的设备。删除或清空历史记录会同步到每台设备，但不会改变任何人此刻剪贴板上的内容。

## 正在路上的内容

复制内容会在服务端内存中等待，直到你的每台设备都收到。查看正在等待的内容及其等待的设备，或撤回尚未到达任何设备的内容：

```sh
$ mg queue
$ mg queue rm 57          # 仅当你没有设备在线时
$ mg devices              # 你的设备，无论是否在线
$ mg devices forget <id>  # 不再使用的设备：复制内容不再等待它
```

## 你的设备

```sh
$ mg devices
name     online  last seen            has up to  id
laptop   yes     2026-09-26 18:02:40  57         4f1c…
desktop  no      2026-09-26 09:12:03  52         9a0e…
```

一台设备就是一次 midgard 安装，以保存在其配置目录中的 id 区分。`mg daemon ls` 输出相同的列表。

## 分享链接

将文件或剪贴板内容分享为一个任何人都能打开的链接：

```sh
$ mg share                          # 分享剪贴板，使用随机链接
https://changkun.de/midgard/s/fboVP8u4xNMHfvsv2EeLzL

$ mg share -f report.pdf            # 分享一个文件
$ mg share notes/today -f a.txt     # 指定名称：链接为 notes/today.txt
https://changkun.de/midgard/notes/today.txt

$ mg share --expires 24h            # 一天后失效
```

链接会自动写入剪贴板，可以直接粘贴。名称先到先得；每个分享同时保留其随机链接。
链接是公开的，但只有你本人能列出或撤销自己的分享：

```sh
$ mg shares                             # 列出你的分享
$ mg shares rm fboVP8u4xNMHfvsv2EeLzL   # 撤销；其链接随即失效
```

守护进程的快捷键也以同样的方式分享剪贴板：

- Linux: **Ctrl+Mod4+s**
- macOS: **Ctrl+Option+s**
- Windows: **Ctrl+Shift+s**

iOS 捷径或其他持有应用令牌的客户端，可以发送
`POST /midgard/api/v1/shares`，内容为 `{"data": "<base64>", "type": "image/png"}`
（不带 data 时分享剪贴板）。旧的 midgard-alloc 捷径使用的接口已移除，无法再使用。

## 跨设备剪贴板共享

midgard 守护进程将自动监控剪贴板并将内容与 midgard 服务器进行同步（仅限于文本和图片数据）。
因此，配合系统截图的一个可能的使用场景为：

1. 对屏幕进行截图
2. 使用 `mg share` 命令或者 **Ctrl+Option+s** （macOS）或者 **Ctrl+Mod4+s** (Linux) 或者 **Ctrl+Shift+s** (Windows) 键盘快捷键
3. 立即使用 **Ctrl+v** 进行粘贴

第二步执行完后将返回一个可以公开访问的 URL，并自动回写到当前设备的剪贴板中，因此第三步可以顺利进行。

此外，因为剪贴板内容将在服务端进行缓存，因此在任何连接的设备上（若接收到广播的剪贴板数据）也可以直接对剪贴板内容进行粘贴。

### iOS, iPadOS, macOS 捷径 - Clipboard

下列捷径是为旧的用户名和密码制作的，服务端已不再接受。使用时请编辑其中的 **获取 URL 内容** 操作，
将 `Authorization` 请求头改为 `Bearer mgt_...`（一个应用令牌，见 `mg server token`），替换原先编码的用户名和密码。

- midgard-getclipboard
  + iOS 14, iPadOS 14: https://www.icloud.com/shortcuts/66c475e013e94dbf9f3714365d6c3f95
  + iOS 15+, iPadOS 15+, macOS 12+: https://www.icloud.com/shortcuts/c88e44b318e74eedb20201e4f513dabf
- midgard-putclipboard
  + iOS 14, iPadOS 14: https://www.icloud.com/shortcuts/c1b98b1ae59045e59c1f302a634e5633
  + iOS 15+, iPadOS 15+, macOS 12+: https://www.icloud.com/shortcuts/e875c142389e4fe6b45bbed4a517f8c8

## 许可

版权所有 2020-2021 [欧长坤](https://changkun.de)。保留所有权利。