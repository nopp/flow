const API = '/api';

async function fetchApi(path, options = {}) {
  const headers = options.headers || {};
  const res = await fetch(API + path, {
    method: options.method || 'GET',
    headers: headers,
    body: options.body,
  });
  return res;
}

async function getApps() {
  const res = await fetchApi('/apps');
  if (!res.ok) throw new Error('Failed to load apps');
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

async function getRuns(appId = '', limit = 30) {
  const params = new URLSearchParams();
  if (appId) params.set('app_id', appId);
  params.set('limit', String(limit));
  const res = await fetchApi(`/runs?${params}`);
  if (!res.ok) throw new Error('Failed to load runs');
  return res.json();
}

async function getRun(id) {
  const res = await fetchApi(`/runs/${id}`);
  if (!res.ok) throw new Error('Failed to load run');
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
    container.innerHTML = `<p class="empty-state">${emptyMsg} <button type="button" class="btn btn-primary" id="add-app-empty">Add app</button></p>`;
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
        <button type="button" class="btn btn-ghost delete-btn btn-delete" data-app-id="${escapeHtml(app.id)}">Delete</button>
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

function renderRuns(container, runs) {
  stopInlineLogPolling();
  const list = Array.isArray(runs) ? runs : [];
  if (!list.length) {
    container.innerHTML = '<p class="empty-state">No runs yet. Trigger a run from the <a href="/apps.html">Apps</a> page.</p>';
    return;
  }
  container.innerHTML = `
    <table class="runs-table">
      <thead>
        <tr>
          <th class="run-expand-col"></th>
          <th>ID</th>
          <th>App</th>
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
            <td><span class="badge ${statusClass(run.status)}">${escapeHtml(run.status)}</span></td>
            <td>${formatDate(run.started_at)}</td>
            <td>${formatDuration(run)}</td>
          </tr>
          <tr class="run-log-row" id="run-log-${run.id}" data-run-id="${run.id}" hidden>
            <td colspan="6">
              <div class="run-log-inline">
                <pre class="run-log-inline-content">${escapeHtml(run.log || '(no log yet)')}</pre>
              </div>
            </td>
          </tr>
        `).join('')}
      </tbody>
    </table>
  `;

  container.querySelectorAll('.run-expand-btn').forEach(btn => {
    btn.addEventListener('click', () => toggleRunLogInline(Number(btn.dataset.runId)));
  });

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
    const runs = await getRuns();
    renderRuns(container, runs);
    if (wasExpanded != null) {
      setTimeout(() => toggleRunLogInline(wasExpanded), 0);
    }
  } catch (e) {
    container.innerHTML = `<p class="error-message">${escapeHtml(e.message || 'Failed to load runs')}</p>`;
  }
}

const appFormOverlay = document.getElementById('app-form-overlay');
const appForm = document.getElementById('app-form');
const appFormTitle = document.getElementById('app-form-title');
const appIdInput = document.getElementById('app-id');

async function openAppForm(appId) {
  const overlay = document.getElementById('app-form-overlay');
  const form = document.getElementById('app-form');
  const titleEl = document.getElementById('app-form-title');
  const idInput = document.getElementById('app-id');
  if (!form || !overlay) return;
  form.dataset.editId = appId || '';
  overlay.setAttribute('aria-hidden', 'false');

  if (appId) {
    if (titleEl) titleEl.textContent = 'Edit app';
    if (idInput) idInput.disabled = true;
    try {
      const app = await getApp(appId);
      document.getElementById('app-id').value = app.id || '';
      document.getElementById('app-name').value = app.name || '';
      document.getElementById('app-repo').value = app.repo || '';
      document.getElementById('app-branch').value = app.branch || 'main';
      document.getElementById('app-test-cmd').value = app.test_cmd || '';
      document.getElementById('app-build-cmd').value = app.build_cmd || '';
      document.getElementById('app-deploy-cmd').value = app.deploy_cmd || '';
      const testSec = app.test_sleep_sec || 0;
      const buildSec = app.build_sleep_sec || 0;
      const deploySec = app.deploy_sleep_sec || 0;
      document.getElementById('app-test-sleep').checked = testSec > 0;
      document.getElementById('app-test-sleep-sec').value = testSec > 0 ? String(testSec) : '';
      document.getElementById('app-test-sleep-sec').disabled = testSec <= 0;
      document.getElementById('app-build-sleep').checked = buildSec > 0;
      document.getElementById('app-build-sleep-sec').value = buildSec > 0 ? String(buildSec) : '';
      document.getElementById('app-build-sleep-sec').disabled = buildSec <= 0;
      document.getElementById('app-deploy-sleep').checked = deploySec > 0;
      document.getElementById('app-deploy-sleep-sec').value = deploySec > 0 ? String(deploySec) : '';
      document.getElementById('app-deploy-sleep-sec').disabled = deploySec <= 0;
    } catch (_) {
      showToast('Failed to load app', 'error');
    }
  } else {
    if (titleEl) titleEl.textContent = 'Add app';
    if (idInput) idInput.disabled = false;
    form.reset();
    document.getElementById('app-branch').value = 'main';
    document.getElementById('app-test-sleep-sec').disabled = true;
    document.getElementById('app-build-sleep-sec').disabled = true;
    document.getElementById('app-deploy-sleep-sec').disabled = true;
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

function getSleepSec(checkboxId, inputId) {
  const cb = document.getElementById(checkboxId);
  const input = document.getElementById(inputId);
  if (!cb || !cb.checked || !input) return 0;
  const n = parseInt(input.value, 10);
  return isNaN(n) || n < 1 ? 0 : Math.min(3600, n);
}

if (appForm) {
  appForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const form = document.getElementById('app-form');
    const editId = (form && form.dataset.editId) || '';
    const app = {
    id: document.getElementById('app-id').value.trim(),
    name: document.getElementById('app-name').value.trim(),
    repo: document.getElementById('app-repo').value.trim(),
    branch: document.getElementById('app-branch').value.trim() || 'main',
    test_cmd: document.getElementById('app-test-cmd').value.trim(),
    build_cmd: document.getElementById('app-build-cmd').value.trim(),
    deploy_cmd: document.getElementById('app-deploy-cmd').value.trim(),
    test_sleep_sec: getSleepSec('app-test-sleep', 'app-test-sleep-sec'),
    build_sleep_sec: getSleepSec('app-build-sleep', 'app-build-sleep-sec'),
    deploy_sleep_sec: getSleepSec('app-deploy-sleep', 'app-deploy-sleep-sec'),
  };
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

function bindSleepCheckbox(checkboxId, inputId) {
  const cb = document.getElementById(checkboxId);
  const input = document.getElementById(inputId);
  if (!cb || !input) return;
  cb.addEventListener('change', () => {
    input.disabled = !cb.checked;
    if (!cb.checked) input.value = '';
  });
}
if (document.getElementById('app-test-sleep')) {
  bindSleepCheckbox('app-test-sleep', 'app-test-sleep-sec');
  bindSleepCheckbox('app-build-sleep', 'app-build-sleep-sec');
  bindSleepCheckbox('app-deploy-sleep', 'app-deploy-sleep-sec');
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
}

async function init() {
  const reachable = await checkServerReachable();
  if (!reachable) {
    showServerUnreachableMessage();
    return;
  }
  try {
    if (document.getElementById('runs-container')) initRunsPage();
    if (document.getElementById('apps-grid')) initAppsPage();
  } catch (_) {
    showServerUnreachableMessage();
  }
}

init();
