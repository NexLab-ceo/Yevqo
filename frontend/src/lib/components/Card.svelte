<script>
    import { createEventDispatcher } from 'svelte';
    export let clickable = false;
    export let padding = 'md';
    const dispatch = createEventDispatcher();
    function handleClick(event) {
        if (clickable) dispatch('click', event);
    }
    function handleKeydown(event) {
        if (!clickable || (event.key !== 'Enter' && event.key !== ' ')) return;
        event.preventDefault();
        dispatch('click', event);
    }
</script>
<div class="card card-{padding}" class:clickable on:click={handleClick} on:keydown={handleKeydown} role={clickable ? 'button' : undefined} tabindex={clickable ? 0 : undefined}><slot /></div>
<style>
.card{background:var(--bg-card);border:1px solid var(--border);border-radius:8px;transition:background .15s,border-color .15s}.card.clickable{cursor:pointer}.card.clickable:hover{border-color:var(--border-hover);background:var(--bg-hover)}.card.clickable:focus-visible{outline:2px solid var(--accent);outline-offset:2px}.card-sm{padding:12px}.card-md{padding:17px}.card-lg{padding:24px}
</style>