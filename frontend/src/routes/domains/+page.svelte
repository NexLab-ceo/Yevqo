<script>
 import {onMount} from 'svelte';
 import {goto} from '$app/navigation';
 onMount(()=>goto('/sites',{replaceState:true}));
</script>
<svelte:head><title>Domains moved · Yevqo</title></svelte:head>
<div class="empty"><strong>Domains теперь находятся в настройках сайта</strong><p>Откройте Project → Applications → нужный сайт → Domains.</p></div>
<style>.empty{max-width:520px;margin:12vh auto;padding:28px;border:1px solid #27272a;border-radius:10px;background:#18181b;color:#a1a1aa;text-align:center;font-size:12px}.empty strong{display:block;color:#fafafa;font-size:15px;margin-bottom:8px}</style>
