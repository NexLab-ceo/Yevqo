<script>
 import {onMount} from 'svelte';
 import {goto} from '$app/navigation';
 onMount(()=>goto('/settings/integrations/github',{replaceState:true}));
</script>
<svelte:head><title>GitHub App · Yevqo</title></svelte:head>
<div class="empty">Открываем GitHub App settings…</div>