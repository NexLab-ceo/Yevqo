<script>
    import { goto } from '$app/navigation';
    let step = 1;
    let name = 'vertlix_db';
    let engine = 'PostgreSQL';
    let host = 'Android Host 02';
    let project = 'No project';
    let mode = 'Existing server';
    let username = 'vertlix_user';
    let access = 'Local + private network';
    let connection = 'Create credentials automatically';
    let error = '';
    function next(){ if(!name.trim()){error='Введите имя базы данных';return} error='';step=2; }
    function finish(){ goto('/databases/'+encodeURIComponent(name.trim())); }
</script>
<svelte:head><title>Create database · VELTRIX Dock</title></svelte:head>
<div class="crumb"><button on:click={()=>goto('/databases')}>Базы данных</button><span>/</span>Создать</div>
<div class="page-header"><div><h1 class="page-title">Создать базу данных</h1><p class="page-desc">Настройте движок и параметры доступа</p></div></div>
<div class="steps"><span class:current={step===1}>1 <b>Database</b></span><i></i><span class:current={step===2}>2 <b>Users & connection</b></span></div>
<section class="panel">
{#if step===1}
<div class="panel-head">Основные параметры</div>
<div class="form-group"><label>Имя базы данных</label><input bind:value={name} placeholder="my_database"/></div>
<div class="form-group"><label>Движок</label><div class="choice"><button class:selected={engine==='PostgreSQL'} on:click={()=>engine='PostgreSQL'}>PostgreSQL</button><button class:selected={engine==='MySQL'} on:click={()=>engine='MySQL'}>MySQL</button></div></div>
<div class="form-group"><label>Устройство</label><select bind:value={host}><option>Android Host 02</option><option>Local server</option></select></div>
<div class="form-group"><label>Проект</label><select bind:value={project}><option>No project</option></select></div>
<div class="form-group"><label>Режим</label><select bind:value={mode}><option>Existing server</option><option>New server</option></select></div>
{:else}
<div class="panel-head">Пользователи, доступ и подключение</div>
<div class="form-group"><label>Первый пользователь</label><input bind:value={username}/></div>
<div class="form-group"><label>Доступ</label><select bind:value={access}><option>Local + private network</option><option>Local only</option><option>Private network</option></select></div>
<div class="form-group"><label>Учетные данные</label><select bind:value={connection}><option>Create credentials automatically</option><option>Use existing user</option></option></select></div>
<div class="hint">Пароль будет создан сервером и показан только один раз после успешного создания.</div>
{/if}
{#if error}<div class="error">{error}</div>{/if}
<div class="footer"><button class="secondary" on:click={()=>step===1?goto('/databases'):step=1}>{step===1?'Отмена':'Назад'}</button><button class="primary" on:click={step===1?next:finish}>{step===1?'Продолжить':'Создать базу'}</button></div>
</section>
<style>
.crumb{display:flex;gap:8px;font-size:12px;color:#84909f;margin-bottom:18px}.crumb button{background:none;border:0;color:#9eb8d4;font:inherit}.steps{display:flex;align-items:center;gap:12px;margin:0 0 20px;color:#657283;font-size:12px}.steps span{display:flex;align-items:center;gap:8px}.steps span.current{color:#b8e0ff}.steps span.current:first-letter{color:#66baff}.steps i{height:1px;background:#29313c;width:50px}
.panel{background:#101318;border:1px solid #252b35;border-radius:12px;padding:22px;max-width:680px}.panel-head{font-size:14px;font-weight:600;margin-bottom:22px}.form-group{margin-bottom:18px}.form-group label{display:block;color:#9ba7b6;font-size:12px;margin-bottom:7px}.form-group input,.form-group select{width:100%;background:#0a0d11;border:1px solid #2b3440;color:#e6edf5;border-radius:7px;padding:11px 12px;font:inherit;font-size:13px}.choice{display:flex;gap:8px}.choice button{flex:1;background:#0a0d11;border:1px solid #2b3440;color:#98a5b5;padding:12px;border-radius:8px}.choice button.selected{border-color:#4a9dd8;background:#142434;color:#c8e8ff}.hint{color:#8190a0;font-size:12px;background:#0b1118;border-radius:7px;padding:12px}.footer{display:flex;justify-content:space-between;margin-top:24px}.footer button{border-radius:7px;padding:10px 15px;font-size:12px;cursor:pointer}.secondary{background:none;border:1px solid #303b4b;color:#aab7c6}.primary{background:#4b9bd5;border:1px solid #4b9bd5;color:#06111b;font-weight:600}.error{color:#ff9999;font-size:12px;margin-top:10px}
</style>