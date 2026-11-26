<script>
    import Button from './Button.svelte';
    export let open=false;export let title='';export let onClose=()=>{};
    function handleBackdrop(e){if(e.target===e.currentTarget)onClose()}
    function handleKey(e){if(e.key==='Escape')onClose()}
</script>
<svelte:window on:keydown={handleKey}/>
{#if open}
<div class="backdrop" on:click={handleBackdrop} on:keydown role="button" tabindex="-1">
    <div class="modal" role="dialog" aria-modal="true" aria-label={title}>
        <div class="modal-header"><h2 class="modal-title">{title}</h2><button class="close" on:click={onClose} aria-label="Close">×</button></div>
        <div class="modal-body"><slot /></div>
        {#if $$slots.footer}<div class="modal-footer"><slot name="footer"/></div>{/if}
    </div>
</div>
{/if}
<style>
.backdrop{position:fixed;inset:0;background:#020617bb;backdrop-filter:blur(5px);display:flex;align-items:center;justify-content:center;z-index:100;padding:16px}.modal{background:var(--bg-card);border:1px solid var(--border);border-radius:14px;width:100%;max-width:480px;max-height:90vh;overflow:auto;box-shadow:0 24px 90px #0009}.modal-header{display:flex;align-items:center;justify-content:space-between;padding:15px 18px;border-bottom:1px solid var(--border)}.modal-title{font-size:13px;font-weight:650}.close{background:none;border:0;color:var(--text-muted);font-size:21px;width:25px;height:25px;border-radius:6px}.close:hover{color:var(--text-main);background:var(--bg-hover)}.modal-body{padding:18px}.modal-footer{padding:12px 18px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:8px}
</style>