<script>
    import { goto } from '$app/navigation';
</script>

<svelte:head><title>Databases · VELTRIX Dock</title></svelte:head>
<div class="page-header">
    <div><h1 class="page-title">Databases</h1><p class="page-desc">Database servers and databases across your Android devices.</p></div>
    <button class="btn btn-primary" on:click={() => goto('/databases/servers/new')}>+ Install Database Server</button>
</div>
<div class="db-empty">
    <div class="db-icon">DB</div>
    <h2>Database infrastructure</h2>
    <p>Управляйте PostgreSQL, MariaDB и MySQL серверами, а также отдельными базами данных на подключённых Android-устройствах.</p>
    <button class="btn btn-primary" on:click={() => goto('/databases/servers/new')}>Установить Database Server <span>→</span></button>
</div>
<style>
    .db-empty{min-height:390px;border:1px dashed #334155;border-radius:12px;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;padding:35px;background:linear-gradient(145deg,#101827,#0b1020)}
    .db-icon{width:52px;height:52px;border-radius:14px;display:grid;place-items:center;background:#1a2140;border:1px solid #373d70;color:#a5b4fc;font:700 14px ui-monospace,monospace;margin-bottom:18px}
    .db-empty h2{font-size:18px}.db-empty p{max-width:440px;color:#94a3b8;font-size:12px;line-height:1.7;margin:8px 0 20px}
    .btn{border:1px solid #6366f1;background:#6366f1;color:white;border-radius:8px;padding:9px 14px;font-size:12px;font-weight:600;cursor:pointer}
    .btn:hover{background:#5558df}
</style>
