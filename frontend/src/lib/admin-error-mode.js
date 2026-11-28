import { writable } from 'svelte/store';

const STORAGE_KEY = 'yevqo-admin-error-mode';

export const errorDisplayMode = writable(true);
// Backward-compatible alias used by the existing Admin error panels.
export const adminErrorMode = errorDisplayMode;

let initialized = false;

export function initializeAdminErrorMode() {
    if (initialized) return;
    initialized = true;
    try {
        errorDisplayMode.set(window.localStorage.getItem(STORAGE_KEY) !== 'general');
    } catch {
        errorDisplayMode.set(true);
    }
}

export function setAdminErrorMode(enabled) {
    const detailed = Boolean(enabled);
    errorDisplayMode.set(detailed);
    try {
        window.localStorage.setItem(STORAGE_KEY, detailed ? 'detailed' : 'general');
    } catch {
        // Keep the in-memory preference for this SPA session if storage is unavailable.
    }
}
