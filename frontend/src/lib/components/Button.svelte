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
    .btn{display:inline-flex;align-items:center;justify-content:center;gap:7px;border-radius:9px;border:1px solid #253149;background:transparent;color:#f1f5f9;font-family:inherit;font-weight:600;cursor:pointer;transition:background .15s,border-color .15s,opacity .15s;white-space:nowrap}
    .btn:hover:not(:disabled){background:#1b2740;border-color:#3b4b68}.btn:disabled{opacity:.48;cursor:not-allowed}.btn.full-width{width:100%}
    .btn-md{padding:8px 13px;font-size:11px;min-height:34px}.btn-sm{padding:5px 9px;font-size:10px;min-height:27px}
    .btn-primary{background:#6366f1;color:#fff;border-color:#6366f1}.btn-primary:hover:not(:disabled){background:#818cf8;border-color:#818cf8}
    .btn-danger{color:#fb7185;border-color:#f43f5e55}.btn-danger:hover:not(:disabled){background:#f43f5e18;border-color:#f43f5e88}
    .btn-ghost{border-color:transparent;color:#94a3b8}.btn-ghost:hover:not(:disabled){background:#172136;color:#f1f5f9}
    .spinner{display:inline-block;width:12px;height:12px;border:2px solid currentColor;border-right-color:transparent;border-radius:50%;animation:spin .6s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}
</style>