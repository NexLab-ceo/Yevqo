<script>
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { user, isAuthenticated } from '$lib/stores.js';
    import { logout, me } from '$lib/api.js';

    let loading = true;
    let mobileNavOpen = false;

    const navigation = [
        { label: 'WORKSPACE', items: [
            { title: 'Overview', href: '/', icon: '◫', active: 'exact' },
            { title: 'Projects', href: '/projects', icon: '▦', soon: true }
        ]},
        { label: 'INFRASTRUCTURE', items: [
            { title: 'Devices', href: '/devices', icon: '▣', active: 'root' },
            { title: 'Websites', href: '/sites', icon: '◉' },
            { title: 'Databases', href: '/databases', icon: '◈' },
            { title: 'Deployments', href: '/deployments', icon: '⇧', soon: true },
            { title: 'Domains', href: '/domains', icon: '◎', soon: true }
        ]},
        { label: 'OPERATIONS', items: [
            { title: 'Monitoring', href: '/monitoring', icon: '⌁', soon: true },
            { title: 'Logs', href: '/logs', icon: '≡', soon: true },
            { title: 'Notifications', href: '/notifications', icon: '♧', soon: true },
            { title: 'Reports & Export', href: '/reports', icon: '▤', soon: true }
        ]},
        { label: 'MANAGEMENT', items: [
            { title: 'Team & Permissions', href: '/team', icon: '♙', soon: true },
            { title: 'Integrations', href: '/integrations', icon: '⌘', soon: true },
            { title: 'Backup & Restore', href: '/backups', icon: '⤓', soon: true },
            { title: 'Maintenance', href: '/maintenance', icon: '⚙', soon: true }
        ]}
    ];

    onMount(async () => {
        const res = await me();
        if (res.ok && res.data?.user) user.set(res.data.user);
        loading = false;
    });

    $: if (!loading && !$isAuthenticated && !$page.url.pathname.startsWith('/login') && !$page.url.pathname.startsWith('/register')) goto('/login');

    async function handleLogout() {
        await logout();
        user.set(null);
        goto('/login');
    }

    function isActive(item) {
        if (item.active === 'exact') return $page.url.pathname === item.href;
        if (item.active === 'root') return $page.url.pathname === item.href || $page.url.pathname.startsWith(item.href + '/');
        return $page.url.pathname === item.href || $page.url.pathname.startsWith(item.href + '/');
    }

    $: currentTitle = [...navigation.flatMap(group => group.items)].find(item => isActive(item))?.title || 'VELTRIX Dock';
    $: pathParts = $page.url.pathname.split('/').filter(Boolean);
</script>

<svelte:head><meta name="theme-color" content="#080B14" /></svelte:head>

{#if loading}
    <div class="loading-screen"><div class="spinner"></div><span>VELTRIX Dock</span></div>
{:else if !$isAuthenticated}
    <slot />
{:else}
    <div class="app-shell">
        {#if mobileNavOpen}
            <button class="mobile-scrim" aria-label="Закрыть меню" on:click={() => mobileNavOpen = false}></button>
        {/if}
        <aside class:mobile-open={mobileNavOpen} class="sidebar">
            <a class="brand-lockup" href="/">
                <span class="brand-mark">V</span>
                <span class="brand-copy"><strong>VELTRIX</strong><small>DOCK</small></span>
                <span class="brand-version">BETA</span>
            </a>

            <div class="workspace-switcher">
                <span class="workspace-avatar">N</span>
                <span class="workspace-copy"><strong>Personal workspace</strong><small>Free plan</small></span>
                <span class="workspace-chevron">⌄</span>
            </div>

            <nav class="sidebar-nav">
                {#each navigation as group}
                    <section class="nav-group">
                        <div class="nav-label">{group.label}</div>
                        {#each group.items as item}
                            {#if item.soon}
                                <div class="nav-item nav-disabled" title="Раздел будет подключён в следующих этапах">
                                    <span class="nav-icon">{item.icon}</span><span>{item.title}</span><small>SOON</small>
                                </div>
                            {:else}
                                <a href={item.href} class:active={isActive(item)} class="nav-item" on:click={() => mobileNavOpen = false}>
                                    <span class="nav-icon">{item.icon}</span><span>{item.title}</span>
                                    {#if item.title === 'Databases'}<span class="nav-count">NEW</span>{/if}
                                </a>
                            {/if}
                        {/each}
                    </section>
                {/each}
                {#if $user?.username === 'admin'}
                    <section class="nav-group">
                        <div class="nav-label">ADMINISTRATION</div>
                        <a href="/admin" class:active={isActive({href:'/admin'})} class="nav-item"><span class="nav-icon">♜</span><span>Admin panel</span></a>
                    </section>
                {/if}
            </nav>

            <div class="sidebar-bottom">
                <a href="/settings" class="nav-item nav-settings"><span class="nav-icon">⚙</span><span>Settings</span></a>
                <div class="account-row">
                    <span class="account-avatar">{($user?.username || 'U').slice(0,1).toUpperCase()}</span>
                    <span class="account-copy"><strong>{$user?.username}</strong><small>{$user?.email || 'Personal account'}</small></span>
                    <button class="account-menu" aria-label="Выйти" on:click={handleLogout}>↗</button>
                </div>
            </div>
        </aside>

        <main class="main-area">
            <header class="topbar">
                <div class="topbar-left">
                    <button class="mobile-menu" aria-label="Открыть меню" on:click={() => mobileNavOpen = true}>☰</button>
                    <div class="breadcrumbs"><span>Workspace</span><b>/</b><strong>{currentTitle}</strong></div>
                </div>
                <div class="topbar-actions">
                    <span class="connection-state"><i></i> Control plane online</span>
                    <button class="icon-action" aria-label="Уведомления" title="Уведомления">♧</button>
                    <a class="profile-action" href="/profile">{($user?.username || 'U').slice(0,1).toUpperCase()}</a>
                </div>
            </header>
            <div class="page-scroll"><div class="page-container"><slot /></div></div>
        </main>
    </div>
{/if}

<style>
    .loading-screen{min-height:100vh;display:flex;align-items:center;justify-content:center;gap:12px;background:#080b14;color:#94a3b8;font-size:12px}
    .spinner{width:18px;height:18px;border:2px solid #253149;border-top-color:#818cf8;border-radius:50%;animation:spin .7s linear infinite}
    @keyframes spin{to{transform:rotate(360deg)}}
    .app-shell{display:flex;min-height:100vh;background:#080b14}
    .sidebar{width:252px;flex:0 0 252px;background:#0b1020;border-right:1px solid #253149;display:flex;flex-direction:column;padding:18px 12px 12px;min-height:100vh}
    .brand-lockup{height:42px;display:flex;align-items:center;gap:10px;padding:0 8px;margin-bottom:18px}
    .brand-mark{width:30px;height:30px;display:grid;place-items:center;border-radius:9px;background:linear-gradient(145deg,#818cf8,#4f46e5);color:#fff;font-weight:800;font-size:16px;box-shadow:0 5px 18px #4f46e544}
    .brand-copy{display:flex;align-items:baseline;gap:5px;letter-spacing:.08em}.brand-copy strong{font-size:13px;color:#f1f5f9}.brand-copy small{font-size:9px;color:#818cf8;font-weight:700;letter-spacing:.16em}
    .brand-version{margin-left:auto;border:1px solid #334155;color:#94a3b8;border-radius:5px;padding:2px 5px;font-size:8px;letter-spacing:.08em}
    .workspace-switcher{display:flex;align-items:center;gap:9px;padding:10px;background:#101727;border:1px solid #253149;border-radius:10px;margin:0 2px 22px}
    .workspace-avatar,.account-avatar{width:28px;height:28px;border-radius:8px;display:grid;place-items:center;background:#1e293b;color:#c7d2fe;font-weight:700;font-size:11px;flex-shrink:0}
    .workspace-copy,.account-copy{display:flex;flex-direction:column;min-width:0;flex:1}.workspace-copy strong,.account-copy strong{font-size:11px;color:#e2e8f0;font-weight:600}.workspace-copy small,.account-copy small{font-size:10px;color:#64748b;margin-top:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .workspace-chevron{color:#64748b;font-size:15px}
    .sidebar-nav{overflow:auto;flex:1;scrollbar-width:thin}.nav-group{margin-bottom:20px}.nav-label{padding:0 10px;margin:0 0 7px;color:#64748b;font-size:9px;font-weight:700;letter-spacing:.13em}
    .nav-item{min-height:34px;padding:7px 10px;border-radius:7px;display:flex;align-items:center;gap:10px;color:#94a3b8;font-size:11px;font-weight:500;margin:2px 0;transition:background .15s,color .15s}
    .nav-item:hover{background:#172136;color:#e2e8f0}.nav-item.active{background:#242b50;color:#c7d2fe;box-shadow:inset 2px 0 #818cf8}
    .nav-icon{width:16px;text-align:center;font-size:14px;color:#64748b}.nav-item.active .nav-icon{color:#a5b4fc}.nav-disabled{opacity:.48;cursor:default}.nav-disabled small{margin-left:auto;font-size:8px;color:#64748b;letter-spacing:.08em}.nav-count{margin-left:auto;font-size:8px;color:#7dd3fc;background:#0c3140;border:1px solid #164e63;border-radius:4px;padding:2px 4px}
    .sidebar-bottom{border-top:1px solid #253149;padding-top:10px}.nav-settings{margin-bottom:10px}.account-row{display:flex;align-items:center;gap:9px;padding:9px 6px 3px;border-top:1px solid #1e293b}.account-menu{background:transparent;border:0;color:#64748b;font-size:15px;padding:5px}
    .main-area{min-width:0;flex:1;display:flex;flex-direction:column;min-height:100vh}.topbar{height:54px;flex:0 0 54px;border-bottom:1px solid #1e293b;background:#0b1020cc;display:flex;align-items:center;justify-content:space-between;padding:0 28px;backdrop-filter:blur(12px)}
    .topbar-left,.topbar-actions,.breadcrumbs{display:flex;align-items:center;gap:12px}.breadcrumbs{font-size:11px}.breadcrumbs span{color:#64748b}.breadcrumbs b{color:#334155;font-weight:400}.breadcrumbs strong{color:#e2e8f0;font-weight:600}
    .connection-state{font-size:10px;color:#94a3b8;display:flex;align-items:center;gap:7px}.connection-state i{width:6px;height:6px;border-radius:50%;background:#34d399;box-shadow:0 0 8px #34d39988}
    .icon-action,.profile-action{width:28px;height:28px;border:1px solid #253149;background:#101727;color:#94a3b8;border-radius:7px;display:grid;place-items:center;font-size:12px}.profile-action{background:#312e81;color:#c7d2fe;border-color:#4338ca;font-size:10px;font-weight:700}
    .page-scroll{flex:1;overflow:auto;padding:28px}.page-container{max-width:1440px;margin:0 auto;width:100%}.mobile-menu,.mobile-scrim{display:none}
    @media(max-width:900px){.sidebar{width:220px;flex-basis:220px}.page-scroll{padding:22px}}
    @media(max-width:700px){.sidebar{position:fixed;z-index:50;inset:0 auto 0 0;width:270px;transform:translateX(-100%);transition:transform .2s;box-shadow:20px 0 60px #0008}.sidebar.mobile-open{transform:translateX(0)}.mobile-scrim{display:block;position:fixed;inset:0;background:#020617aa;border:0;z-index:40}.mobile-menu{display:grid;place-items:center;width:30px;height:30px;border:1px solid #253149;background:#101727;color:#cbd5e1;border-radius:7px}.topbar{padding:0 14px}.page-scroll{padding:18px 14px}.connection-state{display:none}}
</style>