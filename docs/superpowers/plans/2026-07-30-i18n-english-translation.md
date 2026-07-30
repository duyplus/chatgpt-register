# English i18n Translation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add complete English i18n translation support and a language switcher UI to `chatgpt-register`.

**Architecture:** Create lightweight `zh.js`, `en.js`, and `i18n.js` scripts in `static/i18n/`. Use DOM data attributes (`data-i18n`, `data-i18n-ph`) and a dynamic JavaScript `t(key)` lookup function to translate all static and dynamic UI elements seamlessly.

**Tech Stack:** JavaScript (ES6+ vanilla), HTML5, CSS3, Go (Gin web server static files).

## Global Constraints

- Preserve all existing API logic, CSS styles, and functionality without breaking any existing features.
- Support both English (`en`) and Chinese (`zh`), defaulting to `zh` if no preference is set in `localStorage`.
- Maintain clean, self-contained code without adding external npm/CDN dependencies.

---

### Task 1: Create i18n Dictionaries (`static/i18n/zh.js` & `static/i18n/en.js`)

**Files:**
- Create: `static/i18n/zh.js`
- Create: `static/i18n/en.js`

**Interfaces:**
- Produces: `window.I18N_DICTS.zh` and `window.I18N_DICTS.en` dictionary objects containing all UI strings.

- [ ] **Step 1: Create `static/i18n/zh.js` dictionary**

```javascript
window.I18N_DICTS = window.I18N_DICTS || {};
window.I18N_DICTS.zh = {
  // Navigation
  "nav.brand": "ChatGPT",
  "nav.group": "导航",
  "nav.dashboard": "仪表盘",
  "nav.mailboxes": "邮箱管理",
  "nav.accounts": "账户管理",
  "nav.settings": "系统设置",
  "nav.logout": "退出登录",

  // Login
  "login.title": "ChatGPT 注册管理系统",
  "login.desc": "请输入访问密码登录",
  "login.password": "密码",
  "login.placeholder": "请输入密码",
  "login.btn": "登录",
  "login.copyright": "Copyright © 2026 ChatGPT 注册管理",
  "login.empty_pass": "请输入密码",
  "login.failed": "登录失败",

  // Dashboard
  "dash.title": "仪表盘",
  "dash.checking_browser": "正在检查浏览器...",
  "dash.st_total": "账户总数",
  "dash.st_registered": "已注册",
  "dash.st_registering": "注册中",
  "dash.st_failed": "注册失败",
  "dash.st_stock": "可出库库存",
  "dash.st_mailboxes": "可用邮箱",
  "dash.produce_task": "生产任务",
  "dash.idle": "空闲",
  "dash.running": "运行中",
  "dash.registered_target": "已注册 / 目标",
  "dash.pending": "待生产",
  "dash.failed_pill": "失败",
  "dash.status_ratio": "账户状态占比",
  "dash.account": "账户",
  "dash.no_data": "暂无数据",
  "dash.7days": "近 7 天产量",
  "dash.no_records": "暂无产量记录",
  "dash.structure": "账号结构",
  "dash.mother": "母号",
  "dash.fission": "裂变子号",
  "dash.shipped": "已出库",
  "dash.mb_ratio": "可用邮箱 / 邮箱总数",
  "dash.plan_dist": "套餐分布",
  "dash.recent_accounts": "最近账户",

  // Accounts
  "acc.title": "账户管理",
  "acc.batch_reg": "批量注册",
  "acc.export_auth": "导出 Auth JSON",
  "acc.add": "添加账户",
  "acc.delete": "删除选中",
  "acc.status_pending": "待生产",
  "acc.status_registering": "注册中",
  "acc.status_registered": "已注册",
  "acc.status_failed": "注册失败",
  "acc.col_id": "ID",
  "acc.col_email": "Email",
  "acc.col_mother": "母号",
  "acc.col_status": "状态",
  "acc.col_shipped": "出库",
  "acc.col_actions": "操作",
  "acc.btn_log": "日志",
  "acc.btn_snap": "截图",

  // Mailboxes
  "mb.title": "邮箱管理",
  "mb.add": "添加邮箱",
  "mb.import": "批量导入",
  "mb.verify": "验证 IMAP",
  "mb.fetch": "邮件取件",
  "mb.col_email": "邮箱地址",
  "mb.col_type": "类型",
  "mb.col_status": "状态",
  "mb.col_actions": "操作",

  // Settings
  "sett.title": "系统设置",
  "sett.save": "保存设置",

  // Common
  "common.cancel": "取消",
  "common.confirm": "确认",
  "common.success": "操作成功",
  "common.error": "操作失败",
};
```

- [ ] **Step 2: Create `static/i18n/en.js` dictionary**

```javascript
window.I18N_DICTS = window.I18N_DICTS || {};
window.I18N_DICTS.en = {
  // Navigation
  "nav.brand": "ChatGPT",
  "nav.group": "Navigation",
  "nav.dashboard": "Dashboard",
  "nav.mailboxes": "Mailboxes",
  "nav.accounts": "Accounts",
  "nav.settings": "Settings",
  "nav.logout": "Logout",

  // Login
  "login.title": "ChatGPT Registration Admin",
  "login.desc": "Please enter access password to login",
  "login.password": "Password",
  "login.placeholder": "Enter password",
  "login.btn": "Login",
  "login.copyright": "Copyright © 2026 ChatGPT Reg Admin",
  "login.empty_pass": "Password cannot be empty",
  "login.failed": "Login failed",

  // Dashboard
  "dash.title": "Dashboard",
  "dash.checking_browser": "Checking browser status...",
  "dash.st_total": "Total Accounts",
  "dash.st_registered": "Registered",
  "dash.st_registering": "Registering",
  "dash.st_failed": "Registration Failed",
  "dash.st_stock": "Available Stock",
  "dash.st_mailboxes": "Available Mailboxes",
  "dash.produce_task": "Production Task",
  "dash.idle": "Idle",
  "dash.running": "Running",
  "dash.registered_target": "Registered / Target",
  "dash.pending": "Pending",
  "dash.failed_pill": "Failed",
  "dash.status_ratio": "Account Status Ratio",
  "dash.account": "Accounts",
  "dash.no_data": "No Data",
  "dash.7days": "7-Day Production",
  "dash.no_records": "No Production Records",
  "dash.structure": "Account Structure",
  "dash.mother": "Mother Account",
  "dash.fission": "Fission Sub-account",
  "dash.shipped": "Shipped",
  "dash.mb_ratio": "Available / Total Mailboxes",
  "dash.plan_dist": "Plan Distribution",
  "dash.recent_accounts": "Recent Accounts",

  // Accounts
  "acc.title": "Account Management",
  "acc.batch_reg": "Batch Register",
  "acc.export_auth": "Export Auth JSON",
  "acc.add": "Add Account",
  "acc.delete": "Delete Selected",
  "acc.status_pending": "Pending",
  "acc.status_registering": "Registering",
  "acc.status_registered": "Registered",
  "acc.status_failed": "Failed",
  "acc.col_id": "ID",
  "acc.col_email": "Email",
  "acc.col_mother": "Mother Account",
  "acc.col_status": "Status",
  "acc.col_shipped": "Shipped",
  "acc.col_actions": "Actions",
  "acc.btn_log": "Logs",
  "acc.btn_snap": "Screenshot",

  // Mailboxes
  "mb.title": "Mailbox Management",
  "mb.add": "Add Mailbox",
  "mb.import": "Batch Import",
  "mb.verify": "Verify IMAP",
  "mb.fetch": "Mail Fetcher",
  "mb.col_email": "Mailbox Address",
  "mb.col_type": "Type",
  "mb.col_status": "Status",
  "mb.col_actions": "Actions",

  // Settings
  "sett.title": "System Settings",
  "sett.save": "Save Settings",

  // Common
  "common.cancel": "Cancel",
  "common.confirm": "Confirm",
  "common.success": "Success",
  "common.error": "Error",
};
```

- [ ] **Step 3: Commit Dictionaries**

```bash
git add static/i18n/zh.js static/i18n/en.js
git commit -m "feat(i18n): add Chinese and English dictionary files"
```

---

### Task 2: Create i18n Core Engine (`static/i18n/i18n.js`)

**Files:**
- Create: `static/i18n/i18n.js`

**Interfaces:**
- Produces: `window.i18n` with methods `getLang()`, `setLang(lang)`, `t(key, fallback)`, `translateDOM()`.

- [ ] **Step 1: Write `static/i18n/i18n.js` engine**

```javascript
(function () {
  const LANG_KEY = 'app_lang';

  function getLang() {
    return localStorage.getItem(LANG_KEY) || 'zh';
  }

  function setLang(lang) {
    if (!['zh', 'en'].includes(lang)) return;
    localStorage.setItem(LANG_KEY, lang);
    translateDOM();
    window.dispatchEvent(new CustomEvent('langchanged', { detail: { lang } }));
  }

  function t(key, fallback) {
    const lang = getLang();
    const dict = (window.I18N_DICTS && window.I18N_DICTS[lang]) || {};
    return dict[key] !== undefined ? dict[key] : (fallback !== undefined ? fallback : key);
  }

  function translateDOM(root) {
    const scope = root || document;
    
    // Text elements
    scope.querySelectorAll('[data-i18n]').forEach(el => {
      const key = el.dataset.i18n;
      const translated = t(key);
      if (translated) el.textContent = translated;
    });

    // Placeholders
    scope.querySelectorAll('[data-i18n-ph]').forEach(el => {
      const key = el.dataset.i18nPh;
      const translated = t(key);
      if (translated) el.placeholder = translated;
    });

    // Titles
    scope.querySelectorAll('[data-i18n-title]').forEach(el => {
      const key = el.dataset.i18nTitle;
      const translated = t(key);
      if (translated) el.title = translated;
    });

    // Sync language selector UI elements if present
    document.querySelectorAll('.lang-switcher-select').forEach(sel => {
      sel.value = getLang();
    });
  }

  window.i18n = { getLang, setLang, t, translateDOM };
  window.t = t;

  document.addEventListener('DOMContentLoaded', () => {
    translateDOM();
  });
})();
```

- [ ] **Step 2: Commit i18n Engine**

```bash
git add static/i18n/i18n.js
git commit -m "feat(i18n): create core i18n engine with auto DOM translator"
```

---

### Task 3: Integrate i18n & Language Switcher into `layout.js` & HTML Pages

**Files:**
- Modify: `static/layout.js`
- Modify: `static/login.html`
- Modify: `static/dashboard.html`
- Modify: `static/accounts.html`
- Modify: `static/mailboxes.html`
- Modify: `static/settings.html`

- [ ] **Step 1: Include i18n scripts in HTML heads**

Add the script tags before `layout.js` in all HTML files:
```html
<script src="/static/i18n/zh.js"></script>
<script src="/static/i18n/en.js"></script>
<script src="/static/i18n/i18n.js"></script>
```

- [ ] **Step 2: Add Language Switcher to Sidebar in `layout.js`**

In `layout.js`, update the sidebar generation to render menu labels with `data-i18n` and append a language switcher widget above logout item:
```javascript
  const lang = window.i18n ? window.i18n.getLang() : 'zh';
  const langHtml = `
    <div class="menu-item lang-switcher-item">
      <svg viewBox="0 0 24 24" style="width:18px;height:18px;margin-right:8px;fill:currentColor;"><path d="M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zm6.93 6h-2.95a15.65 15.65 0 0 0-1.38-3.56A8.03 8.03 0 0 1 18.92 8zM12 4.04c.83 1.2 1.48 2.53 1.91 3.96h-3.82c.43-1.43 1.08-2.76 1.91-3.96zM4.26 14C4.1 13.36 4 12.69 4 12s.1-1.36.26-2h3.38c-.08.66-.14 1.32-.14 2 0 .68.06 1.34.14 2H4.26zm.82 2h2.95c.32 1.25.78 2.45 1.38 3.56A8.03 8.03 0 0 1 5.08 16zm2.95-8H5.08a8.03 8.03 0 0 1 4.33-3.56A15.65 15.65 0 0 0 8.03 8zM12 19.96c-.83-1.2-1.48-2.53-1.91-3.96h3.82c-.43 1.43-1.08 2.76-1.91 3.96zM14.34 14H9.66c-.09-.66-.16-1.32-.16-2 0-.68.07-1.35.16-2h4.68c.09.65.16 1.32.16 2 0 .68-.07 1.34-.16 2zm.25 5.56c.6-1.11 1.06-2.31 1.38-3.56h2.95a8.03 8.03 0 0 1-4.33 3.56zM16.36 14c.08-.66.14-1.32.14-2 0-.68-.06-1.34-.14-2h3.38c.16.64.26 1.31.26 2s-.1 1.36-.26 2h-3.38z"/></svg>
      <select class="px-input lang-switcher-select" onchange="window.i18n && window.i18n.setLang(this.value)" style="padding: 2px 6px; font-size: 13px; cursor: pointer; background: transparent; border: none; color: inherit;">
        <option value="zh" ${lang === 'zh' ? 'selected' : ''}>中文</option>
        <option value="en" ${lang === 'en' ? 'selected' : ''}>English</option>
      </select>
    </div>`;
```

- [ ] **Step 3: Tag HTML elements with `data-i18n` in all HTML pages**

Update headers, sidebar labels, titles, table column headers, and form inputs with `data-i18n="key"` and `data-i18n-ph="key"`.

- [ ] **Step 4: Commit UI Changes**

```bash
git add static/layout.js static/*.html
git commit -m "feat(i18n): integrate language switcher and i18n tags in HTML pages"
```

---

### Task 4: Verification & End-to-End Test

- [ ] **Step 1: Open website and test language switching**

Switch language from `中文` to `English` on Dashboard, Accounts, Mailboxes, and Settings.
Verify that page elements reflect English translations immediately.

- [ ] **Step 2: Refresh page to test persistence**

Refresh the browser and ensure the selected language (`en`) persists across page reloads.

- [ ] **Step 3: Commit Final Verification**

```bash
git commit --allow-empty -m "test(i18n): verify language switcher and translation persistence"
```
