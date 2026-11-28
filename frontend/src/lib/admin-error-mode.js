import { writable } from 'svelte/store';

const STORAGE_KEY = 'yevqo-admin-error-mode';

export const adminErrorMode = writable(true);

let initialized = false;

export function initializeAdminErrorMode() {
    if (initialized) return;
    initialized = true;
    try {
        adminErrorMode.set(window.localStorage.getItem(STORAGE_KEY) !== 'general');
    } catch {
        adminErrorMode.set(true);
    }
}

export function setAdminErrorMode(enabled) {
    const detailed = Boolean(enabled);
    adminErrorMode.set(detailed);
    try {
        window.localStorage.setItem(STORAGE_KEY, detailed ? 'detailed' : 'general');
    } catch {
        // Keep the in-memory preference for this SPA session if storage is unavailable.
    }
}
