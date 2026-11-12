<script>
    import { goto } from '$app/navigation';
    import Button from '$lib/components/Button.svelte';

    // UI preview: database records will be supplied by the control API.
    const databases = [
        { name: 'vertlix_db', engine: 'PostgreSQL', device: 'Android Host 02', status: 'ready', version: '—' }
    ];

    function openDatabase(name) {
        goto(`/databases/${encodeURIComponent(name)}`);
    }
</script>

<svelte:head>
    <title>Databases · VELTRIX Dock</title>
</svelte:head>

<div class="page-header">
    <div>
        <h1 class="page-title">Базы данных</h1>
        <p class="page-desc">Управление PostgreSQL и MySQL на подключённых устройствах</p>
    </div>
    <Button variant="primary" on:click={() => goto('/databases/new')}>Создать базу</Button>
</div>

<div class="database-list">
    {#each databases as db}
        <button class="database-card" on:click={() => openDatabase(db.name)}>
            <div class="db-icon">DB</div>
            <div class="db-main">
                <div class="db-title">{db.name}<span class="engine">{db.engine}</span></div>
                <div class="db-subtitle">{db.device} · версия {db.version}</div>
            </div>
            <span class="status"><i></i> Подготовлена</span>
            <span class="arrow">›</span>
        </button>
    {/each}
</div>

<style>
.database-list{display:grid;gap:10px}
.database-card{width:100%;display:flex;align-items:center;gap:16px;text-align:left;padding:18px;background:#101318;border:1px solid #252b35;border-radius:12px;color:#f4f6fa;cursor:pointer;transition:border-color .15s,transform .15s}
.database-card:hover{border-color:#53657c;transform:translateY(-1px)}
.db-icon{width:42px;height:42px;display:grid;place-items:center;border-radius:10px;background:#172b3c;color:#70c8ff;font-weight:700;font-size:12px}
.db-main{flex:1;min-width:0}.db-title{font-size:14px;font-weight:600;display:flex;gap:10px;align-items:center;flex-wrap:wrap}.engine{font-size:11px;color:#8ba6c2;background:#1a2531;padding:3px 7px;border-radius:5px;font-weight:500}.db-subtitle{font-size:12px;color:#84909f;margin-top:5px}
.status{font-size:12px;color:#8bd6a4;display:flex;align-items:center;gap:7px}.status i{width:6px;height:6px;border-radius:50%;background:#43c77a}.arrow{font-size:24px;color:#667181}
@media(max-width:650px){.database-card{gap:10px;padding:13px}.status{display:none}}
</style>