<script>
 import {onMount} from 'svelte';
 import {goto} from '$app/navigation';
 import {listDevices,listSites} from '$lib/api.js';
 import {devices,notify,user} from '$lib/stores.js';
 import Button from '$lib/components/Button.svelte';
 import StatusDot from '$lib/components/StatusDot.svelte';
 import Modal from '$lib/components/Modal.svelte';
 let loading=true,showAddModal=false,deviceLimit=1,allSites=[];
 onMount(async()=>{
  const [d,s]=await Promise.all([listDevices(),listSites()]);
  if(d.ok){devices.set(d.data.devices||[]);deviceLimit=d.data.limit||1}
  else notify(d.error||'Не удалось загрузить устройства','error');
  if(s.ok)allSites=s.data.sites||[];
  loading=false;
 });
 $: onlineCount=$devices.filter(x=>x.status==='online').length;
 $: runningSites=allSites.filter(x=>x.site?.status==='running').length;
 $: offlineCount=$devices.length-onlineCount;
 function openDevice(device){goto('/devices/'+device.id)}
</script>
<svelte:head><title>Обзор · Yevqo</title></svelte:head>
<div class="dashboard-heading">
 <div><div class="eyebrow"><span class="eyebrow-dot"></span> WORKSPACE / OVERVIEW</div><h1 class="page-title">Обзор</h1><p class="page-desc">Состояние вашей инфраструктуры и подключённых Android-устройств.</p></div>
 <Button variant="primary" on:click={()=>{$devices.length>=deviceLimit?goto('/devices'):showAddModal=true}}>{$devices.length>=deviceLimit?'Лимит устройств достигнут':'＋ Добавить устройство'}</Button>
</div>
{#if loading}
 <div class="loading-state"><span class="loading-pulse"></span> Загружаем инфраструктуру…</div>
{:else}
 <section class="summary-grid" aria-label="Сводка инфраструктуры">
  <article class="summary-card"><div class="summary-label">СТАТУС СЕРВЕРА <span class="summary-glyph">⌁</span></div><div class="summary-value status-value"><i class="live-dot"></i> В сети</div><div class="summary-foot">Control Plane отвечает</div></article>
  <article class="summary-card"><div class="summary-label">УСТРОЙСТВА <span class="summary-glyph">▣</span></div><div class="summary-value">{onlineCount}<small> / {$devices.length}</small></div><div class="summary-foot">{offlineCount} не в сети</div></article>
  <article class="summary-card"><div class="summary-label">РАБОТАЮЩИЕ САЙТЫ <span class="summary-glyph">◫</span></div><div class="summary-value">{runningSites}<small> / {allSites.length}</small></div><div class="summary-foot">Приложения на устройствах</div></article>
  <article class="summary-card"><div class="summary-label">ЛИМИТ УСТРОЙСТВ <span class="summary-glyph">⌗</span></div><div class="summary-value">{Math.max(0,deviceLimit-$devices.length)}<small> свободно</small></div><div class="summary-foot">Текущий тариф</div></article>
 </section>
 <section class="content-grid">
  <article class="panel resource-panel">
   <div class="panel-heading"><div><h2>Ресурсы и состояние</h2><p>Сводка подключённой инфраструктуры</p></div><span class="panel-period">В РЕАЛЬНОМ ВРЕМЕНИ</span></div>
   <div class="resource-body">
    <div class="resource-visual" aria-hidden="true">
     <div class="chart-grid"><i></i><i></i><i></i><i></i></div>
     <svg viewBox="0 0 620 190" preserveAspectRatio="none" role="presentation"><path d="M0 142 C28 138 37 105 66 119 S105 154 133 112 S174 91 197 119 S232 132 260 89 S301 113 329 91 S365 101 394 75 S432 94 458 64 S495 84 523 52 S563 65 590 35 S608 27 620 18" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" opacity=".18"/></svg>
     <div class="chart-empty"><span class="chart-empty-icon">⌁</span><strong>Телеметрия устройства</strong><small>Подключите Agent, чтобы получать данные о CPU, памяти и нагрузке</small></div>
    </div>
    <div class="resource-legend"><span><i class="legend-green"></i> Подключённые устройства</span><span>Данные обновляются через Agent</span></div>
   </div>
  </article>
  <article class="panel activity-panel">
   <div class="panel-heading"><div><h2>Состояние системы</h2><p>Ключевые компоненты Yevqo</p></div><span class="panel-more">•••</span></div>
   <div class="activity-list">
    <div class="activity-item"><span class="activity-icon green">✓</span><span><strong>Control Plane</strong><small>Панель управления доступна</small></span><em class="state-ok">Активен</em></div>
    <div class="activity-item"><span class="activity-icon green">⌁</span><span><strong>Android Agent</strong><small>{$devices.length} подключено · {onlineCount} онлайн</small></span><em class:state-ok={onlineCount>0} class:state-muted={onlineCount===0}>{onlineCount>0?'Онлайн':'Ожидание'}</em></div>
    <div class="activity-item"><span class="activity-icon green">◫</span><span><strong>Приложения</strong><small>{runningSites} из {allSites.length} работают</small></span><em class:state-ok={runningSites>0} class:state-muted={runningSites===0}>{runningSites>0?'Работают':'Нет активных'}</em></div>
   </div>
   <a class="activity-link" href="/monitoring">Открыть мониторинг <span>→</span></a>
  </article>
 </section>
 <div class="section-heading"><div><h2>Подключённые устройства</h2><p>Android-хосты и их текущее состояние</p></div><a href="/devices">Все устройства <span>→</span></a></div>
 {#if $devices.length===0}
  <div class="empty-state"><div class="empty-mark">▣</div><h3>Пока нет подключённых устройств</h3><p>Установите Yevqo Agent в Termux, чтобы добавить первый Android-хост.</p><Button variant="primary" on:click={()=>showAddModal=true}>Подключить устройство</Button></div>
 {:else}
  <div class="device-grid">{#each $devices as device (device.id)}<button class="device-card" on:click={()=>openDevice(device)}><div class="device-top"><span class="device-symbol">▣</span><StatusDot status={device.status}/></div><div class="device-title">{device.hostname||device.device_id?.slice(0,8)||'Android device'}</div><div class="device-id">{device.device_id?.slice(0,18)||'—'}</div><div class="device-divider"></div><div class="device-stats"><span>IP <b>{device.tailscale_ip||'—'}</b></span><span>Сайты <b>{device.site_count||0}</b></span></div></button>{/each}</div>
 {/if}
 <div class="section-heading"><div><h2>Последние сайты</h2><p>Приложения на подключённых устройствах</p></div><a href="/sites">Все сайты <span>→</span></a></div>
 {#if allSites.length===0}<div class="empty-state compact-empty">Здесь появятся сайты, размещённые на ваших устройствах.</div>{:else}
  <div class="sites-table-wrap"><table class="sites-table"><thead><tr><th>САЙТ</th><th>УСТРОЙСТВО</th><th>RUNTIME</th><th>ПОРТ</th><th>СТАТУС</th></tr></thead><tbody>{#each allSites.slice(0,6) as item (item.device_id+'-'+item.site.site_id)}<tr on:click={()=>openDevice({id:item.device_id})}><td><strong>{item.site.site_id}</strong></td><td>{item.device_name||'Android host'}</td><td>{item.site.runtime}</td><td class="mono">:{item.site.port}</td><td><StatusDot status={item.site.status}/></td></tr>{/each}</tbody></table></div>
 {/if}
{/if}
<Modal open={showAddModal} title="Добавить устройство" onClose={()=>showAddModal=false}><p class="modal-text">Бесплатный тариф позволяет подключить один телефон. Чтобы заменить телефон, остановите Agent на старом устройстве, откройте его карточку в Devices и нажмите «Отвязать».</p><p class="modal-text">Чтобы подключить телефон:</p><ol class="modal-steps"><li>Установите Termux на телефон.</li><li>Запустите <code>bash scripts/install-agent.sh</code>.</li><li>Введите URL сервера и код привязки.</li><li>Запустите <code>sv start yevqo</code>.</li></ol><p class="modal-text">Код привязки: <code class="token">{$user?.referral_code||'—'}</code></p><svelte:fragment slot="footer"><Button on:click={()=>showAddModal=false}>Закрыть</Button></svelte:fragment></Modal>
<style>
.dashboard-heading{display:flex;align-items:flex-end;justify-content:space-between;gap:18px;margin-bottom:22px}.eyebrow{display:flex;align-items:center;gap:8px;margin-bottom:7px;color:var(--text-muted);font-size:9px;font-weight:750;letter-spacing:.15em}.eyebrow-dot{width:6px;height:6px;border-radius:50%;background:var(--accent);box-shadow:0 0 9px #A5EC7966}.dashboard-heading .page-title{font-size:24px;letter-spacing:-.045em}.dashboard-heading .page-desc{font-size:11px;color:var(--text-muted)}
.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));border:1px solid var(--border);border-radius:12px;background:var(--bg-card);overflow:hidden}.summary-card{min-height:104px;padding:15px 17px;border-right:1px solid var(--border)}.summary-card:last-child{border-right:0}.summary-label{display:flex;align-items:center;justify-content:space-between;color:var(--text-muted);font-size:8px;font-weight:750;letter-spacing:.1em}.summary-glyph{font-size:13px;color:var(--accent)}.summary-value{margin-top:10px;font-size:25px;font-weight:750;letter-spacing:-.055em;color:var(--text-main);line-height:1.1}.summary-value small{font-size:11px;color:var(--text-muted);font-weight:500;letter-spacing:0}.summary-foot{margin-top:6px;color:var(--text-muted);font-size:9px}.status-value{display:flex;align-items:center;gap:8px;font-size:18px}.live-dot{width:7px;height:7px;border-radius:50%;background:var(--accent);box-shadow:0 0 9px #A5EC79}
.content-grid{display:grid;grid-template-columns:minmax(0,1.6fr) minmax(270px,.9fr);gap:12px;margin-top:12px}.panel{min-width:0;border:1px solid var(--border);border-radius:12px;background:var(--bg-card);padding:16px}.panel-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}.panel-heading h2{font-size:11px;font-weight:700;color:var(--text-main)}.panel-heading p{margin-top:3px;color:var(--text-muted);font-size:9px}.panel-period{padding:4px 7px;border:1px solid var(--border);border-radius:5px;color:var(--text-muted);font-size:7px;letter-spacing:.08em;white-space:nowrap}.panel-more{color:var(--text-muted);letter-spacing:2px;font-size:10px}
.resource-visual{height:205px;position:relative;margin-top:15px;overflow:hidden}.chart-grid{position:absolute;inset:10px 0 8px;display:flex;flex-direction:column;justify-content:space-between}.chart-grid i{display:block;border-top:1px solid var(--border);opacity:.65}.resource-visual svg{position:absolute;inset:0;width:100%;height:100%;color:var(--accent)}.chart-empty{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;background:linear-gradient(90deg,var(--bg-card)dd,transparent,var(--bg-card)dd)}.chart-empty-icon{display:grid;place-items:center;width:29px;height:29px;border:1px solid var(--border);border-radius:8px;color:var(--accent);font-size:16px;margin-bottom:8px}.chart-empty strong{font-size:10px;color:var(--text-main)}.chart-empty small{max-width:260px;margin-top:4px;color:var(--text-muted);font-size:8px}.resource-legend{display:flex;justify-content:space-between;gap:10px;padding-top:10px;border-top:1px solid var(--border);color:var(--text-muted);font-size:8px}.resource-legend span:first-child{display:flex;align-items:center;gap:6px}.legend-green{width:6px;height:6px;border-radius:50%;background:var(--accent)}
.activity-list{margin-top:10px}.activity-item{display:flex;align-items:center;gap:9px;padding:12px 0;border-bottom:1px solid var(--border)}.activity-item:last-child{border-bottom:0}.activity-icon{flex:0 0 25px;width:25px;height:25px;display:grid;place-items:center;border-radius:7px;background:var(--bg-elevated);font-size:11px}.activity-icon.green{color:var(--accent)}.activity-item>span:nth-child(2){display:grid;gap:2px;min-width:0;flex:1}.activity-item strong{font-size:9px;font-weight:650}.activity-item small{font-size:8px;color:var(--text-muted)}.activity-item em{font-style:normal;font-size:8px;white-space:nowrap}.state-ok{color:var(--accent)}.state-muted{color:var(--text-muted)}.activity-link{display:flex;justify-content:space-between;align-items:center;margin-top:12px;padding-top:11px;border-top:1px solid var(--border);font-size:9px;color:var(--accent)}
.section-heading{display:flex;align-items:flex-end;justify-content:space-between;gap:12px;margin:24px 0 11px}.section-heading h2{font-size:12px;font-weight:700}.section-heading p{margin-top:3px;font-size:9px;color:var(--text-muted)}.section-heading>a{font-size:9px;color:var(--accent)}.section-heading>a span{margin-left:4px}
.device-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(215px,1fr));gap:10px}.device-card{text-align:left;background:var(--bg-card);border:1px solid var(--border);border-radius:11px;padding:14px;color:var(--text-main);transition:border-color .16s,transform .16s}.device-card:hover{border-color:var(--accent);transform:translateY(-1px)}.device-top{display:flex;justify-content:space-between;align-items:center}.device-symbol{width:28px;height:28px;border-radius:8px;display:grid;place-items:center;background:var(--bg-elevated);color:var(--accent)}.device-title{font-size:11px;font-weight:650;margin-top:11px}.device-id{font:8px var(--mono);color:var(--text-muted);margin-top:3px}.device-divider{height:1px;background:var(--border);margin:10px 0 8px}.device-stats{display:flex;justify-content:space-between;font-size:8px;color:var(--text-muted)}.device-stats b{color:var(--text-main);font-weight:500;margin-left:4px}
.sites-table-wrap{border:1px solid var(--border);border-radius:11px;overflow:auto;background:var(--bg-card)}.sites-table{width:100%;border-collapse:collapse;text-align:left;font-size:9px}.sites-table th{padding:10px 12px;color:var(--text-muted);font-size:8px;letter-spacing:.1em;font-weight:750;border-bottom:1px solid var(--border)}.sites-table td{padding:11px 12px;border-bottom:1px solid var(--border);color:var(--text-muted)}.sites-table tbody tr{cursor:pointer}.sites-table tbody tr:hover{background:var(--bg-elevated)}.sites-table tbody tr:last-child td{border:0}.sites-table td strong{color:var(--text-main)}.mono{font-family:var(--mono)}
.loading-state,.empty-state{padding:34px 20px;text-align:center;border:1px solid var(--border);border-radius:12px;background:var(--bg-card);color:var(--text-muted);font-size:10px}.loading-pulse{display:inline-block;width:7px;height:7px;margin-right:7px;border-radius:50%;background:var(--accent);animation:pulse 1s infinite alternate}@keyframes pulse{to{opacity:.25;transform:scale(.7)}}.empty-mark{display:grid;place-items:center;width:38px;height:38px;margin:0 auto 10px;border-radius:10px;background:var(--bg-elevated);color:var(--accent);font-size:17px}.empty-state h3{font-size:12px;color:var(--text-main)}.empty-state p{margin:5px 0 14px;font-size:9px;color:var(--text-muted)}.compact-empty{padding:20px;font-size:9px}.modal-text{font-size:11px;color:var(--text-muted);margin-bottom:12px}.modal-steps{padding-left:19px;color:var(--text-muted);font-size:11px;margin-bottom:14px}.modal-steps li{margin:5px 0}.modal-steps code,.token{font-family:var(--mono);background:var(--bg-base);border:1px solid var(--border);border-radius:5px;padding:3px 6px;color:var(--accent);font-size:10px}
@media(max-width:950px){.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.summary-card:nth-child(2){border-right:0}.summary-card:nth-child(-n+2){border-bottom:1px solid var(--border)}.content-grid{grid-template-columns:1fr}}
@media(max-width:620px){.dashboard-heading{align-items:flex-start;flex-direction:column}.dashboard-heading .page-title{font-size:21px}.summary-card{padding:12px}.summary-value{font-size:22px}.section-heading{align-items:flex-start}.device-grid{grid-template-columns:1fr}.resource-legend{flex-direction:column}}
</style>