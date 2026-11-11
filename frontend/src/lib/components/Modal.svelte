<script>
    /**
     * Modal — модальное окно в стиле Dokploy.
     * Открывается/закрывается через свойство `open`.
     */
    import Button from './Button.svelte';

    export let open = false;
    export let title = '';
    export let onClose = () => {};

    function handleBackdrop(e) {
        if (e.target === e.currentTarget) {
            onClose();
        }
    }

    function handleKey(e) {
        if (e.key === 'Escape') {
            onClose();
        }
    }
</script>

<svelte:window on:keydown={handleKey} />

{#if open}
    <div
        class="backdrop"
        on:click={handleBackdrop}
        on:keydown
        role="button"
        tabindex="-1"
    >
        <div class="modal" role="dialog" aria-modal="true">
            <div class="modal-header">
                <h2 class="modal-title">{title}</h2>
                <button class="close" on:click={onClose} aria-label="Close">×</button>
            </div>
            <div class="modal-body">
                <slot />
            </div>
            {#if $$slots.footer}
                <div class="modal-footer">
                    <slot name="footer" />
                </div>
            {/if}
        </div>
    </div>
{/if}

<style>
    .backdrop {
        position: fixed;
        inset: 0;
        background: rgba(0, 0, 0, 0.7);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 100;
        padding: 16px;
    }

    .modal {
        background: #09090b;
        border: 1px solid #27272a;
        border-radius: 8px;
        width: 100%;
        max-width: 440px;
        max-height: 90vh;
        overflow: auto;
    }

    .modal-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 16px 20px;
        border-bottom: 1px solid #27272a;
    }

    .modal-title {
        font-size: 14px;
        font-weight: 600;
        margin: 0;
    }

    .close {
        background: none;
        border: none;
        color: #71717a;
        font-size: 20px;
        line-height: 1;
        padding: 0;
        width: 24px;
        height: 24px;
        cursor: pointer;
        border-radius: 4px;
    }

    .close:hover {
        color: #fafafa;
        background: #18181b;
    }

    .modal-body {
        padding: 20px;
    }

    .modal-footer {
        padding: 12px 20px;
        border-top: 1px solid #27272a;
        display: flex;
        justify-content: flex-end;
        gap: 8px;
    }
</style>
