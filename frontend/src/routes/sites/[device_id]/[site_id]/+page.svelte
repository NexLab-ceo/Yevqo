<script>
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { getDevice, getSite, getSiteGit, saveSiteGit, sendCommand, runSiteCommand, listLogs, listSiteDatabaseConnections } from '$lib/api.js';
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
    let activeTab = 'general';
    let git = { repository_url: '', branch: 'main', build_command: '', root_directory: '.', has_token: false, commit: '', deployed_at: null };
    let gitForm = { repository_url: '', branch: 'main', build_command: '', root_directory: '.', github_token: '' };
    let connections = [];
    let buildPlan = null;
    let analyzingBuild = false;

    onMount(async () => {
        deviceId = $page.params.device_id;
        siteId = $page.params.site_id;
        await refresh();
        loading = false;
    });

    async function refresh() {
        const [deviceRes, siteRes, gitRes, envRes] = await Promise.all([
            getDevice(deviceId), getSite(deviceId, siteId), getSiteGit(deviceId, siteId),
            listSiteDatabaseConnections(deviceId, siteId)
        ]);
        if (!deviceRes.ok || !siteRes.ok) {
            notify(deviceRes.error || siteRes.error || 'Не удалось загрузить сайт', 'error');
            return;
        }
        device = deviceRes.data.device;
        online = !!deviceRes.data.online;
        site = siteRes.data.site;
        if (gitRes.ok) {
            git = { ...git, ...gitRes.data };
            gitForm = {
                repository_url: git.repository_url || '', branch: git.branch || 'main',
                build_command: git.build_command || '', root_directory: git.root_directory || '.',
                github_token: ''
            };
        }
        connections = envRes.ok ? (envRes.data.connections || []) : [];
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
            activeTab = 'build';
        } catch {
            notify('Получен некорректный ответ анализа', 'error');
        }
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

    <nav class="tabs">
        <button class:active={activeTab === 'general'} on:click={() => activeTab = 'general'}>General</button>
        <button class:active={activeTab === 'logs'} on:click={openTerminal}>Logs</button>
        <button class:active={activeTab === 'environment'} on:click={() => activeTab = 'environment'}>Environment</button>
        <button class:active={activeTab === 'build'} on:click={() => activeTab = 'build'}>Build</button>
        <button class:active={activeTab === 'git'} on:click={() => activeTab = 'git'}>Git</button>
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
                </div>
                <Button variant="primary" disabled={!site.domain && !device?.tailscale_ip} on:click={openSite}>Открыть сайт ↗</Button>
            </Card>
            <Card>
                <h2 class="details-title">Последний деплой</h2>
                <div class="commit">{git.commit ? git.commit.slice(0, 12) : 'Ещё не выполнялся'}</div>
                <p class="muted">{git.deployed_at ? new Date(git.deployed_at).toLocaleString() : 'Нет данных о деплое'}</p>
                <p class="muted">{git.repository_url || 'Git-репозиторий не подключён'}</p>
            </Card>
        </div>
    {:else if activeTab === 'logs'}
        <Card>
            <div class="logs-head"><h2 class="details-title">Terminal / Logs</h2><Button size="sm" on:click={openTerminal} disabled={terminalLoading}>Обновить</Button></div>
            {#if terminalLoading}<p class="terminal-loading">Загрузка…</p>{:else}<pre class="terminal">{terminalText || 'Логи пока не загружены.'}</pre>{/if}
        </Card>
    {:else if activeTab === 'environment'}
        <Card>
            <h2 class="details-title">Environment</h2>
            <p class="muted">Секреты не раскрываются. Значения DB_* передаются процессу выбранного сайта.</p>
            {#if connections.length}
                {#each connections as connection}
                    <div class="env-group">
                        <strong>{connection.database} · {connection.username}</strong>
                        <div class="env-row"><code>DB_HOST</code><code>{connection.host}</code></div>
                        <div class="env-row"><code>DB_PORT</code><code>{connection.port}</code></div>
                        <div class="env-row"><code>DB_DATABASE</code><code>{connection.database}</code></div>
                        <div class="env-row"><code>DB_USERNAME</code><code>{connection.username}</code></div>
                        <div class="env-row"><code>DB_PASSWORD</code><code>••••••••••</code></div>
                    </div>
                {/each}
            {:else}<p class="muted">Управляемые базы данных не подключены.</p>{/if}
        </Card>
    {:else if activeTab === 'build'}
        <Card>
            <div class="logs-head">
                <div><h2 class="details-title">Build Engine</h2><p class="muted">Анализ проекта без выполнения его кода. Root Directory берётся из Git settings.</p></div>
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
                {#if buildPlan.needs_configuration}<p class="build-warning">Не удалось определить способ сборки автоматически. Выберите Dockerfile, Nixpacks, Static Hosting или задайте пользовательские команды.</p>{/if}
                {#if buildPlan.evidence?.length}<p class="muted">Найдено: {buildPlan.evidence.join(', ')}</p>{/if}
                {#if buildPlan.install_command || buildPlan.build_command || buildPlan.start_command}
                    <div class="build-commands">
                        {#if buildPlan.install_command}<div><span>Install</span><code>{buildPlan.install_command}</code></div>{/if}
                        {#if buildPlan.build_command}<div><span>Build</span><code>{buildPlan.build_command}</code></div>{/if}
                        {#if buildPlan.start_command}<div><span>Start</span><code>{buildPlan.start_command}</code></div>{/if}
                    </div>
                {/if}
                {#if buildPlan.warnings?.length}{#each buildPlan.warnings as warning}<p class="muted">{warning}</p>{/each}{/if}
                <p class="security-note">Пока это только план анализа. Команды не запускались и сайт не изменялся.</p>
            {:else}
                <p class="muted">Запусти анализ, чтобы определить стек, сборщик, команды и наличие конфигурации. Неизвестные проекты не считаются ошибкой.</p>
            {/if}
        </Card>
    {:else if activeTab === 'git'}
        <Card>
            <h2 class="details-title">Git Repository</h2>
            <p class="muted">Первый Deploy выполнит clone, следующие — fast-forward pull. Build Command запускается после обновления исходников.</p>
            <div class="git-form">
                <label>Repository URL<input class="input" bind:value={gitForm.repository_url} placeholder="https://github.com/user/repo.git" /></label>
                <label>Branch<input class="input" bind:value={gitForm.branch} placeholder="main" /></label>
                <label class="wide">Build Command<input class="input" bind:value={gitForm.build_command} placeholder="npm run build или composer install" /></label>
                <label class="wide">Root Directory<input class="input" bind:value={gitForm.root_directory} placeholder="." /></label>
                <label class="wide">GitHub Token<input class="input" type="password" bind:value={gitForm.github_token} autocomplete="new-password" placeholder={git.has_token ? 'Токен сохранён — пустое поле оставит его без изменений' : 'Только для приватного репозитория'} />{#if git.has_token}<small>Токен сохранён с AES-GCM шифрованием.</small>{/if}</label>
            </div>
            <p class="security-note">Build Command выполняется на устройстве от имени пользователя Termux. Используйте только доверенные команды.</p>
            <div class="save-row"><Button variant="primary" loading={busyAction === 'git'} on:click={saveGit}>Connect / Save</Button></div>
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
    .tabs{display:flex;gap:6px;border-bottom:1px solid #27272A;margin:24px 0 18px;overflow:auto}
    .tabs button{background:transparent;border:0;border-bottom:2px solid transparent;color:#A1A1AA;padding:11px 14px;cursor:pointer;white-space:nowrap;font-size:13px}
    .tabs button.active{color:#FAFAFA;border-bottom-color:#A3E635}
    .detail-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(250px,1fr));gap:14px}
    .commit{font:18px ui-monospace,monospace;color:#BEF264}.muted{font-size:12px;color:#A1A1AA;line-height:1.6}
    .logs-head{display:flex;justify-content:space-between;align-items:center}
    .terminal{background:#09090B;border:1px solid #27272A;border-radius:8px;color:#D4D4D8;padding:14px;max-height:60vh;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font:12px/1.6 ui-monospace,monospace}
    .env-group{border:1px solid #27272A;border-radius:8px;padding:14px;margin-top:12px}
    .env-group strong{display:block;color:#FAFAFA;font-size:13px;margin-bottom:10px}
    .env-row{display:flex;justify-content:space-between;gap:14px;padding:7px 0;border-bottom:1px solid #27272A;font-size:12px;color:#D4D4D8}
    .build-summary{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px;margin:16px 0}
    .build-summary div,.build-commands div{display:flex;flex-direction:column;gap:6px;background:#09090B;border:1px solid #27272A;border-radius:8px;padding:12px;min-width:0}
    .build-summary span,.build-commands span{font-size:11px;color:#A1A1AA}
    .build-summary strong{font-size:13px;color:#FAFAFA;overflow-wrap:anywhere}
    .build-commands{display:grid;gap:8px;margin-top:14px}
    .build-commands code{font-size:12px;color:#D4D4D8;overflow-wrap:anywhere}
    .build-warning{padding:12px;border:1px solid #854D0E;border-radius:8px;color:#FBBF24;font-size:12px;line-height:1.5}
    .git-form{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin-top:18px}
    .git-form label{display:grid;gap:7px;color:#D4D4D8;font-size:12px}.git-form .wide{grid-column:1/-1}
    .input{width:100%;box-sizing:border-box;background:#09090B;border:1px solid #3F3F46;border-radius:7px;padding:10px 12px;color:#FAFAFA;font-size:13px}
    .git-form small{color:#A1A1AA}.security-note{margin-top:14px;color:#FBBF24;font-size:11px}
    .save-row{display:flex;justify-content:flex-end;margin-top:18px}
    @media(max-width:600px){.git-form{grid-template-columns:1fr}.git-form .wide{grid-column:auto}}
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
