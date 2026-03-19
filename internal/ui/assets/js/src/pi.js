/** @typedef {{ usable: boolean, label: string, color: string, highColor: string }} PinType */
/** @typedef {{ type: PinType, mesh: string, gpioID?: number }} PinEntry */

/** @typedef {'input'|'output'} PinDirection */
/** @typedef {'low'|'high'} PinLevel */
/** @typedef {'none'|'up'|'down'} PullResistor */

/**
 * Pin settings that can be edited by the UI (and later via websocket patch messages).
 * Note: `state` represents the logical level currently observed/desired for the pin.
 */
/** @typedef {{ direction: PinDirection, state: PinLevel, pull: PullResistor }} PinSettings */

/** A single pin model used to render the grid and settings panel. */
/**
 * @typedef {{
 *   headerPin: number,
 *   gpioID?: number,
 *   mesh: string,
 *   type: PinType,
 *   settings: PinSettings,
 *   // Convenience for UI brightness. Can be derived from settings.state, but is kept
 *   // as its own field to make websocket wiring easier later.
 *   valueHigh: boolean
 * }} PinModel
 */

/** Full UI-friendly snapshot of all pins for rendering. */
/** @typedef {{ pins: Record<number, PinModel>, selectedPin: number|null }} PinsSnapshot */

/** Patch shape for editing a single pin's settings over websocket. */
/** @typedef {{ direction?: PinDirection, state?: PinLevel, pull?: PullResistor }} PinSettingsPatch */

/**
 * @typedef {{ type: 'pins_snapshot', snapshot: PinsSnapshot }} PinsSnapshotMessage
 * @typedef {{ type: 'pin_update', pin: number, patch: PinSettingsPatch, valueHigh?: boolean }} PinUpdateMessage
 */

export const PIN_TYPES = {
    GROUND:    { usable: false, label: "Ground", color: "bg-[#202020]", highColor: "bg-[#202020]" },
    V3_3:      { usable: false, label: "3v3 Power", color: "bg-[#755700]", highColor: "bg-[#755700]" },
    V5:        { usable: false, label: "5v Power", color: "bg-[#751614]", highColor: "bg-[#751614]" },
    HAT:       { usable: false, label: "HAT", color: "bg-[#104266]", highColor: "bg-[#104266]" },
    GPIO:      { usable: true,  label: "GPIO", color: "bg-[#7c8f00]", highColor: "bg-[#DCFF00]" },
    GPIO_I2C:  { usable: true,  label: "GPIO (I2C)", color: "bg-[#1f70a9]", highColor: "bg-[#30ABFF]" },
    GPIO_SPI:  { usable: true,  label: "GPIO (SPI)", color: "bg-[#a22461]", highColor: "bg-[#FF3B9A]" },
    GPIO_UART: { usable: true,  label: "GPIO (UART)", color: "bg-[#32367c]", highColor: "bg-[#6970FF]" },
    GPIO_PCM:  { usable: true,  label: "GPIO (PCM)", color: "bg-[#248881]", highColor: "bg-[#42FFF2]" },
};

// Map of Pi Header Pins (1-40)
/** @type {Record<number, PinEntry>} */
export const HEADER_MAP = {
    1:  { type: PIN_TYPES.V3_3, mesh: "Mesh.001" },
    2:  { type: PIN_TYPES.V5, mesh: "Mesh.002" },
    3:  { type: PIN_TYPES.GPIO_I2C, mesh: "Mesh.747", gpioID: 2 },
    4:  { type: PIN_TYPES.V5, mesh: "Mesh.766" },
    5:  { type: PIN_TYPES.GPIO_I2C, mesh: "Mesh.748", gpioID: 3 },
    6:  { type: PIN_TYPES.GROUND, mesh: "Mesh.767" },
    7:  { type: PIN_TYPES.GPIO, mesh: "Mesh.749", gpioID: 4 },
    8:  { type: PIN_TYPES.GPIO_UART, mesh: "Mesh.768", gpioID: 14 },
    9:  { type: PIN_TYPES.GROUND, mesh: "Mesh.750" },
    10: { type: PIN_TYPES.GPIO_UART, mesh: "Mesh.769", gpioID: 15 },
    11: { type: PIN_TYPES.GPIO, mesh: "Mesh.751", gpioID: 17 },
    12: { type: PIN_TYPES.GPIO_PCM, mesh: "Mesh.770", gpioID: 18 },
    13: { type: PIN_TYPES.GPIO, mesh: "Mesh.752", gpioID: 27 },
    14: { type: PIN_TYPES.GROUND, mesh: "Mesh.771" },
    15: { type: PIN_TYPES.GPIO, mesh: "Mesh.753", gpioID: 22 },
    16: { type: PIN_TYPES.GPIO, mesh: "Mesh.772", gpioID: 23 },
    17: { type: PIN_TYPES.V3_3, mesh: "Mesh.754" },
    18: { type: PIN_TYPES.GPIO, mesh: "Mesh.773", gpioID: 24 },
    19: { type: PIN_TYPES.GPIO_SPI, mesh: "Mesh.755", gpioID: 10 },
    20: { type: PIN_TYPES.GROUND, mesh: "Mesh.774" },
    21: { type: PIN_TYPES.GPIO_SPI, mesh: "Mesh.756", gpioID: 9 },
    22: { type: PIN_TYPES.GPIO, mesh: "Mesh.775", gpioID: 25 },
    23: { type: PIN_TYPES.GPIO_SPI, mesh: "Mesh.757", gpioID: 11 },
    24: { type: PIN_TYPES.GPIO_SPI, mesh: "Mesh.776", gpioID: 8 },
    25: { type: PIN_TYPES.GROUND, mesh: "Mesh.758" },
    26: { type: PIN_TYPES.GPIO_SPI, mesh: "Mesh.777", gpioID: 7 },
    27: { type: PIN_TYPES.HAT, mesh: "Mesh.759", gpioID: 0 },
    28: { type: PIN_TYPES.HAT, mesh: "Mesh.778", gpioID: 1 },
    29: { type: PIN_TYPES.GPIO, mesh: "Mesh.760", gpioID: 5 },
    30: { type: PIN_TYPES.GROUND, mesh: "Mesh.779" },
    31: { type: PIN_TYPES.GPIO, mesh: "Mesh.761", gpioID: 6 },
    32: { type: PIN_TYPES.GPIO, mesh: "Mesh.780", gpioID: 12 },
    33: { type: PIN_TYPES.GPIO, mesh: "Mesh.762", gpioID: 13 },
    34: { type: PIN_TYPES.GROUND, mesh: "Mesh.781" },
    35: { type: PIN_TYPES.GPIO_PCM, mesh: "Mesh.763", gpioID: 19 },
    36: { type: PIN_TYPES.GPIO, mesh: "Mesh.782", gpioID: 16 },
    37: { type: PIN_TYPES.GPIO, mesh: "Mesh.764", gpioID: 26 },
    38: { type: PIN_TYPES.GPIO_PCM, mesh: "Mesh.783", gpioID: 20 },
    39: { type: PIN_TYPES.GROUND, mesh: "Mesh.765" },
    40: { type: PIN_TYPES.GPIO_PCM, mesh: "Mesh.784", gpioID: 21 },
};

export const DEFAULT_PIN_SETTINGS = /** @type {PinSettings} */ ({
    direction: 'input',
    state: 'low',
    pull: 'none',
});

export function settingsStateToValueHigh(state) {
    return state === 'high';
}

/**
 * Create an initial snapshot for rendering before websocket wiring.
 * This keeps the state shape deterministic and websocket-friendly.
 *
 * @returns {PinsSnapshot}
 */
export function createInitialPinsSnapshot() {
    /** @type {Record<number, PinModel>} */
    const pins = {};

    let firstUsablePin = null;

    for (const [headerPinStr, entry] of Object.entries(HEADER_MAP)) {
        const headerPin = Number(headerPinStr);
        const type = entry.type;

        const settings = { ...DEFAULT_PIN_SETTINGS };
        const valueHigh = settingsStateToValueHigh(settings.state);

        pins[headerPin] = {
            headerPin,
            gpioID: entry.gpioID,
            mesh: entry.mesh,
            type,
            settings,
            valueHigh,
        };

        if (firstUsablePin === null && type.usable) {
            firstUsablePin = headerPin;
        }
    }

    return { pins, selectedPin: firstUsablePin };
}

/**
 * Apply a settings patch to a pin model (and keep `valueHigh` consistent by default).
 *
 * @param {PinModel} pin
 * @param {PinSettingsPatch} patch
 * @param {boolean|undefined} [valueHighOverride]
 */
export function applyPinSettingsPatch(pin, patch, valueHighOverride = undefined) {
    if (patch.direction) pin.settings.direction = patch.direction;
    if (patch.state) pin.settings.state = patch.state;
    if (patch.pull) pin.settings.pull = patch.pull;

    if (valueHighOverride !== undefined) {
        pin.valueHigh = valueHighOverride;
    } else {
        pin.valueHigh = settingsStateToValueHigh(pin.settings.state);
    }
}
