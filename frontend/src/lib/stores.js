/**
 * Yevqo — глобальное состояние.
 * Svelte stores для данных, которые нужны на нескольких страницах.
 */

import { writable, derived } from 'svelte/store';

// Текущий пользователь (null = не залогинен).
export const user = writable(null);

// Persisted UI mode: simple for new accounts, advanced on explicit choice.
export const uiMode = writable('simple');

// Список устройств пользователя.
export const devices = writable([]);

// Список сайтов.
export const sites = writable([]);

// Уведомления (тосты).
export const notifications = writable([]);

// Флаг загрузки (глобальный).
export const loading = writable(false);

// Производное: залогинен ли пользователь.
export const isAuthenticated = derived(user, ($user) => $user !== null);

// Производное: количество онлайн-устройств.
export const onlineDevicesCount = derived(devices, ($devices) =>
    $devices.filter((d) => d.status === 'online').length
);

/**
 * Добавить уведомление.
 * @param {string} message
 * @param {'info'|'success'|'error'} type
 */
export function notify(message, type = 'info') {
    const id = Date.now() + Math.random();
    notifications.update((list) => [...list, { id, message, type }]);

    setTimeout(() => {
        notifications.update((list) => list.filter((n) => n.id !== id));
    }, 4000);
}

// Persisted appearance preference: dark by default.
export const theme = writable('dark');

// Workspace display preferences shared by the shell and settings screen.
export const workspaceName = writable('Personal workspace');
export const compactMode = writable(false);
