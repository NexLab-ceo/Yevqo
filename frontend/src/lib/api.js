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


// === Workspace ===
export async function listProjects() { return request('GET', '/api/projects'); }
export async function createProject(name, description = '') { return request('POST', '/api/projects', { name, description }); }
export async function getProject(id) { return request('GET', `/api/projects/${id}`); }
export async function updateProject(id, name, description = '') { return request('PATCH', `/api/projects/${id}`, { name, description }); }
export async function deleteProject(id) { return request('DELETE', `/api/projects/${id}`); }
export async function attachProjectDevice(projectId, deviceId) { return request('POST', `/api/projects/${projectId}/devices`, { device_id: deviceId }); }
export async function attachProjectSite(projectId, deviceId, siteId) { return request('POST', `/api/projects/${projectId}/sites`, { device_id: deviceId, site_id: siteId }); }

// === Database inventory ===
export async function listDatabaseServers() { return request('GET', '/api/databases/servers'); }
export async function createDatabaseServer(input) { return request('POST', '/api/databases/servers', input); }
export async function getDatabaseServer(id) { return request('GET', `/api/databases/servers/${id}`); }
export async function deleteDatabaseServer(id) { return request('DELETE', `/api/databases/servers/${id}`); }
export async function listDatabases() { return request('GET', '/api/databases'); }

// === Operations ===
export async function listLogs(limit = 100) { return request('GET', `/api/logs?limit=${limit}`); }
export async function listNotifications() { return request('GET', '/api/notifications'); }
export async function markNotificationRead(id) { return request('POST', '/api/notifications', { id }); }
export async function getSettings() { return request('GET', '/api/settings'); }
export async function saveSettings(settings) { return request('PUT', '/api/settings', settings); }
export async function getReports() { return request('GET', '/api/reports'); }
export async function listBackups() { return request('GET', '/api/backups'); }

export async function deleteSite(deviceId, siteId) { return request('DELETE', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}`); }

export async function listDomains() { return request('GET', '/api/domains'); }
export async function updateSiteDomain(deviceId, siteId, domain) { return request('PATCH', '/api/sites/' + deviceId + '/' + encodeURIComponent(siteId), { domain }); }
export async function listDeployments() { return request('GET', '/api/deployments'); }
export async function getMonitoring() { return request('GET', '/api/monitoring'); }

export async function refreshDeviceMetrics(deviceId) { return request('POST', `/api/devices/${deviceId}/metrics`); }

export async function createSiteBackup(deviceId, siteId) { return request('POST', '/api/backups', { device_id: deviceId, site_id: siteId }); }

export async function installDatabaseServer(id) { return request('POST', `/api/databases/servers/${id}/install`); }
export async function startDatabaseServer(id) { return request('POST', `/api/databases/servers/${id}/start`, {}); }
export async function stopDatabaseServer(id) { return request('POST', `/api/databases/servers/${id}/stop`, {}); }
export async function listDatabaseServerRoles(id) { return request('GET', `/api/databases/servers/${id}/roles`); }
export async function mutateDatabaseServerRole(id, action, name, password = '') { return request('POST', `/api/databases/servers/${id}/roles`, { action, name, password }); }
export async function mutateDatabasePrivilege(serverId, action, database, role, privilege, scope = 'database', schema = '') { return request('POST', `/api/databases/servers/${serverId}/privileges`, { action, database, role, privilege, scope, schema }); }

// === Database provisioning and site connections ===
export async function createDatabase(input) { return request('POST', '/api/databases', input); }
export async function listSiteDatabaseConnections(deviceId, siteId) { return request('GET', `/api/database-connections?device_id=${deviceId}&site_id=${encodeURIComponent(siteId)}`); }
export async function removeSiteDatabaseConnection(id) { return request('DELETE', '/api/database-connections', { id }); }

export async function getDatabase(id) { return request('GET', `/api/databases/${id}`); }
export async function getDatabaseStats(id) { return request('GET', `/api/databases/${id}/stats`); }

export async function createDatabaseBackup(databaseId) { return request('POST', '/api/backups', { database_id: databaseId }); }

export async function listDatabaseConnections(databaseId) { return request('GET', `/api/database-connections?database_id=${databaseId}`); }
export async function attachDatabaseToSite(databaseId, deviceId, siteId) { return request('POST', '/api/database-connections', { database_id: databaseId, device_id: deviceId, site_id: siteId }); }

export async function restoreDatabaseBackup(backupId, input) { return request('POST', '/api/backups/restore', { backup_id: backupId, ...input }); }

export async function testDatabaseConnection(id) { return request('POST', `/api/databases/${id}/test`, {}); }
