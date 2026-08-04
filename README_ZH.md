# chatgpt-register

> **ChatGPT 账号全自动批量注册管理台** · 无头浏览器全自动 · 30 秒极速注册 · 百分百成功率 · 一键裂变子号

---

🌐 **生图站** [vividai.run](https://vividai.run) &nbsp;|&nbsp;
👥 **QQ 交流群** [1106849765](https://qm.qq.com/q/1106849765) &nbsp;|&nbsp;
🐧 **QQ** 1114639355 &nbsp;|&nbsp;
🛒 **小店** [pay.ldxp.cn/shop/chiyi](https://pay.ldxp.cn/shop/chiyi) &nbsp;|&nbsp;
✉️ **邮箱** [vividairun@gmail.com](mailto:vividairun@gmail.com)

Languages: [English](README.md) | [Tiếng Việt](README_VI.md) | [中文](README_ZH.md)

---

## ✨ 核心优势

| 🚀 30 秒极速注册 | ✅ 百分百成功率 | 🔁 母号裂变子号 |
|:---:|:---:|:---:|
| Rod 浏览器自动化 + Stealth 反检测，全程无需人工干预 | 验证码自动从邮箱读取，全流程零手动操作 | 每个邮箱注册 1 个母号 + N 个别名子号，账号数量指数级增长 |

| 🌐 代理池轮转 | 📊 可视化管理台 | 📦 零依赖部署 |
|:---:|:---:|:---:|
| 内置代理池按账号轮转，多 IP 并发注册不封号 | 毛玻璃风格 UI，实时仪表盘 + 执行日志可视化 | 纯 Go 编译单文件，无需安装任何环境，下载即用 |

---

## 🤖 无头注册——技术亮点

> 基于 **go-rod + rod/stealth** 驱动真实 Chromium 内核，模拟真人操作全程自动完成注册，OpenAI 无法识别为机器人。

### 注册全流程（全自动，无需人工）

```
启动浏览器（无头/有头可配）
    ↓
打开 ChatGPT 注册页，注入 Stealth 脚本（绕过 bot 检测）
    ↓
自动填写邮箱 + 随机密码
    ↓
实时监听邮箱，自动读取 6 位验证码并填入（最长等待 3 分钟）
    ↓
完成注册 → 获取 accessToken
    ↓
从 accessToken 解出账号信息（account_id / user_id / plan_type）
    ↓
导出完整 auth.json（含 access_token / account_id 等）
    ↓
写入数据库，账号状态更新为「已注册」
```

### 关键技术点

| 特性 | 说明 |
|------|------|
| **Stealth 反检测** | 注入 rod/stealth 脚本，抹除 `navigator.webdriver` 等浏览器自动化特征，绕过 OpenAI 的机器人识别 |
| **验证码自动读取** | 直接对接邮箱 API（Outlook/Gmail/varymail），每 5 秒轮询一次，无需人工复制粘贴 |
| **IP 与浏览器一致** | 浏览器注册和后续 API 请求走同一个代理出口，保证同 IP 签发 Token，避免风控拦截 |
| **GeoIP 自动检测** | 注册前检测代理 IP 归属地，自动设置匹配的浏览器语言 / 时区，降低异常风控概率 |
| **Chromium 自动下载** | 首次运行自动下载匹配版本的 Chromium，无需手动安装 Chrome |
| **无头模式** | 生产环境开启无头模式，无需显示器，支持服务器 / VPS 部署 |
| **截图存证** | 注册每个关键步骤自动截图，失败时可直接在管理台查看现场图，快速定位问题 |
| **并发安全** | 多个注册任务并发执行，每个任务独立浏览器上下文，互不干扰 |

---

## 截图预览

| 仪表盘 | 账户管理 |
|:---:|:---:|
| ![仪表盘](./screenshots/dashboard.png) | ![账户管理](./screenshots/accounts.png) |

| 执行日志 | 邮箱管理 |
|:---:|:---:|
| ![执行日志](./screenshots/accounts-log.png) | ![邮箱管理](./screenshots/mailboxes.png) |

| 邮件取件（自动读取验证码） |
|:---:|
| ![邮件取件](./screenshots/mailboxes-mail.png) |

---

## 🏗️ 项目架构

```
chatgpt-register/
├── main.go                  # 入口：Gin 路由注册 + 静态文件嵌入
├── internal/
│   ├── auth/                # JWT 鉴权服务（单 token、自动续期、落库）
│   ├── browserboot/         # Rod 浏览器生命周期管理（启动时自动下载 Chromium）
│   ├── codexreg/            # ChatGPT 注册核心逻辑（浏览器自动化 + Stealth）
│   │   ├── browser.go       # 浏览器实例封装
│   │   ├── codex.go         # 注册流程自动化
│   │   ├── geoip.go         # IP 归属地检测（代理验证）
│   │   └── codexreg.go      # 注册任务入口
│   ├── db/                  # SQLite 数据库初始化（纯 Go 驱动，无需 CGO）
│   ├── emailalias/          # 邮箱别名生成（裂变子号）
│   ├── handlers/            # HTTP 接口层（Gin Handler）
│   │   ├── auth.go          # 登录 / 改密接口
│   │   ├── registration.go  # 账户 CRUD + 日志 + 截图接口
│   │   ├── produce.go       # 批量生产控制（启动 / 状态 / 停止）
│   │   ├── mailbox.go       # 邮箱 CRUD + 取件接口
│   │   ├── proxy.go         # 代理测试接口
│   │   └── settings.go      # 系统设置接口
│   ├── mailfetch/           # 邮件取件（自动读取验证码）
│   ├── models/              # GORM 数据模型（Admin / Registration / Mailbox / Setting）
│   └── producer/            # 批量注册调度器（并发控制 + 裂变策略）
└── static/                  # 前端静态页面（嵌入二进制，无需 Web 服务器）
    ├── dashboard.html        # 仪表盘
    ├── accounts.html/js      # 账户管理
    ├── mailboxes.html/js     # 邮箱管理
    ├── settings.html         # 系统设置
    ├── login.html            # 登录页
    ├── layout.js             # 公共布局 / 侧边栏
    └── style.css             # 毛玻璃主题 CSS（35KB 精心打磨）
```

**技术栈：** Go · Gin · GORM · SQLite（纯 Go 驱动）· go-rod · rod/stealth · JWT · 原生 H5

---

## 🚀 快速开始

### 方式一：直接运行（推荐）

下载 Release 中对应系统的可执行文件，双击运行或：

```bash
# Windows
./chatgpt-register.exe

# Linux
./chatgpt-register-linux
```

浏览器打开 [http://localhost:9000](http://localhost:9000)

### 方式二：源码运行

```bash
git clone https://github.com/duyplus/chatgpt-register
cd chatgpt-register
go run .
```

### 方式三：自行编译

```bash
# Windows
go build -o chatgpt-register.exe .

# Linux
GOOS=linux go build -o chatgpt-register-linux .
```

### 自定义端口

```bash
ADDR=8080 ./chatgpt-register.exe
```

---

## 🔐 登录

- 默认账号：`admin` / `admin123`
- 首次登录后请立即在「系统设置」修改密码（密码长度 > 6 位）

---

## 📋 功能说明

### 批量生产（核心功能）

1. 在「邮箱管理」导入邮箱（支持批量 CSV 导入）
2. 在「系统设置」配置并发数、裂变数量、代理池
3. 在「仪表盘」点击「生产」，设置目标数量，一键启动
4. 实时查看进度、成功率、执行日志和注册截图

---

## ⚙️ 使用指南

### 第一步：导入邮箱

进入「邮箱管理」，支持两种方式导入：

- **手动添加**：填写邮箱地址、密码、服务商
- **批量导入**：点击「批量导入邮箱」，每行一条，格式：
  ```
  email|password|refresh_token|client_id
  ```

---

## ❓ 常见问题

**Q：浏览器第一次启动很慢？**
> A：首次运行会自动下载 Chromium（约 150MB），下载完成后后续启动秒开。

**Q：不配置代理可以用吗？**
> A：可以，留空即直连。但大量并发注册建议配置代理池，避免 IP 被限流。

---

## ⭐ Star History

如果觉得好用，欢迎 Star！
