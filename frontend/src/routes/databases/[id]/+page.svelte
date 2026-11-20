<script>
    import { onMount } from 'svelte';
    import { page as route } from '$app/stores';
    import { goto } from '$app/navigation';
    import { getDatabase, getDatabaseStats, getDatabaseBackupChunk, listDatabaseConnections, listSites, attachDatabaseToSite, removeSiteDatabaseConnection, listBackups, createDatabaseBackup, restoreDatabaseBackup, testDatabaseConnection } from '$lib/api.js';
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
    let actionError = '';
    let databaseBackups = [];
    let backupBusy = false;
    let restoreBackupID = '';
    let restoreName = '';
    let restoreUsername = '';
    let restorePassword = '';
    let restoreSiteID = '';
    let restoreBusy = false;
    let restoreNotice = '';
    let testingConnection = false;
    let connectionTestResult = '';
    let restoreTargetId = '';
    let liveStats = null;
    let statsBusy = false;
    let statsError = '';
    let downloadingBackupID = '';

    function scopedQuery() { const q = new URLSearchParams(); const p = $route.url.searchParams.get('project_id'); const e = $route.url.searchParams.get('environment_id'); if (p) q.set('project_id', p); if (e) q.set('environment_id', e); const query = q.toString(); return query ? '?' + query : ''; }

    $: deviceSites = database ? sites.filter((entry) => Number(entry.device_id) === Number(database.device_id)) : [];
    $: availableSites = deviceSites.filter((entry) => !connections.some((link) => link.site_id === entry.site?.site_id));

    async function addSiteLink() {
        if (!database || !siteChoice || linking) return;
        linking = true;
        const selected = availableSites.find((entry) => siteChoice === entry.site?.site_id);
        const result = selected ? await attachDatabaseToSite(database.id, selected.device_id, selected.site.site_id) : { ok: false };
        if (result.ok) {
            const links = await listDatabaseConnections(database.id);
            if (links.ok) connections = links.data?.connections || [];
            siteChoice = '';
        } else actionError = result.error || 'Could not link website';
        linking = false;
    }

    async function refreshStats() { if(!database||statsBusy)return;statsBusy=true;statsError='';const result=await getDatabaseStats(database.id);statsBusy=false;if(result.ok)liveStats=result.data?.stats||null;else statsError=result.error||'Could not load live statistics'; }
    
    async function runConnectionTest() {
        if (!database || testingConnection) return;
        testingConnection = true; connectionTestResult = '';
        const result = await testDatabaseConnection(database.id);
        testingConnection = false;
        connectionTestResult = result.ok ? 'Connection successful · PostgreSQL returned SELECT 1' : (result.error || 'Connection test failed');
    }

    async function createBackup() {
        if (!database || backupBusy) return;
        backupBusy = true; actionError = '';
        const result = await createDatabaseBackup(database.id);
        if (result.ok) {
            databaseBackups = [{ ...result.data.backup, created_at: new Date().toISOString() }, ...databaseBackups];
        } else actionError = result.error || 'Could not queue database backup';
        backupBusy = false;
    }

    async function restoreBackup() {
        if (!restoreBackupID || !restoreName || !restoreUsername || restorePassword.length < 12 || restoreBusy) return;
        restoreBusy = true; actionError = ''; restoreNotice = '';
        const result = await restoreDatabaseBackup(Number(restoreBackupID), { name: restoreName, username: restoreUsername, password: restorePassword, site_id: restoreSiteID });
        restoreBusy = false;
        if (result.ok) { restorePassword = ''; restoreTargetId = String(result.data?.database?.id || ''); restoreNotice = 'Restore queued. A new independent database is being prepared; the source database and backup remain unchanged.'; }
        else actionError = result.error || 'Could not queue database restore';
    }

    async function downloadBackup(backup) {
        if (!backup || backup.status !== 'verified' || downloadingBackupID) return;
        downloadingBackupID = String(backup.id); actionError = '';
        try {
            const parts = []; let offset = 0; const total = Number(backup.size_bytes);
            while (offset < total) {
                const result = await getDatabaseBackupChunk(backup.id, offset, 262144); if (!result.ok) throw new Error(result.error || 'Backup download failed');
                const encoded = result.data?.data; if (typeof encoded !== 'string') throw new Error('Invalid backup chunk');
                const raw = atob(encoded); if (!raw.length || raw.length > 262144) throw new Error('Invalid backup chunk size');
                const bytes = new Uint8Array(raw.length); for (let i=0;i<raw.length;i++) bytes[i]=raw.charCodeAt(i); parts.push(bytes); offset += bytes.length; if (offset > total) throw new Error('Backup size mismatch');
            }
            if (offset !== total) throw new Error('Incomplete backup download');
            const blob = new Blob(parts,{type:'application/octet-stream'}); const url=URL.createObjectURL(blob); const link=document.createElement('a'); link.href=url; link.download=backup.file_name; document.body.appendChild(link); link.click(); link.remove(); URL.revokeObjectURL(url);
        } catch(e) { actionError=e.message || 'Backup download failed'; } finally { downloadingBackupID=''; }
    }

    async function refreshBackups() {
        const result = await listBackups();
        if (result.ok) databaseBackups = (result.data?.backups || []).filter((item) => item.resource_type === 'database' && item.resource_id === String(database.id));
    }

    async function removeSiteLink(id) {
        const result = await removeSiteDatabaseConnection(id);
        if (result.ok) connections = connections.filter((item) => item.id !== id);
        else actionError = result.error || 'Could not remove website link';
    }

    onMount(async () => {
        const result = await getDatabase($route.params.id);
        if (result.ok) {
            database = result.data?.database;
            restoreName = (database.name + '_restored').slice(0,63);
            restoreUsername = ((database.username || 'app_user') + '_restored').slice(0,63);
            const [linkResult, siteResult, backupResult] = await Promise.all([listDatabaseConnections(database.id), listSites($route.url.searchParams.get('project_id')||''), listBackups()]);
            if (linkResult.ok) connections = linkResult.data?.connections || [];
            if (siteResult.ok) sites = siteResult.data?.sites || [];
            if (backupResult.ok) databaseBackups = (backupResult.data?.backups || []).filter((item) => item.resource_type === 'database' && item.resource_id === String(database.id));
        } else error = result.error || 'Не удалось загрузить базу данных';
        loading = false;
    });
</script>

<svelte:head><title>{database ? database.name + ' · Database · VELTRIX Dock' : 'Database · VELTRIX Dock'}</title></svelte:head>

<div class="page">
    <div class="breadcrumbs"><button on:click={() => goto('/databases'+scopedQuery())}>Databases</button><span>/</span><span>{database?.name || 'Database details'}</span></div>
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
            <div class="status"><StatusDot status={database.status === 'ready' ? 'online' : 'offline'} />{database.status === 'planned' && database.restore_status ? 'restore: ' + database.restore_status : database.status}</div>
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
                <article><small>STATUS</small><strong>{database.status === 'planned' && database.restore_status ? database.restore_status : database.status}</strong><span>Database health</span></article>
                <article><small>CREATED</small><strong>{new Date(database.created_at).toLocaleDateString()}</strong><span>Creation date</span></article>
            </div>
            <section class="panel"><h2>Database information</h2><div class="rows">
                <div><span>Database name</span><strong>{database.name}</strong></div>
                <div><span>Database Server</span><strong>{database.server_name}</strong></div>
                <div><span>Device</span><strong>Android Host #{database.device_id}</strong></div>
                <div><span>Application role</span><strong>{database.username || 'Not available'}</strong></div>
                <div><span>Website links</span><strong>{database.linked_sites}</strong></div>
            </div></section>
            <section class="panel"><div class="backup-heading"><div><h2>Live PostgreSQL statistics</h2><p class="muted">Collected directly from pg_stat_database and pg_database_size on the Android host.</p></div><button class="attach" disabled={statsBusy||database.status!=='ready'} on:click={refreshStats}>{statsBusy?'Refreshing…':'Refresh stats'}</button></div>
                {#if statsError}<div class="error">{statsError}</div>{:else if liveStats}
                <div class="stats"><article><small>DATABASE SIZE</small><strong>{(liveStats.size_bytes/1048576).toFixed(2)} MB</strong></article><article><small>ACTIVE SESSIONS</small><strong>{liveStats.active_connections} / {liveStats.max_connections}</strong></article><article><small>COMMITTED</small><strong>{liveStats.transactions_committed}</strong></article><article><small>ROLLBACKS</small><strong>{liveStats.transactions_rolled_back}</strong></article></div><div class="rows"><div><span>Deadlocks</span><strong>{liveStats.deadlocks}</strong></div><div><span>Captured at</span><strong>{new Date(liveStats.captured_at).toLocaleString()}</strong></div></div>
                {:else}<p class="muted">Press Refresh stats to query live values. The database is not polled automatically.</p>{/if}</section>
        {:else if active === 'Connection'}
            <section class="panel"><h2>Private connection</h2><p class="muted">This database is bound to localhost on its Android device. Credentials are never returned to the browser.</p>
                <div class="rows">
                    <div><span>Host</span><strong>127.0.0.1</strong></div>
                    <div><span>Port</span><strong>{database.port}</strong></div>
                    <div><span>Database</span><strong>{database.name}</strong></div>
                    <div><span>Username</span><strong>{database.username}</strong></div>
                    <div><span>Password</span><strong>Encrypted · hidden</strong></div>
                    <div><span>Network access</span><strong>Private / localhost</strong></div>
                    <div><span>TLS mode</span><strong>{database.ssl_mode}</strong></div>
                </div>
                <div class="connection">postgresql://{database.username}@127.0.0.1:{database.port}/{database.name}?sslmode={database.ssl_mode}</div>
                <div class="test-row"><button class="attach" disabled={testingConnection || database.status !== 'ready'} on:click={runConnectionTest}>{testingConnection ? 'Testing…' : 'Test connection'}</button>{#if connectionTestResult}<span class:good={connectionTestResult.startsWith('Connection successful')}>{connectionTestResult}</span>{/if}</div>
                <div class="notice">The connection is provisioned to linked websites as process environment variables. Existing .env files are not modified. Restart a website after changing its database link.</div>
                {#if actionError}<div class="error">{actionError}</div>{/if}
                <div class="link-heading"><strong>Connected websites</strong><span>{connections.length} linked</span></div>
                {#each connections as link}
                    <div class="site-link"><span><strong>{link.site_id}</strong><small>Android Host #{link.device_id}</small></span><button on:click={() => removeSiteLink(link.id)}>Remove</button></div>
                {/each}
                <div class="attach-row"><select bind:value={siteChoice}><option value="">Select website on this device</option>{#each availableSites as entry}<option value={entry.site.site_id}>{entry.site.site_id}</option>{/each}</select><button class="attach" disabled={!siteChoice || linking} on:click={addSiteLink}>{linking ? 'Linking…' : 'Link website'}</button></div>
            </section>
        {:else if active === 'Users & Permissions'}
            <section class="panel"><h2>Users & Permissions</h2><p class="muted">Application roles are separate from the PostgreSQL administrator account.</p>
                <a class="attach" href={'/databases/servers/'+database.server_id}>Manage server roles and database grants →</a>
                <div class="role"><div><strong>{database.username}</strong><small>Application user · not database owner · password encrypted</small></div><span class="role-tag">LOGIN</span></div>
                <div class="permissions">{#each database.permissions || [] as permission}<span>{permission}</span>{/each}</div>
            </section>
        {:else if active === 'Backups'}
            <section class="panel"><div class="backup-heading"><div><h2>Database backups</h2><p class="muted">Local PostgreSQL custom-format dumps. Each archive is checked with pg_restore before it is marked verified.</p></div><div class="backup-actions"><button on:click={refreshBackups}>Refresh</button><button class="attach" disabled={backupBusy || database.status !== 'ready'} on:click={createBackup}>{backupBusy ? 'Queuing…' : 'Create backup'}</button></div></div>
                {#if databaseBackups.length === 0}<div class="empty compact">No database backups yet.</div>{:else}{#each databaseBackups as backup}<div class="site-link"><span><strong>{backup.file_name}</strong><small>{new Date(backup.created_at).toLocaleString()} · {(backup.size_bytes / 1048576).toFixed(2)} MB</small></span><span class="role-tag">{backup.status}</span><button class="site-link-action" disabled={backup.status!=='verified'||downloadingBackupID!==''} on:click={()=>downloadBackup(backup)}>{downloadingBackupID===String(backup.id)?'Downloading…':'Download'}</button></div>{/each}{/if}
                <div class="notice">Backups stay in private Termux storage on the database device. Restore always creates a new database; it never overwrites the source.</div>
                {#if databaseBackups.some((item) => item.status === 'verified')}
                    <div class="restore-form"><h3>Restore as a new database</h3>
                        <label>Verified backup<select bind:value={restoreBackupID}><option value="">Choose backup</option>{#each databaseBackups.filter((item) => item.status === 'verified') as backup}<option value={String(backup.id)}>{backup.file_name} · {(backup.size_bytes / 1048576).toFixed(2)} MB</option>{/each}</select></label>
                        <div class="restore-fields"><label>New database name<input bind:value={restoreName} maxlength="63" /></label><label>New application username<input bind:value={restoreUsername} maxlength="63" /></label></div>
                        <label>New application password<input type="password" bind:value={restorePassword} minlength="12" maxlength="256" autocomplete="new-password" placeholder="At least 12 characters" /></label>
                        <label>Connect website (optional)<select bind:value={restoreSiteID}><option value="">Do not connect yet</option>{#each deviceSites as site}<option value={site.site.site_id}>{site.site.site_id}</option>{/each}</select></label>
                        <button class="attach" disabled={!restoreBackupID || !/^[a-zA-Z_][a-zA-Z0-9_]{0,62}$/.test(restoreName) || !/^[a-zA-Z_][a-zA-Z0-9_]{0,62}$/.test(restoreUsername) || restorePassword.length < 12 || restoreBusy} on:click={restoreBackup}>{restoreBusy ? 'Queuing restore…' : 'Restore into new database'}</button>
                        {#if restoreNotice}<div class="notice">{restoreNotice} {#if restoreTargetId}<a href={'/databases/'+restoreTargetId}>Open restored database</a>{/if}</div>{/if}
                    </div>
                {/if}</section>
        {:else if active === 'Import / Export'}
            <section class="panel"><div class="backup-heading"><div><h2>Import / Export</h2><p class="muted">Export a consistent PostgreSQL custom-format archive using pg_dump. Export files are stored privately on the database device and can be restored into a new database.</p></div><button class="attach" disabled={backupBusy || database.status !== 'ready'} on:click={createBackup}>{backupBusy ? 'Exporting…' : 'Export database'}</button></div>
                <div class="notice">Exports use the same verified backup pipeline as scheduled backups. The source database is read only and is never replaced.</div>
                <div class="link-heading"><strong>Available export archives</strong><button class="site-link-action" on:click={refreshBackups}>Refresh</button></div>
                {#if databaseBackups.length === 0}<div class="empty compact">No export archives yet. Select Export database to create one.</div>{:else}{#each databaseBackups as backup}<div class="site-link"><span><strong>{backup.file_name}</strong><small>{new Date(backup.created_at).toLocaleString()} · {(backup.size_bytes / 1048576).toFixed(2)} MB</small></span><span class="role-tag">{backup.status}</span></div>{/each}{/if}
                <div class="restore-form"><h3>Import archive</h3><p class="muted">To import an existing archive, use Backups → Restore as a new database. Direct SQL file upload is not available yet.</p><button class="attach" on:click={() => active = 'Backups'}>Open restore workflow</button></div>
                {#if actionError}<div class="error">{actionError}</div>{/if}
            </section>
        {:else if active === 'Logs'}
            <section class="panel"><h2>Database activity</h2><p class="muted">Database lifecycle and backup events are recorded in the account activity log. PostgreSQL query logs are not collected by the agent.</p><a class="attach" href="/logs">Open account activity logs</a></section>
        {:else}
            <section class="panel"><h2>Database settings</h2><p class="muted">Server-wide parameters are managed on the Database Server. Changes there can affect every database on that server.</p><div class="rows"><div><span>Engine</span><strong>PostgreSQL</strong></div><div><span>Server</span><strong>{database.server_name}</strong></div><div><span>Database owner</span><strong>Server administrator</strong></div></div></section>
        {/if}
    {/if}
</div>

<style>
.page{max-width:1050px;margin:0 auto;padding:25px 0 60px;color:#f1f5f9}.breadcrumbs{display:flex;gap:9px;align-items:center;font-size:10px;color:#64748b;margin-bottom:22px}.breadcrumbs button{background:none;border:0;color:#94a3b8;cursor:pointer;font:inherit}.heading{display:flex;align-items:center;justify-content:space-between;margin-bottom:22px}.identity{display:flex;align-items:center;gap:13px}.engine{width:44px;height:44px;border-radius:11px;background:#1e3a5f;color:#93c5fd;display:grid;place-items:center;font-size:13px;font-weight:800}.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:5px}h1{font-size:22px;margin:0 0 4px}.identity p{font-size:10px;color:#94a3b8;margin:0}.status{display:flex;align-items:center;gap:7px;color:#94a3b8;font-size:10px}.tabs{display:flex;gap:4px;border-bottom:1px solid #253149;overflow:auto;margin-bottom:18px}.tabs button{white-space:nowrap;background:none;border:0;border-bottom:2px solid transparent;color:#94a3b8;padding:11px 12px;font-size:10px;cursor:pointer}.tabs button.active{color:#c7d2fe;border-color:#818cf8}.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;margin-bottom:15px}.stats article,.panel{background:#101727;border:1px solid #253149;border-radius:12px;padding:16px}.stats small{display:block;color:#64748b;font-size:8px;letter-spacing:.12em;margin-bottom:9px}.stats strong{display:block;font-size:16px}.stats span{display:block;color:#64748b;font-size:9px;margin-top:5px}.panel h2{font-size:13px;margin:0 0 8px}.muted{font-size:10px;color:#94a3b8;line-height:1.6}.rows{margin-top:15px}.rows>div{display:flex;justify-content:space-between;gap:20px;padding:11px 0;border-bottom:1px solid #1e293b;font-size:10px}.rows>div:last-child{border:0}.rows span{color:#94a3b8}.rows strong{font-weight:600;text-align:right}.restore-form{margin-top:20px;padding-top:18px;border-top:1px solid #253149;display:grid;gap:12px}.restore-form h3{font-size:11px;margin:0}.restore-form label{display:grid;gap:6px;color:#94a3b8;font-size:9px}.restore-form input,.restore-form select{background:#0b1020;border:1px solid #334155;border-radius:8px;padding:9px;color:#f1f5f9;font-size:10px}.restore-fields{display:grid;grid-template-columns:1fr 1fr;gap:10px}.backup-heading{display:flex;justify-content:space-between;gap:14px;align-items:flex-start}.backup-actions{display:flex;gap:7px;flex-shrink:0}.backup-actions button:first-child{background:#172136;color:#cbd5e1;border:0;border-radius:8px;padding:9px 11px;font-size:10px;cursor:pointer}.link-heading{display:flex;justify-content:space-between;align-items:center;margin-top:22px;font-size:10px}.link-heading span{color:#94a3b8}.site-link-action{background:none;border:0;color:#a5b4fc;font-size:9px;cursor:pointer}.site-link{display:flex;align-items:center;justify-content:space-between;padding:10px 0;border-bottom:1px solid #1e293b}.site-link strong,.site-link small{display:block}.site-link strong{font-size:10px}.site-link small{font-size:9px;color:#94a3b8;margin-top:4px}.site-link button{background:none;border:0;color:#fda4af;font-size:9px;cursor:pointer}.attach-row{display:flex;gap:8px;margin-top:12px}.attach-row select{flex:1;min-width:0;background:#0b1020;border:1px solid #334155;border-radius:8px;padding:9px;color:#f1f5f9;font-size:10px}.attach{border:0;border-radius:8px;padding:9px 12px;background:#6366f1;color:white;font-size:10px;cursor:pointer}.attach:disabled{opacity:.45;cursor:not-allowed}.test-row{display:flex;align-items:center;gap:12px;margin-top:12px}.test-row span{font-size:10px;color:#fda4af}.test-row span.good{color:#6ee7b7}.connection{margin-top:16px;padding:12px;background:#080b14;border:1px solid #253149;border-radius:8px;color:#c7d2fe;font:10px 'JetBrains Mono',monospace;overflow-wrap:anywhere}.notice{margin-top:12px;color:#94a3b8;font-size:10px;line-height:1.6}.role{display:flex;justify-content:space-between;align-items:center;padding:13px 0;border-top:1px solid #253149;border-bottom:1px solid #253149;margin-top:15px}.role strong,.role small{display:block}.role strong{font-size:11px}.role small{font-size:9px;color:#94a3b8;margin-top:4px}.role-tag{font-size:8px;color:#a5b4fc;background:#242b50;padding:5px 8px;border-radius:5px}.permissions{display:flex;flex-wrap:wrap;gap:7px;margin-top:13px}.permissions span{font-size:9px;color:#a7f3d0;background:#12332d;border:1px solid #1f594a;padding:6px 8px;border-radius:6px}.empty{padding:28px;text-align:center;color:#94a3b8;font-size:11px}.empty.compact{padding:18px;margin-top:14px;border:1px dashed #334155;border-radius:8px}.error{padding:14px;color:#fda4af;background:#3f1725;border-radius:8px;font-size:11px}@media(max-width:700px){.stats{grid-template-columns:repeat(2,1fr)}.heading{align-items:flex-start;gap:10px;flex-direction:column}.page{padding-top:15px}.rows>div{align-items:flex-start}.rows strong{max-width:60%;overflow-wrap:anywhere}}
</style>
