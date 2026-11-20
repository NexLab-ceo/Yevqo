<script>
    import { onMount } from 'svelte';
    import { locale } from '$lib/i18n.js';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { getDevice, deleteDevice, deleteSite, sendCommand, listAppTemplates, attachProjectSite, listLogs } from '$lib/api.js';
    import { notify } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';
    import StatusDot from '$lib/components/StatusDot.svelte';
    import Modal from '$lib/components/Modal.svelte';

    let device = null;
    let activeTab='overview',deviceLogs=[],logsLoading=false;
    $: ru=$locale!=='en';
    let sites = [];
    let appTemplates = [];
    let online = false;
    let loading = true;
    let showDeleteModal = false;
    let showAddSiteModal = false;
    let showLogsModal = false;
    let currentLogs = '';
    let logsSiteId = '';
    let busy = false;

    // Форма нового сайта. Шаблоны используют существующий безопасный Agent create_site lifecycle.
    let newSite = { site_id: '', port: 8000, template_id: 'php-basic' };

    // Загружаем данные устройства.
    onMount(async () => {
        await Promise.all([loadDevice(), loadTemplates()]);
        loading = false;
    });

    async function loadTemplates() {
        const res = await listAppTemplates();
        if (res.ok) appTemplates = res.data.templates || [];
        else notify(res.error || 'Не удалось загрузить каталог шаблонов', 'error');
    }

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

    async function selectTab(tab){activeTab=tab;if(tab==='activity'&&!deviceLogs.length){logsLoading=true;const r=await listLogs(100);if(r.ok)deviceLogs=(r.data.logs||[]).filter(x=>x.device===device?.hostname);else notify(r.error||'Could not load device activity','error');logsLoading=false}}

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

        const selectedTemplate = appTemplates.find((template) => template.id === newSite.template_id);
        if (!selectedTemplate) {
            notify('Выберите шаблон из каталога', 'error');
            return;
        }

        busy = true;
        const created = await sendCommand(device.id, 'create_site', {
            site_id: siteId,
            port,
            runtime: selectedTemplate.runtime,
            template_id: selectedTemplate.id,
        });
        if (!created.ok || !created.data?.ok) {
            busy = false;
            notify(created.error || created.data?.message || 'Не удалось создать сайт', 'error');
            return;
        }

        const projectId = $page.url.searchParams.get('project_id');
        if (projectId) {
            const linked = await attachProjectSite(projectId, Number(device.id), siteId);
            if (!linked.ok) {
                busy = false;
                notify('Сайт создан, но не удалось добавить его в проект: ' + (linked.error || 'ошибка привязки'), 'warning');
            }
        }

        // Go starters are compiled on the target Agent before first start.
        if (selectedTemplate.runtime === 'go') {
            const built = await sendCommand(device.id, 'build_site', {
                site_id: siteId,
                stack: 'go',
                allow_project_code: true,
                timeout_seconds: 300,
                max_log_bytes: 1048576,
            });
            if (!built.ok || !built.data?.ok) {
                busy = false;
                showAddSiteModal = false;
                newSite = { site_id: '', port: 8000, template_id: 'php-basic' };
                await loadDevice();
                notify('Go-проект создан, но сборка не прошла: ' + (built.error || built.data?.message || 'ошибка сборки'), 'error');
                return;
            }
        }

        // Каталог и стартовый файл уже созданы. Если запуск не удастся,
        // сайт всё равно останется в панели в состоянии «Остановлен».
        const started = await sendCommand(device.id, 'start_site', {
            site_id: siteId,
            path: `sites/${siteId}`,
            port,
            runtime: selectedTemplate.runtime,
        });
        busy = false;
        showAddSiteModal = false;
        newSite = { site_id: '', port: 8000, template_id: 'php-basic' };
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

    <nav class="device-tabs"><button class:active={activeTab==='overview'} on:click={()=>selectTab('overview')}>{ru?'Обзор':'Overview'}</button><button class:active={activeTab==='projects'} on:click={()=>selectTab('projects')}>{ru?'Проекты':'Projects'}</button><button class:active={activeTab==='activity'} on:click={()=>selectTab('activity')}>{ru?'Активность':'Activity'}</button><button class:active={activeTab==='settings'} on:click={()=>selectTab('settings')}>{ru?'Настройки':'Settings'}</button></nav>
    {#if activeTab==='overview'}
    <div class="device-capabilities">
        <span class="device-platform">{device.platform || 'Платформа не определена'} · {device.architecture || 'архитектура неизвестна'}</span>
        {#each (device.capabilities || []) as capability (capability)}
            <span class="capability-chip">{capability}</span>
        {/each}
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

    {:else if activeTab==='projects'}
      <section class="panel device-tab-panel"><h2>{ru?'Проекты устройства':'Device projects'}</h2>{#if $page.url.searchParams.get('project_id')}<p>{ru?'Открыта карточка устройства из проекта.':'This device was opened from a project.'}</p><a class="device-tab-link" href={'/projects/'+$page.url.searchParams.get('project_id')}>{ru?'Открыть проект':'Open project'} →</a>{:else}<p>{ru?'Связи устройств с проектами управляются из карточки проекта.':'Device-to-project membership is managed from the project details page.'}</p><a class="device-tab-link" href="/projects">{ru?'Открыть проекты':'Open projects'} →</a>{/if}</section>
    {:else if activeTab==='activity'}
      <section class="panel device-tab-panel"><h2>{ru?'Активность устройства':'Device activity'}</h2>{#if logsLoading}<div class="empty">{ru?'Загрузка…':'Loading…'}</div>{:else if deviceLogs.length}{#each deviceLogs as event}<div class="device-log-row"><strong>{String(event.action||'event').replaceAll('_',' ')}</strong><small>{event.created_at?new Date(event.created_at).toLocaleString():'—'}</small><p>{typeof event.details==='string'?event.details:JSON.stringify(event.details||{})}</p></div>{/each}{:else}<div class="empty">{ru?'Событий для этого устройства пока нет.':'No activity recorded for this device yet.'}</div>{/if}</section>
    {:else}
      <section class="panel device-tab-panel"><h2>{ru?'Настройки устройства':'Device settings'}</h2><div class="device-setting"><span>{ru?'Имя устройства':'Device name'}</span><strong>{device.hostname||'—'}</strong></div><div class="device-setting"><span>Device ID</span><strong>{device.device_id}</strong></div><div class="device-setting"><span>{ru?'Операционная система':'Operating system'}</span><strong>{device.platform||'—'} · {device.architecture||'—'}</strong></div><div class="device-setting"><span>Agent</span><strong>{device.agent_version||'—'}</strong></div><div class="device-setting"><span>IP</span><strong>{device.tailscale_ip||'—'}</strong></div><p class="settings-note">{ru?'Переименование и изменение сетевых параметров пока не поддерживаются Agent API.':'Renaming and network configuration changes are not supported by the Agent API yet.'}</p><Button variant="danger" on:click={()=>showDeleteModal=true}>{ru?'Отвязать устройство':'Unpair device'}</Button></section>
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
            <label class="form-label" for="runtime">Шаблон приложения</label>
            <select id="runtime" class="input" bind:value={newSite.template_id}>
                {#each appTemplates as template (template.id)}
                    <option value={template.id}>{template.name} — {template.description}</option>
                {/each}
            </select>
            <p class="form-hint">Yevqo создаст файлы проекта и yevqo.template.json с параметрами запуска, затем запустит приложение на выбранном порту.</p>
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
.device-tabs{display:flex;gap:4px;overflow-x:auto;padding:4px;margin-bottom:14px;border:1px solid var(--border);border-radius:8px;background:var(--bg-card)}.device-tabs button{padding:7px 10px;white-space:nowrap;border:0;border-radius:6px;background:transparent;color:var(--text-muted);font-size:10px}.device-tabs button.active{background:var(--bg-elevated);color:var(--accent)}.device-tab-panel{padding:16px}.device-tab-panel h2{font-size:13px;margin-bottom:10px}.device-tab-panel p,.settings-note{font-size:10px;color:var(--text-muted);line-height:1.6}.device-tab-link{display:inline-block;margin-top:10px;color:var(--accent);font-size:10px}.device-log-row,.device-setting{padding:10px 0;border-bottom:1px solid var(--border)}.device-log-row{display:grid;gap:4px}.device-log-row strong,.device-setting strong{font-size:10px}.device-log-row small,.device-setting span{font-size:9px;color:var(--text-muted)}.device-log-row p{overflow-wrap:anywhere}.device-setting{display:flex;justify-content:space-between;gap:12px}.settings-note{margin:12px 0}
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

    .device-capabilities{display:flex;align-items:center;gap:7px;flex-wrap:wrap;margin:-8px 0 22px}.device-platform,.capability-chip{font-size:10px;color:#9aa9c2;background:#111b2d;border:1px solid #253149;border-radius:7px;padding:5px 8px}.capability-chip{color:#a5b4fc}
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
