<script>
    import { onMount, onDestroy, tick } from 'svelte';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { getDevice, getSite, getSiteMetrics, updateSiteDomain, listSitePreviews, createSitePreview, deleteSitePreview, getSiteGit, saveSiteGit, sendCommand, runSiteCommand, getSiteReleases, pruneSiteReleases, listLogs, listSiteEnvironment, saveSiteEnvironment, deleteSiteEnvironment, getSiteWebhook, setSiteWebhook, listScheduledTasks, createScheduledTask, setScheduledTaskEnabled, deleteScheduledTask, openSiteLogStream, getCloudflareConfig, saveCloudflareConfig, disconnectCloudflare, getCloudflareTunnelRoute, configureCloudflareTunnelRoute, removeCloudflareTunnelRoute } from '$lib/api.js';
    import { notify } from '$lib/stores.js';
    import { listIntegrations, listIntegrationRepositories } from '$lib/api.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';
    import StatusDot from '$lib/components/StatusDot.svelte';
    import Modal from '$lib/components/Modal.svelte';

    let deviceId = '';
    let siteId = '';
    let projectId = '';
    let environmentId = '';
    let device = null;
    let site = null;
    let online = false;
    let loading = true;
    let busyAction = '';
    let restartPolicy = 'on-failure';
    let savingRestartPolicy = false;
    let deploymentJob = null;
    let deploymentJobBusy = false;
    let showTerminal = false;
    let terminalLoading = false;
    let terminalText = '';
    let logSearch = '';
    let logLevel = 'all';
    let autoScroll = true;
    let terminalElement;
    let logSocket = null;
    let logConnected = false;
    let lastLogSeq = 0;
    let logStreamID = '';
    let agentLogOnline = false;
    let logStreamEnabled = false;
    let logReconnectTimer = null;
    let logStableTimer = null;
    let logReconnectAttempts = 0;
    let logStatus = 'disconnected';
    const logReconnectDelays = [1000, 2000, 5000, 10000, 20000, 30000];
    let activeTab = 'general';
    const siteTabs = ['general','git','settings','environment','domains','deployments','build','tasks','monitoring','logs','previews'];
    $: routeTab = $page.url.searchParams.get('tab') || 'general';
    $: if (siteTabs.includes(routeTab) && routeTab !== activeTab) {
        activeTab = routeTab;
        if (routeTab === 'tasks') refreshScheduledTasks();
        if (routeTab === 'deployments') loadReleases();
        if (routeTab === 'build') loadBuildHistory();
        if (routeTab === 'monitoring') loadProcessMetrics();
    }
    function selectTab(tab) {
        if (!siteTabs.includes(tab)) return;
        activeTab = tab;
        const url = new URL($page.url);
        if (tab === 'general') url.searchParams.delete('tab'); else url.searchParams.set('tab', tab);
        goto(url.pathname + url.search, { replaceState: true, keepfocus: true, noScroll: true });
        if (tab === 'tasks') refreshScheduledTasks();
        if (tab === 'deployments') loadReleases();
        if (tab === 'build') loadBuildHistory();
        if (tab === 'monitoring') loadProcessMetrics();
    }
    let domainDraft = '';
    let domainTunnelToken = '';
    let domainBusy = false;
    let cloudflareConfig = { connected: false };
    let cloudflareTokenDraft = '';
    let cloudflareZoneDraft = '';
    let cloudflareTunnelIdDraft = '';
    let cloudflareRoute = { configured: false };
    let cloudflareConfigBusy = false;
    let cloudflareRouteBusy = false;
    let releases = [];
    let previews = [];
    let previewsLoading = false;
    let previewCreating = false;
    let previewBranch = 'main';
    let deletingPreview = '';
    let releasesLoading = false;
    let pruningReleases = false;
    let releaseKeep = 10;
    let rollbackTarget = '';
    let git = { repository_url: '', branch: 'main', build_command: '', root_directory: '.', health_check_path: '', clone_timeout_seconds: 600, build_timeout_seconds: 1200, health_check_timeout_seconds: 15, start_command: '', has_token: false, commit: '', deployed_at: null };
    let gitForm = { repository_url: '', branch: 'main', build_command: '', root_directory: '.', health_check_path: '', clone_timeout_seconds: 600, build_timeout_seconds: 1200, health_check_timeout_seconds: 15, start_command: '', github_token: '' };
    let gitHubIntegrations = [];
    let gitHubRepositories = [];
    let selectedGitHubIntegration = '';
    let selectedGitHubRepository = '';
    let gitHubRepositoriesLoading = false;
    let gitHubRepositoriesError = '';
    let deployCommit = '';
    let envVariables = [];
    let newEnvName = '';
    let newEnvValue = '';
    let savingEnv = false;
    let envImportText = '';
    let importingEnv = false;
    let envSearch = '';
    let webhookEnabled = false;
    let webhookPath = '';
    let webhookSecret = '';
    let webhookBusy = false;
    let buildPlan = null;
    let analyzingBuild = false;
    let buildRunning = false;
    let buildResult = null;
    let buildHistory = [];
    let buildHistoryLoading = false;
    let processMetrics = null;
    let metricsPoller = null;
    let metricsBusy = false;
    let scheduledTasks = [];
    let taskName = '';
    let taskCommand = '';
    let taskInterval = 1440;
    let taskBusy = false;
    let taskOutputOpen = {};

    onDestroy(() => {
        if (metricsPoller) clearInterval(metricsPoller);
        logStreamEnabled = false;
        clearLogReconnectTimer();
        clearLogStableTimer();
        if (logSocket) {
            const socket = logSocket;
            logSocket = null;
            socket.close();
        }
    });

    onMount(async () => {
        deviceId = $page.params.device_id;
        siteId = $page.params.site_id;
        projectId = $page.url.searchParams.get('project_id') || '';
        environmentId = $page.url.searchParams.get('environment_id') || '';
        const requestedTab = $page.url.searchParams.get('tab') || 'general';
        activeTab = siteTabs.includes(requestedTab) ? requestedTab : 'general';
        await refresh();
        await loadGitHubConnections();
        if (activeTab === 'deployments') loadReleases();
        if (activeTab === 'build') loadBuildHistory();
        if (activeTab === 'monitoring') loadProcessMetrics();
        if (activeTab === 'tasks') refreshScheduledTasks();
        loading = false;
        metricsPoller = setInterval(() => {
            if (!online) return;
            if (activeTab === 'general' || activeTab === 'monitoring') loadProcessMetrics();
            if (activeTab === 'tasks') refreshScheduledTasks();
        }, 10000);
    });

    async function refresh() {
        const [deviceRes, siteRes, gitRes, variablesRes, webhookRes, tasksRes] = await Promise.all([
            getDevice(deviceId), getSite(deviceId, siteId), getSiteGit(deviceId, siteId),
            listSiteEnvironment(deviceId, siteId), getSiteWebhook(deviceId, siteId), listScheduledTasks(deviceId, siteId)
        ]);
        if (!deviceRes.ok || !siteRes.ok) {
            notify(deviceRes.error || siteRes.error || 'Не удалось загрузить сайт', 'error');
            return;
        }
        device = deviceRes.data.device;
        online = !!deviceRes.data.online;
        site = siteRes.data.site;
        domainDraft = site.domain || '';
        await loadCloudflareIntegration();
        if (gitRes.ok) {
            git = { ...git, ...gitRes.data };
            gitForm = {
                repository_url: git.repository_url || '', branch: git.branch || 'main',
                build_command: git.build_command || '', root_directory: git.root_directory || '.',
                health_check_path: git.health_check_path || '', clone_timeout_seconds: Number(git.clone_timeout_seconds || 600), build_timeout_seconds: Number(git.build_timeout_seconds || 1200), health_check_timeout_seconds: Number(git.health_check_timeout_seconds || 15), start_command: git.start_command || '', github_token: ''
            };
        }
        envVariables = variablesRes.ok ? (variablesRes.data.variables || []) : [];
        if (webhookRes.ok) { webhookEnabled = !!webhookRes.data.enabled; webhookPath = webhookRes.data.path || ''; }
        scheduledTasks = tasksRes.ok ? (tasksRes.data.tasks || []) : [];
        if (online) {
            const policyRes = await sendCommand(deviceId, 'get_site_restart_policy', { site_id: siteId });
            if (policyRes.ok && policyRes.data?.ok && policyRes.data.payload) {
                try { restartPolicy = JSON.parse(decodeLogs(policyRes.data.payload)).policy || 'on-failure'; } catch { /* keep safe default */ }
            }
        }
    }


    async function loadCloudflareIntegration() {
        const [configRes, routeRes] = await Promise.all([getCloudflareConfig(), getCloudflareTunnelRoute(deviceId, siteId)]);
        cloudflareConfig = configRes.ok ? configRes.data : { connected: false };
        cloudflareRoute = routeRes.ok ? routeRes.data : { configured: false };
        if (cloudflareRoute.configured) cloudflareTunnelIdDraft = cloudflareRoute.tunnel_id || '';
    }

    function cloudflareTunnelIDFromToken(token) {
        const value = String(token || '').trim();
        if (!value) return '';
        try {
            const normalized = value.replace(/-/g, '+').replace(/_/g, '/');
            const decoded = JSON.parse(atob(normalized + '='.repeat((4 - normalized.length % 4) % 4)));
            const id = String(decoded.t || decoded.TunnelID || '').trim();
            return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id) ? id : '';
        } catch {
            return '';
        }
    }

    async function connectCloudflare() {
        if (cloudflareConfigBusy || !cloudflareTokenDraft.trim() || !cloudflareZoneDraft.trim()) return;
        cloudflareConfigBusy = true;
        const res = await saveCloudflareConfig(cloudflareTokenDraft.trim(), cloudflareZoneDraft.trim());
        cloudflareConfigBusy = false;
        if (!res.ok) { notify(res.error || 'Не удалось подключить Cloudflare', 'error'); return; }
        cloudflareTokenDraft = '';
        notify('Cloudflare zone подключена', 'success');
        await loadCloudflareIntegration();
    }

    async function disconnectCloudflareAccount() {
        if (cloudflareConfigBusy || !confirm('Отключить Cloudflare API? Сначала удалите все управляемые Tunnel routes.')) return;
        cloudflareConfigBusy = true;
        const res = await disconnectCloudflare();
        cloudflareConfigBusy = false;
        if (!res.ok) { notify(res.error || 'Не удалось отключить Cloudflare', 'error'); return; }
        cloudflareConfig = { connected: false };
        notify('Cloudflare отключён', 'success');
    }

    async function configureCloudflareRoute() {
        if (cloudflareRouteBusy || !site?.domain || !cloudflareTunnelIdDraft.trim()) return;
        cloudflareRouteBusy = true;
        const res = await configureCloudflareTunnelRoute(deviceId, siteId, cloudflareTunnelIdDraft.trim());
        cloudflareRouteBusy = false;
        if (!res.ok) { notify(res.error || 'Не удалось настроить Cloudflare route', 'error'); return; }
        cloudflareRoute = res.data;
        if (res.data?.ingress_managed === false) notify('DNS CNAME настроен. Локальный ingress не изменён — проверь config.yml', 'warning');
        else notify('Cloudflare Tunnel route настроен', 'success');
    }

    async function removeCloudflareRoute() {
        if (cloudflareRouteBusy || !cloudflareRoute?.configured || !confirm('Удалить Cloudflare ingress route и управляемую DNS-запись?')) return;
        cloudflareRouteBusy = true;
        const res = await removeCloudflareTunnelRoute(deviceId, siteId);
        cloudflareRouteBusy = false;
        if (!res.ok) { notify(res.error || 'Не удалось удалить Cloudflare route', 'error'); return; }
        cloudflareRoute = { configured: false };
        notify(res.data?.message || 'Cloudflare route удалён', 'success');
    }

    async function loadPreviews() {
        if (!online || !device || String(device.platform || '').toLowerCase() !== 'android') return;
        previewsLoading = true;
        const res = await listSitePreviews(deviceId, siteId);
        previewsLoading = false;
        if (!res.ok) { notify(res.error || 'Не удалось загрузить Preview', 'error'); return; }
        previews = res.data.previews || [];
    }

    async function createPreview() {
        if (!online || previewCreating) return;
        previewCreating = true;
        const res = await createSitePreview(deviceId, siteId, { branch: previewBranch.trim() || gitForm.branch || 'main' });
        previewCreating = false;
        if (!res.ok || !res.data?.ok) { notify(res.error || res.data?.message || 'Не удалось создать Preview', 'error'); await loadPreviews(); return; }
        notify('Preview готов', 'success');
        await loadPreviews();
    }

    async function removePreview(preview) {
        if (deletingPreview || !confirm('Удалить этот Preview и остановить его процесс?')) return;
        deletingPreview = preview.key;
        const res = await deleteSitePreview(deviceId, siteId, preview.key);
        deletingPreview = '';
        if (!res.ok) { notify(res.error || 'Не удалось удалить Preview', 'error'); return; }
        previews = previews.filter(item => item.key !== preview.key);
        notify('Preview удалён', 'success');
    }
    async function configureWebhook(enabled) {
        if (webhookBusy) return;
        webhookBusy = true;
        const res = await setSiteWebhook(deviceId, siteId, enabled);
        webhookBusy = false;
        if (!res.ok) { notify(res.error || 'Не удалось изменить webhook', 'error'); return; }
        webhookEnabled = !!res.data.enabled;
        webhookPath = res.data.path || webhookPath;
        webhookSecret = res.data.secret || '';
        notify(enabled ? 'Webhook включён. Скопируйте secret и добавьте его в GitHub.' : 'Webhook отключён', 'success');
    }

    async function copyWebhookSecret() {
        try { await navigator.clipboard.writeText(webhookSecret); notify('Webhook secret скопирован', 'success'); }
        catch { notify('Не удалось скопировать secret', 'error'); }
    }

    async function copyWebhookURL() {
        try { await navigator.clipboard.writeText(window.location.origin + webhookPath); notify('Webhook URL скопирован', 'success'); }
        catch { notify('Не удалось скопировать URL', 'error'); }
    }

    async function saveEnvironmentVariable() {
        const name = newEnvName.trim();
        if (!name || savingEnv) return;
        savingEnv = true;
        const res = await saveSiteEnvironment(deviceId, siteId, name, newEnvValue);
        savingEnv = false;
        if (!res.ok) { notify(res.error || 'Не удалось сохранить переменную', 'error'); return; }
        newEnvName = '';
        newEnvValue = '';
        notify('Переменная сохранена в зашифрованном виде', 'success');
        const refreshed = await listSiteEnvironment(deviceId, siteId);
        if (refreshed.ok) envVariables = refreshed.data.variables || [];
    }

    async function removeEnvironmentVariable(name) {
        if (!confirm('Удалить переменную ' + name + '?')) return;
        const res = await deleteSiteEnvironment(deviceId, siteId, name);
        if (!res.ok) { notify(res.error || 'Не удалось удалить переменную', 'error'); return; }
        envVariables = envVariables.filter(item => item.name !== name);
        notify('Переменная удалена', 'success');
    }

    function parseEnvImport(source) {
        const entries = [], errors = [];
        for (const [index, raw] of source.split(/\r?\n/).entries()) {
            const line = raw.trim();
            if (!line || line.startsWith('#')) continue;
            const normalized = line.startsWith('export ') ? line.slice(7).trim() : line;
            const separator = normalized.indexOf('=');
            if (separator < 1) { errors.push(index + 1); continue; }
            const name = normalized.slice(0, separator).trim();
            let value = normalized.slice(separator + 1).trim();
            if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) value = value.slice(1, -1);
            if (!/^[A-Z_][A-Z0-9_]{0,63}$/.test(name) || /^(PATH|HOME|PREFIX|TMPDIR|PORT|HOST|DB_.*|VELTRIX_.*)$/.test(name)) { errors.push(index + 1); continue; }
            entries.push({ name, value });
        }
        return { entries, errors };
    }

    async function importEnvironmentVariables() {
        if (importingEnv || !envImportText.trim()) return;
        const parsed = parseEnvImport(envImportText);
        if (!parsed.entries.length) { notify('Не найдено корректных переменных для импорта', 'error'); return; }
        if (parsed.errors.length && !confirm('Строки ' + parsed.errors.join(', ') + ' будут пропущены. Импортировать ' + parsed.entries.length + ' переменных?')) return;
        importingEnv = true;
        let saved = 0;
        for (const item of parsed.entries) {
            const res = await saveSiteEnvironment(deviceId, siteId, item.name, item.value);
            if (!res.ok) {
                importingEnv = false;
                notify('Импорт остановлен на ' + item.name + ': ' + (res.error || 'ошибка сохранения') + '. Успешно: ' + saved, 'error');
                const refreshed = await listSiteEnvironment(deviceId, siteId);
                if (refreshed.ok) envVariables = refreshed.data.variables || [];
                return;
            }
            saved += 1;
        }
        importingEnv = false;
        envImportText = '';
        const refreshed = await listSiteEnvironment(deviceId, siteId);
        if (refreshed.ok) envVariables = refreshed.data.variables || [];
        notify('Импортировано переменных: ' + saved, 'success');
    }

    async function addScheduledTask() {
        if (!taskName.trim() || !taskCommand.trim() || taskBusy) return;
        taskBusy = true;
        const res = await createScheduledTask({ device_id: Number(deviceId), site_id: siteId, name: taskName.trim(), command: taskCommand.trim(), interval_minutes: Number(taskInterval) });
        taskBusy = false;
        if (!res.ok) { notify(res.error || 'Не удалось создать задачу', 'error'); return; }
        taskName = ''; taskCommand = '';
        notify('Scheduled task создана', 'success');
        const refreshed = await listScheduledTasks(deviceId, siteId);
        if (refreshed.ok) scheduledTasks = refreshed.data.tasks || [];
    }

    async function toggleScheduledTask(task) {
        const res = await setScheduledTaskEnabled(task.id, !task.enabled);
        if (!res.ok) { notify(res.error || 'Не удалось изменить задачу', 'error'); return; }
        scheduledTasks = scheduledTasks.map(item => item.id === task.id ? { ...item, enabled: !task.enabled } : item);
    }

    async function removeScheduledTask(task) {
        if (!confirm('Удалить задачу ' + task.name + '?')) return;
        const res = await deleteScheduledTask(task.id);
        if (!res.ok) { notify(res.error || 'Не удалось удалить задачу', 'error'); return; }
        scheduledTasks = scheduledTasks.filter(item => item.id !== task.id);
        notify('Задача удалена', 'success');
    }

    async function refreshScheduledTasks() {
        const res = await listScheduledTasks(deviceId, siteId);
        if (res.ok) scheduledTasks = res.data.tasks || [];
    }

    async function loadProcessMetrics() {
        if (!online || metricsBusy) return;
        metricsBusy = true;
        const res = await getSiteMetrics(deviceId, siteId);
        metricsBusy = false;
        if (res.ok) processMetrics = res.data.metrics || null;
        else if (res.status === 400) processMetrics = null;
    }

    function formatBytes(value) {
        const n = Number(value || 0);
        if (n < 1024) return n + ' B';
        if (n < 1048576) return (n / 1024).toFixed(1) + ' KB';
        if (n < 1073741824) return (n / 1048576).toFixed(1) + ' MB';
        return (n / 1073741824).toFixed(2) + ' GB';
    }

    async function saveRestartPolicy() {
        if (!online || savingRestartPolicy) return;
        savingRestartPolicy = true;
        const res = await sendCommand(deviceId, 'set_site_restart_policy', { site_id: siteId, policy: restartPolicy });
        savingRestartPolicy = false;
        if (!res.ok || !res.data?.ok) {
            notify(res.error || res.data?.message || 'Не удалось сохранить политику перезапуска', 'error');
            return;
        }
        notify('Политика перезапуска сохранена', 'success');
    }


    async function cancelDeployment() {
        if (!online || !deploymentJob?.job_id || !deploymentJobBusy) return;
        const res = await sendCommand(deviceId, 'cancel_deployment_job', { site_id: siteId, job_id: deploymentJob.job_id });
        if (!res.ok || !res.data?.ok) notify(res.error || res.data?.message || 'Не удалось отменить деплой', 'error');
        else notify('Отмена деплоя запрошена', 'warning');
    }

    async function runAction(action) {
        if (!online || busyAction) return;
        busyAction = action;
        if (action === 'deploy' || action === 'rebuild') {
            deploymentJobBusy = true;
            deploymentJob = { state: 'queued', progress: 0, message: 'Deployment queued' };
            agentLogOnline = true;
            logStreamEnabled = true;
            connectLiveLogs();
        }
        let res;
        if (action === 'start') {
            res = await sendCommand(deviceId, 'start_site', {
                site_id: site.site_id, path: site.path, port: site.port, runtime: site.runtime
            }, projectId);
        } else {
            res = await runSiteCommand(deviceId, siteId, action, action === 'deploy' || action === 'rebuild', '', action === 'deploy' || action === 'rebuild' ? deployCommit.trim() : '', projectId, environmentId);
        }
        busyAction = '';
        if (action === 'deploy' || action === 'rebuild') deploymentJobBusy = false;
        if (!res.ok || !res.data?.ok) {
            const failure = res.data?.error_code ? `[${res.data.error_code}] ${res.data.message || res.error || 'Команда завершилась ошибкой'}${res.data.retryable ? ' · можно повторить' : ''}` : (res.error || res.data?.message || 'Команда завершилась ошибкой');
            notify(failure, 'error');
            await refresh();
            return;
        }
        notify(res.data.message || 'Готово', 'success');
        await refresh();
    }

    function decodeLogs(payload) {
        if (!payload) return '';
        try {
            const raw = atob(payload);
            const bytes = Uint8Array.from(raw, c => c.charCodeAt(0));
            return new TextDecoder().decode(bytes);
        } catch { return ''; }
    }

    async function openTerminal() {
        activeTab = 'logs';
        showTerminal = false;
        terminalLoading = true;
        terminalText = '';
        if (online) {
            const res = await sendCommand(deviceId, 'get_logs', { site_id: siteId, lines: 100 });
            if (res.ok && res.data?.ok) {
                try {
                    const payload = JSON.parse(decodeLogs(res.data.payload));
                    terminalText = payload.content || '(лог пуст)';
                } catch { terminalText = res.data.message || '(лог пуст)'; }
            } else {
                terminalText = res.error || res.data?.message || 'Не удалось получить логи';
            }
        } else {
            const res = await listLogs(100);
            const rows = res.data?.logs || [];
            const matching = rows.filter(row => row.details?.site_id === siteId && (!device?.hostname || String(row.device || '') === String(device.hostname)));
            terminalText = matching.map(row => {
                const details = typeof row.details === 'string' ? row.details : JSON.stringify(row.details || {});
                return '[' + row.created_at + '] ' + row.action + ' ' + details;
            }).join('\n') || 'Устройство офлайн. В базе нет сохранённых записей для этого сайта.';
        }
        terminalLoading = false;
        if (online) {
            agentLogOnline = true;
            logStreamEnabled = true;
            logReconnectAttempts = 0;
            connectLiveLogs();
        }
    }

    $: visibleTerminalText = logSearch
        ? terminalText.split('\n').filter(line => line.toLowerCase().includes(logSearch.toLowerCase())).join('\n')
        : terminalText;
    $: filteredTerminalText = visibleTerminalText.split('\n').filter(line => {
        if (logLevel === 'all') return true;
        const upper = line.toUpperCase();
        if (logLevel === 'error') return /\b(ERROR|ERR|FATAL|PANIC)\b/.test(upper);
        if (logLevel === 'warn') return /\b(WARN|WARNING)\b/.test(upper);
        if (logLevel === 'info') return /\b(INFO|DEBUG|TRACE)\b/.test(upper) || !/\b(ERROR|ERR|FATAL|PANIC|WARN|WARNING)\b/.test(upper);
        return true;
    }).join('\n');

    function clearTerminal() { terminalText = ''; }

    async function copyTerminal() {
        try {
            await navigator.clipboard.writeText(terminalText);
            notify('Логи скопированы', 'success');
        } catch {
            notify('Не удалось скопировать: браузер не разрешил доступ к буферу обмена', 'error');
        }
    }

    function exportTerminal() {
        const blob = new Blob([terminalText], { type: 'text/plain;charset=utf-8' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = 'yevqo-' + siteId + '-logs.txt';
        link.click();
        URL.revokeObjectURL(url);
    }

    function clearLogReconnectTimer() {
        if (logReconnectTimer) {
            clearTimeout(logReconnectTimer);
            logReconnectTimer = null;
        }
    }

    function clearLogStableTimer() {
        if (logStableTimer) {
            clearTimeout(logStableTimer);
            logStableTimer = null;
        }
    }

    function scheduleLogReconnect() {
        if (!logStreamEnabled || !agentLogOnline || logReconnectTimer) return;
        logStatus = 'reconnecting';
        const index = Math.min(logReconnectAttempts, logReconnectDelays.length - 1);
        const delay = logReconnectDelays[index];
        logReconnectAttempts += 1;
        logReconnectTimer = setTimeout(() => {
            logReconnectTimer = null;
            if (logStreamEnabled && agentLogOnline && !logSocket) connectLiveLogs();
        }, delay);
    }

    function connectLiveLogs() {
        if (!agentLogOnline || !logStreamEnabled || logSocket) return;
        clearLogReconnectTimer();
        logStatus = logReconnectAttempts > 0 ? 'reconnecting' : 'connecting';
        let socket;
        try {
            socket = openSiteLogStream(deviceId, siteId, lastLogSeq);
        } catch {
            scheduleLogReconnect();
            return;
        }
        logSocket = socket;
        socket.onopen = () => {
            if (logSocket !== socket) return;
            logConnected = true;
            logStatus = 'live';
            // Reset backoff only after a stable connection, not immediately:
            // flapping sockets should continue backing off.
            clearLogStableTimer();
            logStableTimer = setTimeout(() => {
                logStableTimer = null;
                if (logSocket === socket && logConnected) logReconnectAttempts = 0;
            }, 30000);
        };
        socket.onmessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                if (message.type === 'agent_status' && String(message.device_id) === String(deviceId)) {
                    agentLogOnline = message.status === 'online';
                    if (!agentLogOnline) {
                        logConnected = false;
                        clearLogStableTimer();
                        logStatus = 'agent-offline';
                    } else if (logStreamEnabled) {
                        if (logSocket && logSocket.readyState === WebSocket.OPEN) {
                            logConnected = true;
                            logStatus = 'live';
                        } else {
                            connectLiveLogs();
                        }
                    }
                    return;
                }
                if (message.type === 'deployment_job' && message.site_id === siteId) {
                    if (message.stream_id && message.stream_id !== logStreamID) { logStreamID = message.stream_id; lastLogSeq = 0; }
                    const seq = Number(message.seq || 0);
                    if (seq && seq <= lastLogSeq) return;
                    if (seq) lastLogSeq = seq;
                    deploymentJob = { job_id: message.job_id, state: message.state, progress: Number(message.progress || 0), message: message.message || '' };
                    deploymentJobBusy = !['healthy', 'failed', 'cancelled'].includes(message.state);
                    return;
                }
                if (message.type !== 'log' || message.site_id !== siteId) return;
                if (message.stream_id && message.stream_id !== logStreamID) {
                    logStreamID = message.stream_id;
                    lastLogSeq = 0;
                }
                const seq = Number(message.seq || 0);
                if (seq && seq <= lastLogSeq) return;
                if (seq) lastLogSeq = seq;
                const stamp = message.timestamp ? new Date(message.timestamp).toLocaleTimeString() : new Date().toLocaleTimeString();
                const lines = (terminalText ? terminalText.split('\n') : []).concat('[' + stamp + '] ' + String(message.line ?? ''));
                terminalText = lines.slice(-500).join('\n');
                if (autoScroll) tick().then(() => { if (terminalElement) terminalElement.scrollTop = terminalElement.scrollHeight; });
            } catch { /* Ignore malformed stream frames. */ }
        };
        socket.onclose = () => {
            if (logSocket !== socket) return;
            logSocket = null;
            logConnected = false;
            clearLogStableTimer();
            if (logStreamEnabled && agentLogOnline) scheduleLogReconnect();
            else logStatus = agentLogOnline ? 'disconnected' : 'agent-offline';
        };
        socket.onerror = () => {
            if (logSocket !== socket) return;
            logConnected = false;
            logStatus = agentLogOnline ? 'reconnecting' : 'agent-offline';
            // The browser will follow this with close; reconnect is scheduled there.
        };
    }

    function toggleLiveLogs() {
        if (logStreamEnabled) {
            logStreamEnabled = false;
            agentLogOnline = false;
            clearLogReconnectTimer();
            clearLogStableTimer();
            logReconnectAttempts = 0;
            if (logSocket) {
                const socket = logSocket;
                logSocket = null;
                socket.close();
            }
            logConnected = false;
            logStatus = 'disconnected';
        } else {
            logStreamEnabled = true;
            logReconnectAttempts = 0;
            connectLiveLogs();
        }
    }

    async function loadBuildHistory() {
        if (!online || buildHistoryLoading) { if (!online) buildHistory = []; return; }
        buildHistoryLoading = true;
        const res = await sendCommand(deviceId, 'list_build_history', { site_id: siteId, limit: 10 }, projectId);
        buildHistoryLoading = false;
        if (!res.ok || !res.data?.ok) { buildHistory = []; return; }
        try { buildHistory = JSON.parse(decodeLogs(res.data.payload)).records || []; }
        catch { buildHistory = []; }
    }
    async function viewBuildHistory(record) {
        if (!record?.id || !online) return;
        const res = await sendCommand(deviceId, 'get_build_history', { site_id: siteId, id: record.id }, projectId);
        if (!res.ok || !res.data?.ok) { notify(res.error || res.data?.message || 'Не удалось открыть лог сборки', 'error'); return; }
        try { buildResult = JSON.parse(decodeLogs(res.data.payload)); }
        catch { notify('Не удалось прочитать запись сборки', 'error'); }
    }

    async function analyzeBuild() {
        if (!online || analyzingBuild) return;
        analyzingBuild = true;
        buildPlan = null;
        const res = await sendCommand(deviceId, 'analyze_site_build', { site_id: siteId, root_dir: gitForm.root_directory || '.' });
        analyzingBuild = false;
        if (!res.ok || !res.data?.ok) {
            notify(res.error || res.data?.message || 'Не удалось проанализировать проект', 'error');
            return;
        }
        try {
            buildPlan = JSON.parse(decodeLogs(res.data.payload));
            if (buildPlan.root_directory) gitForm.root_directory = buildPlan.root_directory;
            activeTab = 'build';
        } catch {
            notify('Получен некорректный ответ анализа', 'error');
        }
    }
    async function runAndroidBuild() {
        if (!online || !buildPlan || buildRunning) return;
        buildRunning = true;
        buildResult = null;
        const res = await sendCommand(deviceId, 'build_site', {
            site_id: siteId,
            root_dir: gitForm.root_directory || '.',
            stack: buildPlan.stack,
            package_tool: buildPlan.package_tool || '',
            install_command: buildPlan.install_command || '',
            build_command: buildPlan.build_command || '',
            allow_project_code: true,
            timeout_seconds: 1200,
            max_log_bytes: 10485760
        }, projectId);
        buildRunning = false;
        if (!res.ok || !res.data) {
            notify(res.error || 'Не удалось выполнить сборку', 'error');
            return;
        }
        try {
            buildResult = res.data.payload ? JSON.parse(decodeLogs(res.data.payload)) : {
                success: !!res.data.ok, error: res.data.message || ''
            };
        } catch {
            buildResult = { success: !!res.data.ok, output: '', error: res.data.message || 'Некорректный ответ сборки' };
        }
        if (buildResult.success) notify('Локальная сборка завершена', 'success');
        else notify(buildResult.error || res.data.message || 'Сборка завершилась ошибкой', 'error');
    }


    async function loadReleases() {
        if (!online || releasesLoading) return;
        releasesLoading = true;
        const res = await getSiteReleases(deviceId, siteId);
        releasesLoading = false;
        if (!res.ok) { notify(res.error || 'Не удалось загрузить историю релизов', 'error'); return; }
        releases = res.data.releases || [];
    }

    async function pruneReleases() {
        if (!online || pruningReleases || !Number.isInteger(Number(releaseKeep)) || Number(releaseKeep) < 1 || Number(releaseKeep) > 50) return;
        if (!window.confirm(`Удалить все релизы, кроме ${releaseKeep} последних и активного? Это действие нельзя отменить.`)) return;
        pruningReleases = true;
        const res = await pruneSiteReleases(deviceId, siteId, Number(releaseKeep));
        pruningReleases = false;
        if (!res.ok || !res.data?.ok) {
            notify(res.error || res.data?.message || 'Не удалось очистить историю релизов', 'error');
            return;
        }
        notify(res.data.message || 'История релизов очищена', 'success');
        await loadReleases();
    }

    async function rollbackRelease(commit) {
        if (!online || busyAction || !commit) return;
        if (!window.confirm('Откатить сайт на релиз ' + commit.slice(0, 12) + '? Текущая версия останется в истории релизов.')) return;
        busyAction = 'rollback';
        rollbackTarget = commit;
        const res = await runSiteCommand(deviceId, siteId, 'rollback', false, commit, '', projectId, environmentId);
        busyAction = '';
        rollbackTarget = '';
        if (!res.ok || !res.data?.ok) {
            notify(res.error || res.data?.message || 'Не удалось выполнить откат', 'error');
            await refresh();
            await loadReleases();
            return;
        }
        notify(res.data.message || 'Откат выполнен', 'success');
        await refresh();
        await loadReleases();
    }

    async function loadGitHubConnections() {
        gitHubRepositoriesError = '';
        const result = await listIntegrations();
        if (!result.ok) { gitHubRepositoriesError = result.error || 'Не удалось загрузить GitHub integration'; return; }
        gitHubIntegrations = (result.data.integrations || []).filter(item => item.provider === 'github_app');
        const savedRepo = String(gitForm.repository_url || '').trim().replace(/\.git$/, '').replace(/\/$/, '');
        const preferred = gitHubIntegrations.find(item => String(item.id) === String(selectedGitHubIntegration)) || gitHubIntegrations[0];
        if (savedRepo) {
            // Search every connected installation so a saved repository is
            // restored even when the user has multiple GitHub App accounts.
            for (const integration of gitHubIntegrations) {
                selectedGitHubIntegration = String(integration.id);
                selectedGitHubRepository = '';
                await loadGitHubRepositories(savedRepo);
                if (selectedGitHubRepository) return;
            }
        }
        selectedGitHubIntegration = preferred ? String(preferred.id) : '';
        selectedGitHubRepository = '';
        if (selectedGitHubIntegration) await loadGitHubRepositories();
    }

    async function loadGitHubRepositories(savedRepo = '') {
        if (!selectedGitHubIntegration) { gitHubRepositories = []; return; }
        gitHubRepositoriesLoading = true;
        gitHubRepositoriesError = '';
        const result = await listIntegrationRepositories(selectedGitHubIntegration);
        gitHubRepositoriesLoading = false;
        if (!result.ok) { gitHubRepositories = []; gitHubRepositoriesError = result.error || 'Не удалось получить репозитории GitHub'; return; }
        gitHubRepositories = result.data.repositories || [];
        const normalizedSaved = String(savedRepo || gitForm.repository_url || '').trim().replace(/\.git$/, '').replace(/\/$/, '').toLowerCase();
        const match = gitHubRepositories.find(repo => [repo.web_url, repo.clone_url, `https://github.com/${repo.full_name}`, `https://github.com/${repo.full_name}.git`]
            .some(url => String(url || '').trim().replace(/\.git$/, '').replace(/\/$/, '').toLowerCase() === normalizedSaved));
        const selected = gitHubRepositories.find(repo => String(repo.full_name) === String(selectedGitHubRepository)) || match;
        if (selected) {
            selectedGitHubRepository = selected.full_name;
            if (!gitForm.repository_url || match) gitForm.repository_url = selected.clone_url || `https://github.com/${selected.full_name}.git`;
            if (!gitForm.branch || !gitForm.branch.trim()) gitForm.branch = selected.default_branch || 'main';
        } else if (!normalizedSaved && !selectedGitHubRepository && gitHubRepositories.length === 1) {
            selectedGitHubRepository = gitHubRepositories[0].full_name;
            gitForm.repository_url = gitHubRepositories[0].clone_url || `https://github.com/${gitHubRepositories[0].full_name}.git`;
            gitForm.branch = gitHubRepositories[0].default_branch || 'main';
        }
    }

    function selectGitHubRepository(fullName) {
        selectedGitHubRepository = fullName;
        const repo = gitHubRepositories.find(item => item.full_name === fullName);
        if (!repo) return;
        gitForm.repository_url = repo.clone_url || `https://github.com/${repo.full_name}.git`;
        gitForm.branch = repo.default_branch || 'main';
    }

    async function saveGit() {
        if (busyAction) return;
        busyAction = 'git';
        const res = await saveSiteGit(deviceId, siteId, gitForm);
        busyAction = '';
        if (!res.ok) { notify(res.error || 'Не удалось сохранить Git настройки', 'error'); return; }
        gitForm.github_token = '';
        notify('Git-репозиторий подключён', 'success');
        await refresh();
    }

    function openSite() {
        const target = site?.domain ? 'https://' + site.domain :
            (device?.tailscale_ip ? 'http://' + device.tailscale_ip + ':' + site.port : '');
        if (!target) { notify('Нет доступного IP или домена', 'warning'); return; }
        window.open(target, '_blank', 'noopener');
    }
</script>

{#if loading}
    <div class="empty">Загрузка сайта...</div>
{:else if site}
    {#if deploymentJob}
        <Card>
            <div class="deployment-job">
                <div><strong>Deployment · {deploymentJob.state}</strong><p class="muted">{deploymentJob.message || 'Выполняется деплой'}</p></div>
                <strong>{deploymentJob.progress}%</strong>
                {#if deploymentJobBusy}<Button size="sm" variant="danger" disabled={!online} on:click={cancelDeployment}>Cancel</Button>{/if}
            </div>
            <div class="deployment-progress"><div style={'width:' + deploymentJob.progress + '%'}></div></div>
        </Card>
    {/if}
    <div class="page-header">
        <div>
            <button class="back" on:click={() => goto('/sites')}>← Все сайты</button>
            <h1 class="page-title">{site.site_id}</h1>
            <p class="page-desc">{site.path} · порт {site.port} · {site.runtime}</p>
        </div>
        <div class="device-state">
            <Button size="sm" on:click={() => goto('/backups?resource_type=site&resource_id=' + encodeURIComponent(deviceId + ':' + siteId))}>Backups</Button>
            <StatusDot status={online ? 'online' : 'offline'} />
            <span>{online ? 'Устройство онлайн' : 'Устройство офлайн'}</span>
        </div>
    </div>

    <Card>
        <div class="panel-head">
            <div>
                <h2>Deploy Settings</h2>
                <p>Управление сборкой и процессом сайта</p>
            </div>
            <div class="site-state">
                <StatusDot status={site.status} />
                <span>{site.status === 'running' ? 'Running' : site.status === 'error' ? 'Error' : 'Stopped'}</span>
            </div>
        </div>
        <div class="actions">
            <Button variant="primary" disabled={!online || !!busyAction} loading={busyAction === 'deploy'} on:click={() => runAction('deploy')}>Deploy</Button>
            <Button disabled={!online || !!busyAction} loading={busyAction === 'rebuild'} on:click={() => runAction('rebuild')}>Rebuild</Button>
            <Button disabled={!online || !!busyAction} loading={busyAction === 'restart'} on:click={() => runAction('restart')}>Reload</Button>
            {#if site.status === 'running'}
                <Button variant="danger" disabled={!online || !!busyAction} loading={busyAction === 'stop'} on:click={() => runAction('stop')}>Stop</Button>
            {:else}
                <Button disabled={!online || !!busyAction} loading={busyAction === 'start'} on:click={() => runAction('start')}>Start</Button>
            {/if}
            <Button variant="ghost" on:click={openTerminal}>Open Terminal</Button>
        </div>
        {#if !online}
            <p class="offline-note">Устройство офлайн. Команды недоступны; просмотр сохранённых записей остаётся доступен.</p>
        {/if}
    </Card>

    <nav class="tabs">
        <button class:active={activeTab === 'general'} on:click={() => selectTab('general')}>Обзор</button>
        <button class:active={activeTab === 'git'} on:click={() => selectTab('git')}>Git</button>
        <button class:active={activeTab === 'settings'} on:click={() => selectTab('settings')}>Настройки</button>
        <button class:active={activeTab === 'environment'} on:click={() => selectTab('environment')}>Переменные</button>
        <button class:active={activeTab === 'domains'} on:click={() => selectTab('domains')}>Домены</button>
        <button class:active={activeTab === 'deployments'} on:click={() => selectTab('deployments')}>Развёртывания</button>
        <button class:active={activeTab === 'build'} on:click={() => selectTab('build')}>Сборки</button>
        <button class:active={activeTab === 'tasks'} on:click={() => selectTab('tasks')}>Расписания</button>
        <button class:active={activeTab === 'monitoring'} on:click={() => selectTab('monitoring')}>Мониторинг</button>
        <button class:active={activeTab === 'logs'} on:click={openTerminal}>Журналы</button>
        {#if String(device?.platform || '').toLowerCase() === 'android'}<button class:active={activeTab === 'previews'} on:click={() => { selectTab('previews'); loadPreviews(); }}>Previews</button>{/if}
    </nav>

    {#if activeTab === 'general'}
        <div class="detail-grid">
            <Card>
                <h2 class="details-title">General</h2>
                <div class="details">
                    <div><span>Site ID</span><strong>{site.site_id}</strong></div>
                    <div><span>Runtime</span><strong>{site.runtime}</strong></div>
                    <div><span>Port</span><strong>{site.port}</strong></div>
                    <div><span>Directory</span><strong>{site.path}</strong></div>
                    <div><span>Domain</span><strong>{site.domain || 'Не привязан'}</strong></div>
                    <div><span>Device</span><strong>{device?.hostname || deviceId}</strong></div>
                    <div class="restart-policy">
                        <span>Restart policy</span>
                        <select bind:value={restartPolicy} disabled={!online || savingRestartPolicy}>
                            <option value="never">Never</option>
                            <option value="on-failure">On failure</option>
                            <option value="always">Always</option>
                        </select>
                        <Button size="sm" disabled={!online || savingRestartPolicy} loading={savingRestartPolicy} on:click={saveRestartPolicy}>Save policy</Button>
                        <p class="muted">Автоперезапуск после неожиданного завершения. Повторные попытки ограничены: 2, 4 и 8 секунд, максимум 3 раза.</p>
                    </div>
                </div>
                <Button variant="primary" disabled={!site.domain && !device?.tailscale_ip} on:click={openSite}>Открыть сайт ↗</Button>
            </Card>
            <Card>
                <div class="metrics-card-head"><h2 class="details-title">Process telemetry</h2><Button size="sm" disabled={!online || metricsBusy} on:click={loadProcessMetrics}>Refresh</Button></div>
                {#if !online}<p class="muted">Устройство офлайн — метрики недоступны.</p>
                {:else if processMetrics}
                    <div class="process-metrics">
                        <div><span>CPU</span><strong>{Number(processMetrics.cpu_percent || 0).toFixed(1)}%</strong></div>
                        <div><span>RSS memory</span><strong>{formatBytes(processMetrics.rss_bytes)}</strong></div>
                        <div><span>Processes</span><strong>{processMetrics.process_count || 1}</strong></div>
                        <div><span>Main PID</span><strong>{processMetrics.pid}</strong></div>
                    </div>
                    <p class="muted">CPU и RSS суммируются по выделенной process group сайта, включая дочерние процессы и workers. Это метрики ОС, не контейнерные лимиты или cgroup-метрики.</p>
                {:else}<p class="muted">Нет активного управляемого процесса для сбора метрик.</p>{/if}
            </Card>
            <Card>
                <h2 class="details-title">Последний деплой</h2>
                <div class="commit">{git.commit ? git.commit.slice(0, 12) : 'Ещё не выполнялся'}</div>
                <p class="muted">{git.deployed_at ? new Date(git.deployed_at).toLocaleString() : 'Нет данных о деплое'}</p>
                <p class="muted">{git.repository_url || 'Git-репозиторий не подключён'}</p>
            </Card>
            <Card>
                <div class="metrics-card-head"><h2 class="details-title">Git</h2><Button size="sm" on:click={() => selectTab('git')}>Изменить</Button></div>
                <div class="details">
                    <div><span>GitHub connection</span><strong>{gitHubIntegrations.find(item => String(item.id) === String(selectedGitHubIntegration))?.account_name || 'Не подключено'}{gitHubIntegrations.find(item => String(item.id) === String(selectedGitHubIntegration)) ? ' · GitHub App' : ''}</strong></div>
                    <div><span>GitHub repository</span><strong>{selectedGitHubRepository || 'Не выбран'}</strong></div>
                    <div><span>Branch</span><strong>{gitForm.branch || 'main'}</strong></div>
                </div>
            </Card>
        </div>
    {:else if activeTab === 'logs'}
        <Card>
            <div class="logs-head"><div><h2 class="details-title">Terminal / Logs</h2><span class:live={logConnected} class="log-connection">{logStatus === 'live' ? '● Live' : logStatus === 'connecting' ? '◌ Connecting…' : logStatus === 'reconnecting' ? '↻ Reconnecting…' : logStatus === 'agent-offline' ? '● Agent offline' : '○ Disconnected'}</span></div><div class="log-actions"><Button size="sm" on:click={copyTerminal} disabled={!terminalText}>Copy</Button><Button size="sm" on:click={exportTerminal} disabled={!terminalText}>Export</Button><Button size="sm" on:click={clearTerminal}>Clear</Button><Button size="sm" on:click={toggleLiveLogs} disabled={!online}>{logStreamEnabled ? 'Остановить поток' : 'Live logs'}</Button><Button size="sm" on:click={openTerminal} disabled={terminalLoading}>Обновить</Button></div></div>
            {#if terminalLoading}<p class="terminal-loading">Загрузка…</p>{:else}<div class="log-filters"><input class="log-search" bind:value={logSearch} placeholder="Поиск по логам…" /><select class="log-level" bind:value={logLevel}><option value="all">Все уровни</option><option value="error">ERROR</option><option value="warn">WARN</option><option value="info">INFO / DEBUG</option></select><label class="autoscroll"><input type="checkbox" bind:checked={autoScroll} /> Автопрокрутка</label></div><pre class="terminal" bind:this={terminalElement} on:scroll={() => autoScroll = terminalElement.scrollHeight - terminalElement.scrollTop - terminalElement.clientHeight < 30}>{filteredTerminalText || (logSearch || logLevel !== 'all' ? 'Совпадений нет.' : 'Логи пока не загружены.')}</pre>{/if}
        </Card>
    {:else if activeTab === 'tasks'}
        <Card>
            <h2 class="details-title">Scheduled Tasks</h2>
            <p class="muted">Запускайте команды в каталоге сайта по фиксированному интервалу. Задачи выполняются от имени пользователя Agent, используют защищённые переменные окружения и ограничены 30 минутами на запуск.</p>
            <div class="task-create">
                <input class="input" bind:value={taskName} placeholder="Task name (e.g. Clear cache)" maxlength="100" />
                <textarea class="input task-command" bind:value={taskCommand} placeholder="Command (e.g. php artisan schedule:run)" maxlength="4096" rows="3"></textarea>
                <div class="task-create-actions">
                    <select class="input" bind:value={taskInterval}>
                        <option value={15}>Every 15 minutes</option><option value={60}>Hourly</option><option value={360}>Every 6 hours</option><option value={1440}>Daily</option><option value={10080}>Weekly</option><option value={43200}>Every 30 days</option>
                    </select>
                    <Button variant="primary" disabled={!online || !taskName.trim() || !taskCommand.trim() || taskBusy} loading={taskBusy} on:click={addScheduledTask}>Add task</Button>
                </div>
            </div>
            {#if scheduledTasks.length}
                <div class="task-list">
                    {#each scheduledTasks as task}
                        <div class="task-item">
                            <div class="task-item-head"><div><strong>{task.name}</strong><span class:task-failed={task.last_status==='failed'} class="task-state">{task.enabled ? task.last_status : 'paused'}</span></div><div class="task-buttons"><Button size="sm" on:click={() => toggleScheduledTask(task)}>{task.enabled ? 'Pause' : 'Enable'}</Button><Button size="sm" variant="danger" on:click={() => removeScheduledTask(task)}>Delete</Button></div></div>
                            <code>{task.command}</code>
                            <small>Every {task.interval_minutes} min · next {new Date(task.next_run_at).toLocaleString()}{task.last_run_at ? ' · last ' + new Date(task.last_run_at).toLocaleString() : ''}</small>
                            {#if task.last_error}<small class="task-failed">{task.last_error}</small>{/if}
                            {#if task.last_output}<button class="task-output-toggle" on:click={() => taskOutputOpen[task.id] = !taskOutputOpen[task.id]}>{taskOutputOpen[task.id] ? 'Hide output' : 'Show last output'}</button>{#if taskOutputOpen[task.id]}<pre class="task-output">{task.last_output}</pre>{/if}{/if}
                        </div>
                    {/each}
                </div>
            {:else}<p class="muted">Scheduled tasks пока не созданы.</p>{/if}
        </Card>
    {:else if activeTab === 'domains'}
        <Card>
            <h2 class="details-title">Domains</h2>
            <p class="muted">Домен сайта и Cloudflare Tunnel. Сохранение домена запускает cloudflared на Agent; публичный маршрут и CNAME можно настроить автоматически ниже.</p>
            <label class="field-label" for="site-domain">Primary domain</label>
            <input id="site-domain" class="input" bind:value={domainDraft} placeholder="app.example.com" autocomplete="off"/>
            {#if cloudflareConfig.connected && cloudflareConfig.zone_name}
                <Button size="sm" on:click={() => { const label = siteId.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/^-+|-+$/g, ''); domainDraft = label + '.' + cloudflareConfig.zone_name; }}>Предложить домен для приложения</Button>
            {/if}
            <label class="field-label" for="site-tunnel-token">Cloudflare Tunnel token</label>
            <input id="site-tunnel-token" class="input" type="password" bind:value={domainTunnelToken} placeholder="Оставьте пустым, чтобы использовать сохранённый token" autocomplete="new-password"/>
            <div class="actions">
                <Button variant="primary" loading={domainBusy} disabled={domainBusy || !online} on:click={async()=>{
                    domainBusy=true;
                    if (cloudflareRoute?.configured && domainDraft.trim().toLowerCase() !== String(site?.domain || '').toLowerCase()) {
                        const removed=await removeCloudflareTunnelRoute(deviceId,siteId);
                        if(!removed.ok){domainBusy=false;notify(removed.error||'Сначала удалите Cloudflare route','error');return;}
                        cloudflareRoute={configured:false};
                    }
                    const suppliedTunnelToken=domainTunnelToken.trim();
                    const r=await updateSiteDomain(deviceId,siteId,domainDraft.trim(),suppliedTunnelToken);
                    if(!r.ok){domainBusy=false;notify(r.error||'Не удалось сохранить домен','error');return}
                    domainTunnelToken='';
                    if(!r.data?.ok){domainBusy=false;notify(r.data?.message||'Домен сохранён, но Tunnel не активирован','error');await refresh();return}
                    const inferredTunnelId=cloudflareTunnelIDFromToken(suppliedTunnelToken)||cloudflareTunnelIdDraft.trim();
                    if(inferredTunnelId && cloudflareConfig.connected){
                        cloudflareTunnelIdDraft=inferredTunnelId;
                        const route=await configureCloudflareTunnelRoute(deviceId,siteId,inferredTunnelId);
                        if(route.ok){
                            cloudflareRoute=route.data;
                            if(route.data?.ingress_managed===false) notify('Tunnel подключён, DNS CNAME настроен. Проверь локальный ingress в config.yml.','warning');
                            else notify('Tunnel подключён, DNS и публичный маршрут настроены','success');
                        } else notify('Tunnel подключён, но публичный маршрут не настроен: '+(route.error||'проверь Cloudflare API permissions'),'warning');
                    }else{
                        notify('Tunnel подключён, но публичный маршрут не настроен. Укажи Tunnel ID и подключи Cloudflare API для автоматической маршрутизации.','warning');
                    }
                    domainBusy=false;
                    await refresh()
                }}>Save domain</Button>
                <Button variant="danger" disabled={domainBusy||!site?.domain||!online} on:click={async()=>{
                    if(!confirm('Удалить домен и остановить Cloudflare Tunnel?'))return;
                    domainBusy=true;
                    if(cloudflareRoute?.configured){
                        const removed=await removeCloudflareTunnelRoute(deviceId,siteId);
                        if(!removed.ok){domainBusy=false;notify(removed.error||'Не удалось удалить Cloudflare route','error');return;}
                        cloudflareRoute={configured:false};
                    }
                    const r=await updateSiteDomain(deviceId,siteId,'','');
                    domainBusy=false;
                    if(r.ok){notify('Домен удалён','success');domainDraft='';await refresh()}else notify(r.error||'Не удалось удалить домен','error')
                }}>Remove domain</Button>
            </div>
        </Card>
        <Card>
            <h2 class="details-title">Cloudflare API integration</h2>
            {#if cloudflareConfig.connected}
                <p class="muted">Подключено: {cloudflareConfig.zone_name} ({cloudflareConfig.zone_id})</p>
                <label class="field-label" for="cloudflare-tunnel-id">Cloudflare Tunnel ID</label>
                <input id="cloudflare-tunnel-id" class="input" bind:value={cloudflareTunnelIdDraft} placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" autocomplete="off"/>
                <div class="actions">
                    <Button variant="primary" loading={cloudflareRouteBusy} disabled={cloudflareRouteBusy||!site?.domain||!cloudflareTunnelIdDraft.trim()} on:click={configureCloudflareRoute}>Configure public route</Button>
                    {#if cloudflareRoute?.configured}<Button variant="danger" loading={cloudflareRouteBusy} disabled={cloudflareRouteBusy} on:click={removeCloudflareRoute}>Remove route</Button>{/if}
                    <Button size="sm" disabled={cloudflareConfigBusy} on:click={disconnectCloudflareAccount}>Disconnect API</Button>
                </div>
                {#if cloudflareRoute?.configured}
                    {#if cloudflareRoute.ingress_managed === false}
                        <p class="muted">DNS CNAME: <strong>{cloudflareRoute.hostname}</strong> → <code>{cloudflareRoute.tunnel_id}.cfargotunnel.com</code></p>
                        <p class="muted">Tunnel локально управляется из Termux. Yevqo не меняет его ingress: проверь в config.yml правило <code>{cloudflareRoute.hostname}</code> → <code>{cloudflareRoute.origin}</code>.</p>
                    {:else}
                        <p class="muted">Route: <strong>{cloudflareRoute.hostname}</strong> → <code>{cloudflareRoute.origin}</code></p>
                    {/if}
                {:else}
                    <p class="muted">Yevqo определит тип Tunnel. Для remotely-managed настроит ingress и CNAME; для locally-managed — только DNS CNAME, не меняя config.yml на устройстве.</p>
                {/if}
            {:else}
                <label class="field-label" for="cloudflare-api-token">Cloudflare API token</label>
                <input id="cloudflare-api-token" class="input" type="password" bind:value={cloudflareTokenDraft} placeholder="API token" autocomplete="new-password"/>
                <label class="field-label" for="cloudflare-zone-id">Cloudflare Zone ID</label>
                <input id="cloudflare-zone-id" class="input" bind:value={cloudflareZoneDraft} placeholder="Zone ID" autocomplete="off"/>
                <Button variant="primary" loading={cloudflareConfigBusy} disabled={cloudflareConfigBusy||!cloudflareTokenDraft.trim()||!cloudflareZoneDraft.trim()} on:click={connectCloudflare}>Connect Cloudflare</Button>
                <p class="muted">Токену нужны Account: Cloudflare Tunnel — Edit (API permission: Cloudflare Tunnel Write), Zone: DNS — Edit и Zone — Read. API token хранится зашифрованным. Настройку выполняет владелец workspace.</p>
            {/if}
        </Card>
    {:else if activeTab === 'environment'}
        <Card>
            <div class="env-header">
                <div><h2 class="details-title">Environment Variables</h2><p class="muted">Переменные приложения хранятся отдельно от исходного кода и шифруются AES-GCM.</p></div>
                <span class="env-count">{envVariables.length} variables</span>
            </div>
            <div class="env-notice"><strong>Область действия</strong><span>Сейчас один набор переменных используется и при Build, и при Runtime. Значения из подключённых баз данных управляются отдельно и имеют приоритет.</span></div>
            <div class="env-add-panel">
                <h3>Add / Update variable</h3>
                <div class="env-add">
                    <input class="input" bind:value={newEnvName} placeholder="VARIABLE_NAME" maxlength="64" autocomplete="off" aria-label="Имя переменной" />
                    <input class="input" bind:value={newEnvValue} placeholder="Value (может быть пустым)" type="password" autocomplete="new-password" aria-label="Значение переменной" />
                    <Button variant="primary" loading={savingEnv} disabled={!newEnvName.trim() || savingEnv} on:click={saveEnvironmentVariable}>Save variable</Button>
                </div>
                <p class="muted">Сохранение доступно даже когда Agent офлайн. Новые значения применятся при следующем запуске или деплое приложения.</p>
            </div>
            <div class="env-import-panel">
                <div><h3>Import from .env</h3><p class="muted">Вставь содержимое файла .env. Поддерживаются KEY=value, export KEY=value и комментарии #.</p></div>
                <textarea class="env-import-input" bind:value={envImportText} rows="6" spellcheck="false" placeholder="APP_ENV=production&#10;API_BASE_URL=https://example.com&#10;# comment&#10;FEATURE_FLAG=true"></textarea>
                <div class="env-import-actions"><span class="muted">Существующие ключи будут обновлены.</span><Button disabled={!envImportText.trim() || importingEnv} loading={importingEnv} on:click={importEnvironmentVariables}>Import variables</Button></div>
            </div>
            <div class="env-list-head">
                <h3>Stored variables</h3>
                <input class="input env-search" bind:value={envSearch} placeholder="Search variables…" aria-label="Поиск переменных" />
            </div>
            {#if envVariables.filter(item => item.name.toLowerCase().includes(envSearch.toLowerCase())).length}
                <div class="env-list">{#each envVariables.filter(item => item.name.toLowerCase().includes(envSearch.toLowerCase())) as variable}
                    <div class="env-row"><div class="env-variable-name"><code>{variable.name}</code><small>Encrypted · masked</small></div><span class="env-masked" aria-label="Значение скрыто">••••••••••••</span><Button size="sm" on:click={() => { newEnvName = variable.name; newEnvValue = ''; document.querySelector('.env-add-panel')?.scrollIntoView({ behavior: 'smooth', block: 'center' }); }}>Update</Button><Button size="sm" variant="danger" on:click={() => removeEnvironmentVariable(variable.name)}>Delete</Button></div>
                {/each}</div>
            {:else if envVariables.length}
                <p class="muted">Ничего не найдено по запросу «{envSearch}».</p>
            {:else}<div class="env-empty"><strong>No environment variables yet</strong><span>Добавь первую переменную или импортируй готовый .env.</span></div>{/if}
            <p class="env-security-note">Значения намеренно не возвращаются в браузер после сохранения — поэтому показать или скопировать ранее сохранённый секрет нельзя. Чтобы заменить значение, укажи новое для существующего ключа.</p>
        </Card>
    {:else if activeTab === 'monitoring'}
        <Card>
            <div class="metrics-card-head"><div><h2 class="details-title">Мониторинг приложения</h2><p class="muted">Метрики процесса именно этого приложения на выбранном устройстве.</p></div><Button size="sm" disabled={!online || metricsBusy} on:click={loadProcessMetrics}>Обновить</Button></div>
            {#if !online}<p class="muted">Устройство офлайн — метрики недоступны.</p>
            {:else if processMetrics}
                <div class="process-metrics">
                    <div><span>CPU</span><strong>{Number(processMetrics.cpu_percent || 0).toFixed(1)}%</strong></div>
                    <div><span>RSS memory</span><strong>{formatBytes(processMetrics.rss_bytes)}</strong></div>
                    <div><span>Processes</span><strong>{processMetrics.process_count || 1}</strong></div>
                    <div><span>Main PID</span><strong>{processMetrics.pid}</strong></div>
                </div>
                <p class="muted">CPU и RSS суммируются по выделенной process group приложения, включая дочерние процессы и workers. Это метрики ОС, не контейнерные лимиты или cgroup-метрики.</p>
            {:else}<p class="muted">Нет активного управляемого процесса для сбора метрик.</p>{/if}
        </Card>
    {:else if activeTab === 'build'}
        <Card>
            <div class="logs-head">
                <div><h2 class="details-title">Build Engine</h2><p class="muted">Анализ проекта без выполнения его кода. Root Directory берётся из настроек сайта.</p></div>
                <Button variant="primary" disabled={!online || analyzingBuild} loading={analyzingBuild} on:click={analyzeBuild}>Analyze project</Button>
            </div>
            {#if !online}
                <p class="offline-note">Устройство должно быть онлайн для анализа файлов проекта.</p>
            {:else if buildPlan}
                <div class="build-summary">
                    <div><span>Detected stack</span><strong>{buildPlan.stack || 'unknown'}</strong></div>
                    <div><span>Provider</span><strong>{buildPlan.provider || 'auto'}</strong></div>
                    <div><span>Confidence</span><strong>{buildPlan.confidence || 'low'}</strong></div>
                    <div><span>Project root</span><strong>{buildPlan.root || '.'}</strong></div>
                </div>
                {#if buildPlan.needs_configuration}<p class="build-warning">Не удалось определить поддерживаемый стек. На Android/Termux выполняются только поддерживаемые локальные сборщики; Dockerfile, Nixpacks и произвольные shell-команды пока отключены.</p>{/if}
                {#if buildPlan.evidence?.length}<p class="muted">Найдено: {buildPlan.evidence.join(', ')}</p>{/if}
                {#if buildPlan.install_command || buildPlan.build_command || buildPlan.start_command}
                    <div class="build-commands">
                        {#if buildPlan.install_command}<div><span>Install</span><code>{buildPlan.install_command}</code></div>{/if}
                        {#if buildPlan.build_command}<div><span>Build</span><code>{buildPlan.build_command}</code></div>{/if}
                        {#if buildPlan.start_command}<div><span>Start</span><code>{buildPlan.start_command}</code></div>{/if}
                    </div>
                {/if}
                {#if buildPlan.warnings?.length}{#each buildPlan.warnings as warning}<p class="muted">{warning}</p>{/each}{/if}
                <div class="android-build-panel">
                    <h3>Android Local Build</h3>
                    <p class="muted">Сборка выполняется непосредственно на Android/Termux. Docker, Podman и Nixpacks не используются.</p>
                    {#if buildPlan.android_environment}
                        <div class="build-summary">
                            <div><span>Environment</span><strong>{buildPlan.android_environment.termux ? 'Termux' : buildPlan.android_environment.android ? 'Android' : 'Host'}</strong></div>
                            <div><span>Architecture</span><strong>{buildPlan.android_environment.architecture}</strong></div>
                            <div><span>CPU cores</span><strong>{buildPlan.android_environment.cpus}</strong></div>
                            <div><span>Available RAM</span><strong>{buildPlan.android_environment.available_memory_bytes ? Math.round(buildPlan.android_environment.available_memory_bytes / 1048576) + ' MB' : 'Unknown'}</strong></div>
                        </div>
                        {#if buildPlan.android_environment.available_tools?.length}
                            <p class="muted">Available tools: {buildPlan.android_environment.available_tools.join(', ')}</p>
                        {/if}
                    {/if}
                    {#if (['nodejs','python','go','rust','php','static','java','jvm','ruby','dotnet'].includes(buildPlan.stack) || (buildPlan.provider === 'veltrix' && (buildPlan.install_command || buildPlan.build_command))) && !buildPlan.needs_configuration}
                        <Button variant="primary" disabled={!online || buildRunning} loading={buildRunning} on:click={runAndroidBuild}>Build on this Android device</Button>
                    {:else}
                        <p class="build-warning">Этот тип проекта пока не поддерживается локальным Android provider. Сборка не будет запускаться.</p>
                    {/if}
                    <div class="build-history">
                        <div class="metrics-card-head"><h3>История сборок</h3><Button size="sm" on:click={loadBuildHistory} disabled={!online || buildHistoryLoading}>{buildHistoryLoading?'Загрузка…':'Обновить'}</Button></div>
                        {#if buildHistoryLoading}<p class="muted">Загрузка истории сборок…</p>
                        {:else if buildHistory.length}
                            {#each buildHistory as record (record.id)}
                                <button class="build-history-row" on:click={() => viewBuildHistory(record)}>
                                    <span><strong>{record.id}</strong><small>{record.started_at ? new Date(record.started_at).toLocaleString() : record.created_at ? new Date(record.created_at).toLocaleString() : '—'}</small></span>
                                    <em class:build-success-text={record.result?.success} class:build-failure-text={record.result?.success===false}>{record.result?.success===true?'Успешно':record.result?.success===false?'Ошибка':record.status||'Запись'}</em>
                                </button>
                            {/each}
                        {:else}<p class="muted">История сборок пока пуста.</p>{/if}
                    </div>
                    {#if buildResult}
                        <div class:build-success={buildResult.success} class:build-failure={!buildResult.success} class="build-result">
                            <strong>{buildResult.success ? 'Build completed' : 'Build failed'}</strong>
                            {#if buildResult.error}<p>{buildResult.error}</p>{/if}
                            {#if buildResult.duration_ms}<p class="muted">Duration: {(buildResult.duration_ms / 1000).toFixed(1)} s</p>{/if}
                            <pre class="terminal">{buildResult.output || '(no output)'}</pre>
                            {#if buildResult.output_truncated}<p class="muted">Логи обрезаны по установленному лимиту.</p>{/if}
                        </div>
                    {/if}
                    <p class="security-note">Ограничения памяти и диска проверяются предварительно, но без root это не жёсткие лимиты. Сборка не изолирована от Android-пользователя.</p>
                </div>
                <p class="security-note">Пока это только план анализа. Команды не запускались и сайт не изменялся.</p>
            {:else}
                <p class="muted">Запусти анализ, чтобы определить стек, сборщик, команды и наличие конфигурации. Неизвестные проекты не считаются ошибкой.</p>
            {/if}
        </Card>

    {:else if activeTab === 'deployments'}
        <Card>
            <div class="release-heading">
                <div><h2 class="details-title">Release history</h2><p class="muted">Неизменяемые версии из .yevqo/releases. Откат не запускает повторную сборку.</p></div>
                <div class="release-tools">
                    <label class="release-keep">Оставить <input type="number" min="1" max="50" bind:value={releaseKeep} disabled={!online || pruningReleases} /> релизов</label>
                    <Button size="sm" variant="danger" on:click={pruneReleases} disabled={!online || pruningReleases || releases.length <= Number(releaseKeep)} loading={pruningReleases}>Очистить старые</Button>
                    <Button size="sm" on:click={loadReleases} disabled={!online || releasesLoading}>{releasesLoading ? 'Загрузка…' : 'Обновить'}</Button>
                </div>
            </div>
            {#if !online}
                <p class="muted">Подключите Agent, чтобы посмотреть историю релизов.</p>
            {:else if releasesLoading}
                <p class="muted">Загрузка истории…</p>
            {:else if releases.length === 0}
                <div class="empty-state">Релизов пока нет. Выполните первый Deploy из вкладки Git.</div>
            {:else}
                <div class="release-list">
                    {#each releases as release (release.commit)}
                        <article class="release-row">
                            <div class="release-info">
                                <div class="release-title">
                                    <code>{release.commit.slice(0, 12)}</code>
                                    {#if release.active}<span class="release-active">Активен</span>{/if}
                                </div>
                                <p class="muted">{release.date ? new Date(release.date).toLocaleString() : 'Дата коммита недоступна'}</p>
                            </div>
                            <Button size="sm" disabled={release.active || !!busyAction} loading={busyAction === 'rollback' && rollbackTarget === release.commit} on:click={() => rollbackRelease(release.commit)}>{release.active ? 'Текущий' : 'Откатить'}</Button>
                        </article>
                    {/each}
                </div>
            {/if}
        </Card>
    {:else if activeTab === 'git'}
        <Card>
            <h2 class="details-title">GitHub connection</h2>
            <p class="muted">Подключение GitHub App, выбор репозитория и основной ветки для этого сайта.</p>
            {#if gitHubIntegrations.length}
                <div class="git-form">
                    <label>GitHub connection
                        <select class="input" bind:value={selectedGitHubIntegration} on:change={() => { selectedGitHubRepository = ''; gitHubRepositories = []; loadGitHubRepositories(); }}>
                            <option value="">Select connection</option>
                            {#each gitHubIntegrations as integration (integration.id)}
                                <option value={String(integration.id)}>{integration.account_name} · GitHub App</option>
                            {/each}
                        </select>
                    </label>
                    <label>GitHub repository
                        <select class="input" value={selectedGitHubRepository} on:change={(event) => selectGitHubRepository(event.currentTarget.value)} disabled={gitHubRepositoriesLoading || !gitHubRepositories.length}>
                            <option value="">{gitHubRepositoriesLoading ? 'Loading repositories…' : 'Select repository'}</option>
                            {#each gitHubRepositories as repository (repository.id)}
                                <option value={repository.full_name}>{repository.full_name}{repository.private ? ' · Private' : ''}</option>
                            {/each}
                        </select>
                    </label>
                    <label>Branch
                        <input class="input" bind:value={gitForm.branch} placeholder="main" maxlength="128" />
                    </label>
                </div>
                {#if gitHubRepositoriesError}<p class="muted">{gitHubRepositoriesError}</p>{/if}
            {:else}
                <p class="muted">GitHub App не подключён. Подключите приложение в Settings → Integrations, затем обновите страницу.</p>
            {/if}
            <div class="save-row"><Button variant="primary" loading={busyAction === 'git'} disabled={!selectedGitHubIntegration || !selectedGitHubRepository} on:click={saveGit}>Save Git connection</Button></div>
        </Card>
    {:else if activeTab === 'settings'}
        <Card>
            <h2 class="details-title">Настройки сборки и запуска</h2>
            <p class="muted">Параметры применяются к сборке и развёртыванию выбранного Git-репозитория.</p>
            <div class="git-form">
                <label class="wide">Build Command<input class="input" bind:value={gitForm.build_command} placeholder="npm run build или composer install" /></label>
                <label class="wide">Root Directory<input class="input" bind:value={gitForm.root_directory} placeholder="." /></label>
                <label class="wide">Custom Start Command (optional)<input class="input" bind:value={gitForm.start_command} placeholder="npm run start · node server.js · ./bin/server" /><small>Запускается напрямую без shell. Поддерживаются allowlisted tools или исполняемый файл внутри каталога сайта; shell-операторы запрещены.</small></label>
                <label class="wide">HTTP Health Check Path (optional)<input class="input" bind:value={gitForm.health_check_path} placeholder="/health" /><small>После запуска релиза Agent ждёт HTTP 2xx на локальном адресе. Пустое поле использует TCP readiness.</small></label>
                <label>Clone Timeout (seconds)<input class="input" type="number" min="30" max="1800" step="30" bind:value={gitForm.clone_timeout_seconds} /><small>30–1800 сек. По умолчанию 600.</small></label>
                <label>Build Timeout (seconds)<input class="input" type="number" min="60" max="7200" step="60" bind:value={gitForm.build_timeout_seconds} /><small>60–7200 сек. По умолчанию 1200.</small></label>
                <label class="wide">HTTP Health Check Timeout (seconds)<input class="input" type="number" min="3" max="300" step="1" bind:value={gitForm.health_check_timeout_seconds} /><small>3–300 сек. По умолчанию 15. Применяется только при заданном Health Check Path.</small></label>
                <label class="wide">GitHub Token<input class="input" type="password" bind:value={gitForm.github_token} autocomplete="new-password" placeholder={git.has_token ? 'Токен сохранён — пустое поле оставит его без изменений' : 'Только для приватного репозитория'} />{#if git.has_token}<small>Токен сохранён с AES-GCM шифрованием.</small>{/if}</label>
            </div>
            <div class="save-row"><Button variant="primary" loading={busyAction === 'git'} on:click={saveGit}>Сохранить настройки</Button></div>
            <section class="webhook-panel">
                <h3>GitHub Push Deployments</h3>
                <p class="muted">При push в выбранную ветку Yevqo проверит HMAC-подпись и развернёт именно commit из события. Повторные webhook delivery игнорируются.</p>
                {#if webhookEnabled}
                    <div class="webhook-state">Webhook enabled</div>
                    <div class="webhook-url"><code>{webhookPath}</code><Button size="sm" on:click={copyWebhookURL}>Copy URL</Button></div>
                    <Button size="sm" variant="danger" loading={webhookBusy} on:click={() => configureWebhook(false)}>Disable webhook</Button>
                {:else}
                    <Button size="sm" variant="primary" disabled={!git.repository_url || webhookBusy} loading={webhookBusy} on:click={() => configureWebhook(true)}>Enable GitHub webhook</Button>
                {/if}
                {#if webhookSecret}
                    <div class="webhook-secret"><strong>Webhook secret — показывается только один раз</strong><code>{webhookSecret}</code><Button size="sm" on:click={copyWebhookSecret}>Copy secret</Button><small>В GitHub откройте Settings → Webhooks → Add webhook. Content type: application/json; Events: Just the push event.</small></div>
                {/if}
            </section>
        </Card>
    {:else if activeTab === 'previews'}
        <Card>
            <h2 class="details-title">Android Preview Deployments</h2>
            <p class="muted">Изолированная копия выбранной Git-ветки на Android/Termux. Preview использует отдельную директорию, процесс и порт и не переключает production-релиз. Срок жизни — 24 часа.</p>
            {#if String(device?.platform || '').toLowerCase() !== 'android'}
                <p class="muted">Preview доступен только на Android/Termux Agent.</p>
            {:else if !online}
                <p class="muted">Подключите Android Agent для управления Preview.</p>
            {:else}
                <div class="git-form">
                    <label>Git branch<input class="input" bind:value={previewBranch} placeholder={gitForm.branch || 'main'} maxlength="128" /></label>
                </div>
                <div class="save-row"><Button variant="primary" disabled={!git.repository_url || previewCreating} loading={previewCreating} on:click={createPreview}>Create Preview</Button><Button size="sm" disabled={previewsLoading} on:click={loadPreviews}>Refresh</Button></div>
                {#if previewsLoading}
                    <p class="muted">Загрузка Preview…</p>
                {:else if previews.length === 0}
                    <div class="empty-state">Preview пока нет.</div>
                {:else}
                    <div class="release-list">
                        {#each previews as preview (preview.key)}
                            <article class="release-row">
                                <div class="release-info">
                                    <div class="release-title"><code>{preview.branch || 'main'}</code><span class="release-active">{preview.status}</span></div>
                                    <p class="muted">Port {preview.port} · {preview.commit ? preview.commit.slice(0,12) : 'commit pending'} · истекает {preview.expires_at ? new Date(preview.expires_at).toLocaleString() : 'через 24 часа'}</p>
                                    {#if preview.error}<p class="muted">{preview.error}</p>{/if}
                                    {#if preview.url}<a href={preview.url} target="_blank" rel="noreferrer">{preview.url} ↗</a>{/if}
                                </div>
                                <Button size="sm" variant="danger" disabled={!!deletingPreview} loading={deletingPreview === preview.key} on:click={() => removePreview(preview)}>Delete</Button>
                            </article>
                        {/each}
                    </div>
                {/if}
            {/if}
        </Card>
    {/if}
{:else}
    <div class="empty">Сайт не найден</div>
{/if}

<Modal open={showTerminal} title="Open Terminal · Logs" onClose={() => showTerminal = false}>
    {#if terminalLoading}
        <p class="terminal-loading">Загрузка последних 100 строк...</p>
    {:else}
        <pre class="terminal">{terminalText}</pre>
    {/if}
    <svelte:fragment slot="footer">
        <Button on:click={openTerminal} disabled={terminalLoading}>Обновить</Button>
        <Button on:click={() => showTerminal = false}>Закрыть</Button>
    </svelte:fragment>
</Modal>

<style>
.task-create{display:grid;gap:8px;margin:14px 0;padding:12px;border:1px solid var(--border);border-radius:9px;background:var(--bg-base)}.task-command{width:100%;font:11px/1.5 ui-monospace,monospace;resize:vertical}.task-create-actions{display:flex;gap:8px}.task-create-actions .input{flex:1}.task-list{display:grid;gap:8px}.task-item{display:grid;gap:7px;padding:12px;border:1px solid var(--border);border-radius:9px;background:var(--bg-base)}.task-item-head{display:flex;align-items:center;justify-content:space-between;gap:8px}.task-item-head>div:first-child{display:flex;align-items:center;gap:9px}.task-item-head strong{font-size:11px}.task-state{font-size:9px;color:var(--success)}.task-state.task-failed,.task-failed{color:var(--danger)}.task-buttons{display:flex;gap:6px}.task-item code{font-size:10px;color:var(--text-main);overflow-wrap:anywhere}.task-item small{font-size:9px;color:var(--text-muted)}.task-output-toggle{justify-self:start;background:transparent;border:0;color:#8AB4FF;font-size:10px}.task-output{max-height:220px;overflow:auto;padding:10px;background:var(--bg-base);border-radius:7px;color:var(--text-muted);font-size:10px;white-space:pre-wrap;overflow-wrap:anywhere}@media(max-width:560px){.task-create-actions{flex-direction:column}.task-item-head{align-items:flex-start;flex-direction:column}}
.build-history{margin-top:18px;border-top:1px solid var(--border);padding-top:14px}.build-history-row{width:100%;display:flex;align-items:center;justify-content:space-between;gap:12px;padding:10px 0;border:0;border-bottom:1px solid var(--border);background:transparent;color:var(--text-main);text-align:left}.build-history-row span{display:grid;gap:4px}.build-history-row strong{font:10px var(--mono,monospace)}.build-history-row small,.build-history-row em{font-size:9px;color:var(--text-muted);font-style:normal}.build-history-row .build-success-text{color:var(--success)}.build-history-row .build-failure-text{color:var(--danger)}
.metrics-card-head{display:flex;justify-content:space-between;align-items:center;gap:8px}.process-metrics{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin:12px 0}.process-metrics>div{display:grid;gap:5px;padding:10px;border:1px solid var(--border);border-radius:8px;background:var(--bg-base)}.process-metrics span{font-size:9px;color:var(--text-muted)}.process-metrics strong{font-size:15px;color:var(--text-main)}@media(max-width:560px){.process-metrics{grid-template-columns:1fr}}
.webhook-panel{display:grid;gap:9px;margin-top:18px;padding:14px;border:1px solid var(--border);border-radius:10px;background:var(--bg-base)}.webhook-panel h3{font-size:13px}.webhook-state{color:var(--success);font-size:11px}.webhook-url{display:flex;align-items:center;justify-content:space-between;gap:8px}.webhook-url code,.webhook-secret code{overflow-wrap:anywhere;color:var(--accent);font-size:11px}.webhook-secret{display:grid;gap:8px;padding:10px;border:1px solid var(--warning);border-radius:8px}.webhook-secret small{color:var(--text-muted);font-size:10px;line-height:1.5}
.env-header,.env-list-head,.env-import-actions{display:flex;align-items:center;justify-content:space-between;gap:12px}
.env-count{font-size:11px;color:var(--text-muted);border:1px solid var(--border);border-radius:6px;padding:5px 8px;white-space:nowrap}
.env-notice{display:grid;gap:5px;padding:12px 14px;margin:14px 0;border:1px solid var(--border);border-radius:8px;background:var(--bg-card)}
.env-notice strong,.env-add-panel h3,.env-import-panel h3,.env-list-head h3,.env-managed h3{font-size:12px;color:var(--text-main);margin:0}
.env-notice span{font-size:11px;color:var(--text-muted);line-height:1.5}
.env-add-panel,.env-import-panel{display:grid;gap:10px;padding:14px;margin:12px 0;border:1px solid var(--border);border-radius:9px;background:var(--bg-base)}
.env-add-panel .muted,.env-import-panel .muted{margin:0}
.env-add{display:grid;grid-template-columns:1fr 1.4fr auto;gap:8px}
.env-import-input{width:100%;box-sizing:border-box;resize:vertical;min-height:110px;background:var(--bg-base);color:var(--text-main);border:1px solid var(--border);border-radius:7px;padding:11px;font:11px/1.6 ui-monospace,SFMono-Regular,monospace}
.env-import-actions{flex-wrap:wrap}
.env-list-head{margin:20px 0 9px}.env-search{max-width:230px}
.env-list{display:grid;gap:6px}.env-list .env-row{display:flex;align-items:center;gap:10px;padding:9px 10px;border:1px solid var(--border);border-radius:8px;background:var(--bg-base)}
.env-variable-name{display:grid;gap:3px;flex:1;min-width:120px}.env-variable-name code{color:var(--accent);font-size:11px}.env-variable-name small{color:var(--text-dim);font-size:9px}
.env-masked{color:var(--text-muted);font-size:11px;letter-spacing:1px}
.env-empty{display:grid;gap:5px;text-align:center;padding:28px 12px;border:1px dashed var(--border);border-radius:8px;color:var(--text-muted);font-size:11px}.env-empty strong{color:var(--text-main);font-size:12px}
.env-security-note{font-size:10px;line-height:1.6;color:var(--text-dim);margin-top:14px}
.env-managed{border-top:1px solid var(--border);margin-top:20px;padding-top:16px;display:grid;gap:8px}
.env-group{display:grid;gap:6px;margin:8px 0;padding:12px;border:1px solid var(--border);border-radius:8px}.env-group>strong{font-size:11px;color:var(--text-main)}
@media(max-width:700px){.env-add{grid-template-columns:1fr}.env-list .env-row{flex-wrap:wrap}.env-search{max-width:100%;width:100%}.env-list-head{align-items:stretch;flex-direction:column}}
.env-add{display:grid;grid-template-columns:1fr 1.4fr auto;gap:8px}.env-list{display:grid;gap:6px}.env-list .env-row{display:flex;align-items:center;gap:10px;padding:9px 10px;border:1px solid var(--border);border-radius:8px;background:var(--bg-base)}.env-list .env-row code{flex:1;color:var(--accent)}.env-list .env-row span{color:var(--text-muted);font-size:10px}@media(max-width:700px){.env-add{grid-template-columns:1fr}.env-list .env-row{flex-wrap:wrap}}
    .deployment-job{display:flex;align-items:center;gap:16px;justify-content:space-between}.deployment-job p{margin:4px 0 0}.deployment-progress{height:6px;background:#27272A;border-radius:999px;overflow:hidden;margin-top:12px}.deployment-progress div{height:100%;background:var(--accent);transition:width .2s ease}
    .field-label{display:block;margin:12px 0 6px;color:var(--text-muted);font-size:11px}.tabs{display:flex;gap:6px;border-bottom:1px solid #27272A;margin:24px 0 18px;overflow:auto}
    .tabs button{background:transparent;border:0;border-bottom:2px solid transparent;color:var(--text-muted);padding:11px 14px;cursor:pointer;white-space:nowrap;font-size:13px}
    .tabs button.active{color:var(--text-main);border-bottom-color:var(--accent)}
    .detail-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(250px,1fr));gap:14px}
    .commit{font:18px ui-monospace,monospace;color:var(--accent)}.muted{font-size:12px;color:var(--text-muted);line-height:1.6}
    .logs-head{display:flex;justify-content:space-between;align-items:center}
    .log-connection{display:block;margin-top:-10px;margin-bottom:12px;font-size:10px;color:var(--text-muted)}
    .log-connection.live{color:var(--success)}
    .log-actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.log-filters{display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin:10px 0}.log-search{flex:1;min-width:160px;box-sizing:border-box;background:var(--bg-base);border:1px solid #27272A;border-radius:7px;padding:9px 12px;color:var(--text-main);font-size:12px}.log-level{background:var(--bg-base);border:1px solid #27272A;border-radius:7px;padding:9px;color:var(--text-main);font-size:12px}.autoscroll{display:flex;gap:6px;align-items:center;color:var(--text-muted);font-size:11px;white-space:nowrap}.autoscroll input{accent-color:var(--accent)}
    .terminal{background:var(--bg-base);border:1px solid #27272A;border-radius:8px;color:var(--text-main);padding:14px;max-height:60vh;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font:12px/1.6 ui-monospace,monospace}
    .env-group{border:1px solid #27272A;border-radius:8px;padding:14px;margin-top:12px}
    .env-group strong{display:block;color:var(--text-main);font-size:13px;margin-bottom:10px}
    .env-row{display:flex;justify-content:space-between;gap:14px;padding:7px 0;border-bottom:1px solid #27272A;font-size:12px;color:var(--text-main)}
    .android-build-panel{display:grid;gap:12px;margin-top:20px;padding-top:18px;border-top:1px solid #27272A}
    .android-build-panel h3{font-size:14px;color:var(--text-main);margin:0}
    .build-result{border:1px solid #3F3F46;border-radius:8px;padding:12px}
    .build-success{border-color:var(--border-hover)}.build-failure{border-color:var(--warning)}
    .build-summary{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px;margin:16px 0}
    .build-summary div,.build-commands div{display:flex;flex-direction:column;gap:6px;background:var(--bg-base);border:1px solid #27272A;border-radius:8px;padding:12px;min-width:0}
    .build-summary span,.build-commands span{font-size:11px;color:var(--text-muted)}
    .build-summary strong{font-size:13px;color:var(--text-main);overflow-wrap:anywhere}
    .build-commands{display:grid;gap:8px;margin-top:14px}
    .build-commands code{font-size:12px;color:var(--text-main);overflow-wrap:anywhere}
    .build-warning{padding:12px;border:1px solid var(--warning);border-radius:8px;color:var(--warning);font-size:12px;line-height:1.5}
    .release-tools{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.release-keep{display:flex;align-items:center;gap:6px;color:var(--text-muted);font-size:10px}.release-keep input{width:54px;padding:6px 7px;border:1px solid var(--border);border-radius:6px;background:var(--bg-card);color:var(--text-main);font:inherit}.release-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:14px}
    .release-list{display:grid;gap:10px;margin-top:16px}
    .release-row{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:14px;border:1px solid #27272A;border-radius:10px;background:var(--bg-card)}
    .release-title{display:flex;align-items:center;gap:10px;font-weight:600}
    .release-title code{font-size:13px;color:var(--text-main)}
    .release-active{font-size:11px;padding:3px 7px;border-radius:999px;background:var(--accent-soft);color:var(--accent)}
    .release-info p{margin:6px 0 0}
    .git-form{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin-top:18px}
    .git-form label{display:grid;gap:7px;color:var(--text-main);font-size:12px}.git-form .wide{grid-column:1/-1}
    .input{width:100%;box-sizing:border-box;background:var(--bg-base);border:1px solid #3F3F46;border-radius:7px;padding:10px 12px;color:var(--text-main);font-size:13px}
    .git-form small{color:var(--text-muted)}.security-note{margin-top:14px;color:var(--warning);font-size:11px}
    .save-row{display:flex;justify-content:flex-end;margin-top:18px}
    @media(max-width:600px){.release-heading,.release-row{align-items:flex-start;flex-direction:column}.git-form{grid-template-columns:1fr}.git-form .wide{grid-column:auto}}
    .back { background: transparent; border: 0; color: #8AB4FF; cursor: pointer; padding: 0; margin-bottom: 12px; font-size: 13px; }
    .device-state,.site-state { display:flex; align-items:center; gap:8px; color:var(--text-muted); font-size:12px; }
    .panel-head { display:flex; justify-content:space-between; align-items:flex-start; gap:16px; }
    h2 { margin:0; color:var(--text-main); font-size:15px; font-weight:600; }
    .panel-head p { color:var(--text-muted); font-size:12px; margin:6px 0 0; }
    .actions { display:flex; flex-wrap:wrap; gap:8px; margin-top:22px; }
    .offline-note { color:var(--danger); font-size:12px; margin:16px 0 0; }
    .details-title { margin-bottom:16px; }
    .details { display:grid; grid-template-columns:repeat(auto-fit,minmax(160px,1fr)); gap:16px; }
    .details div { display:flex; flex-direction:column; gap:5px; }
    .details span { color:var(--text-muted); font-size:11px; }
    .details strong { color:var(--text-main); font-size:13px; font-weight:500; overflow-wrap:anywhere; }
    .terminal { background:var(--bg-base); border:1px solid #27272A; border-radius:8px; color:var(--text-muted); padding:14px; max-height:55vh; overflow:auto; white-space:pre-wrap; overflow-wrap:anywhere; font:11px/1.6 ui-monospace,monospace; }
    .terminal-loading { color:var(--text-muted); font-size:13px; }
    @media(max-width:600px) { .panel-head { flex-direction:column; } .device-state { margin-top:8px; } }
</style>