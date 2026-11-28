<script>
    import { onMount } from 'svelte';
    import { adminErrorMode, initializeAdminErrorMode, setAdminErrorMode } from '$lib/admin-error-mode.js';

    onMount(initializeAdminErrorMode);
</script>

<div class="error-mode-control">
    <span>Ошибки:</span>
    <label>
        <input type="checkbox" checked={$adminErrorMode} on:change={(event) => setAdminErrorMode(event.currentTarget.checked)} />
        Полная диагностика
    </label>
    <small>{$adminErrorMode ? 'Показывать операцию, HTTP, код и ID диагностики' : 'Показывать только обобщённое сообщение'}</small>
</div>

<style>
    .error-mode-control { display:flex; align-items:center; flex-wrap:wrap; gap:10px; margin:12px 0; color:var(--text-muted); font-size:12px; }
    .error-mode-control label { display:inline-flex; align-items:center; gap:7px; color:var(--text-main); cursor:pointer; }
    .error-mode-control input { accent-color:var(--accent); }
    .error-mode-control small { color:var(--text-muted); }
</style>
