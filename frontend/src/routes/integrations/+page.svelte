<script>
 import {onMount} from 'svelte';
 import {locale} from '$lib/i18n.js';
 import {notify} from '$lib/stores.js';
 import {listIntegrations,connectIntegration,disconnectIntegration,listIntegrationRepositories} from '$lib/api.js';
 let provider='github', integrations=[], token='', loading=true, connecting=false, reposLoading=false, repos=[], selected=null, search='';
 const providers=[
  {id:'github',name:'GitHub',mark:'GH',enabled:true,scopes:'repo, read:user'},
  {id:'gitlab',name:'GitLab',mark:'GL',enabled:true,scopes:'read_api, read_repository'},
  {id:'bitbucket',name:'Bitbucket',mark:'BB',enabled:false},
  {id:'gitea',name:'Gitea',mark:'GT',enabled:false},
  {id:'docker',name:'Docker',mark:'D',enabled:false},
  {id:'git',name:'Git URL',mark:'GIT',enabled:false},
  {id:'drop',name:'Drop',mark:'↓',enabled:false}
 ];
 $: ru=$locale!=='en';
 $: activeProvider=providers.find(p=>p.id===provider);
 $: linked=integrations.filter(x=>x.provider===provider);
 $: visibleRepos=repos.filter(r=>(r.full_name||r.name).toLowerCase().includes(search.toLowerCase()));
 onMount(load);
 async function load(){loading=true;const r=await listIntegrations();if(r.ok)integrations=r.data.integrations||[];else notify(r.error||'Не удалось загрузить интеграции','error');loading=false}
 async function connect(){if(!token.trim()||connecting)return;connecting=true;const r=await connectIntegration(provider,token.trim());connecting=false;if(r.ok){token='';notify(ru?'Аккаунт подключён':'Account connected','success');await load()}else notify(r.error||(ru?'Не удалось подключить аккаунт':'Could not connect account'),'error')}
 async function disconnect(id){if(!confirm(ru?'Отключить этот аккаунт?':'Disconnect this account?'))return;const r=await disconnectIntegration(id);if(r.ok){if(selected?.id===id){selected=null;repos=[]}notify(ru?'Доступ отозван':'Access revoked','success');await load()}else notify(r.error||'Error','error')}
 async function openRepos(item){selected=item;repos=[];search='';reposLoading=true;const r=await listIntegrationRepositories(item.id);reposLoading=false;if(r.ok)repos=r.data.repositories||[];else notify(r.error||(ru?'Не удалось загрузить репозитории':'Could not load repositories'),'error')}
</script>
<svelte:head><title>Интеграции · VELTRIX Dock</title></svelte:head>
<div class="page-header"><div><div class="eyebrow">{ru?'ПОДКЛЮЧЕНИЯ':'CONNECTIVITY'}</div><h1 class="page-title">{ru?'Интеграции':'Integrations'}</h1><p class="page-desc">{ru?'Подключайте Git-провайдеры и выбирайте репозитории для развёртывания.':'Connect Git providers and select repositories for deployment.'}</p></div></div>
<section class="provider-panel">
 <div class="section-heading"><div><h2>{ru?'Источник кода':'Source provider'}</h2><p>{ru?'Выберите сервис для подключения':'Choose a service to connect'}</p></div><span class="secure">⌑ {ru?'Зашифрованное хранение':'Encrypted storage'}</span></div>
 <div class="provider-tabs">
  {#each providers as p}
   <button class:active={provider===p.id} class:disabled={!p.enabled} disabled={!p.enabled} on:click={()=>{provider=p.id;selected=null;repos=[]}}><span class="provider-mark">{p.mark}</span>{p.name}{#if !p.enabled}<small>{ru?'Скоро':'Soon'}</small>{/if}</button>
  {/each}
 </div>
 {#if activeProvider?.enabled}
  <div class="provider-content">
   <div class="provider-title"><span class="provider-mark big">{activeProvider.mark}</span><div><h3>{activeProvider.name}</h3><p>{ru?'Подключите аккаунт с помощью токена доступа':'Connect your account using an access token'}</p></div></div>
   <div class="connect-form">
    <label for="integration-token">{ru?'Personal Access Token':'Personal Access Token'}</label>
    <input id="integration-token" type="password" autocomplete="off" bind:value={token} placeholder={provider==='github'?'ghp_••••••••••••••••••••': 'glpat-••••••••••••••••••••'} />
    <div class="form-hint">{ru?'Токен проверяется через API провайдера и сохраняется только в зашифрованном виде. Повторно он не показывается.':'The token is verified against the provider API and stored encrypted. It is never displayed again.'}</div>
    <div class="scope-note"><strong>{ru?'Рекомендуемые права':'Recommended scopes'}:</strong> {provider==='github'?'repo, read:user':'read_api, read_repository'}</div>
    <div class="form-actions"><a href={provider==='github'?'https://github.com/settings/tokens/new':'https://gitlab.com/-/user_settings/personal_access_tokens'} target="_blank" rel="noreferrer">{ru?'Создать токен':'Create token'} ↗</a><button class="primary" disabled={!token.trim()||connecting} on:click={connect}>{connecting?(ru?'Проверка…':'Verifying…'):(ru?'Подключить аккаунт':'Connect account')}</button></div>
   </div>
   <div class="accounts-head"><h3>{ru?'Подключённые аккаунты':'Connected accounts'}</h3><span>{linked.length}</span></div>
   {#if loading}<div class="empty">{ru?'Загрузка…':'Loading…'}</div>
   {:else if linked.length===0}<div class="empty">{ru?'Пока нет подключённых аккаунтов. Добавьте токен выше.':'No connected accounts yet. Add a token above.'}</div>
   {:else}
    <div class="account-list">
     {#each linked as item}
      <div class="account-row"><div class="account-avatar">{item.account_name.slice(0,1).toUpperCase()}</div><div class="account-meta"><strong>{item.account_name}</strong><small>{item.account_email||item.provider+' · '+(ru?'Подключено':'Connected')}</small></div><span class="connected-state"><i></i>{ru?'Подключено':'Connected'}</span><button class="outline" on:click={()=>openRepos(item)}>{ru?'Репозитории':'Repositories'}</button><button class="remove" title={ru?'Отключить':'Disconnect'} on:click={()=>disconnect(item.id)}>×</button></div>
     {/each}
    </div>
   {/if}
  </div>
 {:else}
  <div class="coming-panel"><span>⌑</span><div><strong>{ru?'Интеграция пока не реализована':'Integration not implemented yet'}</strong><p>{ru?'Этот провайдер появится после добавления API-адаптера и безопасного управления credentials.':'This provider will be enabled when its API adapter and secure credential management are implemented.'}</p></div></div>
 {/if}
</section>
{#if selected}
 <section class="repositories-panel">
  <div class="section-heading"><div><h2>{ru?'Репозитории':'Repositories'}</h2><p>{ru?'Аккаунт':'Account'}: <strong>{selected.account_name}</strong></p></div><button class="outline" on:click={()=>{selected=null;repos=[]}}>{ru?'Закрыть':'Close'}</button></div>
  {#if reposLoading}<div class="empty">{ru?'Загружаем репозитории…':'Loading repositories…'}</div>
  {:else}
   <input class="repo-search" bind:value={search} placeholder={ru?'Поиск репозитория…':'Search repositories…'} />
   {#if visibleRepos.length===0}<div class="empty">{ru?'Репозитории не найдены':'No repositories found'}</div>{:else}
    <div class="repo-list">{#each visibleRepos as repo}
     <div class="repo-row"><div class="repo-symbol">⑂</div><div class="repo-meta"><strong>{repo.full_name||repo.name}</strong><small>{repo.private?(ru?'Приватный':'Private'):(ru?'Публичный':'Public')} · {ru?'Ветка':'Branch'}: {repo.default_branch||'—'}</small></div><button class="outline" on:click={()=>GenUI.copy(repo.clone_url)}>{ru?'Копировать URL':'Copy URL'}</button><a href={repo.web_url} target="_blank" rel="noreferrer">↗</a></div>
    {/each}</div>
   {/if}
  {/if}
 </section>
{/if}
<section class="security-foot"><span>⌑</span><div><strong>{ru?'Безопасность':'Security'}</strong><p>{ru?'Токены шифруются AES-GCM на сервере. API возвращает только имя аккаунта и метаданные — секреты никогда не отправляются обратно в браузер. Отключение удаляет сохранённый токен.':'Tokens are encrypted with AES-GCM on the server. The API only returns account metadata — secrets are never sent back to the browser. Disconnecting deletes the stored token.'}</p></div></section>
<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:#60A5FA;font-weight:750;margin-bottom:7px}
.provider-panel,.repositories-panel{border:1px solid #27272A;border-radius:12px;background:#101014;overflow:hidden;margin-bottom:13px}
.section-heading{display:flex;align-items:center;justify-content:space-between;gap:15px;padding:17px 18px;border-bottom:1px solid #27272A}.section-heading h2{font-size:13px;margin:0 0 4px}.section-heading p{font-size:10px;color:#A1A1AA;margin:0}.secure{font-size:9px;color:#34D399}
.provider-tabs{display:flex;align-items:center;gap:4px;padding:13px 17px;border-bottom:1px solid #27272A;overflow-x:auto}
.provider-tabs button{display:flex;align-items:center;gap:8px;white-space:nowrap;padding:9px 11px;border:0;border-bottom:2px solid transparent;background:transparent;color:#A1A1AA;font-size:10px}
.provider-tabs button.active{color:#FAFAFA;border-color:#60A5FA}.provider-tabs button:disabled{opacity:.38;cursor:not-allowed}.provider-tabs small{font-size:8px;color:#71717A}
.provider-mark{width:25px;height:25px;display:grid;place-items:center;border:1px solid #3F3F46;border-radius:7px;background:#18181B;color:#D4D4D8;font-size:9px;font-weight:800}.provider-mark.big{width:39px;height:39px;font-size:12px}
.provider-content{padding:18px}.provider-title{display:flex;align-items:center;gap:11px;margin-bottom:17px}.provider-title h3{font-size:12px;margin:0 0 3px}.provider-title p{font-size:9px;color:#A1A1AA;margin:0}
.connect-form{max-width:700px;padding:14px;border:1px solid #27272A;border-radius:9px;background:#0C0C10}.connect-form label{display:block;font-size:10px;font-weight:650;margin-bottom:7px}.connect-form input,.repo-search{width:100%;height:37px;padding:0 11px;border:1px solid #3F3F46;border-radius:7px;background:#18181B;color:#FAFAFA;font-size:10px;outline:none}.connect-form input:focus,.repo-search:focus{border-color:#3B82F6}.form-hint{font-size:9px;color:#A1A1AA;line-height:1.5;margin-top:7px}.scope-note{font-size:9px;color:#A1A1AA;margin-top:10px}.scope-note strong{color:#D4D4D8}.form-actions{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-top:13px}.form-actions a{font-size:9px;color:#60A5FA}.primary{padding:9px 13px;border:1px solid #2563EB;border-radius:7px;background:#2563EB;color:white;font-size:10px;font-weight:650}.primary:disabled{opacity:.45;cursor:not-allowed}
.accounts-head{display:flex;align-items:center;gap:8px;margin:21px 0 9px}.accounts-head h3{font-size:11px;margin:0}.accounts-head span{font-size:9px;color:#A1A1AA;background:#27272A;padding:2px 6px;border-radius:8px}
.empty{padding:18px;text-align:center;color:#A1A1AA;font-size:10px;border:1px dashed #3F3F46;border-radius:8px}
.account-list,.repo-list{display:grid;gap:7px}.account-row,.repo-row{display:flex;align-items:center;gap:10px;padding:10px;border:1px solid #27272A;border-radius:8px;background:#0C0C10;min-width:0}.account-avatar,.repo-symbol{width:30px;height:30px;display:grid;place-items:center;flex-shrink:0;border-radius:8px;background:#1E293B;color:#93C5FD;font-size:11px;font-weight:750}.account-meta,.repo-meta{display:grid;gap:3px;flex:1;min-width:0}.account-meta strong,.repo-meta strong{font-size:10px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.account-meta small,.repo-meta small{font-size:9px;color:#A1A1AA}.connected-state{display:flex;align-items:center;gap:5px;color:#34D399;font-size:9px;white-space:nowrap}.connected-state i{width:6px;height:6px;border-radius:50%;background:#22C55E}
.outline,.remove{padding:7px 9px;border:1px solid #3F3F46;border-radius:7px;background:#18181B;color:#D4D4D8;font-size:9px;white-space:nowrap}.remove{font-size:15px;color:#F87171;padding:3px 8px}.coming-panel{display:flex;align-items:center;gap:12px;padding:20px;color:#A1A1AA}.coming-panel>span,.security-foot>span{font-size:20px;color:#60A5FA}.coming-panel strong,.security-foot strong{display:block;color:#E4E4E7;font-size:10px;margin-bottom:4px}.coming-panel p,.security-foot p{font-size:9px;line-height:1.55;margin:0;color:#A1A1AA}
.repositories-panel{padding-bottom:15px}.repositories-panel .section-heading{margin-bottom:13px}.repo-search{display:block;width:calc(100% - 30px);margin:0 15px 10px}.repo-list{padding:0 15px}.repo-row a{color:#60A5FA;font-size:12px;text-decoration:none}
.security-foot{display:flex;align-items:flex-start;gap:11px;padding:13px;border:1px solid #334155;border-radius:10px;background:#101727}.security-foot>span{width:27px;height:27px;display:grid;place-items:center;flex-shrink:0;border-radius:8px;background:#1E293B}.security-foot strong{color:#E2E8F0}.security-foot p{color:#94A3B8}
:global(.app-shell[data-theme="light"]) .provider-panel,:global(.app-shell[data-theme="light"]) .repositories-panel{background:#fff;border-color:#E2E6EE;color:#172033}
:global(.app-shell[data-theme="light"]) .section-heading,:global(.app-shell[data-theme="light"]) .provider-tabs{border-color:#E2E6EE}
:global(.app-shell[data-theme="light"]) .section-heading h2,:global(.app-shell[data-theme="light"]) .provider-title h3,:global(.app-shell[data-theme="light"]) .accounts-head h3{color:#172033}
:global(.app-shell[data-theme="light"]) .section-heading p,:global(.app-shell[data-theme="light"]) .provider-title p,:global(.app-shell[data-theme="light"]) .account-meta small,:global(.app-shell[data-theme="light"]) .repo-meta small{color:#667085}
:global(.app-shell[data-theme="light"]) .provider-tabs button{color:#667085}
:global(.app-shell[data-theme="light"]) .provider-tabs button.active{color:#1D4ED8}
:global(.app-shell[data-theme="light"]) .provider-mark,:global(.app-shell[data-theme="light"]) .account-avatar,:global(.app-shell[data-theme="light"]) .repo-symbol{background:#EFF6FF;border-color:#BFDBFE;color:#2563EB}
:global(.app-shell[data-theme="light"]) .connect-form,:global(.app-shell[data-theme="light"]) .account-row,:global(.app-shell[data-theme="light"]) .repo-row{background:#F8FAFC;border-color:#E2E6EE}
:global(.app-shell[data-theme="light"]) .connect-form label,:global(.app-shell[data-theme="light"]) .account-meta strong,:global(.app-shell[data-theme="light"]) .repo-meta strong{color:#172033}
:global(.app-shell[data-theme="light"]) .connect-form input,:global(.app-shell[data-theme="light"]) .repo-search,:global(.app-shell[data-theme="light"]) .outline{background:#fff;color:#172033;border-color:#D0D5DD}
:global(.app-shell[data-theme="light"]) .form-hint,:global(.app-shell[data-theme="light"]) .scope-note,:global(.app-shell[data-theme="light"]) .coming-panel p{color:#667085}
:global(.app-shell[data-theme="light"]) .scope-note strong{color:#344054}
:global(.app-shell[data-theme="light"]) .empty{border-color:#D0D5DD;color:#667085}
:global(.app-shell[data-theme="light"]) .security-foot{background:#EFF6FF;border-color:#BFDBFE}
:global(.app-shell[data-theme="light"]) .security-foot strong{color:#1E3A8A}
:global(.app-shell[data-theme="light"]) .security-foot p{color:#475569}
@media(max-width:700px){.provider-content{padding:12px}.account-row,.repo-row{flex-wrap:wrap}.connected-state{display:none}.account-row .outline{margin-left:auto}.form-actions{align-items:flex-start;flex-direction:column}.form-actions .primary{width:100%}}
</style>
