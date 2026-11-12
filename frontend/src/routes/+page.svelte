<script>
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { listDevices, listSites } from '$lib/api.js';
    import { devices, notify, user } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';
    import StatusDot from '$lib/components/StatusDot.svelte';
    import Modal from '$lib/components/Modal.svelte';

    let loading = true;
    let showAddModal = false;
    let deviceLimit = 1;
    let allSites = [];

    onMount(async () => {
        const [deviceRes, siteRes] = await Promise.all([listDevices(), listSites()]);
        if (deviceRes.ok) {
            devices.set(deviceRes.data.devices || []);
            deviceLimit = deviceRes.data.limit || 1;
        } else notify(deviceRes.error || 'Не удалось загрузить устройства', 'error');
        if (siteRes.ok) allSites = siteRes.data.sites || [];
        loading = false;
    });

    $: onlineCount = $devices.filter(device => device.status === 'online').length;
    $: runningSites = allSites.filter(item => item.site?.status === 'running').length;
    $: offlineCount = $devices.length - onlineCount;

    function openDevice(device) { goto('/devices/' + device.id); }
</script>

<div class="dashboard-heading">
    <div><div class="eyebrow">WORKSPACE OVERVIEW</div><h1 class="page-title">Добро пожаловать, {$user?.username || 'пользователь'}</h1><p class="page-desc">Состояние инфраструктуры и быстрый доступ к ресурсам.</p></div>
    <Button variant="primary" on:click={() => showAddModal = true}>＋ Добавить устройство</Button>
</div>

{#if loading}
    <div class="empty">Загружаем инфраструктуру…</div>
{:else}
    <div class="metrics-grid">
        <div class="metric-card"><div class="metric-label">Всего устройств <span>▣</span></div><div class="metric-value">{ $devices.length }<small> / {deviceLimit}</small></div><div class="metric-foot">Подключённые Android-хосты</div></div>
        <div class="metric-card"><div class="metric-label">Устройства онлайн <span class="green">●</span></div><div class="metric-value">{onlineCount}</div><div class="metric-foot">{offlineCount} офлайн</div></div>
        <div class="metric-card"><div class="metric-label">Размещённые сайты <span>◉</span></div><div class="metric-value">{allSites.length}</div><div class="metric-foot">{runningSites} работают сейчас</div></div>
        <div class="metric-card"><div class="metric-label">Database servers <span>◈</span></div><div class="metric-value">—</div><div class="metric-foot">Управление серверами БД подключается</div></div>
    </div>

    <div class="section-header"><div><h2 class="section-title">Устройства</h2><p class="section-subtitle">Android-хосты, подключённые к VELTRIX</p></div><a href="/devices" class="view-link">Все устройства →</a></div>
    {#if $devices.length === 0}
        <div class="empty"><p>Пока нет подключённых устройств</p><p class="empty-hint">Установите VELTRIX Agent в Termux и подключите устройство.</p><div class="empty-action"><Button variant="primary" on:click={() => showAddModal = true}>Подключить устройство</Button></div></div>
    {:else}
        <div class="device-grid">
            {#each $devices as device (device.id)}
                <button class="device-card" on:click={() => openDevice(device)}>
                    <div class="device-top"><span class="device-symbol">▣</span><StatusDot status={device.status} /></div>
                    <div class="device-title">{device.hostname || device.device_id?.slice(0, 8) || 'Android device'}</div>
                    <div class="device-id">{device.device_id?.slice(0, 14) || '—'}</div>
                    <div class="device-divider"></div>
                    <div class="device-stats"><span>IP <b>{device.tailscale_ip || '—'}</b></span><span>Сайты <b>{device.site_count || 0}</b></span></div>
                </button>
            {/each}
        </div>
    {/if}

    <div class="section-header"><div><h2 class="section-title">Последние сайты</h2><p class="section-subtitle">Сайты на всех подключённых устройствах</p></div><a href="/sites" class="view-link">Все сайты →</a></div>
    {#if allSites.length === 0}
        <div class="empty compact-empty">Сайтов пока нет. Создайте первый сайт на странице устройства.</div>
    {:else}
        <div class="sites-table-wrap"><table class="sites-table"><thead><tr><th>САЙТ</th><th>УСТРОЙСТВО</th><th>RUNTIME</th><th>ПОРТ</th><th>СТАТУС</th></tr></thead><tbody>
            {#each allSites.slice(0, 6) as item (item.device_id + '-' + item.site.site_id)}
                <tr on:click={() => openDevice({id:item.device_id})}><td><strong>{item.site.site_id}</strong></td><td>{item.device_name || 'Android host'}</td><td>{item.site.runtime}</td><td class="mono">:{item.site.port}</td><td><StatusDot status={item.site.status} /></td></tr>
            {/each}
        </tbody></table></div>
    {/if}
{/if}

<Modal open={showAddModal} title="Добавить устройство" onClose={() => showAddModal = false}>
    <p class="modal-text">Чтобы подключить новое устройство:</p>
    <ol class="modal-steps"><li>Установите Termux на телефон.</li><li>Запустите <code>bash scripts/install-agent.sh</code>.</li><li>Введите URL сервера и код привязки.</li><li>Запустите <code>sv start veltrix-agent</code>.</li></ol>
    <p class="modal-text">Код привязки: <code class="token">{$user?.referral_code || '—'}</code></p>
    <svelte:fragment slot="footer"><Button on:click={() => showAddModal = false}>Закрыть</Button></svelte:fragment>
</Modal>

<style>
    .dashboard-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;margin-bottom:24px}.eyebrow{font-size:9px;font-weight:700;letter-spacing:.14em;color:#818cf8;margin-bottom:8px}.dashboard-heading .page-title{font-size:21px}.metrics-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}.metric-card{background:#101727;border:1px solid #253149;border-radius:12px;padding:15px}.metric-label{display:flex;align-items:center;justify-content:space-between;color:#94a3b8;font-size:10px}.metric-label span{font-size:13px;color:#818cf8}.metric-label .green{color:#34d399}.metric-value{font-size:25px;font-weight:650;letter-spacing:-.04em;margin-top:11px}.metric-value small{font-size:11px;color:#64748b;font-weight:500}.metric-foot{font-size:9px;color:#64748b;margin-top:4px}.section-header{margin:27px 0 12px}.section-subtitle{font-size:10px;color:#64748b;margin-top:3px}.view-link{font-size:10px;color:#a5b4fc}.device-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:10px}.device-card{text-align:left;background:#101727;border:1px solid #253149;border-radius:12px;padding:14px;color:#f1f5f9;transition:.15s}.device-card:hover{border-color:#6366f1;background:#131d31}.device-top{display:flex;justify-content:space-between;align-items:center}.device-symbol{width:28px;height:28px;border-radius:8px;display:grid;place-items:center;background:#1b2740;color:#a5b4fc}.device-title{font-size:12px;font-weight:600;margin-top:12px}.device-id{font:9px 'JetBrains Mono',monospace;color:#64748b;margin-top:3px}.device-divider{height:1px;background:#253149;margin:12px 0 9px}.device-stats{display:flex;justify-content:space-between;font-size:9px;color:#64748b}.device-stats b{color:#cbd5e1;font-weight:500;margin-left:4px}.sites-table-wrap{border:1px solid #253149;border-radius:12px;overflow:auto;background:#101727}.sites-table{width:100%;border-collapse:collapse;text-align:left;font-size:10px}.sites-table th{padding:10px 13px;color:#64748b;font-size:8px;letter-spacing:.1em;font-weight:700;border-bottom:1px solid #253149}.sites-table td{padding:11px 13px;border-bottom:1px solid #1e293b;color:#94a3b8}.sites-table tbody tr{cursor:pointer}.sites-table tbody tr:hover{background:#172136}.sites-table tbody tr:last-child td{border-bottom:0}.sites-table td strong{color:#e2e8f0}.mono{font-family:'JetBrains Mono',monospace}.compact-empty{padding:22px;font-size:10px}.empty-action{margin-top:16px}.modal-text{font-size:11px;color:#94a3b8;margin-bottom:12px}.modal-steps{padding-left:19px;color:#94a3b8;font-size:11px;margin-bottom:14px}.modal-steps li{margin:5px 0}.modal-steps code,.token{font-family:'JetBrains Mono',monospace;background:#080b14;border:1px solid #253149;border-radius:5px;padding:3px 6px;color:#c7d2fe;font-size:10px}
    @media(max-width:1000px){.metrics-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
    @media(max-width:600px){.dashboard-heading{flex-direction:column}.metrics-grid{gap:8px}.metric-card{padding:12px}.metric-value{font-size:22px}.device-grid{grid-template-columns:1fr}}
</style>