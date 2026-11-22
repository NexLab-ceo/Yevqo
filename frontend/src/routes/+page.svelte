<script>
 import { onMount } from 'svelte';
 import { goto } from '$app/navigation';
 onMount(() => goto('/overview', { replaceState: true }));
</script>
<svelte:head><title>Yevqo · Overview</title></svelte:head>
<div class="empty" aria-live="polite">Opening Yevqo Overview…</div>
