<script>
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { user, isAuthenticated } from '$lib/stores.js';
    import { logout, me } from '$lib/api.js';

    let loading = true;

    // Загружаем пользователя при старте.
    onMount(async () => {
        const res = await me();
        if (res.ok && res.data?.user) {
            user.set(res.data.user);
        }
        loading = false;
    });

    // Редирект на /login, если не авторизован.
    $: if (!loading && !$isAuthenticated && !$page.url.pathname.startsWith('/login') && !$page.url.pathname.startsWith('/register')) {
        goto('/login');
    }

    // Выход.
    async function handleLogout() {
        await logout();
        user.set(null);
        goto('/login');
    }

    // Проверка активного пути.
    function isActive(path) {
        return $page.url.pathname === path || $page.url.pathname.startsWith(path + '/');
    }

    // Хлебные крошки.
    $: pathParts = $page.url.pathname.split('/').filter(Boolean);
</script>

{#if loading}
    <div class="loading-screen">
        <div class="spinner"></div>
    </div>
{:else if !$isAuthenticated}
    <!-- Страницы логина/регистрации без каркаса -->
    <slot />
{:else}
    <div class="layout">
        <!-- Sidebar -->
        <aside class="sidebar">
            <div class="sidebar-header">
                <span class="brand">VELTRIX Dock</span>
            </div>

            <nav class="nav">
                <div class="nav-group">
                    <div class="nav-label">Панель</div>
                    <a
                        href="/"
                        class="nav-item"
                        class:active={$page.url.pathname === '/'}
                    >
                        Устройства
                    </a>
                    <a
                        href="/sites"
                        class="nav-item"
                        class:active={isActive('/sites')}
                    >
                        Сайты
                    </a>
                    <a
                        href="/databases"
                        class="nav-item"
                        class:active={isActive('/databases')}
                    >
                        Базы данных
                    </a>
                </div>

                {#if $user?.username === 'admin'}
                    <div class="nav-group">
                        <div class="nav-label">Админ</div>
                        <a
                            href="/admin"
                            class="nav-item"
                            class:active={isActive('/admin')}
                        >
                            Пользователи
                        </a>
                    </div>
                {/if}

                <div class="nav-group">
                    <div class="nav-label">Аккаунт</div>
                    <a
                        href="/profile"
                        class="nav-item"
                        class:active={isActive('/profile')}
                    >
                        Профиль
                    </a>
                </div>
            </nav>

            <div class="sidebar-footer">
                <div class="user-info">
                    <span class="user-name">{$user?.username}</span>
                    <button class="logout-btn" on:click={handleLogout}>Выйти</button>
                </div>
            </div>
        </aside>

        <!-- Main -->
        <main class="main">
            <header class="topbar">
                <div class="breadcrumbs">
                    {#each pathParts as part, i}
                        {#if i > 0}
                            <span class="divider">/</span>
                        {/if}
                        <span class="breadcrumb" class:active={i === pathParts.length - 1}>
                            {part}
                        </span>
                    {/each}
                    {#if pathParts.length === 0}
                        <span class="breadcrumb active">Главная</span>
                    {/if}
                </div>
            </header>

            <div class="content">
                <div class="container">
                    <slot />
                </div>
            </div>
        </main>
    </div>
{/if}

<style>
    .loading-screen {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100vh;
        background: #000;
    }

    .spinner {
        width: 24px;
        height: 24px;
        border: 2px solid #27272a;
        border-top-color: #fafafa;
        border-radius: 50%;
        animation: spin 0.6s linear infinite;
    }

    @keyframes spin {
        to { transform: rotate(360deg); }
    }

    /* Layout */
    .layout {
        display: flex;
        height: 100vh;
    }

    /* Sidebar */
    .sidebar {
        width: 240px;
        background: #09090b;
        border-right: 1px solid #27272a;
        display: flex;
        flex-direction: column;
        flex-shrink: 0;
    }

    .sidebar-header {
        padding: 16px;
        border-bottom: 1px solid #27272a;
    }

    .brand {
        font-size: 14px;
        font-weight: 600;
        color: #fafafa;
    }

    .nav {
        flex: 1;
        padding: 16px 8px;
        overflow-y: auto;
    }

    .nav-group {
        margin-bottom: 24px;
    }

    .nav-label {
        font-size: 11px;
        font-weight: 500;
        color: #71717a;
        text-transform: uppercase;
        letter-spacing: 0.04em;
        padding: 0 12px;
        margin-bottom: 6px;
    }

    .nav-item {
        display: block;
        padding: 6px 12px;
        border-radius: 6px;
        color: #a1a1aa;
        font-size: 13px;
        transition: background 0.1s, color 0.1s;
    }

    .nav-item:hover {
        background: #18181b;
        color: #fafafa;
    }

    .nav-item.active {
        background: #18181b;
        color: #fafafa;
    }

    .sidebar-footer {
        padding: 12px;
        border-top: 1px solid #27272a;
    }

    .user-info {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 8px;
    }

    .user-name {
        font-size: 13px;
        color: #a1a1aa;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .logout-btn {
        background: none;
        border: none;
        color: #71717a;
        font-size: 12px;
        cursor: pointer;
        padding: 4px 8px;
        border-radius: 4px;
    }

    .logout-btn:hover {
        color: #fafafa;
        background: #18181b;
    }

    /* Main */
    .main {
        flex: 1;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .topbar {
        height: 48px;
        border-bottom: 1px solid #27272a;
        display: flex;
        align-items: center;
        padding: 0 24px;
        gap: 8px;
        flex-shrink: 0;
    }

    .breadcrumbs {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 13px;
    }

    .breadcrumb {
        color: #71717a;
    }

    .breadcrumb.active {
        color: #fafafa;
    }

    .divider {
        color: #27272a;
    }

    .content {
        flex: 1;
        overflow-y: auto;
        padding: 32px;
    }

    .container {
        max-width: 1000px;
        margin: 0 auto;
    }
</style>
