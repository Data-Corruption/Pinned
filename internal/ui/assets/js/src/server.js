// Server Actions
// Backup modal, stop, restart, and polling functionality

import { releaseCameraConnection } from './camera.js';
import { blockClicks, unblockClicks, showError } from './ui.js';

/** Delay after POST /settings/restart succeeds before the first status check */
const RESTART_POLL_START_DELAY_MS = 500;
/** Interval between restart-status polls while waiting for the new process */
const RESTART_POLL_INTERVAL_MS = 1000;
const RESTART_POLL_FETCH_TIMEOUT_MS = 8000;

function fetchRestartStatus() {
    const c = new AbortController();
    const t = setTimeout(() => c.abort(), RESTART_POLL_FETCH_TIMEOUT_MS);
    return fetch('/settings/restart-status?t=' + Date.now(), {
        cache: 'no-store',
        signal: c.signal,
    }).finally(() => clearTimeout(t));
}

/** Stop the server */
export function stopServer() {
    blockClicks();
    fetch('/settings/stop', { method: 'POST' })
        .then(response => {
            if (response.ok) {
                // Replace title and body, keeping stylesheets loaded
                document.title = 'Server Stopped';
                document.body.className = 'bg-base-100 min-h-screen flex items-center justify-center';
                document.body.innerHTML = `
                    <div class="text-center">
                        <h1 class="text-2xl font-bold mb-2">Server Stopped</h1>
                        <p class="text-base-content/70">You can close this tab.</p>
                    </div>
                `;
            } else {
                throw new Error('Failed to stop server');
            }
        })
        .catch(err => {
            unblockClicks();
            showError('Error: ' + err.message);
        });
}

/** Restart the server with options from the restart modal */
export function restartServer() {
    const updateRequested = document.getElementById('restart-update').checked;

    // Close the modal
    document.getElementById('restart-modal').close();

    releaseCameraConnection();
    blockClicks();
    fetch('/settings/restart', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ update: updateRequested })
    })
        .then(response => {
            if (response.ok || response.status === 202) {
                // Server is restarting, poll for it to come back
                setTimeout(() => pollForRestart(updateRequested), RESTART_POLL_START_DELAY_MS);
            } else {
                throw new Error('Failed to restart server');
            }
        })
        .catch(err => {
            unblockClicks();
            showError('Error: ' + err.message);
        });
}

/** Poll for server restart completion */
export function pollForRestart(updateRequested = false) {
    const startTime = Date.now();
    const timeout = 300000; // 5 minutes

    const check = () => {
        if (Date.now() - startTime > timeout) {
            unblockClicks();
            showError('Restart timed out. Please check logs or try again.');
            return;
        }

        console.log('Polling for restart...', { updateRequested, time: Date.now() - startTime });
        fetchRestartStatus()
            .then((res) => {
                if (!res.ok) {
                    throw new Error('HTTP ' + res.status);
                }
                return res.json();
            })
            .then((data) => {
                console.log('Poll response:', data);
                if (data.restarted) {
                    if (updateRequested && !data.updated) {
                        console.warn('Restart detected but not updated.', data);
                        unblockClicks();
                        showError('Restart completed, but the update did not apply. You may already be on the latest version, or the update failed.');
                    } else {
                        console.log('Restart success (updated=' + data.updated + '), reloading...');
                        window.location.reload();
                    }
                } else {
                    setTimeout(check, RESTART_POLL_INTERVAL_MS);
                }
            })
            .catch((err) => {
                if (err.name === 'AbortError') {
                    console.warn('Restart status poll timed out or aborted, retrying...');
                } else {
                    console.error('Poll network error (expected if restarting):', err);
                }
                setTimeout(check, RESTART_POLL_INTERVAL_MS);
            });
    };

    check();
}
