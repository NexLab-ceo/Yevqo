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

export async function listAppTemplates() {
    return request('GET', '/api/templates');
}

export async function changePassword(currentPassword, newPassword) {
    return request('POST', '/api/password', { current_password: currentPassword, new_password: newPassword });
}

// === Devices ===

export async function listDevices(projectId = '') {
    return request('GET', '/api/devices' + (projectId ? `?project_id=${encodeURIComponent(projectId)}` : ''));
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

// Same-origin authenticated WebSocket stream for a single site.
export function openSiteLogStream(deviceId, siteId, since = 0) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const query = new URLSearchParams({ device_id: String(deviceId), site_id: String(siteId), since: String(since) });
    return new WebSocket(`${protocol}//${window.location.host}/ws/logs?${query.toString()}`);
}

// === Sites ===

export async function listSites(projectId = '') {
    return request('GET', '/api/sites' + (projectId ? `?project_id=${encodeURIComponent(projectId)}` : ''));
}

export async function getSite(deviceId, siteId) {
    return request('GET', `/api/sites/${deviceId}/${siteId}`);
}

export async function runSiteCommand(deviceId, siteId, action, allowProjectCode = false, releaseCommit = '', gitCommit = '') {
    return request('POST', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/cmd`, { action, allow_project_code: allowProjectCode, release_commit: releaseCommit, git_commit: gitCommit });
}
export async function getSiteReleases(deviceId, siteId) {
    return request('GET', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/releases`);
}
export async function pruneSiteReleases(deviceId, siteId, keep = 10) {
    return request('POST', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/releases`, { keep });
}

export async function getSiteGit(deviceId, siteId) {
    return request('GET', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/git`);
}
export async function saveSiteGit(deviceId, siteId, settings) {
    return request('POST', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/git`, settings);
}
export async function listSiteEnvironment(deviceId, siteId) {
    return request('GET', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/env`);
}
export async function saveSiteEnvironment(deviceId, siteId, name, value) {
    return request('POST', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/env`, { name, value });
}
export async function deleteSiteEnvironment(deviceId, siteId, name) {
    return request('DELETE', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/env?name=${encodeURIComponent(name)}`);
}
export async function getSiteWebhook(deviceId, siteId) {
    return request('GET', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/webhook`);
}
export async function setSiteWebhook(deviceId, siteId, enabled, allowProjectCode = false) {
    return request('POST', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/webhook`, { enabled, allow_project_code: allowProjectCode });
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
export async function listWorkspaceMembers() { return request('GET', '/api/workspace/members'); }
export async function inviteWorkspaceMember(email, role) { return request('POST', '/api/workspace/members', { email, role }); }
export async function removeWorkspaceMember(id) { return request('DELETE', `/api/workspace/members/${id}`); }
export async function listWorkspaceInvitations() { return request('GET', '/api/workspace/invitations'); }
export async function acceptWorkspaceInvitation(id) { return request('POST', '/api/workspace/invitations', { id }); }
export async function listProjects() { return request('GET', '/api/projects'); }
export async function createProject(name, description = '') { return request('POST', '/api/projects', { name, description }); }
export async function getProject(id) { return request('GET', `/api/projects/${id}`); }
export async function listProjectEnvironments(id) { return request('GET', `/api/projects/${id}/environments`); }
export async function createProjectEnvironment(id, name, description = '') { return request('POST', `/api/projects/${id}/environments`, { name, description }); }
export async function updateProjectEnvironment(projectId, environmentId, name, description = '') { return request('PATCH', `/api/projects/${projectId}/environments/${environmentId}`, { name, description }); }
export async function deleteProjectEnvironment(projectId, environmentId) { return request('DELETE', `/api/projects/${projectId}/environments/${environmentId}`); }
export async function listEnvironmentSites(projectId, environmentId) { return request('GET', `/api/projects/${projectId}/environments/${environmentId}/sites`); }
export async function attachEnvironmentSite(projectId, environmentId, deviceId, siteId) { return request('POST', `/api/projects/${projectId}/environments/${environmentId}/sites`, { device_id: deviceId, site_id: siteId }); }
export async function detachEnvironmentSite(projectId, environmentId, deviceId, siteId) { return request('DELETE', `/api/projects/${projectId}/environments/${environmentId}/sites`, { device_id: deviceId, site_id: siteId }); }
export async function listEnvironmentDatabases(projectId, environmentId) { return request('GET', `/api/projects/${projectId}/environments/${environmentId}/databases`); }
export async function attachEnvironmentDatabase(projectId, environmentId, serverId) { return request('POST', `/api/projects/${projectId}/environments/${environmentId}/databases`, { server_id: serverId }); }
export async function detachEnvironmentDatabase(projectId, environmentId, serverId) { return request('DELETE', `/api/projects/${projectId}/environments/${environmentId}/databases`, { server_id: serverId }); }
export async function attachProjectDatabaseServer(projectId, serverId) { return request('POST', `/api/projects/${projectId}/databases`, { server_id: serverId }); }
export async function updateProject(id, name, description = '') { return request('PATCH', `/api/projects/${id}`, { name, description }); }
export async function deleteProject(id) { return request('DELETE', `/api/projects/${id}`); }
export async function attachProjectDevice(projectId, deviceId) { return request('POST', `/api/projects/${projectId}/devices`, { device_id: deviceId }); }
export async function attachProjectSite(projectId, deviceId, siteId) { return request('POST', `/api/projects/${projectId}/sites`, { device_id: deviceId, site_id: siteId }); }
export async function detachProjectDevice(projectId, deviceId) { return request('DELETE', `/api/projects/${projectId}/devices`, { device_id: deviceId }); }
export async function detachProjectSite(projectId, deviceId, siteId) { return request('DELETE', `/api/projects/${projectId}/sites`, { device_id: deviceId, site_id: siteId }); }
export async function detachProjectDatabaseServer(projectId, serverId) { return request('DELETE', `/api/projects/${projectId}/databases`, { server_id: serverId }); }

// === Database inventory ===
export async function listDatabaseServers(projectId = '', environmentId = '') { const q = new URLSearchParams(); if (projectId) q.set('project_id', projectId); if (environmentId) q.set('environment_id', environmentId); const query = q.toString(); return request('GET', '/api/databases/servers' + (query ? `?${query}` : '')); }
export async function createDatabaseServer(input) { return request('POST', '/api/databases/servers', input); }
export async function getDatabaseServer(id) { return request('GET', `/api/databases/servers/${id}`); }
export async function deleteDatabaseServer(id) { return request('DELETE', `/api/databases/servers/${id}`); }
export async function listDatabases(projectId = '', environmentId = '') { const q = new URLSearchParams(); if (projectId) q.set('project_id', projectId); if (environmentId) q.set('environment_id', environmentId); const query = q.toString(); return request('GET', '/api/databases' + (query ? `?${query}` : '')); }

// === Operations ===
export async function listLogs(limit = 100, projectId = '') { return request('GET', `/api/logs?limit=${limit}` + (projectId ? `&project_id=${encodeURIComponent(projectId)}` : '')); }
export async function listAuditLogs(limit = 100) { return request('GET', `/api/audit-logs?limit=${limit}`); }
export async function listNotifications() { return request('GET', '/api/notifications'); }
export async function markNotificationRead(id) { return request('POST', '/api/notifications', { id }); }
export async function getSettings() { return request('GET', '/api/settings'); }
export async function saveSettings(settings) { return request('PUT', '/api/settings', settings); }
export async function getReports(projectId = '') { return request('GET', '/api/reports' + (projectId ? `?project_id=${encodeURIComponent(projectId)}` : '')); }
export async function listBackups() { return request('GET', '/api/backups'); }
export async function createControlPlaneBackup() { return request('POST', '/admin/api/backups/control-plane', {}); }
export async function getDatabaseBackupChunk(id, offset, length = 262144) { return request('GET', `/api/backups/${id}/chunk?offset=${offset}&length=${length}`); }
export async function listBackupSchedules() { return request('GET', '/api/backup-schedules'); }
export async function createBackupSchedule(input) { return request('POST', '/api/backup-schedules', input); }
export async function deleteBackupSchedule(id) { return request('DELETE', `/api/backup-schedules/${id}`); }

export async function deleteSite(deviceId, siteId) { return request('DELETE', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}`); }

export async function listDomains(projectId = '') { return request('GET', '/api/domains' + (projectId ? `?project_id=${encodeURIComponent(projectId)}` : '')); }
export async function updateSiteDomain(deviceId, siteId, domain, tunnelToken = '') { return request('PATCH', '/api/sites/' + deviceId + '/' + encodeURIComponent(siteId), { domain, tunnel_token: tunnelToken }); }
export async function listDeployments(projectId = '') { return request('GET', '/api/deployments' + (projectId ? `?project_id=${encodeURIComponent(projectId)}` : '')); }
export async function getMonitoring(projectId = '') { return request('GET', '/api/monitoring' + (projectId ? `?project_id=${encodeURIComponent(projectId)}` : '')); }

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


// === External Git provider integrations ===
export async function listIntegrations() { return request('GET', '/api/integrations'); }
export async function connectIntegration(provider, token) { return request('POST', '/api/integrations', { provider, token }); }
export async function disconnectIntegration(id) { return request('DELETE', `/api/integrations/${id}`); }
export async function listIntegrationRepositories(id) { return request('GET', `/api/integrations/${id}/repositories`); }

export async function installCloudflared(deviceId, siteId) { return request('POST', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/install-cloudflared`, {}); }


// === Cloudflare DNS automation ===
export async function getCloudflareConfig() { return request('GET', '/api/cloudflare'); }
export async function saveCloudflareConfig(token, zoneId) { return request('POST', '/api/cloudflare', { token, zone_id: zoneId }); }
export async function disconnectCloudflare() { return request('DELETE', '/api/cloudflare'); }
export async function listCloudflareDNS() { return request('GET', '/api/cloudflare/dns'); }
export async function createCloudflareTunnelDNS(hostname, tunnelId) { return request('POST', '/api/cloudflare/dns', { hostname, tunnel_id: tunnelId }); }
export async function deleteCloudflareDNS(recordId) { return request('DELETE', '/api/cloudflare/dns', { record_id: recordId }); }

export async function restoreSiteBackup(backupId) { return request('POST', '/api/backups/restore-site', { backup_id: backupId }); }

export async function transferBackup(backupId, target) { return request('POST', `/api/backups/${backupId}/transfer`, target); }

export async function getSiteMetrics(deviceId, siteId) { return request('GET', `/api/sites/${deviceId}/${encodeURIComponent(siteId)}/metrics`); }

export async function listScheduledTasks(deviceId, siteId) { return request('GET', `/api/scheduled-tasks?device_id=${deviceId}&site_id=${encodeURIComponent(siteId)}`); }
export async function createScheduledTask(input) { return request('POST', '/api/scheduled-tasks', input); }
export async function setScheduledTaskEnabled(id, enabled) { return request('PATCH', `/api/scheduled-tasks/${id}`, { enabled }); }
export async function deleteScheduledTask(id) { return request('DELETE', `/api/scheduled-tasks/${id}`); }

export async function listSitePreviews(deviceId, siteId) { return request('GET', '/api/sites/' + deviceId + '/' + encodeURIComponent(siteId) + '/previews'); }
export async function createSitePreview(deviceId, siteId, input) { return request('POST', '/api/sites/' + deviceId + '/' + encodeURIComponent(siteId) + '/previews', input); }
export async function deleteSitePreview(deviceId, siteId, key) { return request('DELETE', '/api/sites/' + deviceId + '/' + encodeURIComponent(siteId) + '/previews/' + encodeURIComponent(key)); }
