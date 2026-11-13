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
    .btn{display:inline-flex;align-items:center;justify-content:center;gap:7px;border-radius:9px;border:1px solid #27272A;background:transparent;color:#FAFAFA;font-family:inherit;font-weight:600;cursor:pointer;transition:background .15s,border-color .15s,opacity .15s;white-space:nowrap}
    .btn:hover:not(:disabled){background:#1b2740;border-color:#3f3f46}.btn:disabled{opacity:.48;cursor:not-allowed}.btn.full-width{width:100%}
    .btn-md{padding:8px 13px;font-size:11px;min-height:34px}.btn-sm{padding:5px 9px;font-size:10px;min-height:27px}
    .btn-primary{background:#FAFAFA;color:#09090B;border-color:#FAFAFA}.btn-primary:hover:not(:disabled){background:#3B82F6;border-color:#3B82F6}
    .btn-danger{color:#EF4444;border-color:#EF444455}.btn-danger:hover:not(:disabled){background:#EF444418;border-color:#EF444488}
    .btn-ghost{border-color:transparent;color:#A1A1AA}.btn-ghost:hover:not(:disabled){background:#27272A;color:#FAFAFA}
    .spinner{display:inline-block;width:12px;height:12px;border:2px solid currentColor;border-right-color:transparent;border-radius:50%;animation:spin .6s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}
</style>