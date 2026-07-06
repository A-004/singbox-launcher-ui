// State
let status = {
  vpnRunning: false,
  coreVersion: '—',
  proxyCount: 0,
  selected: '',
  proxies: []
};

let pollingInterval = null;
const POLL_INTERVAL = 2000; // 2 seconds

// DOM refs
const $ = id => document.getElementById(id);

const vpnLabel = $('vpnLabel');
const vpnSubtitle = $('vpnSubtitle');
const vpnIndicator = $('vpnIndicator');
const statusCard = document.querySelector('.status-card');
const vpnToggle = $('vpnToggle');
const coreVersion = $('coreVersion');
const proxyCount = $('proxyCount');
const activeProxy = $('activeProxy');
const proxyBadge = $('proxyBadge');
const proxyList = $('proxyList');
const statusDot = $('statusDot');

// === Initialization ===
async function init() {
  await fetchStatus();
  startPolling();
}

// === Polling ===
function startPolling() {
  if (pollingInterval) clearInterval(pollingInterval);
  pollingInterval = setInterval(fetchStatus, POLL_INTERVAL);
}

// === API calls ===
async function fetchStatus() {
  try {
    const resp = await fetch('/api/status');
    const data = await resp.json();
    status = data;
    render();
  } catch (err) {
    console.warn('fetchStatus error:', err);
  }
}

async function toggleVPN() {
  try {
    vpnToggle.disabled = true;
    const resp = await fetch('/api/toggle-vpn', { method: 'POST' });
    const data = await resp.json();
    if (data.ok) {
      status.vpnRunning = data.running;
      render();
      // Re-fetch after a moment to get real state
      setTimeout(fetchStatus, 500);
    }
  } catch (err) {
    console.warn('toggleVPN error:', err);
  } finally {
    vpnToggle.disabled = false;
  }
}

async function selectProxy(name) {
  if (!name || name === status.selected) return;
  try {
    await fetch('/api/select-proxy', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name })
    });
    status.selected = name;
    render();
  } catch (err) {
    console.warn('selectProxy error:', err);
  }
}

// === Render ===
function render() {
  // VPN Status
  const connected = status.vpnRunning;
  statusCard.className = `status-card ${connected ? 'connected' : 'disconnected'}`;
  vpnLabel.textContent = connected ? 'Connected' : 'Disconnected';
  vpnSubtitle.textContent = connected ? 'VPN is active and protected' : 'VPN is not active';
  statusDot.className = `status-dot ${connected ? 'connected' : 'disconnected'}`;

  // Core version
  coreVersion.textContent = status.coreVersion || '—';

  // Stats
  proxyCount.textContent = status.proxyCount;
  activeProxy.textContent = status.selected || '—';

  // Badge
  proxyBadge.textContent = status.proxyCount;

  // Proxy list
  renderProxyList();
}

function renderProxyList() {
  if (!status.proxies || status.proxies.length === 0) {
    proxyList.innerHTML = `<div class="proxy-item empty-state"><span>No proxies loaded</span></div>`;
    return;
  }

  proxyList.innerHTML = status.proxies.map(p => {
    const delayClass = getDelayClass(p.delay);
    const selectedClass = p.selected ? ' selected' : '';
    return `
      <div class="proxy-item${selectedClass}" onclick="selectProxy('${escapeHTML(p.name)}')">
        <span class="proxy-select-indicator"></span>
        <div class="proxy-info">
          <span class="proxy-name">${escapeHTML(p.displayName || p.name)}</span>
          <span class="proxy-type">${escapeHTML(p.type || 'proxy')}</span>
        </div>
        <span class="proxy-delay ${delayClass}">${p.delayText || '⏳'}</span>
      </div>
    `;
  }).join('');
}

function getDelayClass(delay) {
  if (delay <= 0) return '';
  if (delay < 200) return 'good';
  if (delay < 600) return 'medium';
  return 'bad';
}

function escapeHTML(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&')
    .replace(/</g, '<')
    .replace(/>/g, '>')
    .replace(/"/g, '"')
    .replace(/'/g, '&#39;');
}

// === Start ===
document.addEventListener('DOMContentLoaded', init);