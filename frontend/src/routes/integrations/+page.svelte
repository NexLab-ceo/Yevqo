<script>
 import {locale} from '$lib/i18n.js';

 let activeTab = 'services';
 let search = '';
 let selected = null;

 const text = {
  ru: {
   eyebrow:'ПОДКЛЮЧЕНИЯ', title:'Интеграции',
   description:'Подключайте внешние сервисы и управляйте обменом данными с VELTRIX Dock.',
   services:'Сервисы', webhooks:'Вебхуки', search:'Поиск интеграций…',
   all:'Все', git:'Git и репозитории', infra:'Инфраструктура', alerts:'Уведомления', identity:'Аккаунты',
   available:'Не подключено', connected:'Подключено', unavailable:'Подключение пока недоступно',
   details:'Возможности', security:'Безопасность подключений',
   securityText:'Перед включением интеграций серверная часть должна безопасно хранить токены, запрашивать минимальные разрешения и поддерживать отзыв доступа. Пока VELTRIX не принимает и не сохраняет секреты.',
   back:'Назад к интеграциям', close:'Закрыть', noResults:'Ничего не найдено',
   webhooksTitle:'Исходящие вебхуки', webhooksDesc:'Отправляйте события VELTRIX во внешние сервисы: автоматизацию, мониторинг или собственные приложения.',
   endpoint:'Адрес получателя', events:'События', signing:'Подпись запросов', deliveries:'История доставки',
   endpointHint:'Настройка endpoint станет доступна после реализации серверного обработчика.',
   signingHint:'Каждый запрос будет подписываться секретным ключом. Сам ключ будет скрыт и храниться в зашифрованном виде.',
   deliveriesHint:'Здесь появятся статусы доставки, ответы сервера и возможность повторной отправки.',
   eventNames:['Сайт запущен','Сайт остановлен','Развёртывание завершено','Ошибка развёртывания','Устройство отключилось'],
   eventCodes:['website.started','website.stopped','deployment.success','deployment.failed','device.offline'],
   coming:'Будет доступно после подключения серверной части',
   providerDetails:'Что можно будет делать',
   gitHub:['Просматривать репозитории и ветки','Получать изменения через Git Pull','Запускать развёртывание из выбранного репозитория','Отслеживать события CI/CD'],
   gitLab:['Подключать проекты и репозитории','Получать события CI/CD','Запускать обновление исходного кода','Отслеживать состояние pipeline'],
   cloudflare:['Управлять DNS-записями и доменами','Создавать и управлять Cloudflare Tunnel','Настраивать маршрутизацию доменов на сайты','Проверять состояние SSL/TLS'],
   discord:['Отправлять уведомления в выбранный канал','Получать оповещения о развёртываниях','Отслеживать состояние устройств и сайтов'],
   telegram:['Отправлять уведомления через Telegram-бота','Получать сообщения об ошибках и сбоях','Настраивать список событий для оповещений'],
   google:['Использовать аккаунт Google для авторизации','Связывать рабочее пространство с аккаунтом'],
   webhooksFeatures:['Выбирать события для отправки','Использовать подпись HMAC для проверки запросов','Просматривать историю доставок и ответы','Повторно отправлять неудачные запросы']
  },
  en: {
   eyebrow:'CONNECTIVITY', title:'Integrations',
   description:'Connect external services and manage data exchange with VELTRIX Dock.',
   services:'Services', webhooks:'Webhooks', search:'Search integrations…',
   all:'All', git:'Git & repositories', infra:'Infrastructure', alerts:'Notifications', identity:'Accounts',
   available:'Not connected', connected:'Connected', unavailable:'Connection unavailable',
   details:'Capabilities', security:'Connection security',
   securityText:'Before integrations are enabled, the server must securely store tokens, request minimal permissions and support access revocation. VELTRIX does not accept or store secrets yet.',
   back:'Back to integrations', close:'Close', noResults:'No integrations found',
   webhooksTitle:'Outgoing webhooks', webhooksDesc:'Send VELTRIX events to external services, automation tools, monitoring systems or your own applications.',
   endpoint:'Destination URL', events:'Events', signing:'Request signing', deliveries:'Delivery history',
   endpointHint:'Endpoint configuration will be available after the server handler is implemented.',
   signingHint:'Each request will be signed with a secret key. The key will be hidden and stored encrypted.',
   deliveriesHint:'Delivery status, server responses and retry actions will appear here.',
   eventNames:['Website started','Website stopped','Deployment succeeded','Deployment failed','Device went offline'],
   eventCodes:['website.started','website.stopped','deployment.success','deployment.failed','device.offline'],
   coming:'Available after the server-side integration is implemented',
   providerDetails:'What you will be able to do',
   gitHub:['Browse repositories and branches','Pull source changes with Git Pull','Deploy from a selected repository','Receive CI/CD events'],
   gitLab:['Connect projects and repositories','Receive CI/CD events','Trigger source updates','Track pipeline status'],
   cloudflare:['Manage DNS records and domains','Create and manage Cloudflare Tunnels','Route domains to hosted sites','Inspect SSL/TLS status'],
   discord:['Send notifications to a selected channel','Receive deployment alerts','Track device and website status'],
   telegram:['Send alerts through a Telegram bot','Receive error and outage messages','Choose which events trigger notifications'],
   google:['Use a Google account for authentication','Link a workspace to your account'],
   webhooksFeatures:['Choose events to send','Use HMAC signatures to verify requests','Inspect delivery history and responses','Retry failed deliveries']
  }
 };

 const providers = [
  {name:'GitHub',category:'git',group:'git',icon:'GH',key:'gitHub',descRu:'Репозитории, Git Pull и запуск развёртывания',descEn:'Repositories, Git Pull and deployment triggers'},
  {name:'GitLab',category:'git',group:'git',icon:'GL',key:'gitLab',descRu:'Проекты, репозитории и события CI/CD',descEn:'Projects, repositories and CI/CD events'},
  {name:'Cloudflare',category:'infra',group:'infra',icon:'CF',key:'cloudflare',descRu:'DNS, домены, SSL/TLS и Cloudflare Tunnel',descEn:'DNS, domains, SSL/TLS and Cloudflare Tunnel'},
  {name:'Discord',category:'alerts',group:'alerts',icon:'DC',key:'discord',descRu:'Оповещения в каналы Discord',descEn:'Operational alerts in Discord channels'},
  {name:'Telegram',category:'alerts',group:'alerts',icon:'TG',key:'telegram',descRu:'Уведомления через Telegram-бота',descEn:'Notifications through a Telegram bot'},
  {name:'Google',category:'identity',group:'identity',icon:'G',key:'google',descRu:'Аккаунт Google и авторизация',descEn:'Google account and authentication'}
 ];

 $: tr = text[$locale] || text.ru;
 $: filtered = providers.filter(p => (activeTab === 'services') && (p.name.toLowerCase().includes(search.toLowerCase()) || ( $locale === 'ru' ? p.descRu : p.descEn).toLowerCase().includes(search.toLowerCase())));
 $: selectedProvider = providers.find(p => p.name === selected);
</script>

<svelte:head><title>Интеграции · VELTRIX Dock</title></svelte:head>

<div class="page-header">
 <div><div class="eyebrow">{tr.eyebrow}</div><h1 class="page-title">{tr.title}</h1><p class="page-desc">{tr.description}</p></div>
</div>

<div class="integration-tabs">
 <button class:active={activeTab==='services'} on:click={()=>{activeTab='services';selected=null}}>{tr.services}<span>{providers.length}</span></button>
 <button class:active={activeTab==='webhooks'} on:click={()=>{activeTab='webhooks';selected=null}}>{tr.webhooks}<span>5</span></button>
</div>

{#if activeTab === 'services'}
 {#if selectedProvider}
  <button class="back-link" on:click={()=>selected=null}>← {tr.back}</button>
  <section class="detail-panel">
   <div class="detail-heading"><span class="provider-icon large">{selectedProvider.icon}</span><div><h2>{selectedProvider.name}</h2><p>{$locale==='ru'?selectedProvider.descRu:selectedProvider.descEn}</p></div><span class="status">{tr.available}</span></div>
   <div class="detail-content"><h3>{tr.providerDetails}</h3><ul>{#each tr[selectedProvider.key] as feature}<li>{feature}</li>{/each}</ul></div>
   <div class="detail-footer"><span class="lock-icon">⌑</span><span>{tr.coming}</span><button disabled>{tr.unavailable}</button></div>
  </section>
 {:else}
  <div class="toolbar"><div class="search-box"><span>⌕</span><input bind:value={search} placeholder={tr.search}/></div><span class="result-count">{filtered.length} / {providers.length}</span></div>
  <div class="provider-grid">
   {#each filtered as p}
    <button class="provider-card" on:click={()=>selected=p.name}>
     <div class="provider-top"><span class="provider-icon">{p.icon}</span><span class="status"><i></i>{tr.available}</span></div>
     <strong>{p.name}</strong>
     <p>{$locale==='ru'?p.descRu:p.descEn}</p>
     <span class="card-link">{tr.details} <b>→</b></span>
    </button>
   {/each}
  </div>
  {#if filtered.length===0}<div class="empty-state">{tr.noResults}</div>{/if}
 {/if}
 <section class="security-note"><span class="security-icon">⌑</span><div><strong>{tr.security}</strong><p>{tr.securityText}</p></div></section>
{:else}
 <section class="webhook-intro"><div class="webhook-symbol">⌁</div><div><h2>{tr.webhooksTitle}</h2><p>{tr.webhooksDesc}</p></div><span class="status pending">{tr.coming}</span></section>
 <div class="webhook-grid">
  <article class="webhook-card"><span class="mini-icon">↗</span><h3>{tr.endpoint}</h3><p>{tr.endpointHint}</p><div class="disabled-field">https://example.com/webhook <span>⌑</span></div></article>
  <article class="webhook-card"><span class="mini-icon">⌁</span><h3>{tr.events}</h3><p>{$locale==='ru'?'Доступные типы событий VELTRIX':'Available VELTRIX event types'}</p><div class="event-list">{#each tr.eventCodes as code,i}<div><code>{code}</code><span>{tr.eventNames[i]}</span></div>{/each}</div></article>
  <article class="webhook-card"><span class="mini-icon">♢</span><h3>{tr.signing}</h3><p>{tr.signingHint}</p><div class="disabled-field">•••••••••••••••• <span>⌑</span></div></article>
  <article class="webhook-card"><span class="mini-icon">≡</span><h3>{tr.deliveries}</h3><p>{tr.deliveriesHint}</p><div class="empty-deliveries">— {$locale==='ru'?'Нет отправок':'No deliveries'} —</div></article>
 </div>
 <section class="security-note"><span class="security-icon">⌑</span><div><strong>{tr.security}</strong><p>{tr.securityText}</p></div></section>
{/if}

<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:#60A5FA;font-weight:750;margin-bottom:7px}
.integration-tabs{display:flex;gap:5px;border-bottom:1px solid #27272A;margin:4px 0 17px}
.integration-tabs button{display:flex;align-items:center;gap:8px;padding:10px 12px;margin-bottom:-1px;background:transparent;border:0;border-bottom:2px solid transparent;color:#A1A1AA;font-size:11px;font-weight:600}
.integration-tabs button.active{color:#FAFAFA;border-color:#3B82F6}
.integration-tabs button span{padding:2px 6px;border-radius:10px;background:#27272A;color:#A1A1AA;font-size:9px}
.toolbar{display:flex;align-items:center;gap:12px;margin-bottom:13px}.search-box{height:35px;display:flex;align-items:center;gap:9px;padding:0 11px;max-width:360px;flex:1;background:#18181B;border:1px solid #27272A;border-radius:8px;color:#71717A}.search-box input{width:100%;min-width:0;border:0;outline:0;background:transparent;color:#FAFAFA;font-size:10px}.result-count{font-size:9px;color:#71717A}
.provider-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:11px}
.provider-card{min-height:145px;padding:14px;text-align:left;border:1px solid #27272A;border-radius:11px;background:#101014;color:#FAFAFA;display:flex;flex-direction:column;align-items:stretch;transition:border-color .16s,transform .16s}
.provider-card:hover{border-color:#3B82F6;transform:translateY(-1px)}
.provider-top{display:flex;align-items:center;justify-content:space-between;margin-bottom:13px}
.provider-icon{width:34px;height:34px;display:grid;place-items:center;border-radius:9px;background:#18181B;border:1px solid #27272A;color:#93C5FD;font-size:11px;font-weight:800;letter-spacing:-.03em;flex-shrink:0}
.provider-icon.large{width:44px;height:44px;font-size:13px}
.status{display:inline-flex;align-items:center;gap:6px;white-space:nowrap;font-size:9px;color:#A1A1AA}
.status i{width:6px;height:6px;border-radius:50%;background:#71717A}
.provider-card>strong{font-size:12px;font-weight:700}.provider-card p{font-size:10px;color:#A1A1AA;line-height:1.5;margin:5px 0 12px;min-height:30px}
.card-link{margin-top:auto;font-size:9px;color:#60A5FA}.card-link b{font-size:12px;margin-left:3px}
.security-note{display:flex;gap:11px;padding:13px;margin-top:15px;border:1px solid #334155;border-radius:10px;background:#101727;color:#A1A1AA}
.security-icon{width:27px;height:27px;display:grid;place-items:center;flex-shrink:0;border-radius:8px;background:#1E293B;color:#93C5FD}
.security-note strong{display:block;font-size:10px;color:#E2E8F0;margin-bottom:4px}.security-note p{font-size:9px;line-height:1.6;margin:0;color:#94A3B8}
.back-link{border:0;background:transparent;color:#60A5FA;font-size:10px;padding:0;margin:0 0 12px}
.detail-panel,.webhook-card,.webhook-intro{border:1px solid #27272A;border-radius:11px;background:#101014}
.detail-panel{overflow:hidden}.detail-heading{display:flex;align-items:center;gap:12px;padding:16px;border-bottom:1px solid #27272A}.detail-heading h2,.webhook-intro h2{font-size:14px;margin:0 0 4px}.detail-heading p,.webhook-intro p{font-size:10px;color:#A1A1AA;margin:0}.detail-heading .status{margin-left:auto}.detail-content{padding:16px}.detail-content h3,.webhook-card h3{font-size:11px;margin:0 0 10px}.detail-content ul{padding-left:17px;margin:0;display:grid;gap:8px}.detail-content li{font-size:10px;color:#A1A1AA;line-height:1.5}
.detail-footer{display:flex;align-items:center;gap:9px;padding:12px 16px;background:#0C0C10;border-top:1px solid #27272A;font-size:9px;color:#A1A1AA}.detail-footer button{margin-left:auto;padding:7px 10px;border:1px solid #27272A;border-radius:7px;background:#18181B;color:#71717A;font-size:9px;cursor:not-allowed}.lock-icon{font-size:14px;color:#60A5FA}
.webhook-intro{display:flex;align-items:center;gap:12px;padding:15px;margin-bottom:12px}.webhook-symbol{width:36px;height:36px;display:grid;place-items:center;background:#172554;border:1px solid #1D4ED8;border-radius:9px;color:#93C5FD;font-size:18px}.webhook-intro>div:nth-child(2){flex:1}.status.pending{white-space:normal;text-align:right;max-width:180px;color:#D6A85B}
.webhook-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:11px}.webhook-card{padding:14px;min-width:0}.mini-icon{display:grid;place-items:center;width:27px;height:27px;margin-bottom:10px;border-radius:8px;background:#18181B;color:#93C5FD;font-size:14px}.webhook-card p{min-height:30px;margin:0 0 11px;color:#A1A1AA;font-size:9px;line-height:1.5}
.disabled-field{display:flex;align-items:center;justify-content:space-between;padding:9px 10px;border:1px dashed #334155;border-radius:7px;background:#0C0C10;color:#64748B;font-size:9px;overflow:hidden}.event-list{display:grid;gap:7px}.event-list>div{display:flex;align-items:center;justify-content:space-between;gap:8px}.event-list code{font-size:9px;color:#93C5FD}.event-list span{font-size:9px;color:#A1A1AA;text-align:right}.empty-deliveries{padding:12px;border:1px dashed #334155;border-radius:7px;text-align:center;color:#64748B;font-size:9px}
.empty-state{padding:25px;text-align:center;color:#A1A1AA;font-size:11px;border:1px dashed #334155;border-radius:10px}
@media(max-width:850px){.provider-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:560px){.provider-grid,.webhook-grid{grid-template-columns:1fr}.webhook-intro{align-items:flex-start;flex-wrap:wrap}.webhook-intro .status{width:100%;max-width:none;text-align:left;margin-left:48px}.detail-heading{flex-wrap:wrap}.detail-heading .status{width:100%;margin-left:56px}.detail-footer{flex-wrap:wrap}.detail-footer button{margin-left:0}}
</style>
