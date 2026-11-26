<script>
    import { onMount } from 'svelte';
    import { goto } from '$app/navigation';
    import { page } from '$app/stores';
    import { listDatabaseServers, listSites, createDatabase } from '$lib/api.js';
    import { notify } from '$lib/stores.js';

    let servers = [];
    let sites = [];
    let loading = true;
    let submitting = false;
    let error = '';
    let serverId = '';
    let name = 'vertlix_db';
    let username = 'vertlix_user';
    let password = '';
    let siteKey = '';

    $: readyServers = servers.filter((server) => server.engine === 'postgresql' && server.status === 'ready');
    $: selectedServer = readyServers.find((server) => String(server.id) === String(serverId));
    $: availableSites = sites.filter((entry) => !selectedServer || Number(entry.device_id) === Number(selectedServer.device_id));
    $: validIdentifier = /^[a-zA-Z_][a-zA-Z0-9_]{0,62}$/;
    $: canCreate = !!selectedServer && validIdentifier.test(name) && validIdentifier.test(username) && password.length >= 12;

    function scopedQuery() { const q = new URLSearchParams(); const p = $page.url.searchParams.get('project_id'); const e = $page.url.searchParams.get('environment_id'); if (p) q.set('project_id', p); if (e) q.set('environment_id', e); const query = q.toString(); return query ? '?' + query : ''; }

    onMount(async () => {
        const projectId = $page.url.searchParams.get('project_id') || '';
        const environmentId = $page.url.searchParams.get('environment_id') || '';
        const [serverResult, siteResult] = await Promise.all([listDatabaseServers(projectId, environmentId), listSites(projectId)]);
        if (serverResult.ok) {
            servers = serverResult.data?.servers || [];
            const first = servers.find((server) => server.engine === 'postgresql' && server.status === 'ready');
            if (first) serverId = String(first.id);
        } else error = serverResult.error || 'Не удалось загрузить Database Servers';
        if (siteResult.ok) sites = siteResult.data?.sites || [];
        loading = false;
    });

    async function submit() {
        if (!canCreate || submitting) return;
        submitting = true; error = '';
        const selected = availableSites.find((entry) => siteKey === `${entry.device_id}:${entry.site?.site_id}`);
        const payload = {
            server_id: Number(serverId), name, username, password,
            ...($page.url.searchParams.get('project_id') ? { project_id: Number($page.url.searchParams.get('project_id')) } : {}),
            ...($page.url.searchParams.get('environment_id') ? { environment_id: Number($page.url.searchParams.get('environment_id')) } : {}),
            ...(selected ? { site_device_id: Number(selected.device_id), site_id: selected.site.site_id } : {})
        };
        const result = await createDatabase(payload);
        submitting = false;
        if (!result.ok) { error = result.error || 'Не удалось создать базу данных'; return; }
        password = '';
        notify('Database and application role created', 'success');
        goto('/databases'+($page.url.searchParams.get('project_id')?'?project_id='+encodeURIComponent($page.url.searchParams.get('project_id')):'' )+($page.url.searchParams.get('environment_id')?'&environment_id='+encodeURIComponent($page.url.searchParams.get('environment_id')):''));
    }
</script>

<svelte:head>
    <title>Create Database · VELTRIX Dock</title>
    <meta name="description" content="Create a PostgreSQL database and application role." />
</svelte:head>

<div class="page">
    <div class="heading">
        <div>
            <div class="eyebrow">DATABASES <span>/</span> NEW DATABASE</div>
            <h1>Create Database</h1>
            <p>Create an independent database inside an existing PostgreSQL server.</p>
        </div>
        <button class="secondary" on:click={() => goto('/databases'+scopedQuery())}>Cancel</button>
    </div>

    {#if loading}
        <div class="panel muted">Loading available database servers…</div>
    {:else if readyServers.length === 0}
        <div class="panel">
            <h2>No ready PostgreSQL server</h2>
            <p class="muted">Create or install a PostgreSQL Database Server first. Existing databases and server data will not be modified.</p>
            <button class="primary" on:click={() => goto('/databases/servers/new'+scopedQuery())}>Add Database Server</button>
        </div>
    {:else}
        <form class="panel" on:submit|preventDefault={submit}>
            <div class="section-title">Database details</div>
            <label>Database Server
                <select bind:value={serverId}>
                    {#each readyServers as server}
                        <option value={String(server.id)}>{server.name} · {server.engine} · Device #{server.device_id} · :{server.port}</option>
                    {/each}
                </select>
            </label>
            <div class="two">
                <label>Database name
                    <input bind:value={name} maxlength="63" autocomplete="off" spellcheck="false" />
                    <small>PostgreSQL identifier: letters, numbers and underscores.</small>
                </label>
                <label>Application username
                    <input bind:value={username} maxlength="63" autocomplete="off" spellcheck="false" />
                    <small>A separate non-superuser role will be created.</small>
                </label>
            </div>
            <label>Application password
                <input type="password" bind:value={password} minlength="12" maxlength="256" autocomplete="new-password" placeholder="At least 12 characters" />
                <small>Password is sent to the selected device over its authenticated connection and stored encrypted in the Control Plane.</small>
            </label>
            <div class="section-title spaced">Optional website connection</div>
            <label>Connect a website
                <select bind:value={siteKey}>
                    <option value="">Do not connect a website yet</option>
                    {#each availableSites as entry}
                        <option value={`${entry.device_id}:${entry.site?.site_id}`}>{entry.site?.site_id} · {entry.device_name || `Device #${entry.device_id}`}</option>
                    {/each}
                </select>
                <small>Only websites on the same Android device are listed because PostgreSQL stays bound to localhost. The website's existing .env file will not be overwritten.</small>
            </label>
            {#if error}<div class="error">{error}</div>{/if}
            <div class="actions">
                <span class="muted">The database is independent from its website and remains after website removal.</span>
                <button class="primary" type="submit" disabled={!canCreate || submitting}>{submitting ? 'Creating…' : 'Create Database'}</button>
            </div>
        </form>
    {/if}
</div>

<style>
.page{max-width:920px;margin:0 auto;padding:28px 0 60px;color:var(--text-main)}.heading{display:flex;align-items:flex-start;justify-content:space-between;margin-bottom:22px}.eyebrow{font-size:9px;letter-spacing:.14em;color:var(--accent);font-weight:700;margin-bottom:8px}.eyebrow span{color:var(--text-muted);margin:0 6px}h1{font-size:23px;margin:0 0 7px;font-weight:650}.heading p{font-size:12px;color:var(--text-muted);margin:0}.panel{background:var(--bg-card);border:1px solid var(--border);border-radius:14px;padding:22px;display:grid;gap:17px}.section-title{font-size:12px;font-weight:700;color:var(--text-main)}.spaced{padding-top:8px;border-top:1px solid var(--border)}label{display:grid;gap:7px;font-size:10px;color:var(--text-main);font-weight:600}input,select{width:100%;box-sizing:border-box;background:var(--bg-base);border:1px solid var(--border);border-radius:8px;padding:11px 12px;color:var(--text-main);font:inherit;font-size:12px;outline:none}input:focus,select:focus{border-color:var(--accent)}small,.muted{font-size:10px;color:var(--text-muted);font-weight:400}.two{display:grid;grid-template-columns:1fr 1fr;gap:14px}.actions{display:flex;align-items:center;justify-content:space-between;gap:16px;padding-top:14px;border-top:1px solid var(--border)}.primary,.secondary{border:0;border-radius:9px;padding:10px 15px;font-size:11px;font-weight:650;cursor:pointer}.primary{background:var(--accent-hover);color:white}.primary:disabled{opacity:.45;cursor:not-allowed}.secondary{background:var(--bg-elevated);color:var(--text-main)}.error{padding:10px 12px;background:var(--bg-elevated);border:1px solid var(--danger);border-radius:8px;color:var(--danger);font-size:11px}.panel h2{font-size:15px;margin:0}.panel p{margin:0}@media(max-width:650px){.page{padding:18px 0 40px}.two{grid-template-columns:1fr}.actions{align-items:stretch;flex-direction:column}.heading{gap:12px}}
</style>
