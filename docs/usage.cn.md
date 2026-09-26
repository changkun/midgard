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

## 显示全部活跃设备

检查所有连接的设备：

```sh
$ mg daemon ls
id      name
1       changkun-perflock
2       changkun-air-arm
3       changkun-pro-intel
4       changkun-ubuntu
5       changkun-win
```

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