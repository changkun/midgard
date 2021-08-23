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

## 分配全局 URL

全局 URL 分配的目的是希望将一个私有的内容创建一个永久的公开连接。例如：分享剪贴板中的内容、分享本地文件中的一个内容等等（不建议作用于大型文件，并只建议文本文件或者图片）

分配全局 URL 并对数据进行持久化：

```sh
$ mg alloc /awesome/filename -f /path/to/the/file # 为指定的文件创建一个指定的路由
https://changkun.de/midgard/awesome/filename      # 创建后的永久链接

$ mg alloc /awesome/clipboard/content # 为当前剪贴板中的内容创建一个指定的路由
https://changkun.de/midgard/awesome/clipboard/content # 创建后的永久链接

$ mg alloc # 当不指定路由时将创建一个随机的路由
https://changkun.de/midgard/random/fboVP8u4xNMHfvsv2EeLzL.txt
```

除了使用命令行之外，还可以使用快捷键进行触发：

- Linux: **Ctrl+Mod4+s**
- macOS: **Ctrl+Option+s**
- Windows: **Ctrl+Shift+s**

_创建好的连接会自动写回到当前的剪贴板，可以立刻直接在其他位置进行粘贴。_

### iOS 捷径 - Alloc

请在 iOS 设备上访问这个链接 [midgard-alloc](https://www.icloud.com/shortcuts/0964c0a651544604bd995cf1e723c573)，并根据提示输入相关配置数据（包括 midgard 服务端域名、服务端配置的用户名及密码）

## 跨设备剪贴板共享

midgard 守护进程将自动监控剪贴板并将内容与 midgard 服务器进行同步（仅限于文本和图片数据）。
因此，配合系统截图的一个可能的使用场景为：

1. 对屏幕进行截图
2. 使用 `mg alloc` 命令或者 **Ctrl+Option+s** （macOS）或者 **Ctrl+Mod4+s** (Linux) 或者 **Ctrl+Shift+s** (Windows) 键盘快捷键
3. 立即使用 **Ctrl+v** 进行粘贴

第二步执行完后将返回一个可以公开访问的 URL，并自动回写到当前设备的剪贴板中，因此第三步可以顺利进行。

此外，因为剪贴板内容将在服务端进行缓存，因此在任何连接的设备上（若接收到广播的剪贴板数据）也可以直接对剪贴板内容进行粘贴。

### iOS 捷径 - Clipboard

请在 iOS 设备上访问这个链接 [midgard-getclipboard](https://www.icloud.com/shortcuts/66c475e013e94dbf9f3714365d6c3f95) 和这个链接 [midgard-putclipboard](https://www.icloud.com/shortcuts/c1b98b1ae59045e59c1f302a634e5633)，并根据提示输入相关配置数据（包括 midgard 服务端域名、服务端配置的用户名及密码）

## 代码转图片 code2img

将任意一个拷贝到剪贴板中的代码转换为图片，可以使用下面的命令：

```sh
$ mg code2img # 读取剪贴板的内容，并转化为可公开访问的图片
https://changkun.de/midgard/code/201218-204010
https://changkun.de/midgard/code/201218-204010.png
```

或者直接读取某个指定的代码文件进行创建：

```sh
$ mg code2img /path/to/your/file  # 读取指定的文件内容，并转化为可公开访问的图片
https://changkun.de/midgard/code/201218-204010
https://changkun.de/midgard/code/201218-204010.png
```

甚至指定文件的行号，选择性的进行转换：

```sh
$ mg code2img /path/to/your/file/ -l 5:10 # 选择行号从 5 到 10
https://changkun.de/midgard/code/201218-204010
https://changkun.de/midgard/code/201218-204010.png
```

code2img 服务的所有内容可以在这个路由下找到合集：/midgard/code，例如 https://changkun.de/midgard/code

### iOS 捷径 - code2img

请在 iOS 设备上访问这个链接 [midgard-code2img](https://www.icloud.com/shortcuts/73f978c0179642b5bc2c31aba300b25a)，并根据提示输入相关配置数据（包括 midgard 服务端域名、服务端配置的用户名及密码）

## 许可

版权所有 2020-2021 [欧长坤](https://changkun.de)。保留所有权利。