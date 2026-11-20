<script>
 import {onMount} from 'svelte';import {page} from '$app/stores';import {getMonitoring} from '$lib/api.js';import {notify} from '$lib/stores.js';import Button from '$lib/components/Button.svelte';import StatusDot from '$lib/components/StatusDot.svelte';
 let devices=[],loading=true;
 async function load(){const r=await getMonitoring($page.url.searchParams.get('project_id')||'');if(r.ok)devices=r.data.devices||[];else notify(r.error||'Не удалось проверить устройства','error');loading=false}
 onMount(load);
</script>
<div class="page-header"><div><div class="eyebrow">SYSTEM</div><h1 class="page-title">Maintenance</h1><p class="page-desc">Совместимость Agent и состояние подключённых Android-хостов.</p></div><Button on:click={load}>↻ Проверить</Button></div>
{#if loading}<div class="empty">Проверяем устройства…</div>{:else if devices.length}<div class="device-list">{#each devices as d}<article class="device"><span class="device-copy"><strong>{d.hostname}</strong><small>Agent version {d.agent_version||'unknown'} · Last heartbeat {d.last_seen?new Date(d.last_seen).toLocaleString():'never'}</small></span><span class="state"><StatusDot status={d.status}/>{d.status}</span></article>{/each}</div>{:else}<div class="empty">Нет подключённых устройств.</div>{/if}
<div class="notice">Обновление Agent, rollback, очистка хранилища и системная диагностика пока не запускаются из панели. Для обновления потребуется подписанный пакет, проверка целостности и сохранение конфигурации на устройстве.</div>
<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:7px}.device-list{border:1px solid #253149;border-radius:11px;background:#101727;overflow:hidden}.device{display:flex;align-items:center;gap:10px;padding:12px;border-bottom:1px solid #1e293b}.device:last-child{border:0}.device-copy{display:grid;gap:4px;flex:1;min-width:0}.device-copy strong{font-size:10px}.device-copy small,.state{font-size:9px;color:#64748b}.state{display:flex;align-items:center;gap:6px}.notice{margin-top:13px;padding:12px;border:1px solid #334155;border-radius:9px;background:#101727;color:#94a3b8;font-size:10px;line-height:1.6}
</style>