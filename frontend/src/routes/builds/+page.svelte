<script>
 import {onMount} from 'svelte';
 import {listSites,listDevices,sendCommand} from '$lib/api.js';
 import {notify} from '$lib/stores.js';
 import Button from '$lib/components/Button.svelte';

 let sites=[],devices=[],loading=true,selectedKey='',rootDirectory='.',plan=null,result=null;
 let buildHistory=[],historyLoading=false;
 let analyzing=false,building=false,cancelling=false,allowProjectCode=false;
 let customInstallCommand='',customBuildCommand='';
 $: selectedSite=sites.find(x=>key(x)===selectedKey)||null;
 $: selectedDevice=selectedSite?devices.find(x=>Number(x.id)===Number(selectedSite.device_id)):null;
 $: online=selectedDevice?.status==='online';
 function key(item){return String(item.device_id)+':'+String(item.site?.site_id||'')}
 function decode(payload){if(!payload)return '';try{const raw=atob(payload);return new TextDecoder().decode(Uint8Array.from(raw,c=>c.charCodeAt(0)))}catch{return ''}}
 async function load(){
   loading=true;
   const [s,d]=await Promise.all([listSites(),listDevices()]);
   if(s.ok)sites=s.data.sites||[];else notify(s.error||'Не удалось загрузить сайты','error');
   if(d.ok)devices=d.data.devices||[];else notify(d.error||'Не удалось загрузить устройства','error');
   if(!selectedKey&&sites.length)selectedKey=key(sites[0]);
   loading=false;
   const target=sites.find(x=>key(x)===selectedKey)||null;
   await loadHistory(target);
 }
 onMount(load);
 async function loadHistory(target=selectedSite){
   if(!target){buildHistory=[];return}
   const device=devices.find(x=>Number(x.id)===Number(target.device_id));
   if(device?.status!=='online'){buildHistory=[];return}
   historyLoading=true;
   const r=await sendCommand(target.device_id,'list_build_history',{site_id:target.site.site_id,limit:10});
   historyLoading=false;
   if(!r.ok||!r.data?.ok){buildHistory=[];return}
   try{const payload=JSON.parse(decode(r.data.payload));buildHistory=payload.records||[]}
   catch{buildHistory=[]}
 }
 async function analyze(){
   if(!selectedSite||!online||analyzing)return;
   analyzing=true;plan=null;result=null;customInstallCommand='';customBuildCommand='';
   const r=await sendCommand(selectedSite.device_id,'analyze_site_build',{site_id:selectedSite.site.site_id,root_dir:rootDirectory||'.'});
   analyzing=false;
   if(!r.ok||!r.data?.ok){notify(r.error||r.data?.message||'Анализ проекта не выполнен','error');return}
   try{plan=JSON.parse(decode(r.data.payload));if(plan.root_directory)rootDirectory=plan.root_directory;notify('Анализ проекта завершён','success')}
   catch{notify('Агент вернул некорректный результат анализа','error')}
 }
 async function cancelBuild(){
   if(!selectedSite||!building||cancelling)return;
   cancelling=true;
   const r=await sendCommand(selectedSite.device_id,'cancel_build',{});
   cancelling=false;
   if(!r.ok||!r.data?.ok){notify(r.error||r.data?.message||'Не удалось отменить сборку','error');return}
   notify('Запрошена остановка процесса сборки','success');
 }
 async function build(){
   if(!selectedSite||!online||!plan||!allowProjectCode||building)return;
   if(plan.needs_configuration&&!customInstallCommand.trim()&&!customBuildCommand.trim()){
     notify('Для неизвестного стека укажи install или build команду','error');return;
   }
   building=true;result=null;
   const r=await sendCommand(selectedSite.device_id,'build_site',{
     site_id:selectedSite.site.site_id,root_dir:rootDirectory||'.',stack:plan.needs_configuration?'custom':plan.stack,
     package_tool:plan.package_tool||'',
     install_command:customInstallCommand.trim()||plan.install_command||'',
     build_command:customBuildCommand.trim()||plan.build_command||'',
     allow_project_code:true,timeout_seconds:1200,max_log_bytes:10485760
   });
   building=false;
   if(!r.ok||!r.data){notify(r.error||'Не удалось запустить сборку','error');return}
   try{result=r.data.payload?JSON.parse(decode(r.data.payload)):{success:!!r.data.ok,error:r.data.message||''}}
   catch{result={success:!!r.data.ok,error:r.data.message||'Некорректный ответ',output:''}}
   notify(result.success?'Сборка завершена':(result.error||r.data.message||'Ошибка сборки'),result.success?'success':'error');
   await loadHistory(selectedSite);
 }
</script>
<svelte:head><title>Build Engine · Yevqo</title></svelte:head>
<div class="page-header"><div><div class="eyebrow">DELIVERY PLATFORM</div><h1 class="page-title">Build Engine</h1><p class="page-desc">Анализируйте проект, проверяйте окружение и запускайте сборку на подключённом Agent.</p></div><Button on:click={load}>↻ Обновить</Button></div>
<section class="builder-shell">
 <div class="builder-toolbar"><div><span class="section-kicker">BUILD TARGET</span><strong>Выберите приложение</strong></div><div class="target-controls">
  <select class="input target-select" bind:value={selectedKey} on:change={()=>{plan=null;result=null;rootDirectory='.';loadHistory(sites.find(x=>key(x)===selectedKey)||null);}}>
   <option value="">Выберите сайт</option>
   {#each sites as item}<option value={key(item)}>{item.device_name||'Device '+item.device_id} / {item.site.site_id}</option>{/each}
  </select>
  <input class="input root-input" bind:value={rootDirectory} placeholder="Project root (.)"/>
 </div></div>
 {#if loading}<div class="empty">Загружаем проекты…</div>
 {:else if !sites.length}<div class="empty"><h3>Нет приложений для сборки</h3><p>Сначала создайте сайт в разделе Websites и подключите Agent.</p></div>
 {:else if !selectedSite}<div class="empty">Выберите приложение для анализа.</div>
 {:else}
 <div class="target-meta"><span class:online={online} class:offline={!online}><i></i>{online?'Agent online':'Agent offline'}</span><span>{selectedSite.site.runtime||'Runtime не указан'}</span><span>Порт :{selectedSite.site.port}</span><span>{selectedSite.site.path}</span></div>
 {#if !online}<div class="warning">Устройство недоступно. Подключите Agent, чтобы запустить анализ и сборку.</div>{/if}
 <div class="actions"><Button variant="primary" disabled={!online||analyzing||building} loading={analyzing} on:click={analyze}>⌕ Analyze project</Button><span class="muted">Анализ не выполняет код проекта.</span></div>
 {/if}
</section>
{#if plan}
<section class="panel">
 <div class="panel-heading"><div><span class="section-kicker">DETECTION RESULT</span><h2>План сборки</h2></div><span class="confidence">{plan.confidence||'confidence unknown'} confidence</span></div>
 <div class="summary-grid">
  <div><small>Detected stack</small><strong>{plan.stack||'Unknown'}</strong></div>
  <div><small>Provider</small><strong>{plan.provider||'Auto'}</strong></div>
  <div><small>Package manager</small><strong>{plan.package_tool||'—'}</strong></div>
  <div><small>Default port</small><strong>{plan.port||'—'}</strong></div>
 </div>
 {#if plan.evidence?.length}<p class="muted">Обнаружено: {plan.evidence.join(', ')}</p>{/if}
 {#if plan.needs_configuration}<div class="warning">Язык не распознан автоматически. Укажи команды для установленного в Termux компилятора или build-инструмента. Shell-команды и Docker/Nixpacks здесь отключены.</div>{/if}
 <div class="commands">
  {#if plan.install_command}<div><small>INSTALL</small><code>{plan.install_command}</code></div>{/if}
  {#if plan.build_command}<div><small>BUILD</small><code>{plan.build_command}</code></div>{/if}
  {#if plan.start_command}<div><small>START</small><code>{plan.start_command}</code></div>{/if}
 </div>
 {#if plan.needs_configuration}
 <div class="custom-build">
  <h3>Универсальная сборка</h3>
  <p class="muted">Укажи команды для установленного в Termux компилятора или build-инструмента. Можно использовать любой язык, если его toolchain доступен на устройстве. Команды запускаются без shell-интерпретации.</p>
  <label>Install command <input class="input" bind:value={customInstallCommand} placeholder="например: nimble install -y"/></label>
  <label>Build command <input class="input" bind:value={customBuildCommand} placeholder="например: nim c -o build/app src/main.nim"/></label>
 </div>
 {/if}
 {#if plan.android_environment}
 <div class="environment"><h3>Agent environment</h3><div class="summary-grid">
  <div><small>Environment</small><strong>{plan.android_environment.termux?'Termux':plan.android_environment.android?'Android':'Host'}</strong></div>
  <div><small>Architecture</small><strong>{plan.android_environment.architecture||'—'}</strong></div>
  <div><small>CPU cores</small><strong>{plan.android_environment.cpus||'—'}</strong></div>
  <div><small>Available RAM</small><strong>{plan.android_environment.available_memory_bytes?Math.round(plan.android_environment.available_memory_bytes/1048576)+' MB':'Unknown'}</strong></div>
 </div>{#if plan.android_environment.available_tools?.length}<p class="muted">Инструменты: {plan.android_environment.available_tools.join(', ')}</p>{/if}</div>
 {/if}
 {#if plan.warnings?.length}<div class="warning">{#each plan.warnings as w}<p>{w}</p>{/each}</div>{/if}
 <div class="consent"><label><input type="checkbox" bind:checked={allowProjectCode}/> Я доверяю этому репозиторию и разрешаю выполнение его build-скриптов с правами пользователя Agent.</label><small>Сборка запускает код проекта. Это не изолированная песочница.</small></div>
 <div class="actions"><Button variant="primary" disabled={!online||building||!allowProjectCode||(plan.needs_configuration&&!customInstallCommand.trim()&&!customBuildCommand.trim())} loading={building} on:click={build}>▶ Build project</Button>{#if building}<Button disabled={cancelling} loading={cancelling} on:click={cancelBuild}>Cancel build</Button>{/if}<span class="muted">Сборка выполняется локально на Android/Termux с установленными инструментами.</span></div>
</section>
{/if}
{#if selectedSite}
<section class="panel">
 <div class="panel-heading"><div><span class="section-kicker">LOCAL DEVICE RECORDS</span><h2>История сборок</h2></div><span class="confidence">{buildHistory.length} / 10</span></div>
 {#if historyLoading}<p class="muted">Загружаем историю…</p>
 {:else if buildHistory.length}
 <div class="history-list">
  {#each buildHistory as record (record.id)}
   <button class="history-item" on:click={()=>{result=record.result;}}>
    <span class:success={record.result.success} class:failure={!record.result.success}>{record.result.success?'SUCCESS':'FAILED'}</span>
    <span><strong>{record.result.stack||'custom'}</strong><small>{new Date(record.result.finished_at).toLocaleString()}</small></span>
    <small>{((record.result.duration_ms||0)/1000).toFixed(1)} s</small>
   </button>
  {/each}
 </div>
 {:else}<p class="muted">Сборок пока нет. История хранится локально на выбранном Android-устройстве.</p>{/if}
</section>
{/if}
{#if result}
<section class="panel"><div class="panel-heading"><h2>Результат сборки</h2><strong class:success={result.success} class:failure={!result.success}>{result.success?'SUCCESS':'FAILED'}</strong></div>
 {#if result.error}<p class="error">{result.error}</p>{/if}
 {#if result.history_warning}<p class="warning">История не сохранена: {result.history_warning}</p>{/if}
 {#if result.duration_ms}<p class="muted">Время: {(result.duration_ms/1000).toFixed(1)} сек.</p>{/if}
 <pre class="terminal">{result.output||'(сборщик не вернул лог)'}</pre>
 {#if result.output_truncated}<p class="muted">Вывод ограничен настроенным лимитом.</p>{/if}
</section>
{/if}
<style>
.eyebrow,.section-kicker{font-size:9px;letter-spacing:.15em;color:#818cf8;font-weight:750}.builder-shell,.panel{border:1px solid #27272A;background:#111114;border-radius:12px;padding:18px;margin-bottom:14px}.builder-toolbar,.panel-heading{display:flex;align-items:center;justify-content:space-between;gap:14px}.builder-toolbar>div:first-child{display:grid;gap:5px}.builder-toolbar strong{font-size:12px}.target-controls{display:flex;gap:8px;min-width:45%}.target-select{min-width:220px;flex:1}.root-input{max-width:160px}.target-meta{display:flex;flex-wrap:wrap;gap:15px;border-top:1px solid #27272A;margin-top:16px;padding-top:13px;color:#A1A1AA;font-size:10px}.target-meta>span:first-child{display:flex;align-items:center;gap:6px}.target-meta i{width:6px;height:6px;border-radius:50%;background:#71717A}.target-meta .online i{background:#22C55E}.target-meta .offline i{background:#F59E0B}.actions{display:flex;align-items:center;gap:12px;margin-top:16px}.muted{font-size:10px;color:#71717A;line-height:1.6}.panel-heading h2{font-size:15px;margin:5px 0 0}.confidence{font-size:9px;color:#A1A1AA;background:#18181B;border:1px solid #27272A;border-radius:6px;padding:5px 8px}.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:9px;margin:15px 0}.summary-grid>div{display:grid;gap:7px;padding:11px;background:#09090B;border:1px solid #27272A;border-radius:8px;min-width:0}.summary-grid small,.commands small{font-size:8px;color:#71717A;letter-spacing:.08em}.summary-grid strong{font-size:11px;overflow-wrap:anywhere}.commands{display:grid;gap:8px;margin:15px 0}.commands>div{display:grid;gap:6px;padding:10px;border:1px solid #27272A;background:#09090B;border-radius:8px}.commands code{font-size:10px;color:#D4D4D8;overflow-wrap:anywhere}.environment{border-top:1px solid #27272A;padding-top:13px;margin-top:14px}.environment h3{font-size:12px;margin:0}.custom-build{display:grid;gap:9px;padding:13px;margin:14px 0;border:1px solid #3F3F46;border-radius:8px;background:#09090B}.custom-build h3{font-size:12px;margin:0}.custom-build label{display:grid;gap:6px;font-size:10px;color:#A1A1AA}.custom-build input{width:100%;font-family:ui-monospace,monospace;font-size:11px}.consent{display:grid;gap:6px;margin-top:16px;padding:12px;background:#18181B;border:1px solid #3F3F46;border-radius:8px}.consent label{display:flex;align-items:flex-start;gap:8px;font-size:10px;color:#D4D4D8}.consent input{accent-color:#3B82F6;margin:0}.consent small{font-size:9px;color:#A1A1AA;margin-left:22px}.warning{padding:10px 12px;border:1px solid #854D0E;background:#291D0B;color:#FCD34D;border-radius:8px;font-size:10px;margin-top:12px}.warning p{margin:3px 0}.history-list{display:grid;gap:6px;margin-top:12px}.history-item{display:flex;align-items:center;gap:12px;width:100%;padding:10px 12px;text-align:left;background:#09090B;border:1px solid #27272A;border-radius:8px;color:#D4D4D8;cursor:pointer}.history-item>span:nth-child(2){display:grid;gap:4px;flex:1}.history-item strong{font-size:11px}.history-item small{font-size:9px;color:#71717A}.history-item .success{font-size:9px;font-weight:700}.history-item .failure{font-size:9px;font-weight:700}.terminal{white-space:pre-wrap;overflow:auto;max-height:460px;background:#09090B;border:1px solid #27272A;border-radius:8px;padding:13px;font:10px/1.6 ui-monospace,monospace;color:#D4D4D8}.success{color:#4ADE80}.failure,.error{color:#F87171}.error{font-size:11px}
@media(max-width:760px){.builder-toolbar,.panel-heading{align-items:stretch;flex-direction:column}.target-controls{min-width:0;flex-direction:column}.target-select{min-width:0}.root-input{max-width:none}.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.actions{align-items:flex-start;flex-direction:column}}
</style>