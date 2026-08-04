let currentTable = 'registrations';
let tablesMeta = [];
let currentSchema = [];
let currentPrimaryKey = 'id';
let currentPage = 1;
let currentRows = [];
let editingRowId = null;
let searchTimer = null;

async function initDBPage() {
  try {
    const res = await api('/api/db/tables');
    const data = await res.json();
    tablesMeta = data.tables || [];
    renderTabs();
    loadTable(currentTable, 1);
  } catch (e) {
    toast('Không thể tải cấu trúc Database', true);
  }
}

function renderTabs() {
  const container = document.getElementById('db-tabs');
  container.innerHTML = tablesMeta.map(t => `
    <div class="db-tab ${t.name === currentTable ? 'active' : ''}" onclick="switchTable('${t.name}')">
      ${esc(t.label || t.name)}
    </div>
  `).join('');
}

function switchTable(name) {
  if (currentTable === name) return;
  currentTable = name;
  currentPage = 1;
  document.getElementById('db-search').value = '';
  renderTabs();
  loadTable(currentTable, 1);
}

function debounceSearch() {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    loadTable(currentTable, 1);
  }, 350);
}

async function loadTable(table, page = 1) {
  currentPage = page;
  const q = document.getElementById('db-search').value.trim();
  const tbody = document.getElementById('db-tbody');
  tbody.innerHTML = `<tr><td colspan="15" style="text-align:center;padding:24px;color:#94a3b8">Đang tải dữ liệu...</td></tr>`;

  try {
    const res = await api(`/api/db/rows?table=${encodeURIComponent(table)}&page=${page}&size=15&q=${encodeURIComponent(q)}`);
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Lỗi tải dữ liệu');

    currentSchema = data.schema || [];
    currentPrimaryKey = data.primaryKey || 'id';
    currentRows = data.rows || [];

    renderHeaders();
    renderRows();
    renderPager(document.getElementById('db-pager'), data.page, Math.ceil(data.total / data.size) || 1, p => loadTable(table, p));
  } catch (e) {
    toast(e.message, true);
    tbody.innerHTML = `<tr><td colspan="15" style="text-align:center;padding:24px;color:#ef4444">Lỗi: ${esc(e.message)}</td></tr>`;
  }
}

function renderHeaders() {
  const thead = document.getElementById('db-thead');
  const colsHtml = currentSchema.map(col => `<th>${esc(col.name)}</th>`).join('');
  const actionsLabel = window.t('db.actions', 'Thao tác');
  thead.innerHTML = `<tr>${colsHtml}<th style="width:120px;text-align:right">${actionsLabel}</th></tr>`;
}

function renderRows() {
  const tbody = document.getElementById('db-tbody');
  if (currentRows.length === 0) {
    const noRec = window.t('db.no_records', 'Không tìm thấy bản ghi nào');
    tbody.innerHTML = `<tr><td colspan="${currentSchema.length + 1}" style="text-align:center;padding:24px;color:#94a3b8">${noRec}</td></tr>`;
    return;
  }

  const editTxt = window.t('common.edit', 'Sửa');
  const delTxt = window.t('common.delete', 'Xóa');

  tbody.innerHTML = currentRows.map(row => {
    const pkVal = row[currentPrimaryKey];
    const cellsHtml = currentSchema.map(col => {
      const val = row[col.name];
      const disp = val === null || val === undefined ? '<span style="color:#64748b">NULL</span>' : esc(String(val));
      return `<td class="cell-truncate" title="${esc(String(val ?? ''))}">${disp}</td>`;
    }).join('');

    return `
      <tr>
        ${cellsHtml}
        <td style="white-space:nowrap;text-align:right">
          <button class="px-btn ghost" onclick="openEditModal('${esc(pkVal)}')">${editTxt}</button>
          <button class="px-btn danger" onclick="confirmDeleteRow('${esc(pkVal)}')">${delTxt}</button>
        </td>
      </tr>
    `;
  }).join('');
}

function openCreateModal() {
  editingRowId = null;
  const titleTpl = window.t('db.add_title', `Thêm bản ghi mới (${currentTable})`);
  document.getElementById('modal-title').textContent = titleTpl.replace('{table}', currentTable);
  buildFormFields({});
  document.getElementById('record-modal').style.display = 'flex';
}

function openEditModal(pkVal) {
  const row = currentRows.find(r => String(r[currentPrimaryKey]) === String(pkVal));
  if (!row) return;
  editingRowId = pkVal;
  const titleTpl = window.t('db.edit_title', `Chỉnh sửa bản ghi #${pkVal} (${currentTable})`);
  document.getElementById('modal-title').textContent = titleTpl.replace('{pk}', currentPrimaryKey).replace('{id}', pkVal).replace('{table}', currentTable);
  buildFormFields(row);
  document.getElementById('record-modal').style.display = 'flex';
}

function buildFormFields(data) {
  const container = document.getElementById('modal-form-fields');
  container.innerHTML = currentSchema.map(col => {
    if (col.readonly && editingRowId === null) return '';
    const val = data[col.name] !== undefined ? data[col.name] : '';
    const isFull = col.name === 'log' || col.name === 'note' || String(val).length > 40;

    let inputHtml = '';
    if (col.readonly) {
      inputHtml = `<input class="px-input" value="${esc(val)}" readonly style="opacity:0.6;background:rgba(15,23,42,0.6)">`;
    } else if (col.type === 'boolean') {
      inputHtml = `
        <select name="${col.name}" class="px-input">
          <option value="true" ${val === true || val === 1 || val === '1' ? 'selected' : ''}>True (1)</option>
          <option value="false" ${val === false || val === 0 || val === '0' ? 'selected' : ''}>False (0)</option>
        </select>
      `;
    } else if (col.name === 'log' || col.name === 'note') {
      inputHtml = `<textarea name="${col.name}" class="px-input" rows="3">${esc(val)}</textarea>`;
    } else {
      inputHtml = `<input name="${col.name}" type="${col.type === 'number' ? 'number' : 'text'}" class="px-input" value="${esc(val)}">`;
    }

    return `
      <div class="form-group ${isFull ? 'full' : ''}">
        <label>${esc(col.name)}</label>
        ${inputHtml}
      </div>
    `;
  }).join('');
}

async function saveRecord(e) {
  e.preventDefault();
  const form = document.getElementById('record-form');
  const formData = new FormData(form);
  const dataObj = {};

  for (let [key, val] of formData.entries()) {
    const col = currentSchema.find(c => c.name === key);
    if (!col || col.readonly) continue;
    if (col.type === 'number') {
      dataObj[key] = val === '' ? null : Number(val);
    } else if (col.type === 'boolean') {
      dataObj[key] = val === 'true';
    } else {
      dataObj[key] = val;
    }
  }

  try {
    let res;
    if (editingRowId !== null) {
      res = await api('/api/db/rows', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ table: currentTable, id: editingRowId, data: dataObj })
      });
    } else {
      res = await api('/api/db/rows', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ table: currentTable, data: dataObj })
      });
    }

    const ret = await res.json();
    if (!res.ok) throw new Error(ret.error || 'Lưu thất bại');

    toast(editingRowId !== null ? 'Đã cập nhật bản ghi thành công' : 'Đã thêm bản ghi mới thành công');
    closeModal('record-modal');
    loadTable(currentTable, currentPage);
  } catch (err) {
    toast(err.message, true);
  }
}

async function confirmDeleteRow(pkVal) {
  if (!confirm(`Bạn có chắc chắn muốn xóa bản ghi ${currentPrimaryKey} #${pkVal} khỏi bảng '${currentTable}' không?`)) return;

  try {
    const res = await api('/api/db/rows', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ table: currentTable, id: pkVal })
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Xóa thất bại');

    toast(`Đã xóa bản ghi ${currentPrimaryKey} #${pkVal}`);
    loadTable(currentTable, currentPage);
  } catch (e) {
    toast(e.message, true);
  }
}

async function confirmTruncateTable() {
  const confirm1 = confirm(`⚠️ CẢNH BÁO NGUY HIỂM!\n\nBạn có CHẮC CHẮN muốn XÓA SẠCH TOÀN BỘ BẢN GHI trong bảng '${currentTable}' không?\nHành động này KHÔNG THỂ HOÀN TÁC!`);
  if (!confirm1) return;

  const confirmText = prompt(`Vui lòng nhập chính xác chữ '${currentTable}' để xác nhận xóa sạch bảng này:`);
  if (confirmText !== currentTable) {
    alert("Xác nhận không chính xác! Đã hủy thao tác Truncate Bảng.");
    return;
  }

  try {
    toast(`Đang làm sạch bảng ${currentTable}...`);
    const res = await api('/api/db/truncate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ table: currentTable })
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Truncate thất bại');

    toast(`Đã làm sạch toàn bộ dữ liệu bảng '${currentTable}'!`);
    loadTable(currentTable, 1);
  } catch (err) {
    toast(err.message, true);
  }
}

function backupDB() {
  const token = getToken();
  window.open('/api/db/backup?token=' + encodeURIComponent(token), '_blank');
}

function triggerRestoreDB() {
  document.getElementById('restore-file-input').click();
}

async function handleRestoreFile(e) {
  const file = e.target.files[0];
  if (!file) return;

  if (!confirm(`CẢNH BÁO: Việc Phục hồi DB sẽ ghi đè toàn bộ dữ liệu hiện tại bằng tệp '${file.name}'.\n\nHệ thống sẽ tự động sao lưu bản cũ thành 'adskull.db.bak'. Bạn có muốn tiếp tục không?`)) {
    e.target.value = '';
    return;
  }

  const formData = new FormData();
  formData.append('file', file);

  try {
    toast('Đang tải lên và phục hồi Database...');
    const res = await api('/api/db/restore', {
      method: 'POST',
      body: formData
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Phục hồi thất bại');

    alert(data.message || 'Phục hồi DB thành công!');
    location.reload();
  } catch (err) {
    toast(err.message, true);
  } finally {
    e.target.value = '';
  }
}

document.addEventListener('DOMContentLoaded', initDBPage);
