<script>
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { getDevice, deleteDevice, deleteSite, sendCommand } from '$lib/api.js';
    import { notify } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';
    import StatusDot from '$lib/components/StatusDot.svelte';
    import Modal from '$lib/components/Modal.svelte';

    let device = null;
    let sites = [];
    let online = false;
    let loading = true;
    let showDeleteModal = false;
    let showAddSiteModal = false;
    let showLogsModal = false;
    let currentLogs = '';
    let logsSiteId = '';
    let busy = false;

    // Форма нового сайта. Шаблоны используют существующий безопасный Agent create_site lifecycle.
    let newSite = { site_id: '', port: 8000, runtime: 'php' };

    // Загружаем данные устройства.
    onMount(async () => {
        await loadDevice();
        loading = false;
    });

    async function loadDevice() {
        const id = $page.params.id;
        const res = await getDevice(id);
        if (!res.ok) {
            notify(res.error || 'Устройство не найдено', 'error');
            goto('/');
            return;
        }
        device = res.data.device;
        sites = res.data.sites || [];
        online = res.data.online;
    }

    // Отправка команды агенту.
    async function runCommand(action, payload = {}) {
        busy = true;
        const res = await sendCommand(device.id, action, payload);
        busy = false;

        if (!res.ok) {
            notify(res.error || 'Ошибка выполнения команды', 'error');
            return;
        }

        if (!res.data.ok) {
            notify(res.data.message || 'Ошибка', 'error');
            return;
        }

        notify('Готово', 'success');
        await loadDevice();
    }

    // Запуск сайта.
    async function startSite(site) {
        await runCommand('start_site', {
            site_id: site.site_id,
            path: site.path,
            port: site.port,
            runtime: site.runtime,
        });
    }

    // Остановка сайта.
    async function stopSite(site) {
        await runCommand('stop_site', { site_id: site.site_id });
    }

    async function removeSite(site) {
        if (!confirm('Удалить запись сайта? Файлы на Android-устройстве останутся на месте.')) return;
        const res = await deleteSite(device.id, site.site_id);
        if (res.ok) { notify('Запись сайта удалена. Файлы сохранены.', 'success'); await loadDevice(); }
        else notify(res.error || 'Не удалось удалить сайт', 'error');
    }

    // Git pull.
    async function gitPull(site) {
        await runCommand('git_pull', { site_id: site.site_id });
    }

    // Просмотр логов.
    async function showLogs(site) {
        logsSiteId = site.site_id;
        currentLogs = 'Загрузка...';
        showLogsModal = true;

        const res = await sendCommand(device.id, 'get_logs', {
            site_id: site.site_id,
            lines: 100,
        });

        if (res.ok && res.data.ok) {
            // payload — сериализованный Protobuf. Сервер присылает base64/bytes.
            // Для MVP просто покажем message (агент может вернуть текст в message).
            currentLogs = res.data.message || '(пусто)';
        } else {
            currentLogs = 'Не удалось загрузить логи';
        }
    }

    // Отвязка устройства.
    async function confirmDelete() {
        const res = await deleteDevice(device.id);
        if (res.ok) {
            notify('Устройство отвязано', 'success');
            goto('/');
        } else {
            notify(res.error || 'Ошибка', 'error');
        }
    }

    // Создание сайта и его запуск — две отдельные Agent-команды.
    async function addSite() {
        const siteId = newSite.site_id.trim();
        const port = Number(newSite.port);
        if (!/^[a-zA-Z0-9_-]{1,64}$/.test(siteId)) {
            notify('Используйте 1–64 символа: латинские буквы, цифры, дефис или подчёркивание', 'error');
            return;
        }
        if (!Number.isInteger(port) || port < 1024 || port > 65535) {
            notify('Порт должен быть от 1024 до 65535', 'error');
            return;
        }

        busy = true;
        const created = await sendCommand(device.id, 'create_site', {
            site_id: siteId,
            port,
            runtime: newSite.runtime,
        });
        if (!created.ok || !created.data?.ok) {
            busy = false;
            notify(created.error || created.data?.message || 'Не удалось создать сайт', 'error');
            return;
        }

        // Каталог и стартовый файл уже созданы. Если запуск не удастся,
        // сайт всё равно останется в панели в состоянии «Остановлен».
        const started = await sendCommand(device.id, 'start_site', {
            site_id: siteId,
            path: `sites/${siteId}`,
            port,
            runtime: newSite.runtime,
        });
        busy = false;
        showAddSiteModal = false;
        newSite = { site_id: '', port: 8000, runtime: 'php' };
        await loadDevice();

        if (!started.ok || !started.data?.ok) {
            notify('Сайт создан, но не запущен: ' + (started.error || started.data?.message || 'ошибка запуска'), 'warning');
            return;
        }
        notify('Сайт создан и запущен', 'success');
    }
</script>

{#if loading}
    <div class="empty">Загрузка...</div>
{:else if device}
    <!-- Header -->
    <div class="page-header">
        <div>
            <h1 class="page-title">{device.hostname || 'Устройство'}</h1>
            <p class="page-desc">
                {device.device_id.slice(0, 8)} · {device.tailscale_ip || 'без IP'}
            </p>
        </div>
        <div class="header-actions">
            <span class="status">
                <StatusDot status={online ? 'online' : 'offline'} />
                {online ? 'Онлайн' : 'Офлайн'}
            </span>
            <Button variant="danger" size="sm" on:click={() => showDeleteModal = true}>
                Отвязать
            </Button>
        </div>
    </div>

    <!-- Сайты -->
    <div class="section-header">
        <h2 class="section-title">Сайты</h2>
        <Button
            variant="primary"
            size="sm"
            disabled={!online}
            on:click={() => showAddSiteModal = true}
        >
            Добавить сайт
        </Button>
    </div>

    {#if sites.length === 0}
        <div class="empty">
            <p>Пока нет сайтов на этом устройстве</p>
        </div>
    {:else}
        <div class="sites-list">
            {#each sites as site (site.site_id)}
                <Card>
                    <div class="site-header">
                        <div class="site-info">
                            <button class="site-name site-link" on:click={() => goto(`/sites/${device.id}/${encodeURIComponent(site.site_id)}`)}>{site.site_id}</button>
                            <div class="site-meta">
                                :{site.port} · {site.path} · {site.runtime}
                            </div>
                        </div>
                        <StatusDot status={site.status} />
                    </div>

                    <div class="site-actions">
                        {#if site.status === 'running'}
                            <Button size="sm" on:click={() => stopSite(site)} disabled={busy}>
                                Остановить
                            </Button>
                            <Button size="sm" on:click={() => startSite(site)} disabled={busy}>
                                Перезапустить
                            </Button>
                        {:else}
                            <Button variant="primary" size="sm" on:click={() => startSite(site)} disabled={busy}>
                                Запустить
                            </Button>
                        {/if}
                        <Button variant="ghost" size="sm" on:click={() => gitPull(site)} disabled={busy}>
                            Git Pull
                        </Button>
                        <Button variant="ghost" size="sm" on:click={() => showLogs(site)}>
                            Логи
                        </Button>
                        <Button variant="danger" size="sm" on:click={() => removeSite(site)} disabled={busy}>
                            Удалить запись
                        </Button>
                    </div>
                </Card>
            {/each}
        </div>
    {/if}

    <!-- Модалка: удаление -->
    <Modal open={showDeleteModal} title="Отвязать устройство?" onClose={() => showDeleteModal = false}>
        <p class="modal-text">
            Устройство будет отвязано от аккаунта. Сайты на нём останутся, но управлять ими через панель будет нельзя.
        </p>
        <svelte:fragment slot="footer">
            <Button on:click={() => showDeleteModal = false}>Отмена</Button>
            <Button variant="danger" on:click={confirmDelete}>Отвязать</Button>
        </svelte:fragment>
    </Modal>

    <!-- Модалка: новый сайт -->
    <Modal open={showAddSiteModal} title="Добавить сайт" onClose={() => showAddSiteModal = false}>
        <div class="form-group">
            <label class="form-label" for="site_id">site_id</label>
            <input
                id="site_id"
                class="input"
                type="text"
                bind:value={newSite.site_id}
                placeholder="blog"
            />
        </div>
        <div class="form-group">
            <label class="form-label" for="port">Порт</label>
            <input
                id="port"
                class="input"
                type="number"
                bind:value={newSite.port}
                min="1"
                max="65535"
            />
        </div>
        <div class="form-group">
            <label class="form-label" for="runtime">Стартовый шаблон</label>
            <select id="runtime" class="input" bind:value={newSite.runtime}>
                <option value="static">Static Website — готовая index.html</option>
                <option value="php">PHP Website — готовая index.php</option>
                <option value="nodejs">Node.js HTTP — package.json + server.js</option>
                <option value="go">Go Application — структура проекта и README</option>
            </select>
            <p class="form-hint">Yevqo создаст каталог и стартовые файлы на устройстве. После создания сайт можно подключить к Git и развернуть через Deployment Engine.</p>
        </div>

        <svelte:fragment slot="footer">
            <Button on:click={() => showAddSiteModal = false}>Отмена</Button>
            <Button variant="primary" on:click={addSite} loading={busy}>Создать</Button>
        </svelte:fragment>
    </Modal>

    <!-- Модалка: логи -->
    <Modal open={showLogsModal} title={`Логи: ${logsSiteId}`} onClose={() => showLogsModal = false}>
        <pre class="logs">{currentLogs}</pre>
        <svelte:fragment slot="footer">
            <Button on:click={() => showLogsModal = false}>Закрыть</Button>
        </svelte:fragment>
    </Modal>
{/if}

<style>
    .header-actions {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .status {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        font-size: 12px;
        color: #94A3B8;
    }

    .section-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin: 32px 0 16px;
    }

    .section-title {
        font-size: 14px;
        font-weight: 600;
        color: #F1F5F9;
    }

    .sites-list {
        display: flex;
        flex-direction: column;
        gap: 12px;
    }

    .site-header {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        margin-bottom: 16px;
    }

    .site-name {
        font-weight: 500;
        font-size: 14px;
        color: #F1F5F9;
    }
    .site-link {
        display: inline-block;
        padding: 0;
        border: 0;
        background: transparent;
        font: inherit;
        text-align: left;
        cursor: pointer;
    }
    .site-link:hover { color: #A3E635; }

    .site-meta {
        font-family: ui-monospace, monospace;
        font-size: 11px;
        color: #64748B;
        margin-top: 4px;
    }

    .site-actions {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
    }

    .modal-text {
        font-size: 13px;
        color: #94A3B8;
        line-height: 1.6;
    }

    .logs {
        background: #080B14;
        border: 1px solid #253149;
        border-radius: 6px;
        padding: 12px;
        font-family: ui-monospace, monospace;
        font-size: 11px;
        color: #94A3B8;
        max-height: 400px;
        overflow: auto;
        white-space: pre-wrap;
        word-break: break-all;
    }
</style>
