<script>
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { adminListUsers, adminStats, adminGetPlatformSettings, adminSavePlatformSettings } from '$lib/api.js';
    import { notify } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';

    let loading = true;
    let users = [];
    let stats = null;
    let page = 1;
    let total = 0;
    let activeTab = 'overview';
    let platformLoading = false;
    let platformSaving = false;
    let platform = { public_url: '', cloudflare_oauth_client_id: '', cloudflare_oauth_token_auth_method: 'client_secret_post', cloudflare_oauth_secret_set: false, cloudflare_oauth_client_secret: '', clear_cloudflare_oauth_secret: false, callback_url: '', github_app_configured: false, github_app_id: '', github_app_slug: '', github_app_private_key_set: false, github_app_private_key: '', clear_github_app_private_key: false };

    // Загружаем статистику и пользователей.
    onMount(async () => {
        await loadAll();
        await loadPlatformSettings();
        loading = false;
    });

    async function loadAll() {
        const [statsRes, usersRes] = await Promise.all([
            adminStats(),
            adminListUsers(page),
        ]);

        if (statsRes.ok) {
            stats = statsRes.data.stats;
        } else {
            notify(statsRes.error || 'Не удалось загрузить статистику', 'error');
        }

        if (usersRes.ok) {
            users = usersRes.data.users || [];
            total = usersRes.data.total || 0;
        } else {
            notify(usersRes.error || 'Не удалось загрузить пользователей', 'error');
        }
    }


    async function loadPlatformSettings() {
        platformLoading = true;
        const res = await adminGetPlatformSettings();
        platformLoading = false;
        if (!res.ok) { notify(res.error || 'Не удалось загрузить настройки платформы', 'error'); return; }
        platform = { ...platform, ...res.data, cloudflare_oauth_client_secret: '', clear_cloudflare_oauth_secret: false, github_app_private_key: '', clear_github_app_private_key: false };
    }

    async function savePlatformSettings() {
        if (platformSaving) return;
        platformSaving = true;
        const res = await adminSavePlatformSettings({
            public_url: platform.public_url,
            cloudflare_oauth_client_id: platform.cloudflare_oauth_client_id,
            cloudflare_oauth_client_secret: platform.cloudflare_oauth_client_secret,
            cloudflare_oauth_token_auth_method: platform.cloudflare_oauth_token_auth_method,
            clear_cloudflare_oauth_secret: !!platform.clear_cloudflare_oauth_secret,
            github_app_id: platform.github_app_id,
            github_app_slug: platform.github_app_slug,
            github_app_private_key: platform.github_app_private_key,
            clear_github_app_private_key: !!platform.clear_github_app_private_key
        });
        platformSaving = false;
        if (!res.ok) { notify(res.error || 'Не удалось сохранить настройки платформы', 'error'); return; }
        platform = { ...platform, ...res.data, cloudflare_oauth_client_secret: '', clear_cloudflare_oauth_secret: false, github_app_private_key: '', clear_github_app_private_key: false };
        notify('Настройки платформы сохранены', 'success');
    }

    function openUser(user) {
        goto(`/admin/${user.id}`);
    }

    async function changePage(delta) {
        const newPage = page + delta;
        if (newPage < 1) return;
        const maxPage = Math.ceil(total / 50);
        if (newPage > maxPage) return;

        page = newPage;
        loading = true;
        await loadAll();
        loading = false;
    }

    function statusLabel(status) {
        if (status === 'free') return 'Бесплатный';
        if (status === 'commercial') return 'Коммерческий';
        if (status === 'blocked') return 'Заблокирован';
        return status;
    }

    function statusClass(status) {
        if (status === 'commercial') return 'status-commercial';
        if (status === 'blocked') return 'status-blocked';
        return 'status-free';
    }
</script>

<div class="page-header">
    <div>
        <h1 class="page-title">Админка</h1>
        <p class="page-desc">Статистика и управление пользователями</p><Button on:click={async () => { loading = true; await loadAll(); loading = false; }}>Обновить данные</Button>
            <a class="export-link" href="/admin/api/users/export.csv">Экспорт CSV</a>
    </div>
</div>

{#if loading && !stats}
    <div class="empty">Загрузка...</div>
{:else}

    <div class="admin-tabs">
        <button class:active={activeTab === 'overview'} on:click={() => activeTab = 'overview'}>Обзор и пользователи</button>
        <button class:active={activeTab === 'settings'} on:click={() => activeTab = 'settings'}>Настройки платформы</button>
    </div>
    {#if activeTab === 'settings'}
        <div class="platform-settings">
            <div class="section-header"><h2 class="section-title">Основные параметры</h2><Button size="sm" on:click={loadPlatformSettings} disabled={platformLoading}>Обновить</Button></div>
            {#if platformLoading}<div class="empty">Загрузка настроек…</div>{:else}
                <Card>
                    <h3>Публичный адрес Yevqo</h3>
                    <p class="setting-help">Базовый HTTPS URL Control Plane. Используется для callback и внешних интеграций.</p>
                    <label>Public URL</label>
                    <input bind:value={platform.public_url} placeholder="https://app.yevqo.cc.cd" />
                    <label>Cloudflare OAuth callback URL</label>
                    <div class="callback-row"><code>{platform.public_url ? platform.public_url.replace(/\/+$/, '') + '/oauth/cloudflare/callback' : 'Сначала укажите Public URL'}</code><button on:click={() => navigator.clipboard.writeText(platform.public_url.replace(/\/+$/, '') + '/oauth/cloudflare/callback')}>Копировать</button></div>
                </Card>
                <Card>
                    <h3>Cloudflare OAuth</h3>
                    <p class="setting-help">Создайте OAuth Client в Cloudflare Dashboard → Manage Account → OAuth clients. Для SaaS выберите Public visibility (после проверки домена издателя), Authorization Code + refresh_token и client_secret_post или client_secret_basic. Добавьте callback URL выше и scopes: zone.read, dns.write, argotunnel.read, argotunnel.write, offline_access. Secret хранится зашифрованным и никогда не показывается повторно. Сам OAuth-вход пока не активен — callback появится после отдельной реализации OAuth flow.</p>
                    <label>OAuth Client ID</label>
                    <input bind:value={platform.cloudflare_oauth_client_id} placeholder="Cloudflare OAuth Client ID" autocomplete="off" />
                    <label>Token endpoint authentication</label>
                    <select bind:value={platform.cloudflare_oauth_token_auth_method}>
                        <option value="client_secret_post">client_secret_post</option>
                        <option value="client_secret_basic">client_secret_basic</option>
                    </select>
                    <label>OAuth Client Secret</label>
                    <input type="password" bind:value={platform.cloudflare_oauth_client_secret} placeholder={platform.cloudflare_oauth_secret_set ? 'Секрет уже сохранён — введите новый только для замены' : 'Вставьте Client Secret'} autocomplete="new-password" />
                    <div class="secret-state">{#if platform.cloudflare_oauth_secret_set}<span class="configured">Client Secret сохранён</span>{:else}<span>Client Secret ещё не задан</span>{/if}</div>
                    {#if platform.cloudflare_oauth_secret_set}
                        <label class="checkbox-line"><input type="checkbox" bind:checked={platform.clear_cloudflare_oauth_secret} /> Удалить сохранённый Client Secret</label>
                    {/if}
                    <div class="setting-actions"><Button on:click={savePlatformSettings} disabled={platformSaving}>{platformSaving ? 'Сохранение…' : 'Сохранить настройки'}</Button></div>
                </Card>
                <Card>
                    <h3>GitHub App</h3>
                    <p class="setting-help">Параметры используются для установки Yevqo в аккаунты GitHub. Private key шифруется на сервере и не возвращается в браузер. Переменные Termux остаются резервным источником.</p>
                    <label>GitHub App ID</label>
                    <input bind:value={platform.github_app_id} placeholder="Например, 5859351" inputmode="numeric" autocomplete="off" />
                    <label>GitHub App slug</label>
                    <input bind:value={platform.github_app_slug} placeholder="yevqo" autocomplete="off" />
                    <label>GitHub App private key (PEM)</label>
                    <textarea bind:value={platform.github_app_private_key} rows="6" placeholder={platform.github_app_private_key_set ? 'Ключ уже сохранён — вставьте новый только для замены' : '-----BEGIN RSA PRIVATE KEY-----'} spellcheck="false"></textarea>
                    <div class="secret-state">{#if platform.github_app_private_key_set}<span class="configured">Private key сохранён</span>{:else}<span>Private key ещё не задан</span>{/if}</div>
                    {#if platform.github_app_private_key_set}
                        <label class="checkbox-line"><input type="checkbox" bind:checked={platform.clear_github_app_private_key} /> Удалить сохранённый private key</label>
                    {/if}
                    <div class="setting-actions"><Button on:click={savePlatformSettings} disabled={platformSaving}>{platformSaving ? 'Сохранение…' : 'Сохранить настройки'}</Button></div>
                </Card>
            {/if}
        </div>
    {:else}
    <!-- Статистика -->
    {#if stats}
        <div class="stats-grid">
            <Card padding="sm">
                <div class="stat-label">Пользователей</div>
                <div class="stat-value">{stats.total_users || 0}</div>
            </Card>
            <Card padding="sm">
                <div class="stat-label">Коммерческих</div>
                <div class="stat-value">{stats.commercial_users || 0}</div>
            </Card>
            <Card padding="sm">
                <div class="stat-label">Заблокированных</div>
                <div class="stat-value">{stats.blocked_users || 0}</div>
            </Card>
            <Card padding="sm">
                <div class="stat-label">Устройств</div>
                <div class="stat-value">{stats.total_devices || 0}</div>
            </Card>
            <Card padding="sm">
                <div class="stat-label">Сайтов</div>
                <div class="stat-value">{stats.total_sites || 0}</div>
            </Card>
            <Card padding="sm">
                <div class="stat-label">Запущенных</div>
                <div class="stat-value">{stats.running_sites || 0}</div>
            </Card>
            <Card padding="sm">
                <div class="stat-label">Онлайн</div>
                <div class="stat-value">{stats.online_devices || 0}</div>
            </Card>
        </div>
    {/if}

    <!-- Пользователи -->
    <div class="section-header">
        <h2 class="section-title">Пользователи</h2>
        <span class="section-count">всего: {total}</span>
    </div>

    {#if users.length === 0}
        <div class="empty">
            <p>Пользователей пока нет</p>
        </div>
    {:else}
        <div class="users-list">
            {#each users as user (user.id)}
                <Card clickable on:click={() => openUser(user)}>
                    <div class="user-row">
                        <div class="user-info">
                            <div class="user-name">
                                {user.username}
                                <span class="user-status {statusClass(user.status)}">
                                    {statusLabel(user.status)}
                                </span>
                            </div>
                            <div class="user-meta">
                                {user.email} · ID: {user.id}
                            </div>
                        </div>
                        <div class="user-stats">
                            <div class="user-stat">
                                <div class="user-stat-label">Устройств</div>
                                <div class="user-stat-value">{user.max_devices + user.bonus_devices}</div>
                            </div>
                            <div class="user-stat">
                                <div class="user-stat-label">Создан</div>
                                <div class="user-stat-value">{new Date(user.created_at).toLocaleDateString()}</div>
                            </div>
                        </div>
                    </div>
                </Card>
            {/each}
        </div>

        <!-- Пагинация -->
        {#if total > 50}
            <div class="pagination">
                <Button size="sm" disabled={page === 1} on:click={() => changePage(-1)}>
                    Назад
                </Button>
                <span class="page-info">Страница {page} из {Math.ceil(total / 50)}</span>
                <Button size="sm" disabled={page >= Math.ceil(total / 50)} on:click={() => changePage(1)}>
                    Вперёд
                </Button>
            </div>
        {/if}
    {/if}
{/if}
{/if}

<style>
    .admin-tabs { display:flex; gap:8px; margin: 18px 0 24px; border-bottom:1px solid var(--border); }
    .admin-tabs button { background:transparent; color:var(--text-muted); border:0; border-bottom:2px solid transparent; padding:10px 12px; cursor:pointer; }
    .admin-tabs button.active { color:var(--text-main); border-color:var(--accent, #8AB4FF); }
    .platform-settings { display:flex; flex-direction:column; gap:12px; max-width:860px; }
    .platform-settings h3 { margin:0 0 8px; font-size:14px; }
    .platform-settings label { display:block; margin:12px 0 6px; color:var(--text-muted); font-size:12px; }
     .platform-settings input:not([type=checkbox]), .platform-settings textarea, .platform-settings select { width:100%; box-sizing:border-box; padding:10px 12px; border:1px solid var(--border); border-radius:8px; background:var(--bg-surface); color:var(--text-main); }
    .setting-help { color:var(--text-muted); font-size:12px; line-height:1.6; margin:4px 0 12px; }
    .callback-row { display:flex; gap:8px; align-items:center; flex-wrap:wrap; }
    .callback-row code { flex:1; padding:10px; background:var(--bg-elevated); border-radius:7px; overflow-wrap:anywhere; }
    .callback-row button { padding:8px 10px; border:1px solid var(--border); background:transparent; color:var(--text-main); border-radius:7px; }
    .secret-state { font-size:12px; color:var(--text-muted); margin-top:10px; }
    .configured { color:var(--success); }
    .checkbox-line { display:flex!important; align-items:center; gap:8px; }
    .setting-actions { display:flex; justify-content:flex-end; margin-top:16px; }
    .export-link {
        display: inline-flex;
        align-items: center;
        margin-top: 10px;
        padding: 8px 12px;
        border: 1px solid var(--border);
        border-radius: 8px;
        color: var(--text-main);
        text-decoration: none;
        font-size: 12px;
    }
    .export-link:hover { background: var(--bg-elevated); color: #fff; }

    .stats-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
        gap: 8px;
        margin-bottom: 32px;
    }

    .stat-label {
        font-size: 11px;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }

    .stat-value {
        font-size: 20px;
        font-weight: 600;
        color: var(--text-main);
        margin-top: 4px;
        font-family: ui-monospace, monospace;
    }

    .section-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 16px;
    }

    .section-title {
        font-size: 14px;
        font-weight: 600;
        color: var(--text-main);
    }

    .section-count {
        font-size: 12px;
        color: var(--text-muted);
    }

    .users-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .user-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 16px;
    }

    .user-info {
        min-width: 0;
        flex: 1;
    }

    .user-name {
        font-size: 14px;
        font-weight: 500;
        color: var(--text-main);
        display: flex;
        align-items: center;
        gap: 8px;
        flex-wrap: wrap;
    }

    .user-status {
        font-size: 10px;
        padding: 2px 6px;
        border-radius: 4px;
        font-weight: 500;
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }

    .status-free {
        background: var(--bg-elevated);
        color: var(--text-muted);
    }

    .status-commercial {
        background: rgba(34, 197, 94, 0.1);
        color: var(--success);
    }

    .status-blocked {
        background: rgba(239, 68, 68, 0.1);
        color: var(--danger);
    }

    .user-meta {
        font-family: ui-monospace, monospace;
        font-size: 11px;
        color: var(--text-muted);
        margin-top: 4px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .user-stats {
        display: flex;
        gap: 24px;
        flex-shrink: 0;
    }

    .user-stat {
        text-align: right;
    }

    .user-stat-label {
        font-size: 11px;
        color: var(--text-muted);
    }

    .user-stat-value {
        font-size: 13px;
        color: var(--text-muted);
        font-family: ui-monospace, monospace;
        margin-top: 2px;
    }

    .pagination {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 16px;
        margin-top: 24px;
    }

    .page-info {
        font-size: 12px;
        color: var(--text-muted);
    }
</style>
