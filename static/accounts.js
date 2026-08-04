/* ===== Account Management (ChatGPT + Codex Production) ===== */
function getAccStatusText(status, note) {
  if (status === 'logging_in' || (status === 'registering' && note === 'login_only')) {
    return window.t('acc.filter_logging_in', 'Logging in');
  }
  const keys = {
    pending: 'acc.sub_pending',
    registering: 'acc.filter_registering',
    logging_in: 'acc.filter_logging_in',
    registered: 'acc.filter_registered',
    register_failed: 'acc.filter_failed',
    already_registered: 'acc.filter_disabled',
  };
  return window.t(keys[status] || status, status || '');
}
let page = 1;
const size = 20;
let accCache = {};
let accTotal = 0;
const accSelected = new Set();

async function load() {
  const q = document.getElementById('search').value.trim();
  const status = document.getElementById('filter-status').value;
  const params = new URLSearchParams({ page, size });
  if (q) params.set('q', q);
  if (status) params.set('status', status);
  const r = await api('/api/registrations?' + params);
  const d = await r.json();
  accCache = {};
  accTotal = d.total || 0;
  (d.data || []).forEach(x => { accCache[x.id] = x; });
  const noDataText = window.t('dash.no_data', 'No Data');
  document.getElementById('rows').innerHTML = (d.data || []).map(rowHtml).join('')
    || `<tr><td colspan="7" style="text-align:center;color:var(--text-3)">${noDataText}</td></tr>`;
  const maxPage = Math.max(1, Math.ceil((d.total || 0) / size));
  renderPager('pager', page, maxPage, p => { page = p; load(); });
  syncBatchBar();
}

function rowHtml(x) {
  const canDownload = x.status === 'registered';
  const canCheck = x.status === 'registered' || x.status === 'already_registered';
  const statusText = getAccStatusText(x.status, x.note);
  const badgeClass = (x.status === 'registering' && x.note === 'login_only') ? 'logging_in' : x.status;
  const shippedYes = window.t('acc.shipped_yes', 'Shipped');
  const shippedNo = window.t('acc.shipped_no', 'Unshipped');
  const logTitle = window.t('acc.btn_log', 'Logs');
  const testTitle = window.t('acc.batch_test_alive', 'Alive Test');
  const dlTitle = window.t('acc.batch_download', 'Download');
  const delTitle = window.t('acc.batch_delete', 'Delete');
  const isFailed = x.status === 'register_failed' || x.status === 'failed';
  const retryTitle = window.t('acc.btn_retry', 'Thử lại');
  const get2faTitle = window.t('mb.get_2fa', 'Get 2FA Code');
  return `
    <tr class="${accSelected.has(x.id) ? 'row-sel' : ''}">
      <td class="col-check"><input type="checkbox" ${accSelected.has(x.id) ? 'checked' : ''} onclick="toggleSelect(${x.id}, this.checked)"></td>
      <td>${esc(x.email)}${x.two_factor_secret ? ' <span class="badge twofa">2FA</span>' : ''}</td>
      <td><span class="badge ${esc(badgeClass)}">${statusText}</span></td>
      <td class="ship-cell">
        <span class="badge ${x.shipped ? 'registered' : 'pending'}">${x.shipped ? shippedYes : shippedNo}</span>
      </td>
      <td>${fmtTime(x.created_at)}</td>
      <td>${fmtTime(x.updated_at)}</td>
      <td>
        ${isFailed ? `<button class="icon-btn warning" title="${retryTitle}" onclick="retryAcc(${x.id}, ${x.mailbox_id || 0})">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg>
        </button>` : ''}
        <button class="icon-btn" title="${logTitle}" onclick="showLog(${x.id})">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/><path d="M8 13h8M8 17h5"/></svg>
        </button>
        ${x.two_factor_secret ? `<button class="icon-btn" title="${get2faTitle}" onclick="get2FAAcc(${x.id})">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
        </button>` : ''}
        <button class="icon-btn" title="${testTitle}" ${canCheck ? '' : 'disabled'} onclick="checkAlive(${x.id})">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
        </button>
        <button class="icon-btn" title="${dlTitle}" ${canDownload ? '' : 'disabled'} onclick="downloadAcc(${x.id})">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="m7 10 5 5 5-5"/><path d="M12 15V3"/></svg>
        </button>
        <button class="icon-btn danger" title="${delTitle}" onclick="del(${x.id})">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2m2 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M10 11v6M14 11v6"/></svg>
        </button>
      </td>
    </tr>`;
}

async function retryAcc(id, mailboxId) {
  try {
    toast(window.t('acc.starting_retry', 'Đang bắt đầu thử lại...'));
    const body = { count: 1 };
    if (mailboxId > 0) {
      body.mailbox_ids = [mailboxId];
    }
    const r = await api('/api/produce', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const d = await r.json();
    if (!r.ok) return toast(d.error || window.t('common.error', 'Thao tác thất bại'), true);
    toast(window.t('common.success', 'Thành công'));
    loadProduce();
    load();
  } catch (e) {
    toast(String(e), true);
  }
}

async function retrySelected() {
  const ids = [...accSelected];
  if (!ids.length) return;
  const mbIds = [];
  ids.forEach(id => {
    const acc = accCache[id];
    if (acc && acc.mailbox_id) mbIds.push(acc.mailbox_id);
  });
  if (!mbIds.length) {
    return toast(window.t('acc.no_failed_selected', 'Không có tài khoản thất bại nào được chọn'), true);
  }
  try {
    toast(window.t('acc.starting_retry', 'Đang bắt đầu thử lại...'));
    const r = await api('/api/produce', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ count: mbIds.length, mailbox_ids: mbIds }),
    });
    const d = await r.json();
    if (!r.ok) return toast(d.error || window.t('common.error', 'Thao tác thất bại'), true);
    toast(window.t('common.success', 'Đã thêm tài khoản vào hàng chờ thử lại!'));
    loadProduce();
    load();
  } catch (e) {
    toast(String(e), true);
  }
}

/* ===== Production Progress ===== */
async function loadProduce() {
  try {
    const r = await api('/api/produce/status');
    const s = await r.json();
    document.getElementById('pd-pending').textContent = s.pending || 0;
    document.getElementById('pd-running').textContent = s.running_num || 0;
    document.getElementById('pd-registered').textContent = s.registered || 0;
    document.getElementById('pd-failed').textContent = s.failed || 0;
    document.getElementById('pd-stop').style.display = s.running ? '' : 'none';
  } catch (e) { /* ignore */ }
}

/* Browser ready gate: disable production if not ready */
let browserReady = true;
async function loadBrowserGate() {
  try {
    const s = await (await api('/api/browser/status')).json();
    browserReady = !!s.ready;
    const btn = document.getElementById('produce-btn');
    if (btn) {
      btn.disabled = !browserReady;
      btn.title = browserReady ? '' : (s.message || 'Browser missing');
    }
    const msg = document.getElementById('pd-msg');
    if (!browserReady && msg) msg.textContent = '⚠ ' + (s.message || 'Browser missing, production disabled for now');
  } catch (e) { /* ignore */ }
}

let produceMailboxesCache = [];

function toggleProduceMode(mode) {
  const isAuto = mode === 'auto';
  document.getElementById('pd-mode-auto-wrap').style.display = isAuto ? '' : 'none';
  document.getElementById('pd-mode-manual-wrap').style.display = isAuto ? 'none' : '';
  document.getElementById('pd-mode-auto-radio').checked = isAuto;
  document.getElementById('pd-mode-manual-radio').checked = !isAuto;
}

async function loadVerifiedMailboxes(preSelectIDs = []) {
  const container = document.getElementById('pd-mb-list');
  container.innerHTML = '<div style="color:var(--text-3);font-size:12px;padding:8px;text-align:center;">Đang tải danh sách hòm thư...</div>';
  try {
    const r = await api('/api/mailboxes?status=verified&size=10000');
    const d = await r.json();
    produceMailboxesCache = d.data || [];
    if (!produceMailboxesCache.length) {
      container.innerHTML = '<div style="color:var(--text-3);font-size:12px;padding:8px;text-align:center;">Không có hòm thư khả dụng (đã xác minh)</div>';
      return;
    }
    const set = new Set(preSelectIDs);
    container.innerHTML = produceMailboxesCache.map(mb => `
      <label class="pd-mb-item" data-email="${esc(mb.email).toLowerCase()}" style="display:flex;align-items:center;justify-content:space-between;padding:4px 8px;border-bottom:1px solid rgba(255,255,255,0.05);cursor:pointer;font-size:13px;">
        <span style="display:flex;align-items:center;gap:8px;">
          <input type="checkbox" class="pd-mb-cb" value="${mb.id}" ${set.has(mb.id) ? 'checked' : ''} onchange="updateProduceSelectedCount()">
          <span style="font-family:monospace;color:var(--text-1);">${esc(mb.email)}</span>
        </span>
        <span style="font-size:11px;color:var(--text-3);">${mb.provider || 'outlook'}</span>
      </label>
    `).join('');
    updateProduceSelectedCount();
  } catch (e) {
    container.innerHTML = '<div style="color:var(--text-3);font-size:12px;padding:8px;text-align:center;">Tải hòm thư thất bại</div>';
  }
}

function updateProduceSelectedCount() {
  const checked = document.querySelectorAll('#pd-mb-list input[type="checkbox"]:checked').length;
  document.getElementById('pd-mb-selected-count').textContent = checked;
  const allCbs = document.querySelectorAll('#pd-mb-list input[type="checkbox"]');
  document.getElementById('pd-mb-select-all').checked = allCbs.length > 0 && checked === allCbs.length;
}

function filterProduceMailboxes() {
  const q = document.getElementById('pd-mb-search').value.trim().toLowerCase();
  document.querySelectorAll('#pd-mb-list .pd-mb-item').forEach(item => {
    const email = item.dataset.email || '';
    item.style.display = (!q || email.includes(q)) ? 'flex' : 'none';
  });
}

function toggleAllProduceMailboxes(checked) {
  document.querySelectorAll('#pd-mb-list .pd-mb-item').forEach(item => {
    if (item.style.display !== 'none') {
      const cb = item.querySelector('input[type="checkbox"]');
      if (cb) cb.checked = checked;
    }
  });
  updateProduceSelectedCount();
}

function openProduceModal(preSelectIDs = []) {
  if (!browserReady) return toast('Browser missing, download in progress or failed', true);
  document.getElementById('produce-count').value = 10;
  if (Array.isArray(preSelectIDs) && preSelectIDs.length > 0) {
    toggleProduceMode('manual');
    loadVerifiedMailboxes(preSelectIDs);
  } else {
    toggleProduceMode('auto');
    loadVerifiedMailboxes();
  }
  document.getElementById('produce-modal').style.display = 'flex';
}

async function startProduce() {
  const isManual = document.getElementById('pd-mode-manual-radio').checked;
  let count = 0;
  let mailboxIDs = [];

  if (isManual) {
    const checkedBoxes = document.querySelectorAll('#pd-mb-list input[type="checkbox"]:checked');
    mailboxIDs = Array.from(checkedBoxes).map(cb => Number(cb.value)).filter(Boolean);
    if (!mailboxIDs.length) return toast('Vui lòng chọn ít nhất 1 hòm thư', true);
    count = mailboxIDs.length;
  } else {
    count = parseInt(document.getElementById('produce-count').value, 10);
    if (!count || count < 1) return toast('Please enter valid count', true);
  }

  const loginOnly = !!document.getElementById('produce-login-only')?.checked;
  const r = await api('/api/produce', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ count, login_only: loginOnly, mailbox_ids: mailboxIDs }),
  });
  if (!r.ok) {
    const d = await r.json().catch(() => ({}));
    return toast(d.error || 'Failed to start production', true);
  }
  closeModal('produce-modal');
  toast(window.t('common.success', 'Success'));
  loadProduce();
  load();
}

async function stopProduce() {
  if (!confirm('Are you sure you want to stop the current production task?')) return;
  await api('/api/produce/stop', { method: 'POST' });
  toast('Stop requested');
  loadProduce();
}

let logTimer = null;
let logAccId = null;

async function showLog(id) {
  logAccId = id;
  document.getElementById('log-title').textContent = window.t('acc.modal_log_title', 'Execution Logs');
  document.getElementById('log-body').textContent = window.t('mb.loading', 'Loading...');
  document.getElementById('log-shot-btn').style.display = 'none';
  document.getElementById('log-modal').style.display = 'flex';
  document.body.style.overflow = 'hidden';
  await refreshLog(false);
  clearInterval(logTimer);
  logTimer = setInterval(() => refreshLog(true), 200);
}

async function refreshLog(silent) {
  if (logAccId == null) return;
  const r = await api('/api/registrations/' + logAccId + '/logs');
  if (!r.ok) { if (!silent) toast('Failed to read logs', true); return; }
  const d = await r.json();
  const titleLog = window.t('acc.modal_log_title', 'Execution Logs');
  document.getElementById('log-title').textContent = titleLog + ' · ' + d.email;
  document.getElementById('log-shot-btn').style.display = d.has_shot ? '' : 'none';
  const parts = [];
  if (d.note) parts.push('Note: ' + d.note);
  if (parts.length) parts.push('');
  parts.push(d.log || '(No execution log available)');

  const logEl = document.getElementById('log-body');
  const isAtBottom = logEl.scrollHeight - logEl.clientHeight - logEl.scrollTop < 60;

  logEl.textContent = parts.join('\n');

  if (!silent || isAtBottom) {
    logEl.scrollTop = logEl.scrollHeight;
    requestAnimationFrame(() => {
      logEl.scrollTop = logEl.scrollHeight;
    });
  }
}

function closeLog() {
  clearInterval(logTimer);
  logTimer = null;
  logAccId = null;
  document.getElementById('log-modal').style.display = 'none';
  document.body.style.overflow = '';
  document.getElementById('log-body').textContent = '';
}

/* ===== Error Screenshot ===== */
async function viewShot() {
  if (logAccId == null) return;
  const r = await api('/api/registrations/' + logAccId + '/shot');
  if (!r.ok) return toast('No error screenshot available', true);
  const blob = await r.blob();
  const img = document.getElementById('shot-img');
  if (img.dataset.url) URL.revokeObjectURL(img.dataset.url);
  img.src = img.dataset.url = URL.createObjectURL(blob);
  document.getElementById('shot-modal').style.display = 'flex';
}
function closeShot() {
  document.getElementById('shot-modal').style.display = 'none';
}

/* ===== Batch Selection ===== */
function toggleSelect(id, checked) {
  if (checked) accSelected.add(id); else accSelected.delete(id);
  syncBatchBar();
}
function toggleSelectAll(checked) {
  Object.keys(accCache).forEach(id => {
    if (checked) accSelected.add(Number(id)); else accSelected.delete(Number(id));
  });
  load();
}
function clearSelection() { accSelected.clear(); load(); }
function syncBatchBar() {
  const bar = document.getElementById('acc-batch');
  bar.style.display = accSelected.size ? 'flex' : 'none';
  const selTpl = window.t('acc.selected', `Selected ${accSelected.size} items`);
  document.getElementById('acc-batch-count').textContent = selTpl.replace('{n}', accSelected.size);
  const all = document.getElementById('acc-check-all');
  const ids = Object.keys(accCache).map(Number);
  all.checked = ids.length > 0 && ids.every(id => accSelected.has(id));
}

/* ===== Download ===== */
async function downloadAcc(id) {
  const item = accCache[id];
  const email = item ? (item.email || '') : '';
  const prefix = email ? email.split('@')[0] : id;
  const filename = 'account_gpt_' + prefix + '.txt';
  await downloadByIds([id], filename);
}
async function downloadSelected() {
  const ids = [...accSelected];
  if (!ids.length) return;
  const filename = 'account_gpt_' + ids.length + '.txt';
  await downloadByIds(ids, filename);
}
async function downloadByIds(ids, filename) {
  const r = await api('/api/download', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids }),
  });
  if (!r.ok) {
    const d = await r.json().catch(() => ({}));
    return toast(d.error || 'Download failed', true);
  }
  const blob = await r.blob();
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = filename;
  a.click();
  URL.revokeObjectURL(a.href);
  load();
}

/* ===== Alive Test ===== */
async function checkAlive(id) {
  await checkAliveByIds([id]);
}
async function checkAliveSelected() {
  const ids = [...accSelected];
  if (!ids.length) return;
  await checkAliveByIds(ids);
}
async function checkAliveByIds(ids) {
  toast('Checking... (' + ids.length + ')');
  const r = await api('/api/check-alive', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids }),
  });
  if (!r.ok) {
    const d = await r.json().catch(() => ({}));
    return toast(d.error || 'Alive test failed', true);
  }
  const d = await r.json();
  let m = 'Alive: ' + (d.alive || 0) + ', Dead: ' + (d.dead || 0);
  if (d.error) m += ', Err: ' + d.error;
  toast(m);
  load();
}

/* ===== Delete ===== */
async function delSelected() {
  const ids = [...accSelected];
  if (!ids.length) return;
  if (!confirm('Delete selected ' + ids.length + ' accounts?')) return;
  for (const id of ids) {
    await api('/api/registrations/' + id, { method: 'DELETE' });
    accSelected.delete(id);
  }
  toast(window.t('common.success', 'Success'));
  load();
}
async function get2FAAcc(id) {
  const item = accCache[id];
  const r = await api('/api/registrations/' + id + '/2fa' + (item && item.two_factor_secret ? '?secret=' + encodeURIComponent(item.two_factor_secret) : ''));
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

async function del(id) {
  if (!confirm('Delete account #' + id + ' ?')) return;
  const r = await api('/api/registrations/' + id, { method: 'DELETE' });
  if (!r.ok) return toast('Delete failed', true);
  accSelected.delete(id);
  toast(window.t('common.success', 'Success'));
  load();
}

document.getElementById('search').addEventListener('keydown', e => {
  if (e.key === 'Enter') { page = 1; load(); }
});
document.getElementById('filter-status').addEventListener('change', () => { page = 1; load(); });

window.addEventListener('langchanged', () => {
  load();
  loadProduce();
  loadBrowserGate();
});

load();
loadProduce();
loadBrowserGate();
setInterval(load, 3000);
setInterval(loadProduce, 2000);
setInterval(loadBrowserGate, 2500);

(function checkUrlParams() {
  const urlParams = new URLSearchParams(window.location.search);
  const produceIdsStr = urlParams.get('produce_ids');
  if (produceIdsStr) {
    const ids = produceIdsStr.split(',').map(s => Number(s.trim())).filter(Boolean);
    if (ids.length) {
      setTimeout(() => openProduceModal(ids), 300);
    }
  }
})();
