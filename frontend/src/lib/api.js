/**
 * VELTRIX Dock — API client.
 * Все запросы к серверу идут через этот файл.
 * Ошибки возвращаются в едином формате: { ok: false, error: string }.
 */

const BASE = '';

/**
 * Базовый запрос к API.
 * @param {string} method - GET, POST, DELETE
 * @param {string} path - путь (например, "/api/devices")
 * @param {object} body - тело запроса (для POST)
 * @returns {Promise<{ok: boolean, data?: any, error?: string}>}
 */
async function request(method, path, body = null) {
    const opts = {
        method,
        headers: {},
        credentials: 'same-origin',
    };

    if (body !== null) {
        opts.headers['Content-Type'] = 'application/json';
        opts.body = JSON.stringify(body);
    }

    try {
        const res = await fetch(BASE + path, opts);

        // Пытаемся распарсить JSON.
        let data = null;
        const text = await res.text();
        if (text) {
            try {
                data = JSON.parse(text);
            } catch {
                data = { error: text };
            }
        }

        if (!res.ok) {
            return {
                ok: false,
                error: data?.error || `HTTP ${res.status}`,
                status: res.status,
            };
        }

        return { ok: true, data };
    } catch (e) {
        return { ok: false, error: e.message || 'network error' };
    }
}

// === Auth ===

export async function register(username, email, password, referralCode = '') {
    return request('POST', '/api/register', {
        username,
        email,
        password,
        referral_code: referralCode,
    });
}

export async function login(username, password) {
    return request('POST', '/api/login', { username, password });
}

export async function logout() {
    return request('POST', '/api/logout');
}

export async function me() {
    return request('GET', '/api/me');
}

// === Devices ===

export async function listDevices() {
    return request('GET', '/api/devices');
}

export async function getDevice(id) {
    return request('GET', `/api/devices/${id}`);
}

export async function deleteDevice(id) {
    return request('DELETE', `/api/devices/${id}`);
}

export async function sendCommand(deviceId, action, payload = {}) {
    return request('POST', `/api/devices/${deviceId}/cmd`, { action, payload });
}

// === Sites ===

export async function listSites() {
    return request('GET', '/api/sites');
}

export async function getSite(deviceId, siteId) {
    return request('GET', `/api/sites/${deviceId}/${siteId}`);
}

// === Admin ===

export async function adminListUsers(page = 1) {
    return request('GET', `/admin/api/users?page=${page}`);
}

export async function adminGetUser(id) {
    return request('GET', `/admin/api/users/${id}`);
}

export async function adminBlockUser(id) {
    return request('POST', `/admin/api/users/${id}/block`);
}

export async function adminUnblockUser(id) {
    return request('POST', `/admin/api/users/${id}/unblock`);
}

export async function adminStats() {
    return request('GET', '/admin/api/stats');
}
