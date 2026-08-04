# Manual Mailbox Production & Exact Mailbox Table Ordering Design

## Problem Description
Currently, initiating production in the ChatGPT Register Admin application only allows specifying a numeric count. When running in "Login & Setup 2FA Only" (`login_only`) mode, all verified mailboxes in the database are converted into `registered` status regardless of the specified count. Additionally, users want the ability to manually select specific mailboxes for production, and ensure that accounts displayed in the Accounts table match the exact order of mailboxes in the Mailboxes table (`id ASC`).

## Proposed Architecture & Workflow

### 1. Backend API & Producer Updates (`internal/handlers/produce.go`, `internal/producer/producer.go`)
- **API Request Payload**: Update `/api/produce` request struct:
  ```go
  type ProduceRequest struct {
      Count      int    `json:"count"`
      LoginOnly  bool   `json:"login_only"`
      MailboxIDs []uint `json:"mailbox_ids"`
  }
  ```
- **Producer Config**:
  - Add `MailboxIDs []uint` to `producer.Config`.
- **Producer `nextJob()` & Registration Creation Logic**:
  - Respect `MailboxIDs` filtering: If `MailboxIDs` is non-empty, only query mailboxes matching `id IN (mailbox_ids)`.
  - Cap `LoginOnly` initialization: In `LoginOnly` mode, only create `Registration` records for mailboxes up to `target` count (or within `MailboxIDs`), ordered by `mailbox_id ASC`.
  - Prevent converting all DB mailboxes at once when `LoginOnly` is enabled.
- **Accounts Table Ordering (`internal/handlers/registration.go`)**:
  - Update `List` query in `registration.go` to order by `mailbox_id ASC, id ASC` so accounts retain the exact sequence matching the Mailboxes table.

### 2. Frontend Modal & Selection (`static/accounts.html`, `static/accounts.js`)
- **Modal Mode Selection**:
  - In `#produce-modal`, add a mode selector:
    - `Auto by Count` (`#produce-mode-auto`)
    - `Manual Mailbox Selection` (`#produce-mode-manual`)
- **Manual Selection UI**:
  - When `Manual Mailbox Selection` is selected:
    - Fetch verified mailboxes from `/api/mailboxes?status=verified&size=1000` (sorted by `id ASC`).
    - Render a scrollable list with checkboxes, email address, provider badge, and a "Select All" toggle + search box.
    - Show selected count dynamically.
- **Start Production Payload**:
  - If manual mode is active, collect checked mailbox IDs and send `mailbox_ids: [...]`.
  - If auto mode is active, send `count: N` and `mailbox_ids: []`.

### 3. Mailboxes Page Direct Production Shortcut (`static/mailboxes.html`, `static/mailboxes.js`)
- Add a **"Produce Selected"** button (`#mb-batch-produce`) to the batch selection bar in `mailboxes.html`.
- Clicking this passes selected mailbox IDs directly to `accounts.html?produce_ids=1,2,5` or opens `#produce-modal` with those mailboxes pre-checked.

## Verification Plan
1. `go build ./...`: Ensure binary compiles cleanly.
2. `go test ./...`: Pass all unit tests.
3. Test auto mode with `count = 3` and verify only 3 accounts are claimed.
4. Test manual mode with selected mailbox IDs and verify production runs strictly for those mailboxes.
5. Verify ordering on Accounts page matches Mailboxes page (`id ASC`).
