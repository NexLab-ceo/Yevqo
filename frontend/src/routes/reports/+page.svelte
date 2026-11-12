<script>
 import {onMount} from 'svelte';import {getReports} from '$lib/api.js';import {notify} from '$lib/stores.js';import Button from '$lib/components/Button.svelte';
 let report=null,loading=true;
 async function load(){const r=await getReports();if(r.ok)report=r.data.report;else notify(r.error||'Не удалось собрать отчёт','error');loading=false}
 onMount(load);
 function exportCSV(){if(!report)return;const rows=[['metric','value'],['devices',report.devices],['websites',report.sites],['running_websites',report.running_sites],['database_servers',report.database_servers],['generated_at',report.generated_at]];const csv=rows.map(r=>r.map(v=>'"'+String(v??'').replaceAll('"','""')+'"').join(',')).join('\n');const url=URL.createObjectURL(new Blob([csv],{type:'text/csv;charset=utf-8'}));const a=document.createElement('a');a.href=url;a.download='veltrix-report.csv';a.click();URL.revokeObjectURL(url)}
</script>
<div class="page-header"><div><div class="eyebrow">OPERATIONS</div><h1 class="page-title">Reports & Export</h1><p class="page-desc">Сводка инфраструктуры аккаунта и экспорт текущих показателей.</p></div><Button variant="primary" disabled={!report} on:click={exportCSV}>↓ Экспорт CSV</Button></div>
{#if loading}<div class="empty">Собираем показатели…</div>{:else if report}<div class="report-grid"><div class="metric"><small>Devices</small><strong>{report.devices}</strong></div><div class="metric"><small>Websites</small><strong>{report.sites}</strong></div><div class="metric"><small>Running websites</small><strong>{report.running_sites}</strong></div><div class="metric"><small>Database servers</small><strong>{report.database_servers}</strong></div></div><p class="generated">Сформировано: {new Date(report.generated_at).toLocaleString()}</p>{/if}
<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:7px}.report-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:10px}.metric{padding:16px;background:#101727;border:1px solid #253149;border-radius:11px;display:grid;gap:8px}.metric small{font-size:9px;color:#64748b}.metric strong{font-size:24px}.generated{font-size:9px;color:#64748b;margin-top:12px}
@media(max-width:700px){.report-grid{grid-template-columns:repeat(2,1fr)}}
</style>