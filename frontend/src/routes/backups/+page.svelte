<script>
 import {onMount,onDestroy} from 'svelte';import {listBackups,listSites,listDatabases,createSiteBackup,createDatabaseBackup,createControlPlaneBackup,listBackupSchedules,createBackupSchedule,deleteBackupSchedule} from '$lib/api.js';import {notify,user} from '$lib/stores.js';import Button from '$lib/components/Button.svelte';
 let backups=[],sites=[],databases=[],schedules=[],loading=true,busy=false,controlBusy=false,selected='',poller=null,pollBusy=false,scheduleBusy=false,intervalHours=24,retentionCount=10;
 function syncPolling(){const pending=backups.some((item)=>item.status==='queued'||item.status==='running');if(pending&&!poller){poller=setInterval(async()=>{if(pollBusy)return;pollBusy=true;const r=await listBackups();if(r.ok){backups=r.data.backups||[];if(!backups.some((item)=>item.status==='queued'||item.status==='running')){clearInterval(poller);poller=null}}pollBusy=false},3000)}else if(!pending&&poller){clearInterval(poller);poller=null}}
 async function load(){const [r,s,d,sc]=await Promise.all([listBackups(),listSites(),listDatabases(),listBackupSchedules()]);if(r.ok)backups=r.data.backups||[];else notify(r.error||'Не удалось загрузить резервные копии','error');if(s.ok)sites=s.data.sites||[];if(d.ok)databases=d.data.databases||[];if(sc.ok)schedules=sc.data.schedules||[];loading=false;syncPolling()}
 onMount(load);
 onDestroy(()=>{if(poller)clearInterval(poller)});
 async function create(){if(!selected)return;busy=true;let r;if(selected.startsWith('db:')){r=await createDatabaseBackup(Number(selected.slice(3)))}else{const parts=selected.slice(5).split(':');r=await createSiteBackup(Number(parts[0]),parts.slice(1).join(':'))}busy=false;if(r.ok){notify(r.data?.backup?.status==='queued'?'Backup queued':'Резервная копия создана','success');backups=[{...r.data.backup,created_at:new Date().toISOString()},...backups];syncPolling()}else notify(r.error||'Не удалось создать backup','error')}
 async function createSchedule(){
  if(!selected||scheduleBusy)return;
  let payload;
  if(selected.startsWith('db:')){payload={resource_type:'database',database_id:Number(selected.slice(3)),device_id:Number(databases.find(x=>x.id===Number(selected.slice(3)))?.device_id||0)}}
  else{const parts=selected.slice(5).split(':');payload={resource_type:'site',device_id:Number(parts[0]),site_id:parts.slice(1).join(':')}}
  payload.interval_hours=Number(intervalHours);payload.retention_count=Number(retentionCount);
  scheduleBusy=true;const r=await createBackupSchedule(payload);scheduleBusy=false;
  if(!r.ok){notify(r.error||'Не удалось создать расписание','error');return}
  notify('Расписание резервного копирования создано','success');
  const refreshed=await listBackupSchedules();if(refreshed.ok)schedules=refreshed.data.schedules||[];
 }
 async function removeSchedule(id){
  const r=await deleteBackupSchedule(id);if(!r.ok){notify(r.error||'Не удалось удалить расписание','error');return}
  schedules=schedules.filter(x=>x.id!==id);notify('Расписание удалено','success');
 }
 async function createControl(){controlBusy=true;const r=await createControlPlaneBackup();controlBusy=false;if(r.ok){notify('Бэкап Control Plane сохранён: '+r.data.file_name,'success')}else notify(r.error||'Не удалось создать бэкап Control Plane','error')}
 function size(n){if(!n)return '—';const units=['B','KB','MB','GB'];let i=0,v=n;while(v>=1024&&i<3){v/=1024;i++}return v.toFixed(1)+' '+units[i]}
</script>
<div class="page-header"><div><div class="eyebrow">DATA PROTECTION</div><h1 class="page-title">Backup & Restore</h1><p class="page-desc">Локальные архивы сайтов и баз данных на исходных Android-устройствах. Удаление сайта не удаляет backup.</p></div><div class="header-actions">{#if $user?.is_admin}<Button variant="primary" loading={controlBusy} on:click={createControl}>Бэкап Control Plane</Button>{/if}<Button on:click={load}>↻ Обновить</Button></div></div>
<section class="create-backup"><div><strong>Create backup</strong><small>Создаёт локальный TAR.GZ архив сайта или PostgreSQL custom-format dump на исходном Android-устройстве.</small></div><div class="create-controls"><select class="input" bind:value={selected}><option value="">Выбрать сайт или базу</option><optgroup label="Websites">{#each sites as item}<option value={'site:'+item.device_id+':'+item.site.site_id}>{item.device_name} / {item.site.site_id}</option>{/each}</optgroup><optgroup label="Databases">{#each databases as db}<option value={'db:'+db.id} disabled={db.status!=='ready'}>{db.name} · {db.server_name} ({db.status})</option>{/each}</optgroup></select><Button variant="primary" disabled={!selected||busy} loading={busy} on:click={create}>Create backup</Button></div></section>
<section class="schedule-panel">
 <div class="schedule-heading"><div><strong>Scheduled backups</strong><small>Автоматическое создание копий по расписанию. Если устройство офлайн, запуск будет отмечен как ошибка.</small></div></div>
 <div class="schedule-controls">
  <select class="input" bind:value={intervalHours}><option value={24}>Every day</option><option value={168}>Every week</option><option value={720}>Every 30 days</option></select>
  <select class="input" bind:value={retentionCount}>{#each [3,5,10,15,20,30,50] as count}<option value={count}>Keep {count}</option>{/each}</select>
  <Button disabled={!selected||scheduleBusy} loading={scheduleBusy} on:click={createSchedule}>Create schedule</Button>
 </div>
 {#if schedules.length}
  <div class="schedule-list">{#each schedules as schedule}
   <div class="schedule-row"><div><strong>{schedule.resource_type==='site'?'Website':'Database'} · {schedule.site_id || ('#'+schedule.database_id)}</strong><small>Every {schedule.interval_hours}h · keep {schedule.retention_count} · next {new Date(schedule.next_run_at).toLocaleString()}</small><small class:failed={schedule.last_status==='failed'}>Last: {schedule.last_status}{schedule.last_error?' · '+schedule.last_error:''}</small></div><Button size="sm" variant="danger" on:click={()=>removeSchedule(schedule.id)}>Remove</Button></div>
  {/each}</div>
 {:else}<p class="muted">Расписаний пока нет.</p>{/if}
</section>
{#if loading}<div class="empty">Загрузка резервных копий…</div>{:else if !backups.length}<div class="empty"><strong>Резервных копий пока нет</strong><p>Выберите сайт или базу данных выше, чтобы создать первый архив.</p></div>{:else}
<div class="backup-list">{#each backups as b}<div class="backup-row"><span class="backup-icon">⤓</span><span class="backup-copy"><strong>{b.file_name}</strong><small>{b.resource_type} · {b.resource_id} · {new Date(b.created_at).toLocaleString()}</small></span><span class="size">{size(b.size_bytes)}</span><span class="status">{b.status}</span></div>{/each}</div>
{/if}
<div class="notice">Архивы хранятся локально на Android-устройстве и не удаляются вместе с сайтом. Задания обновляются автоматически, пока архив создаётся. Скачивание и перенос архивов между устройствами пока не поддерживаются.</div>
<style>
.schedule-panel{display:grid;gap:10px;padding:14px;background:#101727;border:1px solid #253149;border-radius:11px;margin-bottom:13px}.schedule-heading{display:flex;justify-content:space-between}.schedule-heading>div{display:grid;gap:4px}.schedule-heading strong{font-size:11px}.schedule-heading small,.schedule-row small{font-size:9px;color:#64748b;line-height:1.5}.schedule-controls{display:flex;gap:8px;flex-wrap:wrap}.schedule-controls .input{min-width:130px}.schedule-list{display:grid;gap:6px}.schedule-row{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:9px;border:1px solid #253149;border-radius:8px}.schedule-row>div{display:grid;gap:3px}.schedule-row strong{font-size:10px}.schedule-row .failed{color:#f87171}

.header-actions{display:flex;gap:8px;flex-wrap:wrap}.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:7px}.create-backup{display:flex;justify-content:space-between;align-items:center;gap:13px;padding:14px;background:#101727;border:1px solid #253149;border-radius:11px;margin-bottom:13px}.create-backup>div:first-child{display:grid;gap:4px}.create-backup strong{font-size:11px}.create-backup small{font-size:9px;color:#64748b;max-width:420px;line-height:1.5}.create-controls{display:flex;gap:7px;min-width:360px}.create-controls select{min-width:0;flex:1}.backup-list{border:1px solid #253149;border-radius:11px;overflow:hidden;background:#101727}.backup-row{display:flex;align-items:center;gap:11px;padding:12px;border-bottom:1px solid #1e293b}.backup-row:last-child{border:0}.backup-icon{width:30px;height:30px;display:grid;place-items:center;background:#172136;color:#a5b4fc;border-radius:8px}.backup-copy{display:grid;gap:3px;min-width:0;flex:1}.backup-copy strong{font-size:10px}.backup-copy small,.size,.status{font-size:9px;color:#64748b}.size,.status{white-space:nowrap}.notice{margin-top:13px;padding:12px;border:1px solid #334155;border-radius:9px;background:#101727;color:#94a3b8;font-size:10px;line-height:1.6}
@media(max-width:700px){.create-backup{align-items:stretch;flex-direction:column}.create-controls{min-width:0}.backup-row{flex-wrap:wrap}}
</style>