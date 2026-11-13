<script>
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { getDevice, getSite, sendCommand, runSiteCommand, listLogs } from '$lib/api.js';
    import { notify } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';
    import StatusDot from '$lib/components/StatusDot.svelte';
    import Modal from '$lib/components/Modal.svelte';

    let deviceId = '';
    let siteId = '';
    let device = null;
    let site = null;
    let online = false;
    let loading = true;
    let busyAction = '';
    let showTerminal = false;
    let terminalLoading = false;
    let terminalText = '';

    onMount(async () => {
        deviceId = $page.params.device_id;
        siteId = $page.params.site_id;
        await refresh();
        loading = false;
    });

    async function refresh() {
        const [deviceRes, siteRes] = await Promise.all([getDevice(deviceId), getSite(deviceId, siteId)]);
        if (!deviceRes.ok || !siteRes.ok) {
            notify(deviceRes.error || siteRes.error || 'Не удалось загрузить сайт', 'error');
            return;
        }
        device = deviceRes.data.device;
        online = !!deviceRes.data.online;
        site = siteRes.data.site;
    }

    async function runAction(action) {
        if (!online || busyAction) return;
        busyAction = action;
        let res;
        if (action === 'start') {
            res = await sendCommand(deviceId, 'start_site', {
                site_id: site.site_id, path: site.path, port: site.port, runtime: site.runtime
            });
        } else {
            res = await runSiteCommand(deviceId, siteId, action);
        }
        busyAction = '';
        if (!res.ok || !res.data?.ok) {
            notify(res.error || res.data?.message || 'Команда завершилась ошибкой', 'error');
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
        showTerminal = true;
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
            const matching = rows.filter(row => row.details?.site_id === siteId && String(row.device_id) === String(deviceId));
            terminalText = matching.map(row => {
                const details = typeof row.details === 'string' ? row.details : JSON.stringify(row.details || {});
                return '[' + row.created_at + '] ' + row.action + ' ' + details;
            }).join('\n') || 'Устройство офлайн. В базе нет сохранённых записей для этого сайта.';
        }
        terminalLoading = false;
    }
</script>

{#if loading}
    <div class="empty">Загрузка сайта...</div>
{:else if site}
    <div class="page-header">
        <div>
            <button class="back" on:click={() => goto('/sites')}>← Все сайты</button>
            <h1 class="page-title">{site.site_id}</h1>
            <p class="page-desc">{site.path} · порт {site.port} · {site.runtime}</p>
        </div>
        <div class="device-state">
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

    <Card>
        <h2 class="details-title">Site information</h2>
        <div class="details">
            <div><span>Site ID</span><strong>{site.site_id}</strong></div>
            <div><span>Runtime</span><strong>{site.runtime}</strong></div>
            <div><span>Port</span><strong>{site.port}</strong></div>
            <div><span>Directory</span><strong>{site.path}</strong></div>
            <div><span>Device</span><strong>{device?.hostname || deviceId}</strong></div>
        </div>
    </Card>
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
    .back { background: transparent; border: 0; color: #3B82F6; cursor: pointer; padding: 0; margin-bottom: 12px; font-size: 13px; }
    .device-state,.site-state { display:flex; align-items:center; gap:8px; color:#A1A1AA; font-size:12px; }
    .panel-head { display:flex; justify-content:space-between; align-items:flex-start; gap:16px; }
    h2 { margin:0; color:#FAFAFA; font-size:15px; font-weight:600; }
    .panel-head p { color:#A1A1AA; font-size:12px; margin:6px 0 0; }
    .actions { display:flex; flex-wrap:wrap; gap:8px; margin-top:22px; }
    .offline-note { color:#EF4444; font-size:12px; margin:16px 0 0; }
    .details-title { margin-bottom:16px; }
    .details { display:grid; grid-template-columns:repeat(auto-fit,minmax(160px,1fr)); gap:16px; }
    .details div { display:flex; flex-direction:column; gap:5px; }
    .details span { color:#A1A1AA; font-size:11px; }
    .details strong { color:#FAFAFA; font-size:13px; font-weight:500; overflow-wrap:anywhere; }
    .terminal { background:#09090B; border:1px solid #27272A; border-radius:8px; color:#A1A1AA; padding:14px; max-height:55vh; overflow:auto; white-space:pre-wrap; overflow-wrap:anywhere; font:11px/1.6 ui-monospace,monospace; }
    .terminal-loading { color:#A1A1AA; font-size:13px; }
    @media(max-width:600px) { .panel-head { flex-direction:column; } .device-state { margin-top:8px; } }
</style>
