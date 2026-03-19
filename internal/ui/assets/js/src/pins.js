import { applyPinSettingsPatch, createInitialPinsSnapshot } from './pi.js';

/** @type {ReturnType<typeof createInitialPinsSnapshot>} */
let snapshot = createInitialPinsSnapshot();

/**
 * @type {Map<number, HTMLButtonElement>}
 */
const pinButtons = new Map();

let gridEl;
let selectedPinNumberEl;
let pinViewModeEl;
let pinPrevBtn;
let pinNextBtn;
let dirSelect;
let stateSelect;
let pullSelect;

function setSelectedPin(pinNumber) {
    if (pinNumber === null) {
        // Default to the first pure general pin.
        snapshot.selectedPin = 7; // GPIO 4
    } else {
        const pin = snapshot.pins[pinNumber];
        if (!pin || !pin.type.usable) return; // Unusable pins should behave like a dead cell.
        snapshot.selectedPin = pinNumber;
    }
    renderSelectedPanel();
    renderGridSelection();
}

function getUsablePins() {
    return Object.keys(snapshot.pins)
        .map(Number)
        .filter((pinNumber) => snapshot.pins[pinNumber]?.type?.usable)
        .sort((a, b) => a - b);
}

function applyPinBgClasses(btn, pin) {
    // `pi.js` provides the Tailwind color classes as source of truth.
    const baseBgClass = pin.type.color;
    const highBgClass = pin.type.highColor ?? pin.type.color;

    // Usable pins: apply base vs high color class.
    btn.classList.remove(baseBgClass, highBgClass);
    btn.classList.add(pin.valueHigh ? highBgClass : baseBgClass);

    // Ensure text stays readable at a glance.
    btn.classList.remove('text-white', 'text-black');
    btn.classList.add(pin.valueHigh ? 'text-black' : 'text-white');
}

function renderGridCell(headerPin) {
    const pin = snapshot.pins[headerPin];
    const btn = pinButtons.get(headerPin);
    if (!pin || !btn) return;

    const labelEl = btn.querySelector('[data-pin-id="true"]');

    const usable = pin.type.usable;

    btn.disabled = !usable;
    btn.classList.toggle('cursor-not-allowed', !usable);
    btn.classList.remove('opacity-70');

    if (!usable) {
        // Darker "tag" look + non-interactive.
        btn.classList.remove('cursor-pointer');
        btn.classList.remove('hover:ring-1', 'hover:ring-primary/20');
        applyPinBgClasses(btn, pin);
    } else {
        btn.classList.add('cursor-pointer', 'hover:ring-1', 'hover:ring-primary/20');
        applyPinBgClasses(btn, pin);
    }
    if (labelEl) {
        const id = pinViewModeEl?.checked ? pin.headerPin : (pin.gpioID ?? '');
        labelEl.textContent = id === '' ? '' : String(id);
    }
}

function renderGridSelection() {
    for (const [pinNumber, btn] of pinButtons.entries()) {
        const isSelected = snapshot.selectedPin === pinNumber;
        if (isSelected) {
            btn.classList.add('ring-2', 'ring-primary/80');
        } else {
            btn.classList.remove('ring-2', 'ring-primary/80');
        }
    }
}

function renderSelectedPanel() {
    const pinNumber = snapshot.selectedPin;
    const pin = pinNumber !== null ? snapshot.pins[pinNumber] : null;

    if (!pin || !pin.type.usable) {
        selectedPinNumberEl.textContent = '-';
        dirSelect.disabled = true;
        stateSelect.disabled = true;
        pullSelect.disabled = true;
        return;
    }

    dirSelect.disabled = false;
    stateSelect.disabled = false;
    pullSelect.disabled = false;

    // Selected pin label always uses the gpioID.
    const id = pin.gpioID ?? '';
    if (id === '') {
        selectedPinNumberEl.textContent = pin.type.label;
    } else {
        selectedPinNumberEl.textContent = `${id} ${pin.type.label}`;
    }

    dirSelect.checked = pin.settings.direction === 'output';
    stateSelect.checked = pin.settings.state === 'high';

    if (pin.settings.direction === 'input') {
        stateSelect.disabled = true;
        pullSelect.disabled = false;
    } else {
        stateSelect.disabled = false;
        pullSelect.disabled = true;
    }

    if (pin.gpioID === 2 || pin.gpioID === 3) {
        pullSelect.value = 'up';
        pullSelect.disabled = true;
    } else {
        pullSelect.value = pin.settings.pull;
    }
}

let ws;

function initPinsWS() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/api/pins/ws`);

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        if (msg.type === 'pin_update') {
            applyPinUpdate(msg.pin, msg.patch, msg.valueHigh);
        } else if (msg.type === 'sync') {
            for (const [pinStr, data] of Object.entries(msg.pins)) {
                applyPinUpdate(Number(pinStr), data.settings, data.valueHigh);
            }
        }
    };

    ws.onclose = () => {
        setTimeout(initPinsWS, 2000); // auto reconnect
    };
}

function sendPinEdit(pinNumber, patch) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
            type: 'pin_update',
            pin: pinNumber,
            patch: patch
        }));
    }
}
function updateSelectedPinFromUI(patch) {
    const pinNumber = snapshot.selectedPin;
    if (pinNumber === null) return;

    const pin = snapshot.pins[pinNumber];
    if (!pin) return;
    if (!pin.type.usable) return;

    applyPinSettingsPatch(pin, patch);

    renderGridCell(pinNumber);
    renderSelectedPanel();
    sendPinEdit(pinNumber, patch);
}

function buildPinGrid() {
    if (!gridEl) return;
    gridEl.replaceChildren();
    pinButtons.clear();

    // Create buttons in header-pin order.
    for (const headerPin of Object.keys(snapshot.pins).map(Number).sort((a, b) => a - b)) {
        const pin = snapshot.pins[headerPin];

        const btn = document.createElement('button');
        btn.type = 'button';
        btn.dataset.pin = String(headerPin);
        btn.className = [
            'h-7 w-7 rounded-md border border-base-content/10',
            'flex items-center justify-center',
            'transition',
            'text-base-100',
            'hover:ring-1 hover:ring-primary/20',
        ].join(' ');

        btn.innerHTML = `
            <span data-pin-id="true" class="text-[10px] leading-none"></span>
        `;

        // Selection highlight is rendered separately, but set it early for perceived responsiveness.
        if (snapshot.selectedPin === headerPin) {
            btn.classList.add('ring-2', 'ring-primary/80');
        }

        pinButtons.set(headerPin, btn);
        renderGridCell(headerPin);
        gridEl.appendChild(btn);
    }
}

export function initPins() {
    gridEl = document.getElementById('pin-grid');
    selectedPinNumberEl = document.getElementById('selected-pin-number');
    pinViewModeEl = document.getElementById('pin-view-mode');
    pinPrevBtn = document.getElementById('pin-prev');
    pinNextBtn = document.getElementById('pin-next');
    dirSelect = document.getElementById('pin-direction');
    stateSelect = document.getElementById('pin-state');
    pullSelect = document.getElementById('pin-pull');

    if (!gridEl || !selectedPinNumberEl || !pinPrevBtn || !pinNextBtn || !dirSelect || !stateSelect || !pullSelect) {
        return;
    }

    // Default selection/presentation.
    setSelectedPin(null);

    buildPinGrid();
    renderSelectedPanel();

    // Pin selection.
    gridEl.addEventListener('click', (e) => {
        const btn = e.target instanceof Element ? e.target.closest('button[data-pin]') : null;
        if (!btn) return;

        const pinNumber = Number(btn.dataset.pin);
        if (Number.isNaN(pinNumber)) return;

        setSelectedPin(pinNumber);
    });

    // Settings controls.
    dirSelect.addEventListener('change', () => {
        updateSelectedPinFromUI({ direction: dirSelect.checked ? 'output' : 'input' });
    });
    stateSelect.addEventListener('change', () => {
        updateSelectedPinFromUI({ state: stateSelect.checked ? 'high' : 'low' });
    });
    pullSelect.addEventListener('change', () => {
        updateSelectedPinFromUI({ pull: /** @type {any} */ (pullSelect.value) });
    });

    // View mode: physical header pin vs GPIO IDs.
    // Uses the checkbox in the left card (DaisyUI swap input).
    if (pinViewModeEl) {
        pinViewModeEl.addEventListener('change', () => {
            for (const pinNumber of pinButtons.keys()) {
                renderGridCell(pinNumber);
            }
            renderSelectedPanel();
            renderGridSelection();
        });
    }

    // Prev/Next buttons for mobile pin selection.
    const selectRelative = (delta) => {
        const usablePins = getUsablePins();
        if (!usablePins.length) return;

        const current = snapshot.selectedPin;
        const currentIdx = usablePins.indexOf(current);
        const idx = currentIdx === -1 ? 0 : currentIdx;
        const nextIdx = (idx + delta + usablePins.length) % usablePins.length;
        setSelectedPin(usablePins[nextIdx]);
    };

    pinPrevBtn.addEventListener('click', () => selectRelative(-1));
    pinNextBtn.addEventListener('click', () => selectRelative(1));

    // Connect WebSocket
    initPinsWS();
}

/**
 * Optional websocket-friendly hooks.
 * Later, call these from your websocket `onmessage` handler.
 */
export function applyPinsSnapshot(nextSnapshot) {
    snapshot = nextSnapshot;
    buildPinGrid();
    renderSelectedPanel();
}

export function applyPinUpdate(pinNumber, patch, valueHigh) {
    const pin = snapshot.pins[pinNumber];
    if (!pin) return;
    applyPinSettingsPatch(pin, patch, valueHigh);
    renderGridCell(pinNumber);
    renderSelectedPanel();
}

