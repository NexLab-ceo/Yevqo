<script>
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';

    $: databaseName = decodeURIComponent($page.params.id || 'vertlix_db');
    let activeTab = 'overview';
    const tabs = [
        { id: 'overview', label: 'Overview' },
        { id: 'connection', label: 'Connection' },
        { id: 'users', label: 'Users & Permissions' },
        { id: 'backups', label: 'Backups' },
        { id: 'transfer', label: 'Import / Export' },
        { id: 'logs', label: 'Logs' },
        { id: 'settings', label: 'Settings' }
    ];
</script>

<svelte:head><title>{databaseName} · VELTRIX Dock</title></svelte:head>

<div class="crumb"><button on:click={() => goto('/databases')}>Базы данных</button><span>/</span>{databaseName}</div>
<div class="page-header">
    <div><h1 class="page-title">{databaseName}</h1><p class="page-desc">PostgreSQL · Android Host 02</p></div>
    <span class="state"><i></i> Ready</span>
</div>
<div class="tabs" role="tablist">
    {#each tabs as tab}
        <button role="tab" aria-selected={activeTab === tab.id} class:active={activeTab === tab.id} on:click={() => activeTab = tab.id}>{tab.label}</button>
    {/each}
</div>

{#if activeTab === 'overview'}
    <div class="overview-grid">
        <section class="panel"><div class="panel-head">Состояние базы <span class="state"><i></i> Ready</span></div><div class="metric">PostgreSQL</div><p class="muted">Движок базы данных</p><div class="line"></div><div class="kv"><span>Устройство</span><b>Android Host 02</b></div><div class="kv"><span>Проект</span><b>No project</b></div><div class="kv"><span>Создана</span><b>—</b></div></section>
        <section class="panel"><div class="panel-head">Ресурсы</div><div class="metric">—</div><p class="muted">Размер базы</p><div class="line"></div><div class="kv"><span>Подключения</span><b>—</b></div><div class="kv"><span>Активные запросы</span><b>—</b></div><div class="kv"><span>Последний backup</span><b>—</b></div></section>
    </div>
    <section class="panel quick"><div class="panel-head">Быстрые действия</div><div class="actions"><button on:click={() => activeTab='connection'}>Показать подключение</button><button on:click={() => activeTab='backups'}>Создать backup</button><button on:click={() => activeTab='transfer'}>Импортировать данные</button></div></section>
{:else if activeTab === 'connection'}
    <section class="panel"><div class="panel-head">Параметры подключения</div><p class="muted">Значения будут загружены из API устройства. Пароль не отображается в интерфейсе.</p><div class="kv"><span>Host</span><b>—</b></div><div class="kv"><span>Port</span><b>5432</b></div><div class="kv"><span>Database</span><b>{databaseName}</b></div><div class="kv"><span>SSL</span><b>Настройки сервера</b></div></section>
{:else if activeTab === 'users'}
    <section class="panel"><div class="panel-head">Users & Permissions <button class="small">Добавить пользователя</button></div><div class="empty">Пользователи базы появятся после подключения API.</div></section>
{:else if activeTab === 'backups'}
    <section class="panel"><div class="panel-head">Резервные копии <button class="small">Создать backup</button></div><div class="empty">История резервных копий пока не загружена.</div></section>
{:else if activeTab === 'transfer'}
    <section class="panel"><div class="panel-head">Import / Export</div><div class="actions"><button>Импорт SQL</button><button>Экспорт SQL</button></div><p class="muted note">Операции будут доступны после подключения обработчиков API.</p></section>
{:else if activeTab === 'logs'}
    <section class="panel"><div class="panel-head">Logs</div><div class="empty">Логи PostgreSQL пока не загружены.</div></section>
{:else}
    <section class="panel"><div class="panel-head">Настройки базы</div><div class="kv"><span>Имя</span><b>{databaseName}</b></div><div class="kv"><span>Движок</span><b>PostgreSQL</b></div><div class="actions"><button class="danger">Удалить базу</button></div></section>
{/if}

<style>
.crumb{display:flex;gap:8px;align-items:center;font-size:12px;color:#7e8998;margin-bottom:18px}.crumb button{background:none;border:0;color:#9eb8d4;cursor:pointer;font:inherit}
.page-header{align-items:center;margin-bottom:22px}.state{font-size:12px;color:#8bd6a4;display:flex;gap:7px;align-items:center}.state i{width:7px;height:7px;background:#43c77a;border-radius:50%}
.tabs{display:flex;gap:5px;overflow:auto;border-bottom:1px solid #252b35;margin-bottom:20px}.tabs button{white-space:nowrap;background:none;border:0;border-bottom:2px solid transparent;padding:12px 13px;color:#8793a3;font-size:12px;cursor:pointer}.tabs button.active{color:#e9f2ff;border-color:#66baff}
.overview-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.panel{background:#101318;border:1px solid #252b35;border-radius:12px;padding:20px;margin-bottom:14px}.panel-head{display:flex;justify-content:space-between;align-items:center;font-weight:600;font-size:13px;margin-bottom:20px}.metric{font-size:24px;font-weight:650;margin-bottom:3px}.muted{color:#83909f;font-size:12px}.line{height:1px;background:#252b35;margin:18px 0}.kv{display:flex;justify-content:space-between;gap:12px;padding:9px 0;font-size:12px}.kv span{color:#83909f}.kv b{font-weight:500;color:#dce4ee;text-align:right}.actions{display:flex;flex-wrap:wrap;gap:8px}.actions button,.small{background:#19212b;border:1px solid #303b4b;border-radius:7px;color:#c9d9ea;padding:9px 12px;font-size:12px;cursor:pointer}.actions button:hover,.small:hover{border-color:#5d85ae}.danger{color:#ff8c8c!important;border-color:#57333a!important}.empty{text-align:center;padding:35px 10px;color:#7d8998;font-size:12px}.note{margin-top:16px}
@media(max-width:700px){.overview-grid{grid-template-columns:1fr}.panel{padding:15px}.page-header{align-items:flex-start;gap:12px;flex-direction:column}}
</style>