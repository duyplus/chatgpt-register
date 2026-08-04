# Manual Mailbox Production & Ordering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to manually select specific mailboxes for production run, cap `LoginOnly` initialization to requested count, and order accounts in table matching `mailboxes` order (`id ASC`).

**Architecture:** Extend `/api/produce` handler and `Producer` to accept `mailbox_ids`. Filter `nextJob()` to target `mailbox_ids` and cap `LoginOnly` creation. Update `RegistrationList` database ordering to `mailbox_id ASC, id ASC`. Update UI modals and batch actions on frontend.

**Tech Stack:** Go (Gin, GORM), Vanilla JavaScript, HTML5/CSS3.

## Global Constraints
- Target workspace: `c:\Users\ADMIN\Documents\chatgpt-register`
- Build command: `go build ./...`
- Test command: `go test ./...`

---

### Task 1: Backend `Produce` Endpoint & `Producer` Filtering

**Files:**
- Modify: `internal/handlers/produce.go`
- Modify: `internal/producer/producer.go`
- Modify: `internal/handlers/registration.go`
- Modify: `internal/handlers/produce_test.go`

**Interfaces:**
- Consumes: `ProduceRequest` JSON with optional `mailbox_ids: []uint`
- Produces: `Producer.Start(count, loginOnly, mailboxIDs)`

- [ ] **Step 1: Write failing test for `Produce` handler & `nextJob` filtering**

Create or update `internal/handlers/produce_test.go` to test passing `mailbox_ids` and verifying target capping.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handlers -v`
Expected: FAIL due to missing parameter or logic.

- [ ] **Step 3: Update `Produce` handler in `internal/handlers/produce.go`**

Bind `mailbox_ids` from JSON input:
```go
var in struct {
    Count      int    `json:"count"`
    LoginOnly  bool   `json:"login_only"`
    MailboxIDs []uint `json:"mailbox_ids"`
}
```
Pass `in.MailboxIDs` to `Producer.Start`.

- [ ] **Step 4: Update `Producer.Start` & `Config` in `internal/producer/producer.go`**

Add `MailboxIDs []uint` to `Config`.
In `nextJob()`:
- If `cfg.MailboxIDs` is provided, filter query `db.Where("id IN ?", cfg.MailboxIDs)`.
- In `LoginOnly` mode, only create `Registration` records up to `target` (or `len(MailboxIDs)`).

- [ ] **Step 5: Update `RegistrationList` ordering in `internal/handlers/registration.go`**

Change query order from `created_at desc, id desc` to `mailbox_id ASC, id ASC`.

- [ ] **Step 6: Run tests and build to verify**

Run: `go test ./...` and `go build ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/handlers/produce.go internal/producer/producer.go internal/handlers/registration.go internal/handlers/produce_test.go
git commit -m "feat: add mailbox_ids filtering and cap login_only initialization"
```

---

### Task 2: Frontend Produce Modal Manual Selection Mode (`accounts.html` & `accounts.js`)

**Files:**
- Modify: `static/accounts.html`
- Modify: `static/accounts.js`

**Interfaces:**
- Consumes: `/api/mailboxes?status=verified&size=1000`
- Produces: `/api/produce` payload with `mailbox_ids`

- [ ] **Step 1: Update `#produce-modal` HTML in `static/accounts.html`**

Add mode switcher tabs (`Auto` vs `Manual Mailbox Selection`), manual mailbox list container with search input and "Select All" checkbox.

- [ ] **Step 2: Add Manual Selection JS logic in `static/accounts.js`**

Implement `switchProduceMode(mode)`, `loadVerifiedMailboxes()`, and update `startProduce()` to collect checked `mailbox_ids`.

- [ ] **Step 3: Test modal interactions locally**

Run: `go build ./...`
Verify modal opens, switching modes loads verified mailboxes, and payload sends `mailbox_ids`.

- [ ] **Step 4: Commit**

```bash
git add static/accounts.html static/accounts.js
git commit -m "feat: add manual mailbox selection mode to produce modal"
```

---

### Task 3: Add Batch Produce Button to Mailboxes Page (`mailboxes.html` & `mailboxes.js`)

**Files:**
- Modify: `static/mailboxes.html`
- Modify: `static/mailboxes.js`

**Interfaces:**
- Consumes: `mbSelected` set in `mailboxes.js`
- Produces: Redirects or opens `accounts.html` with selected mailbox IDs pre-loaded

- [ ] **Step 1: Add `#mb-batch-produce` button in `static/mailboxes.html`**

Add `<button class="px-btn primary" onclick="produceSelectedMailboxes()">Sản xuất mục đã chọn</button>` inside `.batch-bar`.

- [ ] **Step 2: Add `produceSelectedMailboxes()` in `static/mailboxes.js`**

Store selected IDs in `localStorage` or URL query param and navigate to `/accounts?produce_ids=...`.

- [ ] **Step 3: Handle URL param `produce_ids` in `accounts.js`**

When landing on `/accounts?produce_ids=1,2,5`, automatically open `#produce-modal` in manual mode with those mailboxes pre-checked.

- [ ] **Step 4: Build and test end-to-end**

Run: `go build ./...` and `go test ./...`

- [ ] **Step 5: Commit**

```bash
git add static/mailboxes.html static/mailboxes.js static/accounts.js
git commit -m "feat: add direct produce batch action on mailboxes page"
```
