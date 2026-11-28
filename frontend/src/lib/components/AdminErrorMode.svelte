<script>
    import { onMount } from 'svelte';
    import { errorDisplayMode, initializeAdminErrorMode, setAdminErrorMode } from '$lib/admin-error-mode.js';

    onMount(initializeAdminErrorMode);
</script>

<div class="error-mode-control" role="group" aria-label="Режим отображения ошибок">
    <span>Ошибки:</span>
    <label>
        <input type="checkbox" checked={$errorDisplayMode} on:change={(event) => setAdminErrorMode(event.currentTarget.checked)} />
        Полная диагностика
    </label>
    <small>{$errorDisplayMode ? 'Подробности ошибок включены на всём сайте' : 'На всём сайте показываются обобщённые ошибки'}</small>
</div>

<style>
    .error-mode-control { position:fixed; z-index:190; top:66px; right:18px; display:flex; align-items:center; flex-wrap:wrap; gap:9px; max-width:min(520px,calc(100vw - 28px)); padding:8px 10px; border:1px solid var(--border,#2B3548); border-radius:7px; background:var(--bg-card,#111827); color:var(--text-muted,#9AA8BD); box-shadow:0 8px 24px #0005; font-size:11px; }
    .error-mode-control label { display:inline-flex; align-items:center; gap:7px; color:var(--text-main,#E9EEF8); cursor:pointer; white-space:nowrap; }
    .error-mode-control input { accent-color:var(--accent,#8AB4FF); }
    .error-mode-control small { color:var(--text-muted,#9AA8BD); }
    @media(max-width:700px) { .error-mode-control { top:auto; right:12px; bottom:64px; left:12px; justify-content:space-between; } }
</style>
