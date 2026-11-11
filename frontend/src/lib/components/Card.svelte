<script>
    /**
     * Card — карточка с границей (как в Dokploy).
     * Может быть кликабельной (если передан on:click).
     */
    export let clickable = false;
    export let padding = 'md'; // 'sm' | 'md' | 'lg'
</script>

<div
    class="card card-{padding}"
    class:clickable
    on:click
    on:keydown
    role={clickable ? 'button' : undefined}
    tabindex={clickable ? 0 : undefined}
>
    <slot />
</div>

<style>
    .card {
        background: #09090b;
        border: 1px solid #27272a;
        border-radius: 8px;
        transition: border-color 0.1s;
    }

    .card.clickable {
        cursor: pointer;
    }

    .card.clickable:hover {
        border-color: #3f3f46;
    }

    .card.clickable:focus {
        outline: none;
        border-color: #52525b;
    }

    .card-sm {
        padding: 12px;
    }

    .card-md {
        padding: 20px;
    }

    .card-lg {
        padding: 32px;
    }
</style>
