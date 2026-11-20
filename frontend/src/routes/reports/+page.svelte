<script>
 import { onMount } from 'svelte';
 import { page } from '$app/stores';
 import { getReports, listLogs } from '$lib/api.js';
 import { notify } from '$lib/stores.js';
 import Button from '$lib/components/Button.svelte';

 let report = null;
 let logs = [];
 let loading = true;
 let reportType = 'security';
 let periodDays = '30';
 let scope = 'all';

 $: cutoff = Date.now() - Number(periodDays) * 24 * 60 * 60 * 1000;
 $: periodLogs = logs.filter((entry) => {
   const timestamp = new Date(entry.created_at).getTime();
   return Number.isFinite(timestamp) && timestamp >= cutoff;
 });
 $: failedEvents = periodLogs.filter((entry) => entry.details?.ok === false);
 $: sensitiveEvents = periodLogs.filter((entry) => /login|auth|password|credential|permission|role|delete|block|restore|database|device|tunnel|backup/i.test(entry.action || ''));

 async function load() {
   loading = true;
   const projectId = $page.url.searchParams.get('project_id') || ''; const [summary, audit] = await Promise.all([getReports(projectId), listLogs(200, projectId)]);
   if (summary.ok) report = summary.data?.report || null;
   else notify(summary.error || 'Не удалось загрузить сводку', 'error');
   if (audit.ok) logs = audit.data?.logs || [];
   else notify(audit.error || 'Не удалось загрузить журнал аудита', 'error');
   loading = false;
 }

 onMount(load);

 function exportCSV() {
   const rows = [['timestamp', 'action', 'device', 'details'], ...periodLogs.map((entry) => [entry.created_at, entry.action, entry.device, JSON.stringify(entry.details || {})])];
   const csv = rows.map((row) => row.map((value) => '"' + String(value ?? '').replaceAll('"', '""') + '"').join(',')).join('\n');
   const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
   const link = document.createElement('a');
   link.href = url;
   link.download = 'veltrix-security-report.csv';
   link.click();
   URL.revokeObjectURL(url);
 }

 function exportPDF() {
   window.print();
 }
</script>

<svelte:head><title>Reports · VELTRIX Dock</title></svelte:head>

<div class="page-header no-print">
 <div><div class="eyebrow">OPERATIONS</div><h1 class="page-title">Reports & Export</h1><p class="page-desc">Отчёты по инфраструктуре и событиям безопасности.</p></div>
 <Button on:click={load} disabled={loading}>↻ Обновить</Button>
</div>

<section class="generator no-print">
 <div class="generator-heading"><div><h2>Генератор отчётов</h2><p>Выберите состав отчёта и сформируйте документ на основе текущих данных Control Plane.</p></div><span class="live-dot">● Данные аккаунта</span></div>
 <div class="controls">
  <label>Тип отчёта<select bind:value={reportType}><option value="security">Security Report</option><option value="infrastructure">Infrastructure Summary</option></select></label>
  <label>Период<select bind:value={periodDays}><option value="7">Last 7 days</option><option value="30">Last 30 days</option><option value="90">Last 90 days</option></select></label>
  <label>Область<select bind:value={scope}><option value="all">All projects</option></select></label>
  <div class="format"><span>Формат</span><strong>PDF</strong><small>Печать / Save as PDF</small></div>
 </div>
 <div class="actions"><Button variant="primary" on:click={exportPDF} disabled={loading}>Сформировать PDF</Button><Button on:click={exportCSV} disabled={loading || periodLogs.length===0}>Экспорт событий CSV</Button></div>
</section>

{#if loading}
 <div class="empty">Собираем данные отчёта…</div>
{:else}
 <article class="report-document">
  <header class="document-header"><div><div class="eyebrow">VELTRIX DOCK · REPORT</div><h2>{reportType === 'security' ? 'Security Report' : 'Infrastructure Summary'}</h2><p>Период: последние {periodDays} дней · Область: {scope === 'all' ? 'All projects' : scope}</p></div><div class="brand-mark">V</div></header>
  <div class="document-meta"><span>Сформировано: {new Date().toLocaleString()}</span><span>Источники: Control Plane · Audit Log</span></div>
  {#if reportType === 'security'}
   <div class="metrics">
    <div><small>СОБЫТИЯ АУДИТА</small><strong>{periodLogs.length}</strong></div>
    <div><small>СОБЫТИЯ БЕЗОПАСНОСТИ</small><strong>{sensitiveEvents.length}</strong></div>
    <div><small>ОТМЕЧЕНО ОШИБОК</small><strong>{failedEvents.length}</strong></div>
   </div>
   <section class="report-section"><h3>Журнал событий</h3>
    {#if periodLogs.length}
     <div class="table-wrap"><table><thead><tr><th>Время</th><th>Действие</th><th>Устройство</th><th>Результат</th></tr></thead><tbody>
      {#each periodLogs as entry}
       <tr><td>{new Date(entry.created_at).toLocaleString()}</td><td>{(entry.action || 'event').replaceAll('_',' ')}</td><td>{entry.device || '—'}</td><td>{entry.details?.ok === false ? 'Failed' : 'Recorded'}</td></tr>
      {/each}
     </tbody></table></div>
    {:else}<p class="muted">За выбранный период в доступном журнале нет событий.</p>{/if}
   </section>
   <p class="disclaimer">Отчёт сформирован по событиям аудита, доступным через Control Plane (до 200 последних записей). Отсутствие событий в журнале не является доказательством отсутствия инцидентов. Это не замена внешнему security-аудиту.</p>
  {:else if report}
   <div class="metrics"><div><small>DEVICES</small><strong>{report.devices}</strong></div><div><small>WEBSITES</small><strong>{report.sites}</strong></div><div><small>RUNNING WEBSITES</small><strong>{report.running_sites}</strong></div><div><small>DATABASE SERVERS</small><strong>{report.database_servers}</strong></div></div>
   <section class="report-section"><h3>Сводка инфраструктуры</h3><p class="muted">Снимок текущего состояния аккаунта на момент формирования отчёта.</p></section>
  {:else}
   <p class="muted">Нет данных для формирования отчёта.</p>
  {/if}
  <footer class="document-footer"><span>VELTRIX Dock</span><span>Confidential · Account report</span></footer>
 </article>
{/if}

<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:7px}
.generator,.report-document{padding:18px;background:#101727;border:1px solid #253149;border-radius:12px;margin-bottom:14px}
.generator-heading,.document-header{display:flex;justify-content:space-between;align-items:flex-start;gap:14px}
.generator-heading h2{font-size:13px;margin:0 0 5px}.generator-heading p,.document-header p,.muted{font-size:10px;color:#64748b;line-height:1.6;margin:0}
.live-dot{font-size:9px;color:#86efac;white-space:nowrap}
.controls{display:grid;grid-template-columns:repeat(3,minmax(0,1fr)) minmax(130px,.7fr);gap:10px;margin-top:18px}
.controls label,.format{display:grid;gap:6px;font-size:9px;color:#94a3b8}
.controls select{width:100%;background:#0b1220;border:1px solid #334155;border-radius:7px;padding:10px;color:#e2e8f0;font-size:10px}
.format{padding:9px 11px;background:#0b1220;border:1px solid #334155;border-radius:7px}.format strong{font-size:14px;color:#c7d2fe}.format small{font-size:9px;color:#64748b}
.actions{display:flex;gap:8px;margin-top:15px;flex-wrap:wrap}
.document-header h2{font-size:22px;margin:0 0 7px}.brand-mark{width:36px;height:36px;border-radius:10px;background:#312e81;color:#c7d2fe;display:grid;place-items:center;font-weight:800;font-size:20px}
.document-meta{display:flex;justify-content:space-between;gap:12px;border-top:1px solid #253149;border-bottom:1px solid #253149;padding:10px 0;margin:16px 0;color:#64748b;font-size:9px}
.metrics{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:9px;margin:14px 0}.metrics>div{display:grid;gap:8px;padding:13px;background:#0b1220;border:1px solid #253149;border-radius:9px}.metrics small{font-size:8px;color:#64748b;letter-spacing:.08em}.metrics strong{font-size:22px}
.report-section{margin-top:18px}.report-section h3{font-size:12px;margin:0 0 12px}.table-wrap{overflow:auto;border:1px solid #253149;border-radius:8px}
table{border-collapse:collapse;width:100%;font-size:9px;text-align:left}th,td{padding:9px 10px;border-bottom:1px solid #253149;white-space:nowrap}th{color:#94a3b8;background:#0b1220;font-weight:600}td{color:#cbd5e1}tr:last-child td{border-bottom:0}
.disclaimer{font-size:9px;line-height:1.6;color:#64748b;margin-top:14px}.document-footer{display:flex;justify-content:space-between;border-top:1px solid #253149;padding-top:11px;margin-top:22px;color:#64748b;font-size:9px}
@media(max-width:700px){.controls{grid-template-columns:repeat(2,minmax(0,1fr))}.document-meta{flex-direction:column}.metrics{grid-template-columns:1fr 1fr}}
@media print{ :global(body){background:#fff!important;color:#111!important}.no-print{display:none!important}.report-document{background:#fff!important;border:0!important;color:#111!important;padding:0!important;margin:0!important}.report-document *{color:#111!important}.metrics>div,.table-wrap,th{background:#f5f5f5!important;border-color:#ddd!important}td,th{border-color:#ddd!important}.document-meta,.document-footer{border-color:#ddd!important}.disclaimer,.muted{color:#555!important}.page-header{display:none!important} }
</style>