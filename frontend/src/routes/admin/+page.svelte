<script>
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { adminListUsers, adminStats } from '$lib/api.js';
    import { notify } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';

    let loading = true;
    let users = [];
    let stats = null;
    let page = 1;
    let total = 0;

    // Загружаем статистику и пользователей.
    onMount(async () => {
        await loadAll();
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
        <p class="page-desc">Статистика и управление пользователями</p>
    </div>
</div>

{#if loading && !stats}
    <div class="empty">Загрузка...</div>
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

<style>
    .stats-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
        gap: 8px;
        margin-bottom: 32px;
    }

    .stat-label {
        font-size: 11px;
        color: #64748B;
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }

    .stat-value {
        font-size: 20px;
        font-weight: 600;
        color: #F1F5F9;
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
        color: #F1F5F9;
    }

    .section-count {
        font-size: 12px;
        color: #64748B;
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
        color: #F1F5F9;
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
        background: #172136;
        color: #94A3B8;
    }

    .status-commercial {
        background: rgba(34, 197, 94, 0.1);
        color: #4ade80;
    }

    .status-blocked {
        background: rgba(239, 68, 68, 0.1);
        color: #f87171;
    }

    .user-meta {
        font-family: ui-monospace, monospace;
        font-size: 11px;
        color: #64748B;
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
        color: #64748B;
    }

    .user-stat-value {
        font-size: 13px;
        color: #94A3B8;
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
        color: #64748B;
    }
</style>
