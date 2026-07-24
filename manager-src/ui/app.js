/* ==========================================================================
   DevEnv Manager UI Logic & Service State Machine
   Author: Antigravity Team
   Features: Unified Toggle Cards, State Locking, Synchronous Response Handling
   ========================================================================== */

// Service State Store (State machine: 'stopped' | 'starting' | 'running' | 'stopping')
const serviceStates = {
    mariadb: 'stopped',
    pma: 'stopped',
    mailpit: 'stopped'
};

// TIP: Renders non-intrusive notification toast
function showToast(message, type = 'success') {
    const container = document.getElementById('toast-container');
    if (!container) return;

    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    container.appendChild(toast);

    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateY(8px)';
        setTimeout(() => toast.remove(), 300);
    }, 3200);
}

// TIP: Renders the visual state of a service card component
function renderServiceUI(serviceKey) {
    const state = serviceStates[serviceKey];
    const cardElement = document.getElementById(`card-${serviceKey}`);
    const badgeElement = document.getElementById(`badge-${serviceKey}`);
    const toggleBtn = document.getElementById(`btn-${serviceKey}-toggle`);
    const openBtn = document.getElementById(`btn-${serviceKey}-open`);

    if (!cardElement || !badgeElement || !toggleBtn) return;

    // Reset card modifier classes
    cardElement.classList.remove('running', 'transitioning');
    badgeElement.className = 'status-badge ' + state;

    const badgeText = badgeElement.querySelector('.status-text');

    if (state === 'running') {
        cardElement.classList.add('running');
        badgeText.textContent = 'Running';
        
        toggleBtn.className = 'btn-toggle action-stop';
        toggleBtn.disabled = false;
        toggleBtn.innerHTML = '<span>Stop</span>';

        if (openBtn) openBtn.classList.remove('disabled');

    } else if (state === 'starting') {
        cardElement.classList.add('transitioning');
        badgeText.textContent = 'Starting...';

        toggleBtn.className = 'btn-toggle';
        toggleBtn.disabled = true;
        toggleBtn.innerHTML = '<div class="spinner"></div><span>Starting...</span>';

        if (openBtn) openBtn.classList.add('disabled');

    } else if (state === 'stopping') {
        cardElement.classList.add('transitioning');
        badgeText.textContent = 'Stopping...';

        toggleBtn.className = 'btn-toggle';
        toggleBtn.disabled = true;
        toggleBtn.innerHTML = '<div class="spinner"></div><span>Stopping...</span>';

        if (openBtn) openBtn.classList.add('disabled');

    } else { // 'stopped'
        badgeText.textContent = 'Stopped';

        toggleBtn.className = 'btn-toggle action-start';
        toggleBtn.disabled = false;
        toggleBtn.innerHTML = '<span>Start</span>';

        if (openBtn) openBtn.classList.add('disabled');
    }
}

// TIP: Triggers service state transitions (Start/Stop) with automatic UI locking
async function toggleService(serviceKey) {
    const currentState = serviceStates[serviceKey];
    
    // Prevent interaction during active state transition
    if (currentState === 'starting' || currentState === 'stopping') {
        return;
    }

    const isStarting = (currentState === 'stopped');
    const endpoint = `/api/${serviceKey}/${isStarting ? 'start' : 'stop'}`;
    const targetState = isStarting ? 'starting' : 'stopping';

    // Lock UI state immediately
    serviceStates[serviceKey] = targetState;
    renderServiceUI(serviceKey);

    try {
        const response = await fetch(endpoint, { method: 'POST' });
        const data = await response.json();

        if (data.success) {
            serviceStates[serviceKey] = isStarting ? 'running' : 'stopped';
            showToast(`${capitalize(serviceKey)} ${isStarting ? 'started successfully' : 'stopped'}`, 'success');
        } else {
            // Revert state on failure
            serviceStates[serviceKey] = isStarting ? 'stopped' : 'running';
            showToast(data.message || `Failed to ${isStarting ? 'start' : 'stop'} ${serviceKey}`, 'error');
        }
    } catch (err) {
        serviceStates[serviceKey] = isStarting ? 'stopped' : 'running';
        showToast(`Connection error while controlling ${serviceKey}`, 'error');
    } finally {
        renderServiceUI(serviceKey);
    }
}

function capitalize(str) {
    if (str === 'pma') return 'phpMyAdmin';
    if (str === 'mariadb') return 'MariaDB';
    return str.charAt(0).toUpperCase() + str.slice(1);
}

// TIP: Background status monitoring - only updates if service is not in transition
async function fetchServicesStatus() {
    try {
        const res = await fetch('/api/services/status', { method: 'POST' });
        const data = await res.json();
        if (data.success && data.data) {
            const status = data.data;

            ['mariadb', 'mailpit', 'pma'].forEach(key => {
                // Ignore status updates if user just initiated a transition
                if (serviceStates[key] !== 'starting' && serviceStates[key] !== 'stopping') {
                    const isRunning = Boolean(status[key]);
                    const newCalculatedState = isRunning ? 'running' : 'stopped';
                    
                    if (serviceStates[key] !== newCalculatedState) {
                        serviceStates[key] = newCalculatedState;
                        renderServiceUI(key);
                    }
                }
            });
        }
    } catch (err) {
        // Silent catch for background polling
    }
}

// TIP: Fetches infrastructure tool versions for environment status pills
async function fetchVersions() {
    try {
        const response = await fetch('/api/versions', { method: 'POST' });
        const data = await response.json();
        if (data.success && data.data) {
            const v = data.data;
            updateEnvPill('php', v.PHP);
            updateEnvPill('node', v.Node);
            updateEnvPill('go', v.Go);
            updateEnvPill('composer', v.Composer);
        }
    } catch (err) {
        // Silent catch
    }
}

function updateEnvPill(id, versionText) {
    const dot = document.getElementById(`dot-${id}`);
    const pill = document.getElementById(`pill-${id}`);
    if (!dot || !pill) return;

    const isError = !versionText || versionText.toLowerCase().includes('not found') || versionText.toLowerCase().includes('unknown');
    
    if (isError) {
        dot.className = 'env-dot inactive';
        pill.setAttribute('data-tooltip', `${id.toUpperCase()}: Not Installed`);
    } else {
        dot.className = 'env-dot active';
        pill.setAttribute('data-tooltip', versionText);
    }
}

// TIP: Generic API helper for header quick action buttons
async function executeQuickTool(endpoint, successMessage, btnElement) {
    if (btnElement) btnElement.disabled = true;
    try {
        const res = await fetch(endpoint, { method: 'POST' });
        const data = await res.json();
        if (data.success) {
            showToast(data.message || successMessage, 'success');
        } else {
            showToast(data.message || 'Tool execution failed', 'error');
        }
    } catch (err) {
        showToast('Server communication error', 'error');
    } finally {
        if (btnElement) btnElement.disabled = false;
    }
}

// Initialize Application Listeners
window.addEventListener('load', () => {
    // Bind service toggle buttons
    document.getElementById('btn-mariadb-toggle')?.addEventListener('click', () => toggleService('mariadb'));
    document.getElementById('btn-pma-toggle')?.addEventListener('click', () => toggleService('pma'));
    document.getElementById('btn-mailpit-toggle')?.addEventListener('click', () => toggleService('mailpit'));

    // Bind header quick tools
    document.getElementById('btn-reinject')?.addEventListener('click', function() {
        executeQuickTool('/api/path/fix', 'PATH environment variables updated', this);
        setTimeout(fetchVersions, 1200);
    });

    document.getElementById('btn-phpini')?.addEventListener('click', function() {
        executeQuickTool('/api/phpini/view', 'Opened php.ini configuration', this);
    });

    document.getElementById('btn-winnat')?.addEventListener('click', function() {
        executeQuickTool('/api/winnat/reset', 'WINNAT reset initiated', this);
    });

    // Initial data fetch
    fetchVersions();
    fetchServicesStatus();
});

// Periodic heartbeat & status polling (3 seconds)
setInterval(() => {
    fetch('/api/ping', { method: 'POST' }).catch(() => {});
    fetchServicesStatus();
}, 3000);

// Notify backend when window is closed
window.addEventListener('unload', () => {
    navigator.sendBeacon('/api/exit');
});
