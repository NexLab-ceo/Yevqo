<script>
    /**
     * Button — кнопка в стиле Dokploy.
     * Варианты: primary (белая), danger (красная), ghost (прозрачная), default (серая).
     */
    export let variant = 'default'; // 'primary' | 'danger' | 'ghost' | 'default'
    export let size = 'md';         // 'sm' | 'md'
    export let type = 'button';     // 'button' | 'submit'
    export let disabled = false;
    export let loading = false;
    export let fullWidth = false;
</script>

<button
    {type}
    {disabled}
    class="btn btn-{variant} btn-{size}"
    class:full-width={fullWidth}
    class:loading
    on:click
>
    {#if loading}
        <span class="spinner"></span>
    {:else}
        <slot />
    {/if}
</button>

<style>
    .btn {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        gap: 6px;
        border-radius: 6px;
        border: 1px solid #27272a;
        background: transparent;
        color: #fafafa;
        font-family: inherit;
        font-weight: 500;
        cursor: pointer;
        transition: background 0.1s, border-color 0.1s, opacity 0.1s;
        white-space: nowrap;
    }

    .btn:hover:not(:disabled) {
        background: #18181b;
        border-color: #3f3f46;
    }

    .btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .btn.full-width {
        width: 100%;
    }

    /* Sizes */
    .btn-md {
        padding: 6px 12px;
        font-size: 13px;
        min-height: 32px;
    }

    .btn-sm {
        padding: 4px 10px;
        font-size: 12px;
        min-height: 26px;
    }

    /* Variants */
    .btn-primary {
        background: #fafafa;
        color: #09090b;
        border-color: #fafafa;
    }

    .btn-primary:hover:not(:disabled) {
        background: #e4e4e7;
        border-color: #e4e4e7;
    }

    .btn-danger {
        color: #f87171;
        border-color: rgba(239, 68, 68, 0.3);
    }

    .btn-danger:hover:not(:disabled) {
        background: rgba(239, 68, 68, 0.1);
        border-color: rgba(239, 68, 68, 0.5);
    }

    .btn-ghost {
        border-color: transparent;
        color: #a1a1aa;
    }

    .btn-ghost:hover:not(:disabled) {
        background: #18181b;
        color: #fafafa;
        border-color: transparent;
    }

    /* Spinner */
    .spinner {
        display: inline-block;
        width: 12px;
        height: 12px;
        border: 2px solid currentColor;
        border-right-color: transparent;
        border-radius: 50%;
        animation: spin 0.6s linear infinite;
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }
</style>
