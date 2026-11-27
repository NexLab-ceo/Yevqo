<script>
 import {onMount,onDestroy} from 'svelte';import {page} from '$app/stores';import {listBackups,listSites,listDatabases,createSiteBackup,createDatabaseBackup,createControlPlaneBackup,listBackupSchedules,createBackupSchedule,deleteBackupSchedule,getDatabaseBackupChunk,restoreSiteBackup,transferBackup,listDatabaseServers,restoreDatabaseBackup} from '$lib/api.js';import {notify,user} from '$lib/stores.js';import Button from '$lib/components/Button.svelte';
 let backups=[],sites=[],databases=[],dbServers=[],schedules=[],scope={resource_type:'',resource_id:'',device_id:''},loading=true,busy=false,controlBusy=false,selected='',poller=null,pollBusy=false,scheduleBusy=false,intervalHours=24,retentionCount=10,downloadingId=0,restoringId=0,transferTargets={},transferBusyId=0,restoreDBID=0,restoreDBName='',restoreDBUser='',restoreDBPassword='',restoreDBSite='';
 function syncPolling(){const pending=backups.some((item)=>item.status==='queued'||item.status==='running');if(pending&&!poller){poller=setInterval(async()=>{if(pollBusy)return;pollBusy=true;const r=await listBackups(scope);if(r.ok){backups=r.data.backups||[];if(!backups.some((item)=>item.status==='queued'||item.status==='running')){clearInterval(poller);poller=null}}pollBusy=false},3000)}else if(!pending&&poller){clearInterval(poller);poller=null}}
 async function load(){scope={resource_type:$page.url.searchParams.get('resource_type')||'',resource_id:$page.url.searchParams.get('resource_id')||'',device_id:$page.url.searchParams.get('device_id')||''};const scoped=!!(scope.resource_type||scope.resource_id||scope.device_id);const [r,s,d,sc,servers]=await Promise.all([listBackups(scope),listSites(),...(scoped?[]:[listDatabases(),listBackupSchedules(),listDatabaseServers()])]);if(r.ok)backups=r.data.backups||[];else notify(r.error||'Не удалось загрузить резервные копии','error');if(s.ok)sites=s.data.sites||[];if(!scoped){if(d.ok)databases=d.data.databases||[];if(sc.ok)schedules=sc.data.schedules||[];if(servers.ok)dbServers=servers.data.servers||[]}loading=false;syncPolling()}
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
 async function downloadBackup(backup){
  if(downloadingId)return;
  const total=Number(backup.size_bytes||0);
  if(!total||total>2*1024*1024*1024){notify('Размер архива не поддерживается браузерной загрузкой (лимит 2 GiB)','error');return}
  downloadingId=backup.id;
  const chunks=[];let offset=0;let failed='';
  while(offset<total){
   const r=await getDatabaseBackupChunk(backup.id,offset,256*1024);
   if(!r.ok){failed=r.error||'Не удалось скачать часть архива';break}
   const encoded=r.data?.data||'';let bytes;
   try{const raw=atob(encoded);bytes=Uint8Array.from(raw,c=>c.charCodeAt(0))}catch{failed='Получена повреждённая часть архива';break}
   if(!bytes.length||Number(r.data.offset)!==offset||Number(r.data.total_size)!==total){failed='Нарушена последовательность частей архива';break}
   chunks.push(bytes);offset+=bytes.length;
   if(offset>total){failed='Размер архива превышает объявленный';break}
   if(r.data.eof&&offset!==total){failed='Архив завершился раньше ожидаемого размера';break}
   if(offset===total&&!r.data.eof){failed='Последняя часть архива не помечена как завершающая';break}
  }
  downloadingId=0;
  if(failed){notify(failed,'error');return}
  const blob=new Blob(chunks,{type:backup.resource_type==='site'?'application/gzip':'application/octet-stream'});
  const url=URL.createObjectURL(blob);const link=document.createElement('a');link.href=url;link.download=backup.file_name;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
  notify('Архив скачан','success');
 }
 function beginDatabaseRestore(backup){
  restoreDBID=restoreDBID===backup.id?0:backup.id;
  restoreDBName='restored_'+backup.id;restoreDBUser='app_user';restoreDBPassword='';restoreDBSite='';
 }
 async function runDatabaseRestore(){
  const backup=backups.find(x=>x.id===restoreDBID);if(!backup)return;
  if(!restoreDBName.trim()||!restoreDBUser.trim()||restoreDBPassword.length<12){notify('Укажите имя БД, пользователя и пароль не короче 12 символов','warning');return}
  restoringId=backup.id;
  const r=await restoreDatabaseBackup(backup.id,{name:restoreDBName,username:restoreDBUser,password:restoreDBPassword,site_id:restoreDBSite});
  restoringId=0;
  if(!r.ok){notify(r.error||'Не удалось запустить восстановление БД','error');return}
  restoreDBID=0;restoreDBPassword='';
  notify('Восстановление запущено: создаётся новая база, исходная не изменяется','success');
 }
 async function transferBackupToTarget(backup){
  if(transferBusyId)return;
  const choice=transferTargets[backup.id]||'';
  let target={};
  if(backup.resource_type==='site'){
   const parts=choice.split(':');if(parts.length<2||!parts[0]||!parts.slice(1).join(':')){notify('Выберите целевой сайт','warning');return}
   target={target_device_id:Number(parts[0]),target_site_id:parts.slice(1).join(':')};
  }else{
   const serverId=Number(choice);if(!serverId){notify('Выберите целевой совместимый сервер базы данных','warning');return}
   target={target_server_id:serverId};
  }
  transferBusyId=backup.id;const r=await transferBackup(backup.id,target);transferBusyId=0;
  if(!r.ok){notify(r.error||'Не удалось перенести резервную копию','error');return}
  notify('Перенос запущен. Проверка архива выполняется на целевом устройстве.','success');
  await load();
 }
 async function restoreBackup(backup){
  if(restoringId||backup.resource_type!=='site'||backup.status!=='completed')return;
  if(!confirm('Restore '+backup.file_name+' over the current website files? The website will be stopped.'))return;
  restoringId=backup.id;const r=await restoreSiteBackup(backup.id);restoringId=0;
  if(r.ok){notify(r.data.message||'Website restored; site stopped','success')}else notify(r.error||'Не удалось восстановить сайт','error');
 }
 async function createControl(){controlBusy=true;const r=await createControlPlaneBackup();controlBusy=false;if(r.ok){notify('Бэкап Control Plane сохранён: '+r.data.file_name,'success')}else notify(r.error||'Не удалось создать бэкап Control Plane','error')}
 function size(n){if(!n)return '—';const units=['B','KB','MB','GB'];let i=0,v=n;while(v>=1024&&i<3){v/=1024;i++}return v.toFixed(1)+' '+units[i]}
</script>
<div class="page-header"><div><div class="eyebrow">DATA PROTECTION</div><h1 class="page-title">{scope.resource_type==='site'?'Application backups':'Backup & Restore'}</h1><p class="page-desc">{scope.resource_type==='site'?'Резервные копии выбранного приложения.':'Локальные архивы сайтов и баз данных на исходных Android-устройствах. Удаление сайта не удаляет backup.'}</p>{#if scope.resource_type||scope.resource_id||scope.device_id}<a href="/backups">← Все резервные копии</a>{/if}</div><div class="header-actions">{#if $user?.is_admin&&!scope.resource_type&&!scope.resource_id&&!scope.device_id}<Button variant="primary" loading={controlBusy} on:click={createControl}>Бэкап Control Plane</Button>{/if}<Button on:click={load}>↻ Обновить</Button></div></div>
{#if !scope.resource_type&&!scope.resource_id&&!scope.device_id}
<section class="create-backup"><div><strong>Create backup</strong><small>Создаёт локальный TAR.GZ архив сайта или резервную копию базы данных (PostgreSQL dump / MariaDB SQL) на исходном Android-устройстве.</small></div><div class="create-controls"><select class="input" bind:value={selected}><option value="">Выбрать сайт или базу</option><optgroup label="Websites">{#each sites as item}<option value={'site:'+item.device_id+':'+item.site.site_id}>{item.device_name} / {item.site.site_id}</option>{/each}</optgroup><optgroup label="Databases">{#each databases as db}<option value={'db:'+db.id} disabled={db.status!=='ready'}>{db.name} · {db.server_name} ({db.status})</option>{/each}</optgroup></select><Button variant="primary" disabled={!selected||busy} loading={busy} on:click={create}>Create backup</Button></div></section>
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
{/if}
{#if loading}<div class="empty">Загрузка резервных копий…</div>{:else if !backups.length}<div class="empty"><strong>Резервных копий пока нет</strong><p>{scope.resource_type==='site'?'Для этого приложения ещё не создано резервных копий.':'Выберите сайт или базу данных выше, чтобы создать первый архив.'}</p></div>{:else}
<div class="backup-list">{#each backups as b}<div class="backup-row"><span class="backup-icon">⤓</span><span class="backup-copy"><strong>{b.file_name}</strong><small>{b.resource_type} · {b.resource_id} · {new Date(b.created_at).toLocaleString()}</small></span><span class="size">{size(b.size_bytes)}</span><span class="status">{b.status}</span>{#if (b.resource_type==='site'&&b.status==='completed')||(b.resource_type==='database'&&b.status==='verified')}
 <Button size="sm" disabled={!!downloadingId} loading={downloadingId===b.id} on:click={()=>downloadBackup(b)}>Download</Button>
 {#if b.resource_type==='site'}<Button size="sm" variant="danger" disabled={!!restoringId} loading={restoringId===b.id} on:click={()=>restoreBackup(b)}>Restore</Button>{:else}<Button size="sm" disabled={!!restoringId} on:click={()=>beginDatabaseRestore(b)}>Restore to new DB</Button>{/if}
 <select class="transfer-select" bind:value={transferTargets[b.id]}><option value="">Transfer to…</option>
 {#if b.resource_type==='site'}{#each sites as target}<option value={target.device_id+':'+target.site.site_id}>{target.device_name} / {target.site.site_id}</option>{/each}
 {:else}{#each dbServers.filter(server=>server.engine==='postgresql'&&server.status==='ready') as server}<option value={server.id}>{server.name} · {server.hostname||('Device '+server.device_id)}</option>{/each}{/if}
 </select>
 <Button size="sm" disabled={!transferTargets[b.id]||!!transferBusyId} loading={transferBusyId===b.id} on:click={()=>transferBackupToTarget(b)}>Transfer</Button>
{/if}</div>
 {#if restoreDBID===b.id}
  <div class="db-restore-form"><strong>Restore PostgreSQL backup to a new database</strong><div class="db-restore-fields"><input class="input" bind:value={restoreDBName} placeholder="Database name"/><input class="input" bind:value={restoreDBUser} placeholder="Database user"/><input class="input" type="password" bind:value={restoreDBPassword} placeholder="New password (min. 12 chars)" autocomplete="new-password"/><select class="input" bind:value={restoreDBSite}><option value="">Do not attach to a website</option>{#each sites.filter(site=>site.device_id===b.device_id) as site}<option value={site.site.site_id}>Attach to {site.site.site_id}</option>{/each}</select></div><div class="db-restore-actions"><Button size="sm" on:click={()=>restoreDBID=0}>Cancel</Button><Button size="sm" variant="primary" loading={restoringId===b.id} disabled={restoreDBPassword.length<12} on:click={runDatabaseRestore}>Restore backup</Button></div><small>Source database and backup are preserved. Restore creates a separate database on the original or transferred PostgreSQL server.</small></div>
 {/if}
{/each}</div>
{/if}
<div class="notice">Архивы хранятся локально на Android-устройстве и не удаляются вместе с сайтом. Задания обновляются автоматически, пока архив создаётся. Готовые архивы можно скачать по блокам. Website restore заменяет файлы на выбранном устройстве атомарной заменой каталога и оставляет процесс остановленным. Transfer копирует архив между устройствами и проверяет его целостность на получателе.</div>
<style>
.schedule-panel{display:grid;gap:10px;padding:14px;background:var(--bg-card);border:1px solid var(--border);border-radius:11px;margin-bottom:13px}.schedule-heading{display:flex;justify-content:space-between}.schedule-heading>div{display:grid;gap:4px}.schedule-heading strong{font-size:11px}.schedule-heading small,.schedule-row small{font-size:9px;color:var(--text-muted);line-height:1.5}.schedule-controls{display:flex;gap:8px;flex-wrap:wrap}.schedule-controls .input{min-width:130px}.schedule-list{display:grid;gap:6px}.schedule-row{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:9px;border:1px solid var(--border);border-radius:8px}.schedule-row>div{display:grid;gap:3px}.schedule-row strong{font-size:10px}.schedule-row .failed{color:var(--danger)}

.header-actions{display:flex;gap:8px;flex-wrap:wrap}.eyebrow{font-size:9px;letter-spacing:.14em;color:var(--accent);font-weight:700;margin-bottom:7px}.create-backup{display:flex;justify-content:space-between;align-items:center;gap:13px;padding:14px;background:var(--bg-card);border:1px solid var(--border);border-radius:11px;margin-bottom:13px}.create-backup>div:first-child{display:grid;gap:4px}.create-backup strong{font-size:11px}.create-backup small{font-size:9px;color:var(--text-muted);max-width:420px;line-height:1.5}.create-controls{display:flex;gap:7px;min-width:360px}.create-controls select{min-width:0;flex:1}.backup-list{border:1px solid var(--border);border-radius:11px;overflow:hidden;background:var(--bg-card)}.backup-row{display:flex;align-items:center;gap:11px;padding:12px;border-bottom:1px solid var(--border)}.backup-row:last-child{border:0}.transfer-select{max-width:180px;min-width:120px;background:var(--bg-base);border:1px solid var(--border);border-radius:7px;color:var(--text-main);padding:6px;font-size:9px}.db-restore-form{display:grid;gap:9px;padding:12px 14px;margin:0 0 8px;background:var(--bg-base);border:1px solid var(--border);border-radius:9px}.db-restore-form>strong{font-size:11px}.db-restore-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.db-restore-actions{display:flex;justify-content:flex-end;gap:8px}.db-restore-form small{font-size:9px;color:var(--text-muted);line-height:1.5}@media(max-width:700px){.db-restore-fields{grid-template-columns:1fr}}.backup-icon{width:30px;height:30px;display:grid;place-items:center;background:var(--bg-elevated);color:var(--accent);border-radius:8px}.backup-copy{display:grid;gap:3px;min-width:0;flex:1}.backup-copy strong{font-size:10px}.backup-copy small,.size,.status{font-size:9px;color:var(--text-muted)}.size,.status{white-space:nowrap}.notice{margin-top:13px;padding:12px;border:1px solid var(--border);border-radius:9px;background:var(--bg-card);color:var(--text-muted);font-size:10px;line-height:1.6}
@media(max-width:700px){.create-backup{align-items:stretch;flex-direction:column}.create-controls{min-width:0}.backup-row{flex-wrap:wrap}}
</style>