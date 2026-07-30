# chatgpt-register

> **Automated ChatGPT Batch Registration & Management Platform** · Automated Headless Browser · 30s Rapid Registration · 100% Success Rate · One-Click Fission Sub-Accounts

---

🌐 **Image Gen Site** [vividai.run](https://vividai.run) &nbsp;|&nbsp;
👥 **QQ Group** [1106849765](https://qm.qq.com/q/1106849765) &nbsp;|&nbsp;
🐧 **QQ** 1114639355 &nbsp;|&nbsp;
🛒 **Shop** [pay.ldxp.cn/shop/chiyi](https://pay.ldxp.cn/shop/chiyi) &nbsp;|&nbsp;
✉️ **Email** [vividairun@gmail.com](mailto:vividairun@gmail.com)

Languages: [中文](README.md) | [English](README_EN.md) | [Tiếng Việt](README_VI.md)

---

## ✨ Core Advantages

| 🚀 30s Rapid Registration | ✅ 100% Success Rate | 🔁 Mother-to-Fission Sub-Accounts |
|:---:|:---:|:---:|
| Rod browser automation + Stealth anti-detection, 0 manual intervention | Automatic OTP code reading from mailbox APIs | 1 mother account + N alias sub-accounts per mailbox for exponential growth |

| 🌐 Proxy Pool Rotation | 📊 Visual Admin Dashboard | 📦 Zero-Dependency Deployment |
|:---:|:---:|:---:|
| Built-in proxy pool rotation per account, multi-IP concurrent registration | Glassmorphism UI, real-time metrics dashboard + live execution logs | Single pure Go binary, zero setup required, download and run |

---

## 🤖 Headless Registration Technical Highlights

> Powered by **go-rod + rod/stealth** driving genuine Chromium engine to simulate human operations, completely bypassing OpenAI bot detection.

### Registration Workflow (Fully Automated)

```
Launch Browser (Headless / Headed configurable)
    ↓
Open ChatGPT Registration Page & Inject Stealth Script (Bypass bot detection)
    ↓
Auto-fill Email + Random Password
    ↓
Listen to Mailbox API in real-time, auto-extract 6-digit OTP code & fill in (max 3 min timeout)
    ↓
Complete Registration → Retrieve accessToken
    ↓
Parse account details from accessToken (account_id / user_id / plan_type)
    ↓
Export complete auth.json (containing access_token / account_id, etc.)
    ↓
Save to Database & update status to "Registered"
```

### Key Technical Features

| Feature | Description |
|---------|-------------|
| **Stealth Anti-Detection** | Injects rod/stealth scripts to remove `navigator.webdriver` and browser automation flags |
| **Automatic OTP Extraction** | Integrates directly with mailbox APIs (Outlook / Gmail / varymail) with 5s polling |
| **Consistent IP & Browser Context** | Registration and subsequent API requests share the same egress proxy |
| **GeoIP Auto Detection** | Detects proxy location before registration, auto-adjusts browser language & timezones |
| **Automatic Chromium Download** | Auto-downloads matching Chromium binary on first startup |
| **Headless Mode** | Enables server / VPS deployment without requiring a display |
| **Screenshot Auditing** | Captures screenshots at key registration steps for fast troubleshooting |
| **Concurrency Safe** | Multi-task concurrent execution with isolated browser contexts |

---

## 🖼️ Screenshot Preview

| Dashboard | Account Management |
|:---:|:---:|
| ![Dashboard](./screenshots/dashboard.png) | ![Account Management](./screenshots/accounts.png) |

| Execution Logs | Mailbox Management |
|:---:|:---:|
| ![Execution Logs](./screenshots/accounts-log.png) | ![Mailbox Management](./screenshots/mailboxes.png) |

| Mail Fetcher (Auto Read Verification Code) |
|:---:|
| ![Mail Fetcher](./screenshots/mailboxes-mail.png) |

---

## 🏗️ Project Architecture

```
chatgpt-register/
├── main.go                  # Entry: Gin routes & static assets embedding
├── internal/
│   ├── auth/                # JWT Auth service (single token, auto renew, persistence)
│   ├── browserboot/         # Rod browser lifecycle (auto-download Chromium)
│   ├── codexreg/            # ChatGPT registration core logic (browser automation + Stealth)
│   │   ├── browser.go       # Browser instance wrapper
│   │   ├── codex.go         # Registration workflow automation
│   │   ├── geoip.go         # IP location detection
│   │   └── codexreg.go      # Registration task entry
│   ├── db/                  # SQLite database initialization (Pure Go driver, no CGO)
│   ├── emailalias/          # Email alias generation (Sub-account fission)
│   ├── handlers/            # HTTP API Handler layer
│   ├── mailfetch/           # Mailbox fetcher (Auto OTP reader)
│   ├── models/              # GORM Data models (Admin / Registration / Mailbox / Setting)
│   ├── producer/            # Registration task dispatcher & fission manager
│   ├── replenish/           # Auto replenishment service to image2api
│   └── varymail/            # vary.email API integration
└── static/                  # Embedded frontend static assets
    ├── i18n/                # i18n dictionaries (zh.js, en.js, vi.js, i18n.js)
    ├── dashboard.html        # Main dashboard
    ├── accounts.html/js      # Accounts management
    ├── mailboxes.html/js     # Mailboxes management
    ├── settings.html         # System settings
    ├── login.html            # Login page
    ├── layout.js             # Shared layout & navigation sidebar
    └── style.css             # Glassmorphism theme CSS (35KB)
```

**Tech Stack:** Go · Gin · GORM · SQLite (Pure Go) · go-rod · rod/stealth · JWT · Vanilla HTML5/CSS3/JS

---

## 🚀 Quick Start

### Option 1: Direct Run (Recommended)

Download the pre-compiled binary from Releases and run:

```bash
# Windows
./chatgpt-register.exe

# Linux
./chatgpt-register-linux
```

Open your browser at [http://localhost:9000](http://localhost:9000)

### Option 2: Run from Source

```bash
git clone https://github.com/duyplus/chatgpt-register
cd chatgpt-register
go run .
```

### Option 3: Compile Binary

```bash
# Windows
go build -o chatgpt-register.exe .

# Linux
GOOS=linux go build -o chatgpt-register-linux .
```

### Custom Port

```bash
ADDR=8080 ./chatgpt-register.exe
```

---

## 🔐 Authentication

- **Default Credentials:** `admin` / `admin123`
- Please change your password in **System Settings** after first login (length > 6 characters).

---

## ⚙️ User Guide

### Step 1: Import Mailboxes

Go to **Mailbox Management**, supports two import methods:
- **Single Add:** Fill in email, password, and provider.
- **Batch Import:** Click "Batch Import Mailboxes", format per line:
  ```
  email----password----client_id----refresh_token
  ```

### Step 2: Configure System Settings

Go to **System Settings**, configure concurrency, fission count, email source (`Outlook` or `varymail`), and proxy pool.

### Step 3: Start Production

1. Go to **Dashboard**, click **Produce** button.
2. Input target registration count.
3. System automatically registers mother accounts → fissions sub-accounts → auto-retries on failure.

---

## ❓ FAQ

**Q: Browser startup is slow on first launch?**
> A: Chromium binary (~150MB) is downloaded automatically on first startup. Subsequent launches are instant.

**Q: Can I run without proxies?**
> A: Yes. Leave proxy settings blank for direct connection. For large-scale batch registration, using a proxy pool is recommended to avoid rate limits.

---

## ⭐ Star History

If you find this project useful, please give it a Star!
