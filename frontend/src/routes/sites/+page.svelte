<script>
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { listSites, sendCommand } from '$lib/api.js';
    import { notify } from '$lib/stores.js';
    import { locale } from '$lib/i18n.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';
    import StatusDot from '$lib/components/StatusDot.svelte';

    let loading = true;
    let refreshing = false;
    let busySite = '';
    let busyBulk = false;
    let selectedSites = [];
    let sites = [];
    $: ru = $locale !== 'en';

    // Загружаем сайты повторно по запросу пользователя.
    async function loadSites() {
        refreshing = true;
        const res = await listSites($page.url.searchParams.get('project_id') || '');
        if (res.ok) {
            sites = res.data?.sites || [];
        } else {
            notify(res.error || (ru ? 'Не удалось загрузить сайты' : 'Could not load applications'), 'error');
        }
        loading = false;
        refreshing = false;
    }

    onMount(loadSites);

    // Открываем страницу конкретного сайта.
    function openSite(item) {
        goto(`/sites/${item.device_id}/${encodeURIComponent(item.site.site_id)}`+($page.url.searchParams.get('project_id')?'?project_id='+$page.url.searchParams.get('project_id'):''));
    }

    function openDevice(deviceId) {
        goto(`/devices/${deviceId}`);
    }

    function siteKey(item) {
        return item.device_id + ':' + item.site.site_id;
    }

    $: selectedRunning = sites.filter(item => selectedSites.includes(siteKey(item)) && item.site.status === 'running');
    $: allSelected = sites.length > 0 && selectedSites.length === sites.length;

    function toggleSelected(item) {
        const key = siteKey(item);
        selectedSites = selectedSites.includes(key)
            ? selectedSites.filter(value => value !== key)
            : [...selectedSites, key];
    }

    function toggleAll() {
        selectedSites = allSelected ? [] : sites.map(siteKey);
    }

    async function stopSelected() {
        if (busyBulk || selectedRunning.length === 0) return;
        busyBulk = true;
        const total = selectedRunning.length;
        let failed = 0;
        for (const item of selectedRunning) {
            const result = await sendCommand(item.device_id, 'stop_site', { site_id: item.site.site_id });
            if (!result.ok || !result.data?.ok) failed++;
        }
        busyBulk = false;
        selectedSites = [];
        await loadSites();
        notify(failed ? (ru ? `Остановлено: ${total - failed}, ошибок: ${failed}` : `Stopped: ${total - failed}, errors: ${failed}`) : (ru ? `Остановлено сайтов: ${total}` : `Stopped applications: ${total}`), failed ? 'error' : 'success');
    }

    async function runSiteAction(item, action) {
        const site = item.site;
        const key = siteKey(item);
        if (busySite) return;
        busySite = key;
        const payload = action === 'start_site'
            ? { site_id: site.site_id, path: site.path, port: site.port, runtime: site.runtime }
            : { site_id: site.site_id };
        const result = await sendCommand(item.device_id, action, payload);
        if (!result.ok || !result.data?.ok) {
            notify(result.error || result.data?.message || (ru ? 'Не удалось выполнить команду' : 'Could not run application command'), 'error');
        } else {
            notify(action === 'start_site' ? (ru ? 'Сайт запущен' : 'Application started') : (ru ? 'Сайт остановлен' : 'Application stopped'), 'success');
            await loadSites();
        }
        busySite = '';
    }
</script>

<div class="page-header">
    <div>
        <h1 class="page-title">{ru ? 'Сайты' : 'Applications'}</h1>
        <p class="page-desc">{ru ? 'Все сайты на всех подключённых устройствах' : 'Applications across all connected devices'}</p>
    </div>
    <div class="header-actions">
        {#if sites.length > 0}
            <Button size="sm" on:click={toggleAll}>{allSelected ? (ru ? 'Снять выбор' : 'Clear selection') : (ru ? 'Выбрать все' : 'Select all')}</Button>
            <Button size="sm" variant="danger" disabled={selectedRunning.length === 0 || busyBulk} loading={busyBulk} on:click={stopSelected}>{ru ? 'Остановить выбранные' : 'Stop selected'} ({selectedRunning.length})</Button>
        {/if}
        <Button on:click={loadSites} disabled={refreshing || busyBulk} loading={refreshing}>{ru ? '↻ Обновить' : '↻ Refresh'}</Button>
    </div>
</div>

{#if loading}
    <div class="empty">{ru ? 'Загрузка…' : 'Loading…'}</div>
{:else if sites.length === 0}
    <div class="empty">
        <p>{ru ? 'Пока нет сайтов' : 'No applications yet'}</p>
        <p class="empty-hint">
            {ru ? 'Перейдите на устройство и нажмите «Добавить сайт»' : 'Open a device and choose “Add application”.'}
        </p>
    </div>
{:else}
    <div class="sites-list">
        {#each sites as item (item.device_id + '-' + item.site.site_id)}
            <Card clickable on:click={() => openSite(item)}>
                <div class="site-row">
                    <label class="site-select" on:click|stopPropagation>
                        <input type="checkbox" checked={selectedSites.includes(siteKey(item))} on:change={() => toggleSelected(item)} aria-label={(ru ? 'Выбрать ' : 'Select ') + item.site.site_id}/>
                    </label>
                    <div class="site-info">
                        <div class="site-name">
                            {item.site.site_id}
                            <span class="site-device">
                                {ru ? 'на' : 'on'} {item.device_name || (ru ? `устройство ${item.device_id}` : `device ${item.device_id}`)}
                            </span>
                        </div>
                        <div class="site-meta">
                            :{item.site.port} · {item.site.runtime}
                            {#if item.site.domain}
                                · {item.site.domain}
                            {/if}
                        </div>
                    </div>

                    <div class="site-status">
                        <StatusDot status={item.site.status} />
                        <span class="status-text">
                            {item.site.status === 'running' ? (ru ? 'Работает' : 'Running') : item.site.status === 'error' ? (ru ? 'Ошибка' : 'Error') : (ru ? 'Остановлен' : 'Stopped')}
                        </span>
                    </div>
                </div>
                <div class="site-actions" on:click|stopPropagation>
                    {#if item.site.status === 'running'}
                        <Button size="sm" disabled={busySite !== ''} on:click={() => runSiteAction(item, 'stop_site')}>{ru ? 'Остановить' : 'Stop'}</Button>
                        <Button size="sm" disabled={busySite !== ''} on:click={() => runSiteAction(item, 'start_site')}>{ru ? 'Перезапустить' : 'Restart'}</Button>
                    {:else}
                        <Button size="sm" variant="primary" disabled={busySite !== ''} loading={busySite === siteKey(item)} on:click={() => runSiteAction(item, 'start_site')}>{ru ? 'Запустить' : 'Start'}</Button>
                    {/if}
                    <Button size="sm" variant="ghost" on:click={() => openDevice(item.device_id)}>{ru ? 'Устройство' : 'Device'}</Button>
                </div>
            </Card>
        {/each}
    </div>
{/if}

<style>
    .header-actions{display:flex;align-items:center;gap:7px;flex-wrap:wrap}
    .site-select{display:flex;align-items:center;flex-shrink:0;cursor:pointer}
    .site-select input{width:16px;height:16px;accent-color:var(--accent);cursor:pointer}
    .sites-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .site-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 12px;
    }

    .site-info {
        min-width: 0;
    }

    .site-name {
        font-size: 14px;
        font-weight: 500;
        color: var(--text-main);
        display: flex;
        align-items: baseline;
        gap: 8px;
        flex-wrap: wrap;
    }

    .site-device {
        font-size: 12px;
        color: var(--text-muted);
        font-weight: 400;
    }

    .site-meta {
        font-family: ui-monospace, monospace;
        font-size: 11px;
        color: var(--text-muted);
        margin-top: 4px;
    }

    .site-actions {
        display: flex;
        align-items: center;
        gap: 7px;
        flex-wrap: wrap;
        margin-top: 12px;
    }

    .site-status {
        display: flex;
        align-items: center;
        gap: 6px;
        flex-shrink: 0;
    }

    .status-text {
        font-size: 12px;
        color: var(--text-muted);
    }

    .empty-hint {
        font-size: 12px;
        color: var(--text-muted);
        margin-top: 8px;
    }
</style>
