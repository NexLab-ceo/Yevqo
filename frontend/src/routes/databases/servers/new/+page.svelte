<script>
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { page } from '$app/stores';
    import { listDevices, createDatabaseServer } from '$lib/api.js';
    import { notify } from '$lib/stores.js';

    const steps = ['Engine & Device', 'Storage & Resources', 'Network & Security', 'Review & Save'];
    let step = 0;
    let loadingDevices = true;
    let devices = [];
    let error = '';
    let completed = false;

    let engine = 'postgresql';
    let deviceId = '';
    let version = 'latest';
    let serverName = 'postgres-main';
    let description = '';

    let dataDirectory = '$PREFIX/var/lib/postgresql';
    let storageLimit = 'unlimited';
    let ramBudget = 'auto';
    let maxConnections = 50;
    let autoStart = true;
    let monitorStorage = true;
    let processPolicy = 'balanced';

    let bindAddress = '127.0.0.1';
    let port = 5432;
    let networkAccess = 'private';
    let sslMode = 'require';
    let remoteAccess = false;
    let authMethod = 'password';

    $: selectedDevice = devices.find((item) => String(item.id) === String(deviceId));
    $: isPostgres = engine === 'postgresql';
    $: safeServerName = /^[a-zA-Z][a-zA-Z0-9_-]{1,47}$/.test(serverName);
    $: validPort = Number.isInteger(Number(port)) && Number(port) > 0 && Number(port) < 65536;
    $: canContinue = step === 0
        ? safeServerName && !!deviceId
        : step === 1
            ? !!dataDirectory.trim() && Number(maxConnections) > 0
            : validPort && !!bindAddress.trim();

    onMount(async () => {
        const response = await listDevices();
        if (response.ok) {
            devices = response.data?.devices || [];
            const online = devices.find((item) => item.status === 'online');
            const first = online || devices[0];
            if (first) deviceId = String(first.id);
        } else {
            error = response.error || 'Не удалось загрузить устройства';
        }
        loadingDevices = false;
    });

    function setEngine(value) {
        engine = value;
        port = value === 'postgresql' ? 5432 : 3306;
        dataDirectory = value === 'postgresql' ? '$PREFIX/var/lib/postgresql' : '$PREFIX/var/lib/mysql';
        if (value !== 'postgresql' && authMethod === 'scram') authMethod = 'password';
    }

    function next() {
        error = '';
        if (!canContinue) {
            error = step === 0
                ? 'Укажите корректное имя сервера и выберите устройство.'
                : 'Проверьте обязательные поля и значения.';
            return;
        }
        step = Math.min(step + 1, 3);
    }

    function back() {
        error = '';
        step = Math.max(step - 1, 0);
    }

    async function finishPreview() {
        error = '';
        if (remoteAccess || bindAddress !== '127.0.0.1') {
            error = 'Удалённый bind пока заблокирован: сначала настройте firewall и безопасный Agent protocol.';
            return;
        }
        const result = await createDatabaseServer({
            ...( $page.url.searchParams.get('project_id') ? { project_id: Number($page.url.searchParams.get('project_id')) } : {}),
            device_id: Number(deviceId),
            name: serverName,
            engine,
            version: version === 'latest' ? '' : version,
            port: Number(port),
            data_directory: dataDirectory,
            network_access: 'localhost',
            storage_limit_gb: storageLimit,
            ram_budget_mb: ramBudget === 'auto' ? 0 : Number(ramBudget),
            max_connections: Number(maxConnections),
            monitor_storage: monitorStorage,
            auto_start: autoStart,
            process_policy: processPolicy,
            bind_address: bindAddress,
            ssl_mode: sslMode,
            auth_method: authMethod,
            remote_access: remoteAccess
        });
        if (!result.ok) { error = result.error || 'Не удалось сохранить конфигурацию'; return; }
        completed = true;
    }

    function formatStorage(value) {
        return value === 'unlimited' ? 'Без лимита' : value + ' GB';
    }
</script>

<svelte:head>
    <title>Install Database Server · VELTRIX Dock</title>
    <meta name="description" content="Configure a database server on an Android device." />
</svelte:head>

<div class="wizard-page">
    <div class="page-heading">
        <div>
            <div class="eyebrow">DATABASES <span>/</span> DATABASE SERVERS</div>
            <h1>Install Database Server</h1>
            <p>Настройте отдельный сервер баз данных на Android-устройстве.</p>
        </div>
        <button class="btn btn-ghost" on:click={() => goto('/databases')}>Закрыть</button>
    </div>

    <div class="wizard-shell">
        <aside class="step-rail">
            <div class="rail-title">Установка сервера</div>
            {#each steps as label, index}
                <button class="step-item" class:active={step === index} class:done={step > index} on:click={() => step > index && (step = index)}>
                    <span class="step-number">{#if step > index}✓{:else}{index + 1}{/if}</span>
                    <span class="step-copy">
                        <strong>{label}</strong>
                        <small>{['Выбор движка и устройства', 'Диск и системные ресурсы', 'Сетевой доступ и защита', 'Проверка конфигурации'][index]}</small>
                    </span>
                </button>
            {/each}
            <div class="rail-note">
                <span class="note-icon">i</span>
                <span>Сейчас мастер сохраняет конфигурацию. Установка пакета будет отдельным этапом после реализации Agent protocol.</span>
            </div>
        </aside>

        <main class="wizard-content">
            {#if completed}
                <div class="success-state">
                    <div class="success-icon">✓</div>
                    <h2>Конфигурация сохранена</h2>
                    <p>Параметры сервера сохранены в Control Plane. Запись имеет статус Planned — реальная установка PostgreSQL / MariaDB / MySQL ещё не запускается.</p>
                    <div class="notice warning"><strong>Безопасно сохранено.</strong> На Android-устройстве ничего не устанавливалось и существующие данные не изменялись.</div>
                    <div class="summary-actions">
                        <button class="btn btn-secondary" on:click={() => { completed = false; step = 3; }}>Изменить настройки</button>
                        <button class="btn btn-primary" on:click={() => goto('/databases')}>Вернуться в Databases</button>
                    </div>
                </div>
            {:else}
                <div class="content-topline">
                    <div>
                        <span class="step-kicker">STEP {step + 1} OF 4</span>
                        <h2>{steps[step]}</h2>
                    </div>
                    <span class="step-counter">{String(step + 1).padStart(2, '0')} <span>/ 04</span></span>
                </div>

                {#if error}
                    <div class="form-error" role="alert">{error}</div>
                {/if}

                {#if step === 0}
                    <section class="form-section">
                        <div class="section-heading"><h3>Database engine</h3><p>Выберите СУБД, которую будет обслуживать этот сервер.</p></div>
                        <div class="engine-grid">
                            <button class="engine-option" class:selected={engine === 'postgresql'} on:click={() => setEngine('postgresql')}>
                                <span class="engine-logo postgres">PG</span><span class="engine-text"><strong>PostgreSQL</strong><small>Рекомендуется · 5432</small></span>{#if engine === 'postgresql'}<span class="selected-mark">✓</span>{/if}
                            </button>
                            <button class="engine-option" class:selected={engine === 'mariadb'} on:click={() => setEngine('mariadb')}>
                                <span class="engine-logo maria">M</span><span class="engine-text"><strong>MariaDB</strong><small>MySQL-compatible · 3306</small></span>{#if engine === 'mariadb'}<span class="selected-mark">✓</span>{/if}
                            </button>
                            <button class="engine-option" class:selected={engine === 'mysql'} on:click={() => setEngine('mysql')}>
                                <span class="engine-logo mysql">MY</span><span class="engine-text"><strong>MySQL</strong><small>Classic relational DB · 3306</small></span>{#if engine === 'mysql'}<span class="selected-mark">✓</span>{/if}
                            </button>
                        </div>
                    </section>

                    <section class="form-section">
                        <div class="section-heading"><h3>Target device</h3><p>Database Server будет отдельным процессом на выбранном устройстве.</p></div>
                        {#if loadingDevices}
                            <div class="loading-row">Загружаем устройства…</div>
                        {:else if devices.length === 0}
                            <div class="notice warning">Нет доступных устройств. Сначала подключите Android Agent, затем вернитесь к установке.</div>
                        {:else}
                            <div class="device-select">
                                <label for="device">Устройство</label>
                                <select id="device" bind:value={deviceId}>
                                    {#each devices as device}
                                        <option value={String(device.id)}>{device.hostname || device.name || 'Android device'} · {device.status === 'online' ? 'Online' : 'Offline'}</option>
                                    {/each}
                                </select>
                                {#if selectedDevice}
                                    <div class="device-meta"><span class:online={selectedDevice.status === 'online'} class="status-dot"></span>{selectedDevice.status === 'online' ? 'Подключено' : 'Не в сети'} <span>·</span> ID {selectedDevice.id}</div>
                                {/if}
                            </div>
                        {/if}
                        <div class="field-grid">
                            <div class="field"><label for="version">Версия</label><select id="version" bind:value={version}><option value="latest">Latest stable</option><option value="specific">Выбрать конкретную версию (позже)</option></select><small>Агент должен проверить совместимость пакета с архитектурой устройства.</small></div>
                            <div class="field"><label for="serverName">Server name</label><input id="serverName" bind:value={serverName} maxlength="48" placeholder="postgres-main" /><small>Латиница, цифры, дефис и подчёркивание.</small></div>
                        </div>
                        <div class="field"><label for="description">Описание <span class="optional">необязательно</span></label><input id="description" bind:value={description} maxlength="160" placeholder="Основной сервер PostgreSQL" /></div>
                    </section>
                {:else if step === 1}
                    <section class="form-section">
                        <div class="section-heading"><h3>Data storage</h3><p>Каталог данных и допустимое использование хранилища.</p></div>
                        <div class="field"><label for="dataDirectory">Data directory</label><input id="dataDirectory" bind:value={dataDirectory} /><small>Путь будет проверен на устройстве. Значение зависит от фактического пакета Termux.</small></div>
                        <div class="field"><label>Storage allocation</label><div class="choice-grid">
                            <button class:chosen={storageLimit === 'unlimited'} on:click={() => storageLimit = 'unlimited'}><strong>Без лимита</strong><small>Использовать свободное место устройства</small></button>
                            {#each ['5','10','20'] as value}<button class:chosen={storageLimit === value} on:click={() => storageLimit = value}><strong>{value} GB</strong><small>Контролировать рост данных</small></button>{/each}
                        </div></div>
                    </section>
                    <section class="form-section">
                        <div class="section-heading"><h3>Resource budget</h3><p>Мягкие ориентиры для планирования нагрузки, не системные квоты Android.</p></div>
                        <div class="field-grid">
                            <div class="field"><label for="ram">RAM budget</label><select id="ram" bind:value={ramBudget}><option value="auto">Auto</option><option value="256">256 MB</option><option value="512">512 MB</option><option value="1024">1 GB</option></select></div>
                            <div class="field"><label for="connections">Maximum connections</label><input id="connections" type="number" min="1" max="1000" bind:value={maxConnections} /></div>
                        </div>
                        <div class="toggle-row"><div><strong>Storage monitoring</strong><small>Показывать предупреждения при нехватке места.</small></div><input type="checkbox" bind:checked={monitorStorage} aria-label="Storage monitoring" /></div>
                    </section>
                    <section class="form-section">
                        <div class="section-heading"><h3>Startup behavior</h3><p>Android может ограничивать фоновые процессы и автозапуск.</p></div>
                        <div class="toggle-row"><div><strong>Auto-start after device boot</strong><small>Требуется совместимая настройка Termux:Boot.</small></div><input type="checkbox" bind:checked={autoStart} aria-label="Auto-start" /></div>
                        <div class="field"><label for="processPolicy">Background process policy</label><select id="processPolicy" bind:value={processPolicy}><option value="balanced">Balanced — экономия ресурсов</option><option value="persistent">Persistent — пытаться поддерживать процесс</option><option value="manual">Manual — запуск вручную</option></select></div>
                    </section>
                {:else if step === 2}
                    <section class="form-section">
                        <div class="section-heading"><h3>Network binding</h3><p>По умолчанию сервер доступен только локально.</p></div>
                        <div class="field-grid">
                            <div class="field"><label for="bindAddress">Bind address</label><select id="bindAddress" bind:value={bindAddress}><option value="127.0.0.1">127.0.0.1 — localhost</option><option value="0.0.0.0">0.0.0.0 — все интерфейсы</option></select></div>
                            <div class="field"><label for="port">Port</label><input id="port" type="number" min="1" max="65535" bind:value={port} /></div>
                        </div>
                        <div class="field"><label>Network access</label><div class="choice-grid two">
                            <button class:chosen={networkAccess === 'private'} on:click={() => { networkAccess = 'private'; bindAddress = '127.0.0.1'; remoteAccess = false; }}><strong>Private network</strong><small>Локальный доступ и приватная сеть через Agent</small></button>
                            <button class:chosen={networkAccess === 'custom'} on:click={() => { networkAccess = 'custom'; bindAddress = '0.0.0.0'; }}><strong>Custom interface</strong><small>Для опытных пользователей; требует firewall rules</small></button>
                        </div></div>
                        <div class="toggle-row"><div><strong>Allow remote connections</strong><small>Не включайте без точного списка разрешённых адресов и firewall.</small></div><input type="checkbox" bind:checked={remoteAccess} /></div>
                    </section>
                    <section class="form-section">
                        <div class="section-heading"><h3>Connection security</h3><p>Выберите режим защиты соединений и аутентификации.</p></div>
                        <div class="field-grid">
                            <div class="field"><label for="sslMode">SSL / TLS mode</label><select id="sslMode" bind:value={sslMode}><option value="require">Require SSL</option><option value="prefer">Prefer SSL</option><option value="disable">Disable (локальная разработка)</option></select></div>
                            <div class="field"><label for="authMethod">Authentication</label><select id="authMethod" bind:value={authMethod}><option value="password">Password authentication</option>{#if isPostgres}<option value="scram">SCRAM-SHA-256</option>{/if}</select></div>
                        </div>
                        <div class="notice info">Публичный доступ выключен по умолчанию. Секреты и пароли не должны попадать в логи или отображаться повторно.</div>
                    </section>
                {:else}
                    <section class="form-section">
                        <div class="section-heading"><h3>Configuration summary</h3><p>Проверьте параметры перед передачей задачи агенту.</p></div>
                        <div class="review-group"><h4>Engine & Device <button on:click={() => step = 0}>Изменить</button></h4><dl><div><dt>Engine</dt><dd>{engine}</dd></div><div><dt>Server name</dt><dd>{serverName}</dd></div><div><dt>Device</dt><dd>{selectedDevice?.hostname || selectedDevice?.name || deviceId || 'Не выбрано'}</dd></div><div><dt>Version</dt><dd>{version === 'latest' ? 'Latest stable' : 'Specific version'}</dd></div></dl></div>
                        <div class="review-group"><h4>Storage & Resources <button on:click={() => step = 1}>Изменить</button></h4><dl><div><dt>Data directory</dt><dd class="mono">{dataDirectory}</dd></div><div><dt>Storage limit</dt><dd>{formatStorage(storageLimit)}</dd></div><div><dt>RAM budget</dt><dd>{ramBudget === 'auto' ? 'Auto' : ramBudget + ' MB'}</dd></div><div><dt>Max connections</dt><dd>{maxConnections}</dd></div><div><dt>Auto-start</dt><dd>{autoStart ? 'Enabled (if supported)' : 'Disabled'}</dd></div></dl></div>
                        <div class="review-group"><h4>Network & Security <button on:click={() => step = 2}>Изменить</button></h4><dl><div><dt>Bind address</dt><dd class="mono">{bindAddress}</dd></div><div><dt>Port</dt><dd>{port}</dd></div><div><dt>Network</dt><dd>{networkAccess === 'private' ? 'Private network' : 'Custom interface'}</dd></div><div><dt>Remote access</dt><dd>{remoteAccess ? 'Enabled — firewall required' : 'Disabled'}</dd></div><div><dt>SSL / TLS</dt><dd>{sslMode}</dd></div></dl></div>
                        <div class="notice warning"><strong>Backend integration is pending.</strong> The current Agent protocol has no database-server install, configure, or rollback commands. This step will not execute shell commands or change the device.</div>
                    </section>
                {/if}

                <footer class="wizard-footer">
                    <button class="btn btn-secondary" on:click={back} disabled={step === 0}>Назад</button>
                    <div class="footer-right">
                        <span class="autosave-label">Изменения пока в этой форме</span>
                        {#if step < 3}
                            <button class="btn btn-primary" on:click={next} disabled={step === 0 && (loadingDevices || devices.length === 0)}>Продолжить <span>→</span></button>
                        {:else}
                            <button class="btn btn-primary" on:click={finishPreview}>Завершить проверку <span>→</span></button>
                        {/if}
                    </div>
                </footer>
            {/if}
        </main>
    </div>
</div>

<style>
    .wizard-page{max-width:1180px;margin:0 auto;color:#f1f5f9}
    .page-heading{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:26px}
    .eyebrow,.step-kicker{font-size:10px;font-weight:700;letter-spacing:.12em;color:#818cf8}
    .eyebrow span{color:#475569;margin:0 7px}
    h1{font-size:24px;letter-spacing:-.035em;margin-top:8px;font-weight:650}
    .page-heading p{color:#94a3b8;font-size:13px;margin-top:5px}
    .wizard-shell{display:grid;grid-template-columns:248px minmax(0,1fr);border:1px solid #253149;border-radius:14px;background:#0d1422;overflow:hidden;min-height:690px}
    .step-rail{padding:24px 16px;background:#0b1020;border-right:1px solid #253149;display:flex;flex-direction:column}
    .rail-title{font-size:12px;font-weight:650;padding:0 10px 22px;color:#e2e8f0}
    .step-item{display:flex;align-items:flex-start;gap:11px;padding:11px 10px;border:0;background:transparent;text-align:left;color:#94a3b8;border-radius:9px;margin-bottom:5px;cursor:pointer}
    .step-item.active{background:#172136;color:#f1f5f9}
    .step-item.done{color:#a5b4fc}
    .step-number{width:23px;height:23px;display:grid;place-items:center;border:1px solid #334155;border-radius:50%;font-size:11px;flex:none}
    .step-item.active .step-number{background:#6366f1;border-color:#6366f1;color:white}
    .step-item.done .step-number{border-color:#6366f1;color:#a5b4fc}
    .step-copy{display:grid;gap:3px}.step-copy strong{font-size:12px;font-weight:600}.step-copy small{font-size:10px;color:#64748b}
    .rail-note{display:flex;gap:9px;margin-top:auto;padding:12px 10px;border-top:1px solid #253149;color:#64748b;font-size:10px;line-height:1.55}
    .note-icon{width:16px;height:16px;display:grid;place-items:center;border:1px solid #475569;border-radius:50%;flex:none;font-size:10px}
    .wizard-content{padding:28px 32px;min-width:0;display:flex;flex-direction:column}
    .content-topline{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:25px}
    .content-topline h2{font-size:19px;font-weight:650;margin-top:5px;letter-spacing:-.02em}
    .step-counter{font:600 13px ui-monospace,monospace;color:#e2e8f0}.step-counter span{color:#64748b}
    .form-section{border-bottom:1px solid #253149;padding-bottom:22px;margin-bottom:22px}
    .section-heading{margin-bottom:15px}.section-heading h3{font-size:13px;font-weight:650}.section-heading p{font-size:11px;color:#64748b;margin-top:4px}
    .engine-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:9px}
    .engine-option{display:flex;align-items:center;gap:10px;padding:13px 11px;background:#101a2b;border:1px solid #253149;border-radius:10px;color:#f1f5f9;text-align:left;cursor:pointer;min-width:0}
    .engine-option.selected{border-color:#6366f1;background:#171d39;box-shadow:0 0 0 1px #6366f133}
    .engine-logo{width:32px;height:32px;display:grid;place-items:center;border-radius:8px;font-size:10px;font-weight:800;flex:none}
    .postgres{background:#1e3a5f;color:#93c5fd}.maria{background:#3b2b19;color:#fbbf24}.mysql{background:#173b43;color:#67e8f9}
    .engine-text{display:grid;gap:3px;min-width:0}.engine-text strong{font-size:11px}.engine-text small{font-size:9px;color:#64748b;white-space:nowrap}
    .selected-mark{margin-left:auto;color:#a5b4fc;font-size:13px}
    .field{display:grid;gap:6px;margin-top:15px;min-width:0}.field label,.device-select label{font-size:11px;color:#cbd5e1;font-weight:600}
    .field input,.field select,.device-select select{width:100%;height:38px;border:1px solid #334155;border-radius:8px;background:#0a1120;color:#e2e8f0;padding:0 11px;font:inherit;font-size:12px;outline:none}
    .field input:focus,.field select:focus,.device-select select:focus{border-color:#818cf8;box-shadow:0 0 0 3px #6366f122}
    .field small{font-size:10px;color:#64748b;line-height:1.5}
    .field-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:13px}
    .optional{font-weight:400;color:#64748b;margin-left:5px}
    .device-select{display:grid;gap:7px}.device-meta{display:flex;align-items:center;gap:7px;color:#64748b;font-size:10px}
    .status-dot{width:7px;height:7px;border-radius:50%;background:#f59e0b}.status-dot.online{background:#34d399}
    .loading-row{padding:14px;color:#94a3b8;font-size:12px}
    .choice-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:8px}
    .choice-grid.two{grid-template-columns:repeat(2,minmax(0,1fr))}
    .choice-grid button{display:grid;gap:4px;text-align:left;padding:11px;background:#101a2b;border:1px solid #253149;border-radius:8px;color:#e2e8f0;cursor:pointer}
    .choice-grid button.chosen{border-color:#6366f1;background:#171d39}
    .choice-grid strong{font-size:11px}.choice-grid small{font-size:9px;color:#64748b;line-height:1.4}
    .toggle-row{display:flex;justify-content:space-between;align-items:center;gap:18px;padding:12px 0;border-top:1px solid #1e293b;margin-top:12px}
    .toggle-row div{display:grid;gap:3px}.toggle-row strong{font-size:11px;font-weight:600}.toggle-row small{font-size:10px;color:#64748b}
    .toggle-row input{accent-color:#6366f1;width:16px;height:16px;flex:none}
    .notice{border-radius:9px;padding:12px 13px;font-size:11px;line-height:1.55;margin-top:15px}
    .notice.info{background:#0c1e32;border:1px solid #1e4264;color:#93c5fd}
    .notice.warning{background:#2b2112;border:1px solid #70501e;color:#fcd58a}
    .form-error{background:#3b1720;border:1px solid #7f1d35;color:#fda4af;padding:10px 12px;border-radius:8px;font-size:11px;margin-bottom:16px}
    .review-group{padding:14px 0;border-bottom:1px solid #1e293b}.review-group h4{display:flex;justify-content:space-between;font-size:11px;margin-bottom:9px}
    .review-group h4 button{border:0;background:none;color:#a5b4fc;font-size:10px;cursor:pointer}
    dl{display:grid;gap:7px}dl div{display:flex;justify-content:space-between;gap:15px;font-size:10px}dt{color:#64748b}dd{color:#cbd5e1;text-align:right;overflow-wrap:anywhere}.mono{font-family:ui-monospace,monospace}
    .wizard-footer{margin-top:auto;padding-top:22px;display:flex;justify-content:space-between;align-items:center;gap:12px}
    .footer-right{display:flex;align-items:center;gap:14px}.autosave-label{font-size:10px;color:#64748b}
    .btn{border-radius:8px;padding:9px 14px;font-size:11px;font-weight:600;border:1px solid #334155;cursor:pointer}
    .btn:disabled{opacity:.4;cursor:not-allowed}.btn-secondary{background:#111c2d;color:#cbd5e1}.btn-ghost{background:transparent;color:#94a3b8;border-color:transparent}.btn-primary{background:#6366f1;border-color:#6366f1;color:white}.btn-primary:hover:not(:disabled){background:#5558df}
    .success-state{max-width:520px;margin:auto;text-align:center;padding:35px 0}.success-icon{width:48px;height:48px;display:grid;place-items:center;margin:0 auto 18px;border-radius:50%;background:#12362d;color:#34d399;font-size:22px}
    .success-state h2{font-size:21px}.success-state p{font-size:12px;color:#94a3b8;line-height:1.7;margin-top:10px}
    .summary-actions{display:flex;justify-content:center;gap:9px;margin-top:20px}
    @media(max-width:850px){.wizard-shell{grid-template-columns:1fr}.step-rail{border-right:0;border-bottom:1px solid #253149;padding:14px}.rail-title,.rail-note{display:none}.step-item{display:inline-flex;width:calc(25% - 5px);vertical-align:top;margin:0 2px;padding:9px 5px;justify-content:center}.step-copy{display:none}.wizard-content{padding:23px 20px}}
    @media(max-width:560px){.page-heading{gap:10px}.page-heading h1{font-size:20px}.wizard-content{padding:18px 14px}.engine-grid{grid-template-columns:1fr}.engine-option{padding:10px}.field-grid,.choice-grid,.choice-grid.two{grid-template-columns:1fr}.footer-right{gap:8px}.autosave-label{display:none}.summary-actions{flex-direction:column}.wizard-footer{padding-top:16px}}
</style>
