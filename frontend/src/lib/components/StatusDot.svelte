<script>
    /**
     * StatusDot — маленькая точка статуса.
     * Используется для устройств (online/offline) и сайтов (running/stopped).
     */
    export let status = 'offline'; // 'online' | 'offline' | 'running' | 'stopped' | 'error'
</script>

<span class="dot dot-{status}" aria-label={status}></span>

<style>
    .dot {
        display: inline-block;
        width: 6px;
        height: 6px;
        border-radius: 50%;
        flex-shrink: 0;
    }

    .dot-online,
    .dot-running {
        background: #22c55e;
    }

    .dot-offline,
    .dot-stopped {
        background: #52525b;
    }

    .dot-error {
        background: #ef4444;
    }
</style>
