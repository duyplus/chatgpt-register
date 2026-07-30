# Design Spec: English i18n Translation Support for chatgpt-register

## Overview
This specification details the implementation of full English translation support (`i18n`) and a dynamic language switcher for the `chatgpt-register` application. The existing static frontend interface (`login.html`, `dashboard.html`, `accounts.html`, `mailboxes.html`, `settings.html`, `layout.js`, `accounts.js`, `mailboxes.js`) currently features hardcoded Chinese (zh-CN) strings. This project introduces a zero-dependency, lightweight JavaScript i18n engine with complete English (`en`) and Chinese (`zh`) dictionaries, an automatic DOM translator, and a language selector UI.

---

## 1. Architecture & i18n Engine Design

### File Structure
The following i18n files will be created in `static/i18n/`:
- `static/i18n/zh.js`: English-to-Chinese key-value dictionary.
- `static/i18n/en.js`: Key-value dictionary containing all English translations.
- `static/i18n/i18n.js`: Core i18n management script.

### `i18n.js` Core Responsibilities
- **Language Detection & Persistence:** Loads language preference from `localStorage.getItem('app_lang')`. Defaults to `zh` (with full support for `en`).
- **Translation Function (`t(key, fallback)`):** Looks up translation keys in active dictionary (`window.I18N_DICTS[lang][key]`). If key is missing, returns fallback or key itself.
- **Automatic DOM Translation (`i18n.translateDOM()`):**
  - Scans DOM elements with `data-i18n="key"` and updates `textContent` or `innerHTML`.
  - Scans elements with `data-i18n-ph="key"` and updates `placeholder`.
  - Scans elements with `data-i18n-title="key"` and updates `title`.
- **Language Switching (`setLang(lang)`):** Saves the selected language to `localStorage`, updates dictionary, re-translates DOM, and dispatches a custom window event (`langchanged`).

---

## 2. User Interface & Language Switcher

### 1. Sidebar Language Selector (`layout.js`)
- In `layout.js`, add a language switcher element above the "Logout" menu option.
- Options: `🌐 English` and `🌐 中文`.
- Selecting an option immediately triggers `setLang(lang)` without forcing full page reload.

### 2. Login Page Language Selector (`login.html`)
- Position a small, elegant language toggle button (`EN / 中文`) in the top-right corner of `login.html`.

---

## 3. Scope of Translations

### HTML & JS Elements Covered
- **Layout & Navigation:** Sidebar items (Dashboard, Account Management, Mailbox Management, System Settings, Logout).
- **Dashboard (`dashboard.html`):** KPI stats labels, production task statuses, chart labels, donut legend, recent account headers.
- **Account Management (`accounts.html` / `accounts.js`):** Table columns, action buttons (Batch Register, Export Auth, Delete, Logs, Screenshots), status badges, modals.
- **Mailbox Management (`mailboxes.html` / `mailboxes.js`):** Table columns, buttons (Add Mailbox, Import, Verify IMAP, Mail Fetcher), modal forms.
- **System Settings (`settings.html`):** Form labels, placeholders, section headers, tooltips, toast notifications.
- **Login Page (`login.html`):** Form headers, password inputs, submit button, error messages.

---

## 4. Verification & Testing
- Switch languages between `en` and `zh` on each page (`dashboard`, `accounts`, `mailboxes`, `settings`, `login`).
- Ensure all static and dynamic text (tables, modal dialogs, toasts, placeholders) update accurately.
- Verify persistence in `localStorage` across page refreshes and navigation.
