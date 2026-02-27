const API = '/api';

async function fetchApi(path, options = {}) {
  const headers = options.headers || {};
  const res = await fetch(API + path, {
    method: options.method || 'GET',
    headers: headers,
    body: options.body,
    credentials: 'same-origin',
  });
  return res;
}

async function login(username, password) {
  const res = await fetchApi('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Login failed');
  }
  return res.json();
}

async function logout() {
  await fetchApi('/auth/logout', { method: 'POST' });
}

async function getMe() {
  const res = await fetchApi('/auth/me');
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Not authenticated');
  }
  return res.json();
}

async function getProfile() {
  const res = await fetchApi('/auth/profile');
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to load profile');
  }
  return res.json();
}

async function changeMyPassword(currentPassword, newPassword) {
  const res = await fetchApi('/auth/password', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to update password');
  }
  return res.json();
}

async function getApps() {
  const res = await fetchApi('/apps');
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to load apps');
  }
  return res.json();
}

async function triggerRun(appId) {
  const res = await fetchApi(`/apps/${encodeURIComponent(appId)}/run`, {
    method: 'POST',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to start run');
  }
  return res.json();
}

const RUNS_PAGE_SIZE = 15;
let runsCurrentPage = 1;
let currentUser = null;

async function getRuns(appId = '', limit = RUNS_PAGE_SIZE, offset = 0) {
  const params = new URLSearchParams();
  if (appId) params.set('app_id', appId);
  params.set('limit', String(limit));
  params.set('offset', String(offset));
  const res = await fetchApi(`/runs?${params}`);
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to load runs');
  }
  return res.json();
}

async function getRun(id) {
  const res = await fetchApi(`/runs/${id}`);
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to load run');
  }
  return res.json();
}

async function createApp(app) {
  const res = await fetchApi('/apps', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(app),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to create app');
  }
  return res.json();
}

async function updateApp(appId, app) {
  const res = await fetchApi(`/apps/${encodeURIComponent(appId)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(app),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to update app');
  }
  return res.json();
}

async function deleteApp(appId) {
  const res = await fetchApi(`/apps/${encodeURIComponent(appId)}`, {
    method: 'DELETE',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to delete app');
  }
}

async function getUsers() {
  const res = await fetchApi('/users');
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to load users');
  }
  return res.json();
}

async function createUser(user) {
  const res = await fetchApi('/users', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(user),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to create user');
  }
  return res.json();
}

async function setUserGroups(userId, groupIds) {
  const res = await fetchApi(`/users/${encodeURIComponent(String(userId))}/groups`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ group_ids: groupIds }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to update user groups');
  }
  return res.json();
}

async function updateUserPassword(userId, password) {
  const res = await fetchApi(`/users/${encodeURIComponent(String(userId))}/password`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to update user password');
  }
  return res.json();
}

async function deleteUserById(userId) {
  const res = await fetchApi(`/users/${encodeURIComponent(String(userId))}`, {
    method: 'DELETE',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to delete user');
  }
}

async function getGroups() {
  const res = await fetchApi('/groups');
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to load groups');
  }
  return res.json();
}

async function createGroup(name) {
  const res = await fetchApi('/groups', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to create group');
  }
  return res.json();
}

async function getGroup(groupId) {
  const res = await fetchApi(`/groups/${encodeURIComponent(String(groupId))}`);
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to load group');
  }
  return res.json();
}

async function setGroupUsers(groupId, userIds) {
  const res = await fetchApi(`/groups/${encodeURIComponent(String(groupId))}/users`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ user_ids: userIds }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to update group users');
  }
  return res.json();
}

async function setGroupApps(groupId, appIds) {
  const res = await fetchApi(`/groups/${encodeURIComponent(String(groupId))}/apps`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ app_ids: appIds }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to update group apps');
  }
  return res.json();
}

async function getAppGroups(appId) {
  const res = await fetchApi(`/apps/${encodeURIComponent(appId)}/groups`);
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to load app groups');
  }
  return res.json();
}

async function setAppGroups(appId, groupIds) {
  const res = await fetchApi(`/apps/${encodeURIComponent(appId)}/groups`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ group_ids: groupIds }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to update app groups');
  }
  return res.json();
}

async function getSSHKeys() {
  const res = await fetchApi('/ssh-keys');
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to load SSH keys');
  }
  return res.json();
}

async function createSSHKey(name, privateKey) {
  const res = await fetchApi('/ssh-keys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, private_key: privateKey }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to create SSH key');
  }
  return res.json();
}

async function deleteSSHKeyById(keyId) {
  const res = await fetchApi(`/ssh-keys/${encodeURIComponent(String(keyId))}`, {
    method: 'DELETE',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to delete SSH key');
  }
}

function showToast(message, type = 'success') {
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  el.textContent = message;
  document.body.appendChild(el);
  requestAnimationFrame(() => el.classList.add('visible'));
  setTimeout(() => {
    el.classList.remove('visible');
    setTimeout(() => el.remove(), 300);
  }, 3000);
}

function statusClass(status) {
  switch (status) {
    case 'pending': return 'badge-pending';
    case 'running': return 'badge-running';
    case 'success': return 'badge-success';
    case 'failed': return 'badge-failed';
    default: return 'badge-pending';
  }
}

function formatDate(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  const now = new Date();
  const diff = now - d;
  if (diff < 60000) return 'Just now';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function formatDuration(run) {
  if (!run.ended_at || !run.started_at) return '—';
  const start = new Date(run.started_at);
  const end = new Date(run.ended_at);
  const sec = Math.round((end - start) / 1000);
  if (sec < 60) return `${sec}s`;
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  if (s === 0) return `${m}m`;
  return `${m}m ${s}s`;
}

function renderApps(container, apps) {
  if (!container) return;
  const list = Array.isArray(apps) ? apps : [];
  if (!list.length) {
    const emptyMsg = document.getElementById('apps-search') ? 'No apps match your search.' : 'No apps configured.';
    const canCreate = currentUser && currentUser.is_admin;
    container.innerHTML = `<p class="empty-state">${emptyMsg}${canCreate ? ' <button type="button" class="btn btn-primary" id="add-app-empty">Add app</button>' : ''}</p>`;
    const addBtn = container.querySelector('#add-app-empty');
    if (addBtn) addBtn.addEventListener('click', () => openAppForm(null));
    return;
  }
  container.innerHTML = list.map(app => `
    <article class="app-card" data-app-id="${escapeHtml(app.id)}">
      <h3>${escapeHtml(app.name)}</h3>
      <p class="app-id">${escapeHtml(app.id)}</p>
      <div class="card-actions">
        <button type="button" class="btn btn-primary run-btn btn-run" data-app-id="${escapeHtml(app.id)}">Run</button>
        <button type="button" class="btn btn-ghost edit-btn btn-edit" data-app-id="${escapeHtml(app.id)}">Edit</button>
        ${currentUser && currentUser.is_admin ? `<button type="button" class="btn btn-ghost delete-btn btn-delete" data-app-id="${escapeHtml(app.id)}">Delete</button>` : ''}
      </div>
    </article>
  `).join('');

  container.querySelectorAll('.run-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
      const appId = btn.dataset.appId;
      btn.disabled = true;
      try {
        const { run_id } = await triggerRun(appId);
        showToast(`Run #${run_id} started.`, 'success');
        const runsEl = document.getElementById('runs-container');
        if (runsEl) loadRuns();
      } catch (e) {
        showToast(e.message || 'Failed to start run', 'error');
      } finally {
        btn.disabled = false;
      }
    });
  });
  container.querySelectorAll('.edit-btn').forEach(btn => {
    btn.addEventListener('click', () => openAppForm(btn.dataset.appId));
  });
  container.querySelectorAll('.delete-btn').forEach(btn => {
    btn.addEventListener('click', () => confirmDeleteApp(btn.dataset.appId));
  });
}

function escapeHtml(s) {
  const div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

function renderRuns(container, runs, total, page) {
  stopInlineLogPolling();
  const list = Array.isArray(runs) ? runs : [];
  const totalCount = typeof total === 'number' ? total : list.length;
  const totalPages = Math.max(1, Math.ceil(totalCount / RUNS_PAGE_SIZE));
  const currentPage = typeof page === 'number' && page >= 1 ? page : 1;

  if (!list.length && currentPage === 1) {
    container.innerHTML = '<p class="empty-state">No runs yet. Trigger a run from the <a href="/apps.html">Apps</a> page.</p>';
    return;
  }

  let paginationHtml = '';
  if (totalCount > RUNS_PAGE_SIZE) {
    paginationHtml = `
      <div class="pagination">
        <button type="button" class="btn btn-ghost btn-pagination-prev" ${currentPage <= 1 ? 'disabled' : ''}>Previous</button>
        <span class="pagination-info">Page ${currentPage} of ${totalPages}</span>
        <button type="button" class="btn btn-ghost btn-pagination-next" ${currentPage >= totalPages ? 'disabled' : ''}>Next</button>
      </div>
    `;
  }

  container.innerHTML = `
    <table class="runs-table">
      <thead>
        <tr>
          <th class="run-expand-col"></th>
          <th>ID</th>
          <th>App</th>
          <th>By</th>
          <th>Status</th>
          <th>Started</th>
          <th>Duration</th>
        </tr>
      </thead>
      <tbody>
        ${list.map(run => `
          <tr class="run-row" data-run-id="${run.id}">
            <td class="run-expand-col">
              <button type="button" class="run-expand-btn" data-run-id="${run.id}" aria-label="Expand log">▶</button>
            </td>
            <td class="run-id">#${run.id}</td>
            <td>${escapeHtml(run.app_id)}</td>
            <td>${escapeHtml(run.triggered_by || '—')}</td>
            <td><span class="badge ${statusClass(run.status)}">${escapeHtml(run.status)}</span></td>
            <td>${formatDate(run.started_at)}</td>
            <td>${formatDuration(run)}</td>
          </tr>
          <tr class="run-log-row" id="run-log-${run.id}" data-run-id="${run.id}" hidden>
            <td colspan="7">
              <div class="run-log-inline">
                <pre class="run-log-inline-content">${escapeHtml(run.log || '(no log yet)')}</pre>
              </div>
            </td>
          </tr>
        `).join('')}
      </tbody>
    </table>
    ${paginationHtml}
  `;

  container.querySelectorAll('.run-expand-btn').forEach(btn => {
    btn.addEventListener('click', () => toggleRunLogInline(Number(btn.dataset.runId)));
  });

  const prevBtn = container.querySelector('.btn-pagination-prev');
  if (prevBtn && currentPage > 1) {
    prevBtn.addEventListener('click', () => {
      runsCurrentPage = currentPage - 1;
      loadRuns();
    });
  }
  const nextBtn = container.querySelector('.btn-pagination-next');
  if (nextBtn && currentPage < totalPages) {
    nextBtn.addEventListener('click', () => {
      runsCurrentPage = currentPage + 1;
      loadRuns();
    });
  }

  const hasRunning = list.some(r => r.status === 'pending' || r.status === 'running');
  if (hasRunning && !runsListPollInterval) {
    runsListPollInterval = setInterval(loadRuns, RUNS_LIST_POLL_MS);
  } else if (!hasRunning) {
    stopRunsListPolling();
  }
}

let runsListPollInterval = null;
const RUNS_LIST_POLL_MS = 2000;

function stopRunsListPolling() {
  if (runsListPollInterval) {
    clearInterval(runsListPollInterval);
    runsListPollInterval = null;
  }
}

let inlineLogPollInterval = null;
let expandedRunId = null;

function stopInlineLogPolling() {
  if (inlineLogPollInterval) {
    clearInterval(inlineLogPollInterval);
    inlineLogPollInterval = null;
  }
  expandedRunId = null;
}

async function toggleRunLogInline(runId) {
  const logRow = document.getElementById(`run-log-${runId}`);
  const expandBtn = document.querySelector(`.run-expand-btn[data-run-id="${runId}"]`);
  if (!logRow || !expandBtn) return;

  const isExpanded = !logRow.hidden;
  if (isExpanded) {
    logRow.hidden = true;
    expandBtn.textContent = '▶';
    if (expandedRunId === runId) stopInlineLogPolling();
    return;
  }

  stopInlineLogPolling();
  logRow.hidden = false;
  expandBtn.textContent = '▼';
  expandedRunId = runId;

  const pre = logRow.querySelector('.run-log-inline-content');
  async function refreshInlineLog() {
    if (!document.getElementById(`run-log-${runId}`)) {
      stopInlineLogPolling();
      return;
    }
    try {
      const run = await getRun(runId);
      pre.textContent = run.log || '(no log yet)';
      if (run.status === 'success' || run.status === 'failed') stopInlineLogPolling();
    } catch (_) {}
  }
  await refreshInlineLog();
  if (expandedRunId === runId) {
    inlineLogPollInterval = setInterval(refreshInlineLog, LOG_POLL_MS);
  }
}

const logOverlay = document.getElementById('log-overlay');
const logTitle = document.getElementById('log-title');
const logContent = document.getElementById('log-content');
const logClose = document.getElementById('log-close');

let logPollInterval = null;
const LOG_POLL_MS = 1500;

function stopLogPolling() {
  if (logPollInterval) {
    clearInterval(logPollInterval);
    logPollInterval = null;
  }
}

function updateLogFromRun(run) {
  if (logTitle) logTitle.textContent = `Run #${run.id} · ${run.app_id} · ${run.status}${run.status === 'running' || run.status === 'pending' ? ' ● Live' : ''}`;
  if (logContent) logContent.textContent = run.log || '(no log yet)';
  if (run.status === 'success' || run.status === 'failed') {
    stopLogPolling();
  }
}

async function openLogModal(runId) {
  stopLogPolling();
  if (logTitle) logTitle.textContent = `Run #${runId} — Loading…`;
  if (logContent) logContent.textContent = '';
  if (logOverlay) logOverlay.setAttribute('aria-hidden', 'false');

  try {
    const run = await getRun(runId);
    updateLogFromRun(run);
    if (run.status === 'pending' || run.status === 'running') {
      logPollInterval = setInterval(async () => {
        try {
          const r = await getRun(runId);
          updateLogFromRun(r);
        } catch (_) {}
      }, LOG_POLL_MS);
    }
  } catch (e) {
    if (logContent) logContent.textContent = 'Error: ' + (e.message || 'Failed to load run');
  }
}

if (logOverlay && logClose) {
  logClose.addEventListener('click', () => {
    stopLogPolling();
    logOverlay.setAttribute('aria-hidden', 'true');
  });
  logOverlay.addEventListener('click', (e) => {
    if (e.target === logOverlay) {
      stopLogPolling();
      logOverlay.setAttribute('aria-hidden', 'true');
    }
  });
}
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    const lo = document.getElementById('log-overlay');
    if (lo && lo.getAttribute('aria-hidden') === 'false') {
      stopLogPolling();
      lo.setAttribute('aria-hidden', 'true');
    }
  }
});

let allAppsCache = [];

function applyFilter() {
  const container = document.getElementById('apps-grid');
  if (!container) return;
  const searchEl = document.getElementById('apps-search');
  const q = (searchEl && searchEl.value) ? searchEl.value.trim().toLowerCase() : '';
  const filtered = !q ? allAppsCache : allAppsCache.filter(a =>
    (a.name || '').toLowerCase().includes(q) || (a.id || '').toLowerCase().includes(q)
  );
  renderApps(container, filtered);
}

async function loadApps() {
  const container = document.getElementById('apps-grid');
  if (!container) return;
  try {
    const apps = await getApps();
    allAppsCache = apps;
    if (document.getElementById('apps-search')) {
      applyFilter();
    } else {
      renderApps(container, apps);
    }
  } catch (e) {
    container.innerHTML = `<p class="error-message">${escapeHtml(e.message || 'Failed to load apps')}</p>`;
  }
}

async function loadRuns() {
  const container = document.getElementById('runs-container');
  if (!container) return;
  const wasExpanded = expandedRunId;
  try {
    const offset = (runsCurrentPage - 1) * RUNS_PAGE_SIZE;
    const data = await getRuns('', RUNS_PAGE_SIZE, offset);
    const runs = data.runs || data;
    const total = typeof data.total === 'number' ? data.total : runs.length;
    renderRuns(container, runs, total, runsCurrentPage);
    if (wasExpanded != null) {
      setTimeout(() => toggleRunLogInline(wasExpanded), 0);
    }
  } catch (e) {
    container.innerHTML = `<p class="error-message">${escapeHtml(e.message || 'Failed to load runs')}</p>`;
  }
}

const appFormOverlay = document.getElementById('app-form-overlay');
const appForm = document.getElementById('app-form');
const appStepsContainer = document.getElementById('app-steps');
let appSSHKeysCache = [];

function clampStepSleepSec(value) {
  const n = parseInt(String(value || '').trim(), 10);
  if (isNaN(n) || n < 1) return 0;
  return Math.min(3600, n);
}

function legacyStepsFromApp(app) {
  const steps = [];
  if ((app.test_cmd || '').trim()) {
    steps.push({ name: 'test', cmd: app.test_cmd, sleep_sec: app.test_sleep_sec || 0 });
  }
  if ((app.build_cmd || '').trim()) {
    steps.push({ name: 'build', cmd: app.build_cmd, sleep_sec: app.build_sleep_sec || 0 });
  }
  if ((app.deploy_cmd || '').trim()) {
    steps.push({ name: 'deploy', cmd: app.deploy_cmd, sleep_sec: app.deploy_sleep_sec || 0 });
  }
  return steps;
}

function getEffectiveStepsForForm(app) {
  if (Array.isArray(app.steps) && app.steps.length > 0) return app.steps;
  return legacyStepsFromApp(app);
}

function renderStepRow(step = {}) {
  if (!appStepsContainer) return;
  const row = document.createElement('div');
  row.className = 'step-row';
  const stepName = (step.name || '').trim();
  const stepCmd = (step.cmd || '').trim();
  const sleepSec = clampStepSleepSec(step.sleep_sec);
  row.innerHTML = `
    <div class="step-row-top">
      <input type="text" class="form-input step-name" placeholder="Step name (e.g. test, build, deploy)" value="${escapeHtml(stepName)}" />
      <button type="button" class="btn btn-ghost step-remove-btn">Remove</button>
    </div>
    <div class="step-row-bottom">
      <input type="text" class="form-input step-cmd" placeholder="Command or script path" value="${escapeHtml(stepCmd)}" />
      <input type="number" class="form-input step-sleep-sec" min="0" max="3600" placeholder="Sleep (s)" value="${sleepSec > 0 ? String(sleepSec) : ''}" />
    </div>
  `;
  const removeBtn = row.querySelector('.step-remove-btn');
  if (removeBtn) {
    removeBtn.addEventListener('click', () => {
      row.remove();
      ensureAtLeastOneStepRow();
      refreshStepRemoveButtons();
    });
  }
  appStepsContainer.appendChild(row);
}

function refreshStepRemoveButtons() {
  if (!appStepsContainer) return;
  const rows = appStepsContainer.querySelectorAll('.step-row');
  const disabled = rows.length <= 1;
  rows.forEach(row => {
    const btn = row.querySelector('.step-remove-btn');
    if (btn) btn.disabled = disabled;
  });
}

function ensureAtLeastOneStepRow() {
  if (!appStepsContainer) return;
  if (appStepsContainer.querySelectorAll('.step-row').length === 0) {
    renderStepRow({ name: '', cmd: '', sleep_sec: 0 });
  }
}

function setAppStepsInForm(steps) {
  if (!appStepsContainer) return;
  appStepsContainer.innerHTML = '';
  const list = Array.isArray(steps) && steps.length > 0 ? steps : [{ name: '', cmd: '', sleep_sec: 0 }];
  list.forEach(step => renderStepRow(step));
  refreshStepRemoveButtons();
}

function collectAppStepsFromForm() {
  if (!appStepsContainer) return [];
  const rows = Array.from(appStepsContainer.querySelectorAll('.step-row'));
  const steps = [];
  rows.forEach((row, i) => {
    const nameInput = row.querySelector('.step-name');
    const cmdInput = row.querySelector('.step-cmd');
    const sleepInput = row.querySelector('.step-sleep-sec');
    const cmd = (cmdInput && cmdInput.value ? cmdInput.value : '').trim();
    if (!cmd) return;
    const rawName = (nameInput && nameInput.value ? nameInput.value : '').trim();
    steps.push({
      name: rawName || `step-${i + 1}`,
      cmd,
      sleep_sec: clampStepSleepSec(sleepInput ? sleepInput.value : 0),
    });
  });
  return steps;
}

function renderSSHKeyOptions(selectedName, isAdminEditing) {
  const select = document.getElementById('app-ssh-key');
  if (!select) return;
  const selected = (selectedName || '').trim();
  if (isAdminEditing) {
    const options = appSSHKeysCache.map(k => `<option value="${escapeHtml(k.name)}" ${k.name === selected ? 'selected' : ''}>${escapeHtml(k.name)}</option>`);
    select.innerHTML = `<option value="">Select an SSH key</option>${options.join('')}`;
    select.disabled = false;
    select.required = true;
    if (!selected && appSSHKeysCache.length === 1) {
      select.value = appSSHKeysCache[0].name;
    }
    return;
  }
  const value = selected || '';
  select.innerHTML = value
    ? `<option value="${escapeHtml(value)}">${escapeHtml(value)}</option>`
    : `<option value="">No SSH key configured</option>`;
  select.value = value;
  select.disabled = true;
  select.required = false;
}

async function openAppForm(appId) {
  const overlay = document.getElementById('app-form-overlay');
  const form = document.getElementById('app-form');
  const titleEl = document.getElementById('app-form-title');
  if (!form || !overlay) return;
  form.dataset.editId = appId || '';
  overlay.setAttribute('aria-hidden', 'false');

  if (appId) {
    if (titleEl) titleEl.textContent = 'Edit app';
    try {
      const app = await getApp(appId);
      if (currentUser && currentUser.is_admin) {
        appSSHKeysCache = await getSSHKeys();
      }
      document.getElementById('app-name').value = app.name || '';
      document.getElementById('app-repo').value = app.repo || '';
      document.getElementById('app-branch').value = app.branch || 'main';
      renderSSHKeyOptions(app.ssh_key_name || '', !!(currentUser && currentUser.is_admin));
      setAppStepsInForm(getEffectiveStepsForForm(app));
    } catch (_) {
      showToast('Failed to load app', 'error');
    }
  } else {
    if (titleEl) titleEl.textContent = 'Add app';
    form.reset();
    document.getElementById('app-branch').value = 'main';
    try {
      appSSHKeysCache = await getSSHKeys();
      renderSSHKeyOptions('', true);
      if (!appSSHKeysCache.length) {
        showToast('Create at least one SSH key in Access before creating an app.', 'error');
      }
    } catch (err) {
      showToast(err.message || 'Failed to load SSH keys', 'error');
      renderSSHKeyOptions('', true);
    }
    setAppStepsInForm([]);
  }
}

function closeAppForm() {
  const overlay = document.getElementById('app-form-overlay');
  if (overlay) overlay.setAttribute('aria-hidden', 'true');
}

async function getApp(id) {
  const res = await fetchApi(`/apps/${encodeURIComponent(id)}`);
  if (!res.ok) throw new Error('Failed to load app');
  return res.json();
}

if (appForm) {
  appForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const form = document.getElementById('app-form');
    const editId = (form && form.dataset.editId) || '';
    const steps = collectAppStepsFromForm();
    if (!steps.length) {
      showToast('At least one step command is required.', 'error');
      return;
    }
    const app = {
      name: document.getElementById('app-name').value.trim(),
      repo: document.getElementById('app-repo').value.trim(),
      branch: document.getElementById('app-branch').value.trim() || 'main',
      ssh_key_name: (document.getElementById('app-ssh-key') || {}).value || '',
      steps,
    };
    if (!app.ssh_key_name.trim() && (!editId || (currentUser && currentUser.is_admin))) {
      showToast('SSH key is required.', 'error');
      return;
    }
    try {
      if (editId) {
        await updateApp(editId, app);
        showToast('App updated.', 'success');
      } else {
        await createApp(app);
        showToast('App created.', 'success');
      }
      closeAppForm();
      loadApps();
    } catch (err) {
      showToast(err.message || 'Failed to save app', 'error');
    }
  });
}

const addStepBtn = document.getElementById('add-step-btn');
if (addStepBtn) {
  addStepBtn.addEventListener('click', () => {
    renderStepRow({ name: '', cmd: '', sleep_sec: 0 });
    refreshStepRemoveButtons();
  });
}

const addAppBtn = document.getElementById('add-app-btn');
if (addAppBtn) addAppBtn.addEventListener('click', () => openAppForm(null));

const appFormCloseBtn = document.getElementById('app-form-close');
if (appFormCloseBtn) appFormCloseBtn.addEventListener('click', closeAppForm);
const appFormCancelBtn = document.getElementById('app-form-cancel');
if (appFormCancelBtn) appFormCancelBtn.addEventListener('click', closeAppForm);
if (appFormOverlay) appFormOverlay.addEventListener('click', (e) => { if (e.target === appFormOverlay) closeAppForm(); });
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    const overlay = document.getElementById('app-form-overlay');
    if (overlay && overlay.getAttribute('aria-hidden') === 'false') closeAppForm();
  }
});

function confirmDeleteApp(appId) {
  if (!confirm(`Delete app "${appId}"? This cannot be undone.`)) return;
  deleteApp(appId).then(() => {
    showToast('App deleted.', 'success');
    loadApps();
  }).catch(err => showToast(err.message || 'Failed to delete app', 'error'));
}

async function checkServerReachable() {
  try {
    const res = await fetch('/health', { method: 'GET', credentials: 'omit' });
    return res.ok;
  } catch (_) {
    return false;
  }
}

function showServerUnreachableMessage() {
  const errEl = document.getElementById('server-error');
  if (errEl) {
    errEl.textContent = 'Cannot reach server. Start it with: make run (or go run ./cmd/cicd)';
    errEl.hidden = false;
  }
}

function initRunsPage() {
  loadRuns();
  const refreshBtn = document.getElementById('refresh-runs');
  if (refreshBtn) refreshBtn.addEventListener('click', loadRuns);
}

function initAppsPage() {
  loadApps();
  const searchInput = document.getElementById('apps-search');
  if (searchInput) searchInput.addEventListener('input', applyFilter);
  const addAppBtn = document.getElementById('add-app-btn');
  if (addAppBtn && currentUser && !currentUser.is_admin) {
    addAppBtn.hidden = true;
  }
}

function selectedGroupIDsFromContainer(container) {
  if (!container) return [];
  return Array.from(container.querySelectorAll('input[type="checkbox"]:checked'))
    .map(input => Number(input.value))
    .filter(n => Number.isInteger(n) && n > 0);
}

function renderGroupSelector(container, groups, checkedIDs = []) {
  if (!container) return;
  const checked = new Set(checkedIDs);
  if (!groups.length) {
    container.innerHTML = '<p class="empty-state compact">Create a group first.</p>';
    return;
  }
  container.innerHTML = groups.map(group => `
    <label class="form-checkbox-label checkbox-item">
      <input type="checkbox" value="${group.id}" ${checked.has(group.id) ? 'checked' : ''} />
      <span>${escapeHtml(group.name)}</span>
    </label>
  `).join('');
}

function renderGroupsList(groups) {
  const container = document.getElementById('groups-container');
  if (!container) return;
  if (!groups.length) {
    container.innerHTML = '<p class="empty-state compact">No groups created yet.</p>';
    return;
  }
  container.innerHTML = groups.map(g => `
    <article class="list-item">
      <a class="group-link" href="/group.html?group_id=${encodeURIComponent(g.id)}">${escapeHtml(g.name)}</a>
      <span class="muted-mono">#${g.id}</span>
    </article>
  `).join('');
}

function renderUsersList(users, groups) {
  const container = document.getElementById('users-container');
  if (!container) return;
  const groupNameByID = new Map((groups || []).map(g => [g.id, g.name]));
  if (!users.length) {
    container.innerHTML = '<p class="empty-state compact">No users created yet.</p>';
    return;
  }
  container.innerHTML = users.map(user => {
    const tags = (user.group_ids || [])
      .map(id => groupNameByID.get(id) || `#${id}`)
      .map(name => `<span class="pill">${escapeHtml(name)}</span>`)
      .join('');
    return `
      <article class="list-item list-item-column" data-user-id="${user.id}">
        <div class="list-item-head">
          <strong>${escapeHtml(user.username)}</strong>
          <span class="muted-mono">#${user.id}</span>
        </div>
        <div class="pill-row">${tags || '<span class="muted">No groups</span>'}</div>
        <div class="inline-actions">
          <button type="button" class="btn btn-ghost btn-sm edit-user-groups-btn" data-user-id="${user.id}">Edit groups</button>
          <button type="button" class="btn btn-ghost btn-sm edit-user-password-btn" data-user-id="${user.id}">Change password</button>
          ${user.is_admin ? '' : `<button type="button" class="btn btn-ghost btn-sm delete-user-btn" data-user-id="${user.id}" ${currentUser && Number(currentUser.id) === Number(user.id) ? 'disabled' : ''}>Delete</button>`}
        </div>
      </article>
    `;
  }).join('');
}

function renderAppPermissions(apps, groups, appGroupMap) {
  const container = document.getElementById('app-permissions-container');
  if (!container) return;
  if (!apps.length) {
    container.innerHTML = '<p class="empty-state">No apps created yet.</p>';
    return;
  }
  if (!groups.length) {
    container.innerHTML = '<p class="empty-state">Create groups to configure app permissions.</p>';
    return;
  }
  container.innerHTML = apps.map(app => {
    const assigned = new Set(appGroupMap.get(app.id) || []);
    const checkboxes = groups.map(group => `
      <label class="form-checkbox-label checkbox-item">
        <input type="checkbox" class="app-group-checkbox" data-app-id="${escapeHtml(app.id)}" value="${group.id}" ${assigned.has(group.id) ? 'checked' : ''} />
        <span>${escapeHtml(group.name)}</span>
      </label>
    `).join('');
    return `
      <article class="app-card acl-card" data-app-id="${escapeHtml(app.id)}">
        <h3>${escapeHtml(app.name)}</h3>
        <p class="app-id">${escapeHtml(app.id)}</p>
        <div class="checkbox-grid">${checkboxes}</div>
      </article>
    `;
  }).join('');
}

function renderSSHKeysList(keys) {
  const container = document.getElementById('ssh-keys-container');
  if (!container) return;
  if (!keys.length) {
    container.innerHTML = '<p class="empty-state compact">No SSH keys created yet.</p>';
    return;
  }
  container.innerHTML = keys.map(k => `
    <article class="list-item">
      <div class="list-item-head">
        <strong>${escapeHtml(k.name)}</strong>
        <span class="muted-mono">#${k.id}</span>
      </div>
      <button type="button" class="btn btn-ghost btn-sm delete-ssh-key-btn" data-key-id="${k.id}">Delete</button>
    </article>
  `).join('');
}

async function initAccessPage() {
  const groupsContainer = document.getElementById('groups-container');
  const usersContainer = document.getElementById('users-container');
  const appPermissionsContainer = document.getElementById('app-permissions-container');
  if (!groupsContainer || !usersContainer || !appPermissionsContainer) return;

  let groups = [];
  let users = [];
  let apps = [];
  let sshKeys = [];
  const appGroupMap = new Map();
  let editingUserId = null;

  async function reloadAll() {
    groups = await getGroups();
    users = await getUsers();
    apps = await getApps();
    sshKeys = await getSSHKeys();
    const allAppGroups = await Promise.all(apps.map(app => getAppGroups(app.id)));
    appGroupMap.clear();
    for (const item of allAppGroups) {
      appGroupMap.set(item.app_id, item.group_ids || []);
    }
    renderGroupsList(groups);
    renderUsersList(users, groups);
    renderGroupSelector(document.getElementById('user-group-selector'), groups, []);
    renderAppPermissions(apps, groups, appGroupMap);
    renderSSHKeysList(sshKeys);
    bindAccessActions();
  }

  function bindAccessActions() {
    document.querySelectorAll('.edit-user-groups-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const userId = Number(btn.dataset.userId);
        const user = users.find(u => Number(u.id) === userId);
        if (!user) return;
        openUserGroupsEditor(user);
      });
    });

    document.querySelectorAll('.edit-user-password-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const userId = Number(btn.dataset.userId);
        const user = users.find(u => Number(u.id) === userId);
        if (!user) return;
        const password = prompt(`New password for ${user.username}`);
        if (password == null) return;
        const trimmed = password.trim();
        if (!trimmed) {
          showToast('Password cannot be empty.', 'error');
          return;
        }
        try {
          await updateUserPassword(userId, trimmed);
          showToast('Password updated.', 'success');
        } catch (err) {
          showToast(err.message || 'Failed to update password', 'error');
        }
      });
    });

    document.querySelectorAll('.delete-user-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const userId = Number(btn.dataset.userId);
        const user = users.find(u => Number(u.id) === userId);
        if (!user) return;
        if (!confirm(`Delete user "${user.username}"?`)) return;
        try {
          await deleteUserById(userId);
          showToast('User deleted.', 'success');
          await reloadAll();
        } catch (err) {
          showToast(err.message || 'Failed to delete user', 'error');
        }
      });
    });

    document.querySelectorAll('.app-group-checkbox').forEach(input => {
      input.addEventListener('change', async () => {
        const appId = input.dataset.appId;
        const card = input.closest('.acl-card');
        const selectedIDs = selectedGroupIDsFromContainer(card);
        const allInputs = card ? card.querySelectorAll('.app-group-checkbox') : [];
        allInputs.forEach(el => { el.disabled = true; });
        try {
          await setAppGroups(appId, selectedIDs);
          appGroupMap.set(appId, selectedIDs);
          showToast(`Permissions for app "${appId}" updated.`, 'success');
        } catch (err) {
          showToast(err.message || 'Failed to update permissions', 'error');
          await reloadAll();
          return;
        } finally {
          allInputs.forEach(el => { el.disabled = false; });
        }
      });
    });

    document.querySelectorAll('.delete-ssh-key-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const keyId = Number(btn.dataset.keyId);
        const key = sshKeys.find(k => Number(k.id) === keyId);
        const label = key ? key.name : `#${keyId}`;
        if (!confirm(`Delete SSH key "${label}"?`)) return;
        try {
          await deleteSSHKeyById(keyId);
          showToast('SSH key deleted.', 'success');
          await reloadAll();
        } catch (err) {
          showToast(err.message || 'Failed to delete SSH key', 'error');
        }
      });
    });
  }

  function openUserGroupsEditor(user) {
    const overlay = document.getElementById('user-groups-overlay');
    const title = document.getElementById('user-groups-title');
    const selector = document.getElementById('user-groups-selector');
    if (!overlay || !selector) return;
    editingUserId = Number(user.id);
    if (title) title.textContent = `${user.username} groups`;
    renderGroupSelector(selector, groups, user.group_ids || []);
    overlay.setAttribute('aria-hidden', 'false');
  }

  function closeUserGroupsEditor() {
    const overlay = document.getElementById('user-groups-overlay');
    if (overlay) overlay.setAttribute('aria-hidden', 'true');
    editingUserId = null;
  }

  const groupForm = document.getElementById('group-form');
  if (groupForm) {
    groupForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const nameInput = document.getElementById('group-name');
      const name = nameInput ? nameInput.value.trim() : '';
      if (!name) return;
      try {
        await createGroup(name);
        if (nameInput) nameInput.value = '';
        showToast('Group created.', 'success');
        await reloadAll();
      } catch (err) {
        showToast(err.message || 'Failed to create group', 'error');
      }
    });
  }

  const userForm = document.getElementById('user-form');
  if (userForm) {
    userForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const usernameInput = document.getElementById('user-username');
      const passwordInput = document.getElementById('user-password');
      const isAdminInput = document.getElementById('user-is-admin');
      const username = usernameInput ? usernameInput.value.trim() : '';
      const password = passwordInput ? passwordInput.value.trim() : '';
      const isAdmin = !!(isAdminInput && isAdminInput.checked);
      const groupIDs = selectedGroupIDsFromContainer(document.getElementById('user-group-selector'));
      if (!username || !password) return;
      try {
        await createUser({ username: username, password: password, group_ids: groupIDs, is_admin: isAdmin });
        if (usernameInput) usernameInput.value = '';
        if (passwordInput) passwordInput.value = '';
        if (isAdminInput) isAdminInput.checked = false;
        renderGroupSelector(document.getElementById('user-group-selector'), groups, []);
        showToast('User created.', 'success');
        await reloadAll();
      } catch (err) {
        showToast(err.message || 'Failed to create user', 'error');
      }
    });
  }

  const sshKeyForm = document.getElementById('ssh-key-form');
  if (sshKeyForm) {
    sshKeyForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const nameInput = document.getElementById('ssh-key-name');
      const keyInput = document.getElementById('ssh-key-private');
      const name = nameInput ? nameInput.value.trim() : '';
      const privateKey = keyInput ? keyInput.value.trim() : '';
      if (!name || !privateKey) return;
      try {
        await createSSHKey(name, privateKey);
        if (nameInput) nameInput.value = '';
        if (keyInput) keyInput.value = '';
        showToast('SSH key created.', 'success');
        await reloadAll();
      } catch (err) {
        showToast(err.message || 'Failed to create SSH key', 'error');
      }
    });
  }

  const userGroupsSave = document.getElementById('user-groups-save');
  if (userGroupsSave) {
    userGroupsSave.addEventListener('click', async () => {
      if (!editingUserId) return;
      const selector = document.getElementById('user-groups-selector');
      const groupIDs = selectedGroupIDsFromContainer(selector);
      userGroupsSave.disabled = true;
      try {
        await setUserGroups(editingUserId, groupIDs);
        showToast('User groups updated.', 'success');
        closeUserGroupsEditor();
        await reloadAll();
      } catch (err) {
        showToast(err.message || 'Failed to update groups', 'error');
      } finally {
        userGroupsSave.disabled = false;
      }
    });
  }

  const userGroupsClose = document.getElementById('user-groups-close');
  const userGroupsCancel = document.getElementById('user-groups-cancel');
  const userGroupsOverlay = document.getElementById('user-groups-overlay');
  if (userGroupsClose) userGroupsClose.addEventListener('click', closeUserGroupsEditor);
  if (userGroupsCancel) userGroupsCancel.addEventListener('click', closeUserGroupsEditor);
  if (userGroupsOverlay) {
    userGroupsOverlay.addEventListener('click', (e) => {
      if (e.target === userGroupsOverlay) closeUserGroupsEditor();
    });
  }

  try {
    await reloadAll();
  } catch (err) {
    groupsContainer.innerHTML = `<p class="error-message">${escapeHtml(err.message || 'Failed to load access data')}</p>`;
    usersContainer.innerHTML = `<p class="error-message">${escapeHtml(err.message || 'Failed to load access data')}</p>`;
    appPermissionsContainer.innerHTML = `<p class="error-message">${escapeHtml(err.message || 'Failed to load access data')}</p>`;
    const keysContainer = document.getElementById('ssh-keys-container');
    if (keysContainer) keysContainer.innerHTML = `<p class="error-message">${escapeHtml(err.message || 'Failed to load access data')}</p>`;
  }
}

function selectedUserIDs(container) {
  if (!container) return [];
  return Array.from(container.querySelectorAll('input[type="checkbox"]:checked'))
    .map(input => Number(input.value))
    .filter(n => Number.isInteger(n) && n > 0);
}

function selectedAppIDs(container) {
  if (!container) return [];
  return Array.from(container.querySelectorAll('input[type="checkbox"]:checked'))
    .map(input => String(input.value || '').trim())
    .filter(v => v !== '');
}

function renderGroupUsersSelector(container, users, selectedIDs) {
  if (!container) return;
  const selected = new Set(selectedIDs || []);
  container.innerHTML = users.map(user => `
    <label class="form-checkbox-label checkbox-item">
      <input type="checkbox" value="${user.id}" ${selected.has(user.id) ? 'checked' : ''} />
      <span>${escapeHtml(user.username)}</span>
    </label>
  `).join('');
}

function renderGroupAppsSelector(container, apps, selectedIDs) {
  if (!container) return;
  const selected = new Set(selectedIDs || []);
  container.innerHTML = apps.map(app => `
    <label class="form-checkbox-label checkbox-item">
      <input type="checkbox" value="${escapeHtml(app.id)}" ${selected.has(app.id) ? 'checked' : ''} />
      <span>${escapeHtml(app.name)} <span class="muted-mono">(${escapeHtml(app.id)})</span></span>
    </label>
  `).join('');
}

async function initGroupPage() {
  const groupTitle = document.getElementById('group-title');
  if (!groupTitle) return;
  const params = new URLSearchParams(window.location.search);
  const groupID = Number(params.get('group_id'));
  if (!Number.isInteger(groupID) || groupID <= 0) {
    groupTitle.textContent = 'Invalid group';
    return;
  }

  const usersBox = document.getElementById('group-users-selector');
  const appsBox = document.getElementById('group-apps-selector');
  const saveUsersBtn = document.getElementById('save-group-users');
  const saveAppsBtn = document.getElementById('save-group-apps');

  async function reload() {
    const data = await getGroup(groupID);
    groupTitle.textContent = `Group: ${data.name}`;
    renderGroupUsersSelector(usersBox, data.available_users || [], data.user_ids || []);
    renderGroupAppsSelector(appsBox, data.available_apps || [], data.app_ids || []);
  }

  if (saveUsersBtn) {
    saveUsersBtn.addEventListener('click', async () => {
      const userIDs = selectedUserIDs(usersBox);
      saveUsersBtn.disabled = true;
      try {
        await setGroupUsers(groupID, userIDs);
        showToast('Group users updated.', 'success');
      } catch (err) {
        showToast(err.message || 'Failed to save users', 'error');
      } finally {
        saveUsersBtn.disabled = false;
      }
    });
  }

  if (saveAppsBtn) {
    saveAppsBtn.addEventListener('click', async () => {
      const appIDs = selectedAppIDs(appsBox);
      saveAppsBtn.disabled = true;
      try {
        await setGroupApps(groupID, appIDs);
        showToast('Group apps updated.', 'success');
      } catch (err) {
        showToast(err.message || 'Failed to save apps', 'error');
      } finally {
        saveAppsBtn.disabled = false;
      }
    });
  }

  try {
    await reload();
  } catch (err) {
    groupTitle.textContent = err.message || 'Failed to load group';
  }
}

async function initProfilePage() {
  const infoEl = document.getElementById('profile-info');
  const groupsEl = document.getElementById('profile-groups');
  const appsEl = document.getElementById('profile-apps');
  const pwdForm = document.getElementById('profile-password-form');
  if (!infoEl || !groupsEl || !appsEl) return;

  try {
    const p = await getProfile();
    infoEl.innerHTML = `
      <article class="list-item"><strong>User</strong><span>${escapeHtml(p.username || '-')}</span></article>
      <article class="list-item"><strong>Admin</strong><span>${p.is_admin ? 'Yes' : 'No'}</span></article>
    `;

    const groups = Array.isArray(p.groups) ? p.groups : [];
    groupsEl.innerHTML = groups.length
      ? groups.map(g => `<article class="list-item"><strong>${escapeHtml(g.name)}</strong><span class="muted-mono">#${g.id}</span></article>`).join('')
      : '<p class="empty-state compact">No groups.</p>';

    const apps = Array.isArray(p.apps) ? p.apps : [];
    appsEl.innerHTML = apps.length
      ? apps.map(a => `<article class="list-item list-item-column"><strong>${escapeHtml(a.name || a.id)}</strong><span class="muted-mono">${escapeHtml(a.id || '')}</span><span class="muted">${escapeHtml(a.repo || '')}</span></article>`).join('')
      : '<p class="empty-state compact">No linked apps.</p>';
  } catch (err) {
    infoEl.innerHTML = `<p class="error-message">${escapeHtml(err.message || 'Failed to load profile')}</p>`;
    groupsEl.innerHTML = '';
    appsEl.innerHTML = '';
  }

  if (pwdForm) {
    pwdForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const currEl = document.getElementById('profile-current-password');
      const nextEl = document.getElementById('profile-new-password');
      const curr = currEl ? currEl.value.trim() : '';
      const next = nextEl ? nextEl.value.trim() : '';
      if (!curr || !next) {
        showToast('Fill current password and new password.', 'error');
        return;
      }
      try {
        await changeMyPassword(curr, next);
        if (currEl) currEl.value = '';
        if (nextEl) nextEl.value = '';
        showToast('Password changed successfully.', 'success');
      } catch (err) {
        showToast(err.message || 'Failed to change password', 'error');
      }
    });
  }
}

function bindHeaderUser(user) {
  currentUser = user || null;
  const currentUserEl = document.getElementById('current-user');
  if (currentUserEl && user) {
    currentUserEl.textContent = `${user.username}${user.is_admin ? ' (admin)' : ''}`;
  }
  if (!user || !user.is_admin) {
    document.querySelectorAll('a[href="/access.html"]').forEach(el => {
      el.style.display = 'none';
    });
  }
  const logoutBtn = document.getElementById('logout-btn');
  if (logoutBtn) {
    logoutBtn.addEventListener('click', async () => {
      await logout();
      window.location.href = '/login.html';
    });
  }
}

async function initLoginPage() {
  const loginForm = document.getElementById('login-form');
  if (!loginForm) return false;
  try {
    await getMe();
    window.location.href = '/';
    return true;
  } catch (_) {}

  loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const username = (document.getElementById('login-username') || {}).value || '';
    const password = (document.getElementById('login-password') || {}).value || '';
    const errorEl = document.getElementById('login-error');
    if (errorEl) errorEl.hidden = true;
    try {
      await login(username.trim(), password);
      window.location.href = '/';
    } catch (err) {
      if (errorEl) {
        errorEl.textContent = err.message || 'Login failed';
        errorEl.hidden = false;
      }
    }
  });
  return true;
}

async function ensureAuthenticated() {
  try {
    const data = await getMe();
    return data.user;
  } catch (_) {
    window.location.href = '/login.html';
    return null;
  }
}

async function init() {
  const reachable = await checkServerReachable();
  if (!reachable) {
    showServerUnreachableMessage();
    return;
  }
  if (await initLoginPage()) return;
  const me = await ensureAuthenticated();
  if (!me) return;
  bindHeaderUser(me);
  const isAccessPage = !!document.getElementById('groups-container');
  const isGroupPage = !!document.getElementById('group-title');
  if (!me.is_admin && (isAccessPage || isGroupPage)) {
    const main = document.querySelector('.main');
    if (main) main.innerHTML = '';
    return;
  }
  try {
    if (document.getElementById('runs-container')) initRunsPage();
    if (document.getElementById('apps-grid')) initAppsPage();
    if (document.getElementById('profile-info')) initProfilePage();
    if (document.getElementById('group-title')) initGroupPage();
    if (document.getElementById('groups-container')) {
      initAccessPage();
    }
  } catch (_) {
    showServerUnreachableMessage();
  }
}

init();
