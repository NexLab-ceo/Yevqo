<script>
    export let variant = 'default';
    export let size = 'md';
    export let type = 'button';
    export let disabled = false;
    export let loading = false;
    export let fullWidth = false;
</script>
<button {type} {disabled} class="btn btn-{variant} btn-{size}" class:full-width={fullWidth} class:loading on:click>
    {#if loading}<span class="spinner"></span>{:else}<slot />{/if}
</button>
<style>
    .btn{display:inline-flex;align-items:center;justify-content:center;gap:7px;border-radius:9px;border:1px solid var(--border);background:transparent;color:var(--text-main);font-family:inherit;font-weight:600;cursor:pointer;transition:background .15s,border-color .15s,opacity .15s;white-space:nowrap}
    .btn:hover:not(:disabled){background:var(--bg-hover);border-color:var(--border-hover)}.btn:disabled{opacity:.48;cursor:not-allowed}.btn.full-width{width:100%}
    .btn-md{padding:8px 13px;font-size:11px;min-height:34px}.btn-sm{padding:5px 9px;font-size:10px;min-height:27px}
    .btn-primary{background:var(--accent);color:var(--accent-fg);border-color:var(--accent)}.btn-primary:hover:not(:disabled){background:var(--accent-hover);border-color:var(--accent-hover)}
    .btn-danger{color:var(--danger);border-color:color-mix(in srgb,var(--danger) 45%,transparent)}.btn-danger:hover:not(:disabled){background:color-mix(in srgb,var(--danger) 10%,transparent);border-color:var(--danger)}
    .btn-ghost{border-color:transparent;color:var(--text-muted)}.btn-ghost:hover:not(:disabled){background:var(--bg-hover);color:var(--text-main)}
    .spinner{display:inline-block;width:12px;height:12px;border:2px solid currentColor;border-right-color:transparent;border-radius:50%;animation:spin .6s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}
</style>