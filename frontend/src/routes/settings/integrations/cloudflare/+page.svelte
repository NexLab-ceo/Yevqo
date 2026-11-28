<script>
 import {onMount} from 'svelte';
 import {getCloudflareConfig,listCloudflareZones,saveCloudflareConfig,disconnectCloudflare,startCloudflareOAuth,listCloudflareOAuthZones,selectCloudflareOAuthZone} from '$lib/api.js';
 import {locale} from '$lib/i18n.js';
 import {notify} from '$lib/stores.js';
 import Button from '$lib/components/Button.svelte';

 let config={connected:false,needs_zone_selection:false};
 let loading=true,busy=false,error='';
 let tokenDraft='',tokenZones=[],tokenZoneDraft='';
 let oauthZones=[],oauthZoneDraft='';
 $: ru=$locale!=='en';
 $: connected=!!config.connected&&!config.needs_zone_selection;

 onMount(async()=>{
  const result=await getCloudflareConfig();
  if(result.ok)config=result.data||{connected:false};
  else error=result.error||'Could not load Cloudflare connection';
  loading=false;
  const callbackParams=new URLSearchParams(window.location.search);
  const status=callbackParams.get('cloudflare_oauth');
  const oauthError=callbackParams.get('cloudflare_oauth_error')||'';
  if(status==='connected'){
   notify(ru?'Cloudflare авторизован. Теперь выбери DNS-зону.':'Cloudflare authorized. Select a DNS zone next.','success');
   await loadOAuthZones();
  }else if(status==='denied'){
   if(oauthError==='access_denied'){
    error=ru?'Cloudflare отклонил запрос доступа (access_denied). Разреши доступ на странице авторизации Cloudflare и попробуй снова.':'Cloudflare denied the authorization request (access_denied). Approve access on the Cloudflare authorization page and try again.';
   }else if(oauthError==='invalid_scope'){
    error=ru?'Cloudflare отклонил запрошенные разрешения (invalid_scope). Проверь scopes OAuth-приложения.':'Cloudflare rejected the requested permissions (invalid_scope). Check the OAuth application scopes.';
   }else if(oauthError==='unauthorized_client'||oauthError==='invalid_client'){
    error=ru?'Cloudflare не принял OAuth-клиент (unauthorized_client/invalid_client). Проверь Client ID, Client Secret и Redirect URL в настройках приложения.':'Cloudflare rejected the OAuth client (unauthorized_client/invalid_client). Check the Client ID, Client Secret and Redirect URL in the application settings.';
   }else{
    error=(ru?'Cloudflare отказал в OAuth-доступе':'Cloudflare denied OAuth access')+(oauthError?' ('+oauthError+')':'')+'. '+(ru?'Проверь настройки OAuth-приложения и повтори подключение.':'Check the OAuth application settings and try again.');
   }
  }else if(status==='error'){
   error=ru?'Не удалось завершить Cloudflare OAuth. Проверь настройки OAuth приложения.':'Cloudflare OAuth could not be completed. Check the OAuth application settings.';
  }
  if(status)window.history.replaceState({},'',window.location.pathname);
  if(status!=='connected'&&config.connected&&config.needs_zone_selection&&!oauthZones.length&&config.auth_method==='oauth')await loadOAuthZones();
 });

 async function startOAuth(){
  if(busy)return;
  busy=true;error='';
  const result=await startCloudflareOAuth('/settings/integrations/cloudflare');
  busy=false;
  if(!result.ok||!result.data?.authorization_url){error=result.error||'Could not start Cloudflare OAuth';return}
  window.location.assign(result.data.authorization_url);
 }
 async function loadOAuthZones(){
  busy=true;
  const result=await listCloudflareOAuthZones();
  busy=false;
  if(!result.ok){error=result.error||'Could not load Cloudflare zones';return}
  oauthZones=result.data?.zones||[];
  oauthZoneDraft=oauthZones.length===1?oauthZones[0].id:'';
  if(!oauthZones.length)error=ru?'В аккаунте нет активных зон с выданными разрешениями.':'No active zones are available with the granted permissions.';
 }
 async function chooseOAuthZone(){
  if(busy||!oauthZoneDraft)return;
  busy=true;error='';
  const result=await selectCloudflareOAuthZone(oauthZoneDraft);
  busy=false;
  if(!result.ok){error=result.error||'Could not select Cloudflare zone';return}
  config={...config,...result.data,connected:true,needs_zone_selection:false};
  oauthZones=[];
  notify(ru?'Cloudflare подключён':'Cloudflare connected','success');
 }
 async function loadTokenZones(){
  if(busy||!tokenDraft.trim())return;
  busy=true;error='';
  const result=await listCloudflareZones(tokenDraft.trim());
  busy=false;
  if(!result.ok){error=result.error||'Could not load Cloudflare zones';return}
  tokenZones=result.data?.zones||[];
  tokenZoneDraft=tokenZones.length===1?tokenZones[0].id:'';
  if(!tokenZones.length)error=ru?'Активные домены Cloudflare не найдены.':'No active Cloudflare zones found.';
 }
 async function connectToken(){
  if(busy||!tokenDraft.trim()||!tokenZoneDraft)return;
  busy=true;error='';
  const result=await saveCloudflareConfig(tokenDraft.trim(),tokenZoneDraft);
  busy=false;
  if(!result.ok){error=result.error||'Could not connect Cloudflare';return}
  config={...config,...result.data,connected:true,needs_zone_selection:false,auth_method:'api_token'};
  tokenDraft='';tokenZones=[];
  notify(ru?'Cloudflare подключён':'Cloudflare connected','success');
 }
 async function disconnect(){
  if(busy||!confirm(ru?'Отключить Cloudflare? Сначала удалите все управляемые маршруты Tunnel в настройках приложений.':'Disconnect Cloudflare? Remove all managed Tunnel routes in application settings first.'))return;
  busy=true;error='';
  const result=await disconnectCloudflare();
  busy=false;
  if(!result.ok){error=result.error||'Could not disconnect Cloudflare';return}
  config={connected:false};oauthZones=[];tokenZones=[];
  notify(ru?'Cloudflare отключён':'Cloudflare disconnected','success');
 }
</script>

<svelte:head><title>Cloudflare · Yevqo</title></svelte:head>
<div class="page-header"><div><div class="eyebrow">SETTINGS / INTEGRATIONS</div><h1 class="page-title">Cloudflare</h1><p class="page-desc">{ru?'Подключи Cloudflare для управления DNS и автоматической настройки Tunnel для приложений.':'Connect Cloudflare to manage DNS and configure application Tunnels.'}</p></div><a class="back-link" href="/settings">← {ru?'Настройки':'Settings'}</a></div>

{#if loading}
 <div class="empty">{ru?'Проверяем подключение…':'Checking connection…'}</div>
{:else}
 <section class="integration-card">
  <div class="provider-heading"><div class="provider-mark">CF</div><div class="provider-copy"><strong>Cloudflare</strong><small>{ru?'DNS · Cloudflare Tunnel · OAuth':'DNS · Cloudflare Tunnel · OAuth'}</small></div><span class:connected class="state">{connected?(ru?'Подключено':'Connected'):config.connected?(ru?'Выбери зону':'Select a zone'):(ru?'Не подключено':'Not connected')}</span></div>
  {#if connected}
   <div class="connected-info"><div><small>{ru?'Аккаунт Cloudflare':'Cloudflare account'}</small><strong class="mono">{config.account_id||'—'}</strong></div><div><small>{ru?'Активная зона':'Selected zone'}</small><strong>{config.zone_name||'—'}</strong></div><div><small>{ru?'Метод авторизации':'Authentication'}</small><strong>{config.auth_method==='oauth'?'OAuth':'API Token'}</strong></div></div>
   <div class="actions"><a class="outline" href="https://dash.cloudflare.com/" target="_blank" rel="noreferrer">{ru?'Открыть Cloudflare':'Open Cloudflare'} ↗</a><button class="danger" disabled={busy} on:click={disconnect}>{ru?'Отключить':'Disconnect'}</button></div>
   <p class="hint">{ru?'DNS и Tunnel-маршруты настраиваются в параметрах конкретного приложения. Перед отключением удалите все управляемые маршруты.':'DNS and Tunnel routes are configured in each application settings. Remove all managed routes before disconnecting.'}</p>
  {:else if config.connected&&config.needs_zone_selection}
   <div class="connect-section"><h2>{ru?'Выбери DNS-зону':'Select a DNS zone'}</h2><p>{ru?'OAuth авторизация прошла. Выбери активную зону, которой Yevqo сможет управлять.':'OAuth authorization succeeded. Choose the active zone Yevqo should manage.'}</p>
    {#if !oauthZones.length}<Button loading={busy} disabled={busy} on:click={loadOAuthZones}>{ru?'Загрузить зоны':'Load zones'}</Button>
    {:else}<label>{ru?'Доменная зона':'Domain zone'}<select class="input" bind:value={oauthZoneDraft}><option value="">{ru?'Выбери зону':'Select a zone'}</option>{#each oauthZones as zone}<option value={zone.id}>{zone.name} · {zone.account_name}</option>{/each}</select></label><Button variant="primary" loading={busy} disabled={busy||!oauthZoneDraft} on:click={chooseOAuthZone}>{ru?'Подключить выбранную зону':'Connect selected zone'}</Button>{/if}
   </div>
  {:else}
   <div class="connect-section"><h2>{ru?'Подключение через OAuth':'Connect with OAuth'}</h2><p>{ru?'Рекомендуемый способ. Yevqo перенаправит тебя в Cloudflare, где можно разрешить доступ, не копируя API Token. После возврата выбери DNS-зону.':'Recommended. Yevqo redirects you to Cloudflare to authorize access without copying an API token. Select a DNS zone after returning.'}</p><Button variant="primary" loading={busy} disabled={busy} on:click={startOAuth}>{ru?'Подключить Cloudflare':'Connect Cloudflare'} ↗</Button></div>
   <div class="divider"></div>
   <div class="connect-section"><h2>{ru?'Или подключить через API Token':'Or connect with an API Token'}</h2><p>{ru?'Используй ограниченный токен с правами Zone:Read, DNS:Edit и Cloudflare Tunnel:Edit. Токен будет сохранён в зашифрованном виде.':'Use a scoped token with Zone:Read, DNS:Edit and Cloudflare Tunnel:Edit permissions. The token is stored encrypted.'}</p><label>{ru?'Cloudflare API Token':'Cloudflare API Token'}<input class="input" type="password" bind:value={tokenDraft} placeholder="API Token" autocomplete="new-password"/></label><Button loading={busy} disabled={busy||!tokenDraft.trim()} on:click={loadTokenZones}>{ru?'Загрузить доступные зоны':'Load available zones'}</Button>
    {#if tokenZones.length}<label>{ru?'Доменная зона':'Domain zone'}<select class="input" bind:value={tokenZoneDraft}><option value="">{ru?'Выбери зону':'Select a zone'}</option>{#each tokenZones as zone}<option value={zone.id}>{zone.name} · {zone.account_name}</option>{/each}</select></label><Button variant="primary" loading={busy} disabled={busy||!tokenZoneDraft} on:click={connectToken}>{ru?'Сохранить подключение':'Save connection'}</Button>{/if}
   </div>
  {/if}
  {#if error}<div class="error">{error}</div>{/if}
 </section>
 <section class="details-card"><h2>{ru?'Что даёт интеграция':'What this integration enables'}</h2><div class="detail-row"><strong>DNS</strong><span>{ru?'Создание и удаление управляемых CNAME-записей':'Create and remove managed CNAME records'}</span></div><div class="detail-row"><strong>Cloudflare Tunnel</strong><span>{ru?'Создание Tunnel и настройка ingress для приложения':'Create Tunnels and configure application ingress'}</span></div><div class="detail-row"><strong>Credentials</strong><span>{ru?'Токены хранятся зашифрованными; OAuth поддерживает обновление и отзыв токенов':'Tokens are encrypted; OAuth supports token refresh and revocation'}</span></div></section>
{/if}

<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:var(--accent);font-weight:700;margin-bottom:7px}.back-link{font-size:10px;color:var(--text-muted)}.integration-card,.details-card{padding:18px;border:1px solid var(--border);border-radius:10px;background:var(--bg-card);margin-bottom:12px}.provider-heading{display:flex;align-items:center;gap:12px;padding-bottom:15px;border-bottom:1px solid var(--border)}.provider-mark{width:42px;height:42px;display:grid;place-items:center;border-radius:10px;background:var(--bg-elevated);font-size:13px;font-weight:800}.provider-copy{display:grid;gap:4px;flex:1}.provider-copy strong{font-size:13px}.provider-copy small,.connected-info small{font-size:10px;color:var(--text-muted)}.state{padding:4px 8px;border:1px solid var(--border);border-radius:20px;font-size:9px;color:var(--text-muted)}.state.connected{color:var(--accent);border-color:#8AB4FF55;background:#8AB4FF0D}.connected-info{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;padding:17px 0}.connected-info>div{display:grid;gap:5px;min-width:0}.connected-info strong{font-size:11px;overflow-wrap:anywhere}.mono{font-family:var(--mono)}.actions{display:flex;gap:8px;flex-wrap:wrap}.outline,.danger{display:inline-flex;align-items:center;padding:8px 11px;border:1px solid var(--border);border-radius:7px;background:transparent;color:var(--text-main);font-size:10px}.danger{border-color:#EF646455;color:#EF6464}.connect-section{padding:17px 0 3px}.connect-section h2,.details-card h2{font-size:13px;margin-bottom:6px}.connect-section p,.hint{max-width:760px;margin-bottom:13px;color:var(--text-muted);font-size:10px;line-height:1.7}.connect-section label{display:grid;gap:6px;max-width:520px;margin:12px 0;font-size:10px;color:var(--text-muted)}.connect-section .input{width:100%}.divider{height:1px;margin:16px 0;background:var(--border)}.hint{margin-top:12px;margin-bottom:0}.error{margin-top:14px;padding:12px;border:1px solid #EF646455;border-radius:8px;background:#EF64640C;color:#EF6464;font-size:10px}.details-card h2{margin-bottom:8px}.detail-row{display:flex;gap:18px;padding:10px 0;border-top:1px solid var(--border);font-size:10px}.detail-row strong{min-width:145px}.detail-row span{color:var(--text-muted)}@media(max-width:620px){.connected-info{grid-template-columns:1fr}.provider-heading{align-items:flex-start}.state{margin-left:auto}.detail-row{display:grid;gap:5px}}
</style>
