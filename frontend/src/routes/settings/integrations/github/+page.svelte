<script>
 import {formatDate} from '$lib/date.js';
 import {onMount} from 'svelte';
 import {page} from '$app/stores';
 import {goto} from '$app/navigation';
 import {locale} from '$lib/i18n.js';
 import {notify} from '$lib/stores.js';
 import {getGitHubAppConfig,createGitHubAppInstallState,connectGitHubApp,listIntegrations,disconnectIntegration,listIntegrationRepositories} from '$lib/api.js';
 let config={configured:false,app_slug:''},integration=null,repositories=[],loading=true,connecting=false,repositoriesLoading=false,errorState='',integrationStatus='connected';
 $: ru=$locale!=='en';
 $: installURL=config.app_slug?'https://github.com/apps/'+encodeURIComponent(config.app_slug)+'/installations/new':'';
 onMount(async()=>{
  const [cfg,linked]=await Promise.all([getGitHubAppConfig(),listIntegrations()]);
  if(cfg.ok)config=cfg.data;else errorState=cfg.error||'Could not read GitHub App configuration';
  if(linked.ok)integration=(linked.data.integrations||[]).find(x=>x.provider==='github_app')||null;
  loading=false;
  const params=new URLSearchParams(window.location.search);
  const installationID=params.get('installation_id');
  const setupAction=params.get('setup_action');
  const callbackState=params.get('state')||'';
  if(setupAction==='install'&&installationID){
   // GitHub may return to a new tab/window where sessionStorage is unavailable.
   // The authenticated Control Plane validates the one-time state against its
   // server-side, account-bound hash; browser storage must not be authoritative.
   if(!callbackState){errorState=ru?'GitHub не вернул проверочное состояние. Начните подключение заново.':'GitHub did not return the installation state. Start the installation again.';return}
   await finishInstall(installationID,callbackState);
  }else if(setupAction==='request'){
   errorState=ru?'Запрос на установку отправлен владельцу GitHub.':'Installation request sent to the GitHub account owner.';
  }
 });
 async function startInstall(){
  if(!config.configured||!config.app_slug)return;
  connecting=true;errorState='';
  const r=await createGitHubAppInstallState();
  if(!r.ok){connecting=false;errorState=r.error||'Could not start GitHub App installation';return}
  window.location.assign(installURL+'?state='+encodeURIComponent(r.data.state));
 }
 async function finishInstall(id,state){
  connecting=true;
  const r=await connectGitHubApp(id,state);
  window.history.replaceState({},'',window.location.pathname);
  connecting=false;
  if(!r.ok){errorState=r.error||'GitHub App installation verification failed';return}
  notify(ru?'GitHub App успешно подключено':'GitHub App connected successfully','success');
  await loadIntegration();
 }
 async function loadIntegration(){
  const r=await listIntegrations();
  if(r.ok)integration=(r.data.integrations||[]).find(x=>x.provider==='github_app')||null;
 }
 async function loadRepositories(){
  if(!integration)return;
  repositoriesLoading=true;const r=await listIntegrationRepositories(integration.id);
  if(r.ok){repositories=r.data.repositories||[];integrationStatus='connected';errorState=''}else{errorState=r.error||'Repository synchronization failed';if(errorState.toLowerCase().includes('removed'))integrationStatus='removed';else if(errorState.toLowerCase().includes('permission'))integrationStatus='permission';else if(errorState.toLowerCase().includes('rate limit'))integrationStatus='rate_limit';}
  repositoriesLoading=false;
 }
 async function disconnect(){
  if(!integration||!confirm(ru?'Отключить GitHub App?':'Disconnect GitHub App?'))return;
  const r=await disconnectIntegration(integration.id);
  if(r.ok){integration=null;repositories=[];notify(ru?'GitHub отключён':'GitHub disconnected','success')}
  else errorState=r.error||'Could not disconnect GitHub App';
 }
</script>
<svelte:head><title>GitHub App · Yevqo</title></svelte:head>
<div class="page-header"><div><div class="eyebrow">SETTINGS / INTEGRATIONS</div><h1 class="page-title">GitHub App</h1><p class="page-desc">{ru?'Подключение GitHub через официальное приложение Yevqo. Доступ к репозиториям выдаётся установке GitHub App.':'Connect GitHub through the official Yevqo application. Repository access is granted to a GitHub App installation.'}</p></div><a class="back-link" href="/settings">← {ru?'Настройки':'Settings'}</a></div>
<section class="github-card">
 <div class="app-heading"><div class="github-logo">GH</div><div class="app-title"><strong>Yevqo GitHub App</strong><span>{ru?'Официальная интеграция платформы':'Official Yevqo platform integration'}</span></div><span class:connected={!!integration&&integrationStatus==='connected'} class="state-pill">{integrationStatus==='removed'?(ru?'Установка удалена':'Installation removed'):integrationStatus==='permission'?(ru?'Нет разрешения':'Permission denied'):integrationStatus==='rate_limit'?'API rate limit':integration?(ru?'Подключено':'Connected'):(ru?'Не подключено':'Not connected')}</span></div>
 {#if loading}<div class="empty">{ru?'Проверяем подключение…':'Checking connection…'}</div>
 {:else if !config.configured}
  <div class="setup-warning"><strong>{ru?'GitHub App ещё не настроено на сервере':'GitHub App is not configured on this server'}</strong><p>{ru?'Администратору Control Plane нужно зарегистрировать GitHub App и задать серверные переменные YEVQO_GITHUB_APP_ID, YEVQO_GITHUB_APP_SLUG и YEVQO_GITHUB_APP_PRIVATE_KEY. Секреты не вводятся в браузере.':'The Control Plane operator must register a GitHub App and set YEVQO_GITHUB_APP_ID, YEVQO_GITHUB_APP_SLUG and YEVQO_GITHUB_APP_PRIVATE_KEY on the server. Secrets are never entered in the browser.'}</p></div>
 {:else if !integration}
  <div class="intro"><h2>{ru?'Подключи аккаунт или организацию':'Connect an account or organization'}</h2><p>{ru?'Yevqo перенаправит тебя на GitHub. Выбери аккаунт и репозитории, к которым приложению разрешён доступ. После возврата Control Plane проверит Installation ID и подпись приложения.':'Yevqo will redirect you to GitHub. Choose an account and repositories the app may access. On return, the Control Plane verifies the installation ID using the App credentials held on the server.'}</p><button class="primary" disabled={connecting} on:click={startInstall}>{connecting?(ru?'Подготовка…':'Preparing…'):(ru?'Установить Yevqo на GitHub':'Install Yevqo on GitHub')} ↗</button></div>
 {:else}
  <div class="connected-summary"><div><span class="field-label">{ru?'Подключённый аккаунт':'Connected account'}</span><strong>{integration.account_name}</strong></div><div><span class="field-label">Installation ID</span><strong class="mono">{integration.account_id}</strong></div><div><span class="field-label">{ru?'Последняя проверка':'Last verified'}</span><strong>{integration.last_verified_at?formatDate(integration.last_verified_at):'—'}</strong></div></div>
  <div class="actions"><button class="outline" on:click={loadRepositories}>{ru?'Показать репозитории':'View repositories'}</button><a class="outline" href="https://github.com/settings/installations" target="_blank" rel="noreferrer">{ru?'Управлять установкой':'Manage installation'} ↗</a><button class="danger" on:click={disconnect}>{ru?'Отключить':'Disconnect'}</button></div>
  {#if repositoriesLoading}<div class="empty">{ru?'Загружаем репозитории…':'Loading repositories…'}</div>
  {:else if repositories.length}
   <div class="repositories"><div class="repo-heading"><strong>{ru?'Доступные репозитории':'Available repositories'}</strong><span>{repositories.length}</span></div>
   {#each repositories as repo}<div class="repo-row"><div class="repo-mark">⑂</div><div class="repo-copy"><strong>{repo.full_name||repo.name}</strong><small>{repo.private?(ru?'Приватный':'Private'):(ru?'Публичный':'Public')} · {ru?'Ветка':'Branch'}: {repo.default_branch||'—'}</small></div><a href={repo.web_url} target="_blank" rel="noreferrer">↗</a></div>{/each}
   </div>
  {/if}
 {/if}
 {#if errorState}<div class="error-state">{errorState}</div>{/if}
</section>
<section class="permission-card"><h2>{ru?'Запрашиваемые разрешения':'Requested permissions'}</h2><div class="permission-row"><span>Metadata</span><span>{ru?'Только чтение':'Read-only'}</span></div><div class="permission-row"><span>Contents</span><span>{ru?'Только чтение':'Read-only'}</span></div><p>{ru?'Дополнительные разрешения не запрашиваются. Webhook events и автоматический Deploy включаются только после отдельной настройки.':'No additional permissions are requested. Webhook events and automatic deployments require separate configuration.'}</p></section>
<section class="setup-card"><h2>{ru?'Настройка GitHub App для владельца Yevqo':'GitHub App setup for the Yevqo operator'}</h2><ol><li>{ru?'Зарегистрируй GitHub App в GitHub Developer Settings.':'Register a GitHub App in GitHub Developer Settings.'}</li><li>{ru?'Укажи Setup URL: адрес панели + /settings/integrations/github.':'Set the Setup URL to your panel origin + /settings/integrations/github.'}</li><li>{ru?'Включи Metadata: Read-only и Contents: Read-only.':'Enable Metadata: Read-only and Contents: Read-only.'}</li><li>{ru?'Задай YEVQO_GITHUB_APP_ID, YEVQO_GITHUB_APP_SLUG и YEVQO_GITHUB_APP_PRIVATE_KEY в окружении Control Plane.':'Set YEVQO_GITHUB_APP_ID, YEVQO_GITHUB_APP_SLUG and YEVQO_GITHUB_APP_PRIVATE_KEY in the Control Plane environment.'}</li><li>{ru?'Перезапусти Control Plane и нажми «Установить Yevqo на GitHub».':'Restart the Control Plane and click “Install Yevqo on GitHub”.'}</li></ol></section>
<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:var(--accent);font-weight:700;margin-bottom:7px}.back-link{font-size:10px;color:var(--text-muted)}.github-card,.permission-card,.setup-card{border:1px solid var(--border);border-radius:10px;background:var(--bg-card);margin-bottom:12px}.github-card{padding:18px}.app-heading{display:flex;align-items:center;gap:12px;padding-bottom:17px;border-bottom:1px solid var(--border)}.github-logo{width:42px;height:42px;display:grid;place-items:center;border-radius:10px;background:#171d19;color:var(--text-main);font-weight:800;font-size:13px}.app-title{display:grid;gap:3px;flex:1}.app-title strong{font-size:13px}.app-title span,.field-label{font-size:10px;color:var(--text-muted)}.state-pill{padding:4px 8px;border:1px solid var(--border);border-radius:20px;color:var(--text-muted);font-size:9px}.state-pill.connected{color:var(--accent);border-color:#8AB4FF55;background:#8AB4FF0D}.intro{padding:20px 2px 4px}.intro h2,.permission-card h2,.setup-card h2{font-size:13px;margin-bottom:7px}.intro p,.setup-card li,.permission-card p{font-size:10px;line-height:1.7;color:var(--text-muted);max-width:760px}.primary,.outline,.danger{display:inline-flex;align-items:center;gap:7px;padding:8px 11px;border-radius:7px;font-size:10px;font-weight:600}.primary{margin-top:16px;background:var(--accent);border:1px solid var(--accent);color:#080A09}.outline{border:1px solid var(--border);background:transparent;color:var(--text-main)}.danger{border:1px solid #EF646455;background:transparent;color:#EF6464}.setup-warning,.error-state{margin-top:16px;padding:13px;border:1px solid #E9B85C55;border-radius:8px;background:#E9B85C0C;color:var(--warning);font-size:10px}.setup-warning p{margin-top:6px;line-height:1.7;color:var(--text-muted)}.error-state{border-color:#EF646455;background:#EF64640C;color:#EF6464}.connected-summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;padding:17px 0}.connected-summary>div{display:grid;gap:5px;min-width:0}.connected-summary strong{font-size:11px;overflow-wrap:anywhere}.mono{font-family:var(--mono)}.actions{display:flex;flex-wrap:wrap;gap:8px}.repositories{margin-top:18px;border-top:1px solid var(--border);padding-top:13px}.repo-heading{display:flex;justify-content:space-between;margin-bottom:8px;font-size:11px}.repo-heading span{color:var(--text-muted)}.repo-row{display:flex;align-items:center;gap:10px;padding:10px 2px;border-top:1px solid var(--border)}.repo-mark{color:var(--accent)}.repo-copy{display:grid;gap:3px;flex:1;min-width:0}.repo-copy strong{font-size:10px}.repo-copy small{font-size:9px;color:var(--text-muted)}.permission-card,.setup-card{padding:16px}.permission-row{display:flex;justify-content:space-between;padding:9px 0;border-bottom:1px solid var(--border);font-size:10px}.permission-row span:last-child{color:var(--accent)}.permission-card p{margin-top:10px}.setup-card ol{padding-left:19px}.setup-card li{padding:3px 0}
@media(max-width:620px){.connected-summary{grid-template-columns:1fr}.app-heading{align-items:flex-start}.state-pill{margin-left:auto}.actions>*{flex:1;justify-content:center}}
</style>