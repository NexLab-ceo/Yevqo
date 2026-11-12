<script>
    import { onMount } from 'svelte';
    import { page as route } from '$app/stores';
    import { goto } from '$app/navigation';
    import { getDatabase, listDatabaseConnections, listSites, attachDatabaseToSite, removeSiteDatabaseConnection } from '$lib/api.js';
    import StatusDot from '$lib/components/StatusDot.svelte';

    const tabs = ['Overview', 'Connection', 'Users & Permissions', 'Backups', 'Import / Export', 'Logs', 'Settings'];
    let active = 'Overview';
    let database = null;
    let loading = true;
    let error = '';
    let connections = [];
    let sites = [];
    let siteChoice = '';
    let linking = false;

    $: availableSites = database ? sites.filter((entry) => Number(entry.device_id) === Number(database.device_id) && !connections.some((link) => link.site_id === entry.site?.site_id)) : [];

    async function addSiteLink() {
        if (!database || !siteChoice || linking) return;
        linking = true;
        const selected = availableSites.find((entry) => siteChoice === entry.site?.site_id);
        const result = selected ? await attachDatabaseToSite(database.id, selected.device_id, selected.site.site_id) : { ok: false };
        if (result.ok) {
            const links = await listDatabaseConnections(database.id);
            if (links.ok) connections = links.data?.connections || [];
            siteChoice = '';
        } else error = result.error || 'Could not link website';
        linking = false;
    }

    async function removeSiteLink(id) {
        const result = await removeSiteDatabaseConnection(id);
        if (result.ok) connections = connections.filter((item) => item.id !== id);
        else error = result.error || 'Could not remove website link';
    }

    onMount(async () => {
        const result = await getDatabase($route.params.id);
        if (result.ok) {
            database = result.data?.database;
            const [linkResult, siteResult] = await Promise.all([listDatabaseConnections(database.id), listSites()]);
            if (linkResult.ok) connections = linkResult.data?.connections || [];
            if (siteResult.ok) sites = siteResult.data?.sites || [];
        } else error = result.error || 'Не удалось загрузить базу данных';
        loading = false;
    });
</script>

<svelte:head><title>{database ? database.name + ' · Database · VELTRIX Dock' : 'Database · VELTRIX Dock'}</title></svelte:head>

<div class="page">
    <div class="breadcrumbs"><button on:click={() => goto('/databases')}>Databases</button><span>/</span><span>{database?.name || 'Database details'}</span></div>
    {#if loading}
        <div class="empty">Loading database details…</div>
    {:else if error}
        <div class="error">{error}</div>
    {:else if database}
        <div class="heading">
            <div class="identity">
                <div class="engine">PG</div>
                <div><div class="eyebrow">POSTGRESQL DATABASE</div><h1>{database.name}</h1><p>{database.server_name} · Android Host #{database.device_id}</p></div>
            </div>
            <div class="status"><StatusDot status={database.status === 'ready' ? 'online' : 'offline'} />{database.status}</div>
        </div>
        <nav class="tabs" aria-label="Database sections">
            {#each tabs as tab}
                <button class:active={active === tab} on:click={() => active = tab}>{tab}</button>
            {/each}
        </nav>

        {#if active === 'Overview'}
            <div class="stats">
                <article><small>ENGINE</small><strong>PostgreSQL</strong><span>Server #{database.server_id}</span></article>
                <article><small>CONNECTIONS</small><strong>{database.linked_sites}</strong><span>Linked websites</span></article>
                <article><small>STATUS</small><strong>{database.status}</strong><span>Database health</span></article>
                <article><small>CREATED</small><strong>{new Date(database.created_at).toLocaleDateString()}</strong><span>Creation date</span></article>
            </div>
            <section class="panel"><h2>Database information</h2><div class="rows">
                <div><span>Database name</span><strong>{database.name}</strong></div>
                <div><span>Database Server</span><strong>{database.server_name}</strong></div>
                <div><span>Device</span><strong>Android Host #{database.device_id}</strong></div>
                <div><span>Application role</span><strong>{database.username || 'Not available'}</strong></div>
                <div><span>Website links</span><strong>{database.linked_sites}</strong></div>
            </div></section>
        {:else if active === 'Connection'}
            <section class="panel"><h2>Private connection</h2><p class="muted">This database is bound to localhost on its Android device. Credentials are never returned to the browser.</p>
                <div class="rows">
                    <div><span>Host</span><strong>127.0.0.1</strong></div>
                    <div><span>Port</span><strong>{database.port}</strong></div>
                    <div><span>Database</span><strong>{database.name}</strong></div>
                    <div><span>Username</span><strong>{database.username}</strong></div>
                    <div><span>Password</span><strong>Encrypted · hidden</strong></div>
                    <div><span>Network access</span><strong>Private / localhost</strong></div>
                </div>
                <div class="connection">postgresql://{database.username}@127.0.0.1:{database.port}/{database.name}</div>
                <div class="notice">The connection is provisioned to linked websites as process environment variables. Existing .env files are not modified. Restart a website after changing its database link.</div>
                <div class="link-heading"><strong>Connected websites</strong><span>{connections.length} linked</span></div>
                {#each connections as link}
                    <div class="site-link"><span><strong>{link.site_id}</strong><small>Android Host #{link.device_id}</small></span><button on:click={() => removeSiteLink(link.id)}>Remove</button></div>
                {/each}
                <div class="attach-row"><select bind:value={siteChoice}><option value="">Select website on this device</option>{#each availableSites as entry}<option value={entry.site.site_id}>{entry.site.site_id}</option>{/each}</select><button class="attach" disabled={!siteChoice || linking} on:click={addSiteLink}>{linking ? 'Linking…' : 'Link website'}</button></div>
            </section>
        {:else if active === 'Users & Permissions'}
            <section class="panel"><h2>Users & Permissions</h2><p class="muted">Application roles are separate from the PostgreSQL administrator account.</p>
                <div class="role"><div><strong>{database.username}</strong><small>Application user · not database owner · password encrypted</small></div><span class="role-tag">LOGIN</span></div>
                <div class="permissions">{#each database.permissions || [] as permission}<span>{permission}</span>{/each}</div>
            </section>
        {:else if active === 'Backups'}
            <section class="panel"><h2>Backups</h2><p class="muted">Database backup and restore actions are not enabled yet. No backup has been created automatically.</p><div class="empty compact">Backup engine pending: pg_dump, verification and restore-to-new-database workflow.</div></section>
        {:else if active === 'Import / Export'}
            <section class="panel"><h2>Import / Export</h2><p class="muted">SQL import and export are not enabled yet. Existing database contents are untouched.</p><div class="empty compact">Planned formats: Plain SQL and PostgreSQL custom archive.</div></section>
        {:else if active === 'Logs'}
            <section class="panel"><h2>Database logs</h2><p class="muted">Per-database activity and PostgreSQL query logs are not wired yet.</p><div class="empty compact">Connection, error, backup and permission events will appear here.</div></section>
        {:else}
            <section class="panel"><h2>Database settings</h2><p class="muted">Server-wide parameters are managed on the Database Server. Changes there can affect every database on that server.</p><div class="rows"><div><span>Engine</span><strong>PostgreSQL</strong></div><div><span>Server</span><strong>{database.server_name}</strong></div><div><span>Database owner</span><strong>Server administrator</strong></div></div></section>
        {/if}
    {/if}
</div>

<style>
.page{max-width:1050px;margin:0 auto;padding:25px 0 60px;color:#f1f5f9}.breadcrumbs{display:flex;gap:9px;align-items:center;font-size:10px;color:#64748b;margin-bottom:22px}.breadcrumbs button{background:none;border:0;color:#94a3b8;cursor:pointer;font:inherit}.heading{display:flex;align-items:center;justify-content:space-between;margin-bottom:22px}.identity{display:flex;align-items:center;gap:13px}.engine{width:44px;height:44px;border-radius:11px;background:#1e3a5f;color:#93c5fd;display:grid;place-items:center;font-size:13px;font-weight:800}.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:5px}h1{font-size:22px;margin:0 0 4px}.identity p{font-size:10px;color:#94a3b8;margin:0}.status{display:flex;align-items:center;gap:7px;color:#94a3b8;font-size:10px}.tabs{display:flex;gap:4px;border-bottom:1px solid #253149;overflow:auto;margin-bottom:18px}.tabs button{white-space:nowrap;background:none;border:0;border-bottom:2px solid transparent;color:#94a3b8;padding:11px 12px;font-size:10px;cursor:pointer}.tabs button.active{color:#c7d2fe;border-color:#818cf8}.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;margin-bottom:15px}.stats article,.panel{background:#101727;border:1px solid #253149;border-radius:12px;padding:16px}.stats small{display:block;color:#64748b;font-size:8px;letter-spacing:.12em;margin-bottom:9px}.stats strong{display:block;font-size:16px}.stats span{display:block;color:#64748b;font-size:9px;margin-top:5px}.panel h2{font-size:13px;margin:0 0 8px}.muted{font-size:10px;color:#94a3b8;line-height:1.6}.rows{margin-top:15px}.rows>div{display:flex;justify-content:space-between;gap:20px;padding:11px 0;border-bottom:1px solid #1e293b;font-size:10px}.rows>div:last-child{border:0}.rows span{color:#94a3b8}.rows strong{font-weight:600;text-align:right}.link-heading{display:flex;justify-content:space-between;align-items:center;margin-top:22px;font-size:10px}.link-heading span{color:#94a3b8}.site-link{display:flex;align-items:center;justify-content:space-between;padding:10px 0;border-bottom:1px solid #1e293b}.site-link strong,.site-link small{display:block}.site-link strong{font-size:10px}.site-link small{font-size:9px;color:#94a3b8;margin-top:4px}.site-link button{background:none;border:0;color:#fda4af;font-size:9px;cursor:pointer}.attach-row{display:flex;gap:8px;margin-top:12px}.attach-row select{flex:1;min-width:0;background:#0b1020;border:1px solid #334155;border-radius:8px;padding:9px;color:#f1f5f9;font-size:10px}.attach{border:0;border-radius:8px;padding:9px 12px;background:#6366f1;color:white;font-size:10px;cursor:pointer}.attach:disabled{opacity:.45;cursor:not-allowed}.connection{margin-top:16px;padding:12px;background:#080b14;border:1px solid #253149;border-radius:8px;color:#c7d2fe;font:10px 'JetBrains Mono',monospace;overflow-wrap:anywhere}.notice{margin-top:12px;color:#94a3b8;font-size:10px;line-height:1.6}.role{display:flex;justify-content:space-between;align-items:center;padding:13px 0;border-top:1px solid #253149;border-bottom:1px solid #253149;margin-top:15px}.role strong,.role small{display:block}.role strong{font-size:11px}.role small{font-size:9px;color:#94a3b8;margin-top:4px}.role-tag{font-size:8px;color:#a5b4fc;background:#242b50;padding:5px 8px;border-radius:5px}.permissions{display:flex;flex-wrap:wrap;gap:7px;margin-top:13px}.permissions span{font-size:9px;color:#a7f3d0;background:#12332d;border:1px solid #1f594a;padding:6px 8px;border-radius:6px}.empty{padding:28px;text-align:center;color:#94a3b8;font-size:11px}.empty.compact{padding:18px;margin-top:14px;border:1px dashed #334155;border-radius:8px}.error{padding:14px;color:#fda4af;background:#3f1725;border-radius:8px;font-size:11px}@media(max-width:700px){.stats{grid-template-columns:repeat(2,1fr)}.heading{align-items:flex-start;gap:10px;flex-direction:column}.page{padding-top:15px}.rows>div{align-items:flex-start}.rows strong{max-width:60%;overflow-wrap:anywhere}}
</style>
