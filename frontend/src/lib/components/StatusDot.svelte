<script>export let status='offline';</script><span class="dot dot-{status}" aria-label={status}></span>
<style>
.dot{display:inline-block;width:7px;height:7px;border-radius:50%;flex-shrink:0;background:#EF4444}.dot-online,.dot-running{background:var(--success);box-shadow:0 0 8px color-mix(in srgb,var(--success) 40%,transparent)}.dot-offline,.dot-stopped{background:var(--danger);box-shadow:0 0 8px color-mix(in srgb,var(--danger) 40%,transparent)}.dot-error{background:var(--danger);box-shadow:0 0 8px color-mix(in srgb,var(--danger) 40%,transparent)}
</style>