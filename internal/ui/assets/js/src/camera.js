export function initCamera() {
    const container = document.getElementById('camera-container');
    const feed = document.getElementById('camera-feed');
    const status = document.getElementById('camera-status');
    const pauseOverlay = document.getElementById('camera-pause-overlay');
    
    if (!container || !feed || !status || !pauseOverlay) return;

    let isPaused = false;
    const streamUrl = '/api/camera/stream';

    window.toggleCamera = () => {
        isPaused = !isPaused;
        if (isPaused) {
            feed.src = '';
            feed.style.opacity = '0';
            pauseOverlay.classList.remove('hidden');
            pauseOverlay.classList.add('flex');
            status.style.opacity = '0';
        } else {
            feed.src = streamUrl;
            pauseOverlay.classList.add('hidden');
            pauseOverlay.classList.remove('flex');
            status.style.opacity = '1';
            status.innerHTML = `
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Loading...
            `;
        }
    };

    window.handleCameraLoad = () => {
        if (isPaused) return;
        feed.style.opacity = '1';
        status.style.opacity = '0';
    };

    window.handleCameraError = () => {
        if (isPaused) return;
        feed.style.opacity = '0';
        status.style.opacity = '1';
        status.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            Feed Offline
        `;
    };
}
