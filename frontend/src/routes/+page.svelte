<script>
    import { user, devices, notify } from '$lib/stores.js';
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { listDevices } from '$lib/api.js';
    import { devices, notify } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';
    import StatusDot from '$lib/components/StatusDot.svelte';
    import Modal from '$lib/components/Modal.svelte';

    let loading = true;
    let showAddModal = false;
    let deviceLimit = 1;

    // Загружаем устройства при открытии страницы.
    onMount(async () => {
        const res = await listDevices();
        if (res.ok) {
            devices.set(res.data.devices || []);
            deviceLimit = res.data.limit || 1;
        } else {
            notify(res.error || 'Failed to load devices', 'error');
        }
        loading = false;
    });

    function openDevice(device) {
        goto(`/devices/${device.id}`);
    }

    function openAddModal() {
        showAddModal = true;
    }

    function closeAddModal() {
        showAddModal = false;
    }
</script>

<div class="page-header">
    <div>
        <h1 class="page-title">Устройства</h1>
        <p class="page-desc">Телефоны, подключённые к панели</p>
    </div>
    <Button variant="primary" on:click={openAddModal}>
        Добавить устройство
    </Button>
</div>

{#if loading}
    <div class="empty">Загрузка...</div>
{:else if $devices.length === 0}
    <div class="empty">
        <p>Пока нет подключённых устройств</p>
        <p class="empty-hint">Установите агент на телефон и введите токен привязки</p>
    </div>
{:else}
    <div class="grid">
        {#each $devices as device (device.id)}
            <Card clickable on:click={() => openDevice(device)}>
                <div class="device-card-header">
                    <div class="device-name">{device.hostname || device.device_id.slice(0, 8)}</div>
                    <StatusDot status={device.status} />
                </div>

                <div class="device-meta">
                    <div class="meta-row">
                        <span class="meta-label">Статус</span>
                        <span class="meta-value">
                            {device.status === 'online' ? 'Онлайн' : 'Офлайн'}
                        </span>
                    </div>
                    <div class="meta-row">
                        <span class="meta-label">IP (Tailscale)</span>
                        <span class="meta-value mono">{device.tailscale_ip || '—'}</span>
                    </div>
                    <div class="meta-row">
                        <span class="meta-label">Сайтов</span>
                        <span class="meta-value">{device.site_count || 0}</span>
                    </div>
                </div>
            </Card>
        {/each}
    </div>
{/if}

<div class="limit-info">
    Устройств: {$devices.length} из {deviceLimit}
</div>

<!-- Модалка «Добавить устройство» -->
<Modal open={showAddModal} title="Добавить устройство" onClose={closeAddModal}>
    <p class="modal-text">
        Чтобы подключить новое устройство:
    </p>
    <ol class="modal-steps">
        <li>Установите Termux на телефон</li>
        <li>Запустите: <code>bash scripts/install-agent.sh</code></li>
        <li>Введите URL сервера и токен привязки</li>
        <li>Запустите: <code>sv start veltrix-agent</code></li>
    </ol>
    <p class="modal-text">
        Ваш токен привязки: <code class="token">{$user?.referral_code || '—'}</code>
    </p>

    <svelte:fragment slot="footer">
        <Button on:click={closeAddModal}>Закрыть</Button>
    </svelte:fragment>
</Modal>

<style>
    .device-card-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 16px;
    }

    .device-name {
        font-weight: 500;
        font-size: 14px;
        color: #fafafa;
    }

    .device-meta {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .meta-row {
        display: flex;
        justify-content: space-between;
        font-size: 12px;
    }

    .meta-label {
        color: #71717a;
    }

    .meta-value {
        color: #a1a1aa;
    }

    .meta-value.mono {
        font-family: ui-monospace, monospace;
        font-size: 11px;
    }

    .empty-hint {
        font-size: 12px;
        color: #52525b;
        margin-top: 8px;
    }

    .limit-info {
        margin-top: 24px;
        font-size: 12px;
        color: #71717a;
        text-align: right;
    }

    .modal-text {
        font-size: 13px;
        color: #a1a1aa;
        margin-bottom: 12px;
    }

    .modal-steps {
        font-size: 13px;
        color: #a1a1aa;
        padding-left: 20px;
        margin-bottom: 16px;
    }

    .modal-steps li {
        margin-bottom: 6px;
    }

    .modal-steps code,
    .token {
        background: #18181b;
        padding: 2px 6px;
        border-radius: 4px;
        font-family: ui-monospace, monospace;
        font-size: 12px;
        color: #fafafa;
    }

    .token {
        display: inline-block;
        padding: 4px 8px;
    }
</style>
