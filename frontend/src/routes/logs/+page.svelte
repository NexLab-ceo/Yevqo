<script>
 import {onMount} from 'svelte';import {page} from '$app/stores';import {listLogs, listAuditLogs} from '$lib/api.js';import {notify} from '$lib/stores.js';import Button from '$lib/components/Button.svelte';
 let logs=[],auditLogs=[],loading=true,limit=100;
 async function load(){loading=true;const projectId=$page.url.searchParams.get('project_id')||'';const [r,a]=await Promise.all([listLogs(limit,projectId),listAuditLogs(limit)]);if(r.ok)logs=r.data.logs||[];else notify(r.error||'Не удалось загрузить логи','error');if(a.ok)auditLogs=a.data.audit_logs||[];else auditLogs=[];loading=false}
 onMount(load);
 function details(d){if(!d)return '—';if(typeof d==='string')return d;return Object.entries(d).map(([k,v])=>k+': '+String(v)).join(' · ')}
</script>
<div class="page-header"><div><div class="eyebrow">OPERATIONS</div><h1 class="page-title">Activity logs</h1><p class="page-desc">История действий аккаунта и инфраструктуры.</p></div><Button on:click={load}>↻ Обновить</Button></div>
<div class="toolbar"><span>{logs.length} записей</span><select class="input" bind:value={limit} on:change={load}><option value="50">Последние 50</option><option value="100">Последние 100</option><option value="200">Последние 200</option></select></div>
{#if loading}<div class="empty">Загрузка логов…</div>{:else if !logs.length}<div class="empty">Записей пока нет.</div>{:else}<div class="log-list">{#each logs as log}<article class="log-row"><span class="log-mark">≡</span><div class="log-main"><strong>{log.action.replaceAll('_',' ')}</strong><small>{details(log.details)} {#if log.device}· {log.device}{/if}</small></div><time>{new Date(log.created_at).toLocaleString()}</time></article>{/each}</div>{/if}
<section class="audit-section">
 <div class="section-heading"><h2>Security audit</h2><span>{auditLogs.length} событий</span></div>
 {#if !auditLogs.length}<div class="empty audit-empty">Нет доступных событий аудита или роль не разрешает просмотр.</div>{:else}
 <div class="log-list">{#each auditLogs as event (event.id)}<article class="log-row"><span class="log-mark">⌁</span><div class="log-main"><strong>{event.action.replaceAll('_',' ')} · {event.outcome}</strong><small>{event.target_type}:{event.target_id} · actor #{event.actor_user_id || 'system'} · {event.ip || 'IP неизвестен'}</small></div><time>{new Date(event.created_at).toLocaleString()}</time></article>{/each}</div>
 {/if}
</section>
<style>
.audit-section{margin-top:28px}.section-heading{display:flex;justify-content:space-between;align-items:center;margin-bottom:10px}.section-heading h2{font-size:13px}.section-heading span{font-size:10px;color:#64748b}.audit-empty{border:1px solid #253149;border-radius:10px}.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:7px}.toolbar{display:flex;justify-content:space-between;align-items:center;color:#64748b;font-size:10px;margin-bottom:10px}.toolbar select{width:auto}.log-list{border:1px solid #253149;border-radius:11px;background:#101727;overflow:hidden}.log-row{display:flex;align-items:center;gap:11px;padding:11px 13px;border-bottom:1px solid #1e293b}.log-row:last-child{border:0}.log-mark{width:28px;height:28px;display:grid;place-items:center;border-radius:7px;background:#172136;color:#a5b4fc}.log-main{display:grid;gap:3px;flex:1;min-width:0}.log-main strong{font-size:10px;text-transform:capitalize}.log-main small{font-size:9px;color:#64748b;overflow-wrap:anywhere}.log-row time{font-size:9px;color:#64748b;white-space:nowrap}
@media(max-width:650px){.log-row{align-items:flex-start}.log-row time{white-space:normal;text-align:right;max-width:95px}}
</style>