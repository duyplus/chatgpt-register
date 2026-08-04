/* ===== Mailbox Management ===== */
let mbPage = 1;
const size = 20;
let mbCache = {};

function getMbStatusText(status) {
  const keys = {
    unverified: 'mb.status_unverified',
    verifying: 'mb.status_verifying',
    verify_failed: 'mb.status_verify_failed',
    verified: 'mb.status_verified',
  };
  return window.t(keys[status] || status, status || '');
}

let mbLoading = false;
const mbSelected = new Set();

async function loadMailboxes() {
  if (mbLoading) return;
  mbLoading = true;
  try {
    const q = document.getElementById('mb-search').value.trim();
    const status = document.getElementById('mb-filter').value;
    const params = new URLSearchParams({ page: mbPage, size });
    if (q) params.set('q', q);
    if (status) params.set('status', status);
    const r = await api('/api/mailboxes?' + params);
    const d = await r.json();
    mbCache = {};
    (d.data || []).forEach(x => (mbCache[x.id] = x));
    const fetchTitle = window.t('mb.fetch_title', 'Fetch Mail');
    const delTitle = window.t('acc.batch_delete', 'Delete');
    document.getElementById('mb-rows').innerHTML = (d.data || []).map(x => `
      <tr class="${mbSelected.has(x.id) ? 'row-sel' : ''}">
        <td class="col-check"><input type="checkbox" ${mbSelected.has(x.id) ? 'checked' : ''} onclick="toggleSelect(${x.id}, this.checked)"></td>
        <td>${esc(x.email)}</td>
        <td><span class="badge ${esc(x.provider || 'outlook')}">${esc(x.provider || 'outlook')}</span></td>
        <td><span class="badge ${esc(x.status)}" title="${esc(x.note || '')}">${getMbStatusText(x.status)}</span></td>
        <td>${fmtTime(x.created_at)}</td>
        <td>${fmtTime(x.updated_at)}</td>
        <td>
          ${x.status === 'verified' ? `<button class="icon-btn" title="${fetchTitle}" onclick="openMailModal(${x.id})">
            <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3 7 9 6 9-6"/></svg>
          </button>` : ''}
          <button class="icon-btn" title="Edit" onclick="editMailbox(${x.id})">
            <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg>
          </button>
          <button class="icon-btn danger" title="${delTitle}" onclick="delMailbox(${x.id})">
            <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2m2 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M10 11v6M14 11v6"/></svg>
          </button>
        </td>
      </tr>`).join('');
    const maxPage = Math.max(1, Math.ceil((d.total || 0) / size));
    renderPager('mb-pager', mbPage, maxPage, p => { mbPage = p; loadMailboxes(); });
    syncBatchBar();
  } finally {
    mbLoading = false;
  }
}

/* ===== Batch Selection ===== */
function toggleSelect(id, checked) {
  if (checked) mbSelected.add(id); else mbSelected.delete(id);
  syncBatchBar();
}

function toggleSelectAll(checked) {
  Object.keys(mbCache).forEach(id => {
    if (checked) mbSelected.add(Number(id)); else mbSelected.delete(Number(id));
  });
  loadMailboxes();
}

function clearSelection() {
  mbSelected.clear();
  loadMailboxes();
}

function syncBatchBar() {
  const bar = document.getElementById('mb-batch');
  bar.style.display = mbSelected.size ? 'flex' : 'none';
  const selTpl = window.t('mb.selected', `Selected ${mbSelected.size} items`);
  document.getElementById('mb-batch-count').textContent = selTpl.replace('{n}', mbSelected.size);
  const all = document.getElementById('mb-check-all');
  const ids = Object.keys(mbCache).map(Number);
  all.checked = ids.length > 0 && ids.every(id => mbSelected.has(id));
}

async function verifySelected() {
  if (!mbSelected.size) return;
  await runVerify([...mbSelected]);
}

function produceSelectedMailboxes() {
  const ids = [...mbSelected];
  if (!ids.length) return;
  location.href = '/accounts?produce_ids=' + ids.join(',');
}

async function delSelected() {
  const ids = [...mbSelected];
  if (!ids.length) return;
  if (!confirm('Delete selected ' + ids.length + ' mailboxes?')) return;
  for (const id of ids) {
    await api('/api/mailboxes/' + id, { method: 'DELETE' });
    mbSelected.delete(id);
  }
  toast(window.t('common.success', 'Deleted'));
  loadMailboxes();
}

/* ===== Batch Import ===== */
function openImportModal() {
  document.getElementById('import-text').value = '';
  const countTpl = window.t('mb.import_count', 'Identified 0 mailboxes');
  document.getElementById('import-count').textContent = countTpl.replace('{n}', 0);
  document.getElementById('import-modal').style.display = 'flex';
}

function updateImportCount() {
  const n = parseImportLines(document.getElementById('import-text').value).length;
  const countTpl = window.t('mb.import_count', `Identified ${n} mailboxes`);
  document.getElementById('import-count').textContent = countTpl.replace('{n}', n);
}

(function () {
  const ta = document.getElementById('import-text');
  if (!ta) return;
  ta.addEventListener('input', updateImportCount);
  ta.addEventListener('dragover', e => { e.preventDefault(); ta.classList.add('drag'); });
  ta.addEventListener('dragleave', () => ta.classList.remove('drag'));
  ta.addEventListener('drop', e => {
    e.preventDefault();
    ta.classList.remove('drag');
    const f = e.dataTransfer.files && e.dataTransfer.files[0];
    if (!f) return;
    const reader = new FileReader();
    reader.onload = () => { ta.value = reader.result; updateImportCount(); };
    reader.readAsText(f);
  });
})();

function parseImportLines(text) {
  const items = [];
  text.split(/\r?\n/).forEach(line => {
    line = line.trim();
    if (!line) return;
    let parts = [];
    if (line.includes('----')) {
      parts = line.split(/----+/).map(p => p.trim());
    } else if (line.includes('---')) {
      parts = line.split(/---+/).map(p => p.trim());
    } else if (line.includes('--')) {
      parts = line.split(/--+/).map(p => p.trim());
    } else if (line.includes('|')) {
      parts = line.split('|').map(p => p.trim());
    } else if (line.includes('\t')) {
      parts = line.split('\t').map(p => p.trim());
    } else if (line.includes(':')) {
      parts = line.split(':').map(p => p.trim());
    } else {
      parts = line.split(/\s+/).map(p => p.trim());
    }
    if (!parts.length || !parts[0].includes('@')) return;
    const email = parts[0];
    let password = parts[1] || '';
    let client_id = '';
    let refresh_token = '';
    const isUUID = s => /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/.test(s);
    if (parts.length >= 4) {
      if (isUUID(parts[3])) {
        // email|password|refresh_token|client_id
        client_id = parts[3];
        refresh_token = parts[2];
      } else if (isUUID(parts[2])) {
        // email|password|client_id|refresh_token
        client_id = parts[2];
        refresh_token = parts[3];
      } else {
        refresh_token = parts[2];
        client_id = parts[3];
      }
    } else if (parts.length === 3) {
      if (isUUID(parts[2])) {
        client_id = parts[2];
      } else {
        refresh_token = parts[2];
      }
    }

    items.push({ email, password, client_id, refresh_token });
  });
  return items;
}

async function doImport() {
  const text = document.getElementById('import-text').value;
  const items = parseImportLines(text);
  if (!items.length) return toast('No valid lines to import', true);
  const r = await api('/api/mailboxes/import', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items }),
  });
  const d = await r.json().catch(() => ({}));
  if (!r.ok) return toast('Import failed: ' + (d.error || r.status), true);
  closeModal('import-modal');
  toast(`Identified ${items.length}: Added ${d.added}, Skipped ${d.skipped}`);
  mbPage = 1;
  loadMailboxes();
  if (d.added > 0) verifyAll();
}

/* ===== Batch Verification ===== */
let verifying = false;

async function verifyAll() {
  if (verifying) return;
  const ids = [];
  let page = 1;
  for (; ;) {
    const r = await api('/api/mailboxes?' + new URLSearchParams({ page, size: 100 }));
    const d = await r.json().catch(() => ({}));
    const list = d.data || [];
    list.forEach(x => {
      if (x.status === 'unverified' || x.status === 'verify_failed' || x.status === 'verifying') ids.push(x.id);
    });
    if (page * 100 >= (d.total || 0) || !list.length) break;
    page++;
  }
  if (!ids.length) return toast('No mailboxes need verification');
  await runVerify(ids);
}

async function runVerify(ids) {
  if (verifying || !ids.length) return;
  verifying = true;
  let ok = 0, fail = 0;
  const CONCURRENCY = 10;
  let idx = 0;

  async function worker() {
    while (idx < ids.length) {
      const id = ids[idx++];
      try {
        const r = await api('/api/mailboxes/' + id + '/verify', { method: 'POST' });
        const d = await r.json().catch(() => ({}));
        if (d.status === 'verified') ok++; else fail++;
      } catch (e) {
        fail++;
      }
      loadMailboxes();
    }
  }

  const workers = [];
  for (let i = 0; i < CONCURRENCY; i++) workers.push(worker());
  await Promise.all(workers);

  verifying = false;
  toast(`Verification complete: Success ${ok}, Failed ${fail}`);
  loadMailboxes();
}

async function get2FA(id) {
  const r = await api('/api/mailboxes/' + id + '/2fa');
  const d = await r.json().catch(() => ({}));
  if (!r.ok) {
    return toast('2FA Failed: ' + (d.error || 'Could not fetch 2FA code'), true);
  }
  if (d.code) {
    try {
      await navigator.clipboard.writeText(d.code);
    } catch (e) { }
    toast(`🔑 2FA Code: ${d.code} (Copied to clipboard)`);
  }
}

function openMailboxModal(data) {
  const title = data ? (window.t('mb.modal_edit_title', 'Edit Mailbox') + ' #' + data.id) : window.t('mb.modal_add_title', 'Add Mailbox');
  document.getElementById('mb-modal-title').textContent = title;
  document.getElementById('mb-id').value = data ? data.id : '';
  document.getElementById('mb-email').value = data ? data.email : '';
  document.getElementById('mb-password').value = data ? data.password : '';
  document.getElementById('mb-provider').value = data ? data.provider : '';
  document.getElementById('mb-client-id').value = data ? data.client_id : '';
  document.getElementById('mb-refresh-token').value = data ? data.refresh_token : '';
  document.getElementById('mb-status').value = data ? data.status : 'unverified';
  syncSelect('mb-status');
  document.getElementById('mb-note').value = data ? data.note : '';
  document.getElementById('mb-modal').style.display = 'flex';
}

function editMailbox(id) {
  if (mbCache[id]) openMailboxModal(mbCache[id]);
}

async function saveMailbox() {
  const id = document.getElementById('mb-id').value;
  const body = {
    email: document.getElementById('mb-email').value.trim(),
    password: document.getElementById('mb-password').value,
    provider: document.getElementById('mb-provider').value.trim(),
    client_id: document.getElementById('mb-client-id').value.trim(),
    refresh_token: document.getElementById('mb-refresh-token').value.trim(),
    status: document.getElementById('mb-status').value,
    note: document.getElementById('mb-note').value,
  };
  if (!body.email) return toast('Email is required', true);
  const r = await api('/api/mailboxes' + (id ? '/' + id : ''), {
    method: id ? 'PUT' : 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!r.ok) {
    const e = await r.json().catch(() => ({}));
    return toast('Save failed: ' + (e.error || r.status), true);
  }
  closeModal('mb-modal');
  toast(window.t('common.success', 'Saved'));
  loadMailboxes();
}

async function delMailbox(id) {
  if (!confirm('Delete mailbox #' + id + ' ?')) return;
  const r = await api('/api/mailboxes/' + id, { method: 'DELETE' });
  if (!r.ok) return toast('Delete failed', true);
  toast(window.t('common.success', 'Deleted'));
  loadMailboxes();
}

let mailTimer = null;
let mailMailboxId = null;
let mailMsgs = [];
let mailSelected = 0;
let mailFetching = false;
let mailListSig = '';

function openMailModal(id) {
  const mb = mbCache[id];
  if (!mb) return;
  mailMailboxId = id;
  mailMsgs = [];
  mailSelected = 0;
  mailListSig = '';
  const loadingTxt = window.t('mb.loading', 'Loading...');
  document.getElementById('mail-title').textContent = mb.email;
  document.getElementById('mail-sub').textContent = window.t('mb.inbox_sub', 'Inbox · Auto refresh 3s');
  document.getElementById('mail-list').innerHTML = `<div class="mail-empty">${loadingTxt}</div>`;
  document.getElementById('mail-meta').innerHTML = '';
  document.getElementById('mail-frame').srcdoc = '';
  document.getElementById('mail-modal').style.display = 'flex';
  document.body.classList.add('modal-open');
  fetchMail();
  mailTimer = setInterval(fetchMail, 3000);
}

function closeMailModal() {
  if (mailTimer) { clearInterval(mailTimer); mailTimer = null; }
  mailMailboxId = null;
  mailMsgs = [];
  document.getElementById('mail-modal').style.display = 'none';
  document.body.classList.remove('modal-open');
}

async function fetchMail() {
  if (mailFetching || mailMailboxId === null) return;
  mailFetching = true;
  const id = mailMailboxId;
  try {
    const r = await api('/api/mailboxes/' + id + '/messages');
    if (id !== mailMailboxId) return;
    const d = await r.json().catch(() => ({}));
    if (!r.ok) {
      document.getElementById('mail-list').innerHTML =
        `<div class="mail-empty err">${esc(d.error || 'Fetch failed')}</div>`;
      return;
    }
    const prevId = msgKey(mailMsgs[mailSelected]);
    mailMsgs = mergeMsgs(mailMsgs, d.items || []);
    const i = prevId ? mailMsgs.findIndex(m => msgKey(m) === prevId) : -1;
    mailSelected = i >= 0 ? i : 0;
    renderMailList();
    renderMailDetail();
  } finally {
    mailFetching = false;
  }
}

function msgKey(m) {
  if (!m) return '';
  return m.id || (m.subject + '|' + m.received_at);
}

function mergeMsgs(existing, incoming) {
  const map = new Map();
  (existing || []).forEach(m => map.set(msgKey(m), m));
  (incoming || []).forEach(m => map.set(msgKey(m), m));
  return [...map.values()].sort((a, b) =>
    new Date(b.received_at || 0) - new Date(a.received_at || 0));
}

function renderMailList() {
  const list = document.getElementById('mail-list');
  if (!mailMsgs.length) {
    mailListSig = '';
    list.innerHTML = `<div class="mail-empty">${esc(window.t('mb.no_emails', 'No emails yet...'))}</div>`;
    return;
  }
  // Do not redraw when list content & selection haven't changed to avoid flickering during 3s polling
  const sig = mailSelected + '#' + mailMsgs.map(msgKey).join(',');
  if (sig === mailListSig) return;
  mailListSig = sig;
  list.innerHTML = mailMsgs.map((m, i) => `
    <div class="mail-item${i === mailSelected ? ' active' : ''}" onclick="selectMail(${i})">
      <div class="mail-item-from">${esc(m.from_name || m.from)}</div>
      <div class="mail-item-subject">${esc(m.subject)}</div>
      <div class="mail-item-time">${fmtTime(m.received_at)}</div>
    </div>`).join('');
}

function selectMail(i) {
  mailSelected = i;
  renderMailList();
  renderMailDetail();
}

const mailBodyCache = {}; // Cache email body by message ID to prevent re-fetching on poll/switch

function renderMailDetail() {
  const m = mailMsgs[mailSelected];
  const meta = document.getElementById('mail-meta');
  const frame = document.getElementById('mail-frame');
  if (!m) {
    meta.innerHTML = '';
    setFrameHTML(frame, '');
    frame.dataset.cur = '';
    return;
  }
  meta.innerHTML = `
    <div class="mail-subject">${esc(m.subject)}</div>
    <div class="mail-from">${esc(m.from_name || '')} &lt;${esc(m.from)}&gt;</div>
    <div class="mail-time">${fmtTime(m.received_at)}</div>`;
  const body = mailBodyCache[m.id];
  if (!body) {
    if (frame.dataset.cur !== 'loading:' + m.id) {
      frame.dataset.cur = 'loading:' + m.id;
      setFrameHTML(frame, '<!doctype html><meta charset="utf-8"><style>html,body{height:100%;margin:0}.wrap{height:100%;display:flex;align-items:center;justify-content:center}.spin{width:32px;height:32px;border:3px solid #e4e9f2;border-top-color:#3b82f6;border-radius:50%;animation:r .8s linear infinite}@keyframes r{to{transform:rotate(360deg)}}</style><div class="wrap"><div class="spin"></div></div>');
    }
    loadMailBody(m.id);
    return;
  }
  // Show HTML content; iframe isolates email content
  const html = body.html || `<pre style="white-space:pre-wrap;font-family:inherit">${esc(body.text)}</pre>`;
  if (frame.dataset.cur !== 'body:' + m.id) {
    frame.dataset.cur = 'body:' + m.id;
    setFrameHTML(frame, html);
  }
}

// Write directly to iframe document (sandbox preserves allow-same-origin without allow-scripts, so parent page can access doc while email scripts will not execute)
const FRAME_BASE_CSS = '<style>::-webkit-scrollbar{width:0;height:0}html{scrollbar-width:none}</style>';

function setFrameHTML(frame, html) {
  const content = FRAME_BASE_CSS + (html || '<!doctype html><meta charset="utf-8">');
  try {
    const doc = frame.contentDocument || (frame.contentWindow && frame.contentWindow.document);
    if (doc) {
      doc.open();
      doc.write(content);
      doc.close();
      return;
    }
  } catch (e) { /* Fallback to srcdoc */ }
  frame.srcdoc = content;
}

async function loadMailBody(msgId) {
  if (mailBodyCache[msgId] || mailMailboxId === null) return;
  const boxId = mailMailboxId;
  const r = await api('/api/mailboxes/' + boxId + '/message?mid=' + encodeURIComponent(msgId));
  const d = await r.json().catch(() => ({}));
  if (!r.ok) return;
  mailBodyCache[msgId] = { html: d.html || '', text: d.text || '' };
  // If user is still viewing this email, render immediately
  const cur = mailMsgs[mailSelected];
  if (boxId === mailMailboxId && cur && cur.id === msgId) renderMailDetail();
}

/* ===== Table 3-second auto-refresh (paused when tab hidden to save resources) ===== */
let mbTimer = setInterval(() => {
  if (!document.hidden) loadMailboxes();
}, 3000);

document.getElementById('mb-search').addEventListener('keydown', e => {
  if (e.key === 'Enter') { mbPage = 1; loadMailboxes(); }
});
document.getElementById('mb-filter').addEventListener('change', () => { mbPage = 1; loadMailboxes(); });

/* Stop polling when closing mail fetcher modal via backdrop click */
document.getElementById('mail-modal').addEventListener('click', e => {
  if (e.target === e.currentTarget) closeMailModal();
});

window.addEventListener('langchanged', () => {
  loadMailboxes();
});

loadMailboxes();
