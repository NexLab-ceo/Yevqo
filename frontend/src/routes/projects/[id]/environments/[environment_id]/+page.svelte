<script>
 import {onMount} from 'svelte';
 import {page} from '$app/stores';
 import {goto} from '$app/navigation';
 import {listProjectEnvironments,listEnvironmentSites,listSites,attachEnvironmentSite} from '$lib/api.js';
 import {notify} from '$lib/stores.js';
 import Button from '$lib/components/Button.svelte';
 let projectId='',environmentId='',environment=null,items=[],available=[],selected='',loading=true,busy=false;
 async function load(){
  projectId=$page.params.id;environmentId=$page.params.environment_id;
  const [e,s,all]=await Promise.all([listProjectEnvironments(projectId),listEnvironmentSites(projectId,environmentId),listSites(projectId)]);
  if(!e.ok){notify(e.error||'Не удалось загрузить окружения','error');goto('/projects/'+projectId);return}
  environment=(e.data.environments||[]).find(x=>String(x.id)===String(environmentId))||null;
  if(!environment){notify('Окружение не найдено','error');goto('/projects/'+projectId);return}
  items=s.ok?s.data.sites||[]:[];
  const linked=new Set(items.map(x=>x.device_id+':'+x.site_id));
  available=all.ok?(all.data.sites||[]).filter(x=>!linked.has(x.device_id+':'+x.site.site_id)):[];
  loading=false;
 }
 onMount(load);
 async function attach(){if(!selected||busy)return;const [device,site]=selected.split(':');busy=true;const r=await attachEnvironmentSite(projectId,environmentId,Number(device),site);busy=false;if(r.ok){notify('Application добавлено в окружение','success');selected='';await load()}else notify(r.error||'Не удалось добавить сайт','error')}
</script>
{#if loading}<div class="empty">Загрузка окружения…</div>{:else if environment}
<div class="page-header"><div><button class="back" on:click={()=>goto('/projects/'+projectId)}>← Project</button><div class="eyebrow">PROJECT / ENVIRONMENT</div><h1 class="page-title">{environment.name}</h1><p class="page-desc">{environment.description||environment.slug} · {environment.is_protected?'Protected environment':'Environment'}</p></div></div>
<section class="panel"><div class="section-head"><div><h2>Applications</h2><p>Сайты, назначенные в это окружение</p></div><div class="add"><select class="input" bind:value={selected}><option value="">Выбрать приложение проекта</option>{#each available as item}<option value={item.device_id+':'+item.site.site_id}>{item.site.site_id} · {item.device_name}</option>{/each}</select><Button variant="primary" disabled={!selected||busy} on:click={attach}>Добавить</Button></div></div>
{#if items.length}<div class="resource-list">{#each items as item}<a class="resource" href={'/sites/'+item.device_id+'/'+encodeURIComponent(item.site_id)}><span class="resource-icon">◉</span><span><strong>{item.site_id}</strong><small>{item.device_name}</small></span><span class="open">Открыть →</span></a>{/each}</div>{:else}<div class="empty compact">В этом окружении пока нет приложений.</div>{/if}</section>
{/if}
<style>
.eyebrow{font-size:9px;letter-spacing:.12em;color:#a1a1aa;margin:7px 0}.back{border:0;background:none;color:#a1a1aa;font-size:11px;margin-bottom:8px}.panel{background:#18181b;border:1px solid #27272a;border-radius:9px;padding:16px}.section-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:14px}.section-head h2{font-size:13px}.section-head p,.resource small{font-size:10px;color:#a1a1aa;margin-top:4px}.add{display:flex;gap:7px}.add select{min-width:220px}.resource-list{display:grid;gap:7px}.resource{display:flex;align-items:center;gap:10px;padding:10px;border:1px solid #27272a;border-radius:7px;text-decoration:none;color:inherit}.resource-icon{width:29px;height:29px;display:grid;place-items:center;background:#27272a;border-radius:6px}.resource>span:nth-child(2){display:grid;flex:1}.resource strong{font-size:11px}.open{font-size:10px;color:#a1a1aa}.empty.compact{padding:20px;color:#a1a1aa;font-size:11px}@media(max-width:650px){.section-head{align-items:stretch;flex-direction:column}.add select{min-width:0;flex:1}}
</style>
