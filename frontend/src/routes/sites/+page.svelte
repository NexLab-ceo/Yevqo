<script>
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { listSites } from '$lib/api.js';
    import { notify } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';
    import StatusDot from '$lib/components/StatusDot.svelte';

    let loading = true;
    let sites = [];

    // Загружаем все сайты.
    onMount(async () => {
        const res = await listSites();
        if (res.ok) {
            sites = res.data.sites || [];
        } else {
            notify(res.error || 'Не удалось загрузить сайты', 'error');
        }
        loading = false;
    });

    // Переход к устройству (где находится сайт).
    function openDevice(deviceId) {
        goto(`/devices/${deviceId}`);
    }
</script>

<div class="page-header">
    <div>
        <h1 class="page-title">Сайты</h1>
        <p class="page-desc">Все сайты на всех устройствах</p>
    </div>
</div>

{#if loading}
    <div class="empty">Загрузка...</div>
{:else if sites.length === 0}
    <div class="empty">
        <p>Пока нет сайтов</p>
        <p class="empty-hint">
            Перейдите на устройство и нажмите «Добавить сайт»
        </p>
    </div>
{:else}
    <div class="sites-list">
        {#each sites as item (item.device_id + '-' + item.site.site_id)}
            <Card clickable on:click={() => openDevice(item.device_id)}>
                <div class="site-row">
                    <div class="site-info">
                        <div class="site-name">
                            {item.site.site_id}
                            <span class="site-device">
                                на {item.device_name || `устройство ${item.device_id}`}
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
                            {item.site.status === 'running' ? 'Работает' : 'Остановлен'}
                        </span>
                    </div>
                </div>
            </Card>
        {/each}
    </div>
{/if}

<style>
    .sites-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .site-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 16px;
    }

    .site-info {
        min-width: 0;
    }

    .site-name {
        font-size: 14px;
        font-weight: 500;
        color: #F1F5F9;
        display: flex;
        align-items: baseline;
        gap: 8px;
        flex-wrap: wrap;
    }

    .site-device {
        font-size: 12px;
        color: #64748B;
        font-weight: 400;
    }

    .site-meta {
        font-family: ui-monospace, monospace;
        font-size: 11px;
        color: #64748B;
        margin-top: 4px;
    }

    .site-status {
        display: flex;
        align-items: center;
        gap: 6px;
        flex-shrink: 0;
    }

    .status-text {
        font-size: 12px;
        color: #94A3B8;
    }

    .empty-hint {
        font-size: 12px;
        color: #64748B;
        margin-top: 8px;
    }
</style>
