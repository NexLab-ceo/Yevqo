<script>
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { listSites, sendCommand } from '$lib/api.js';
    import { notify } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';
    import StatusDot from '$lib/components/StatusDot.svelte';

    let loading = true;
    let refreshing = false;
    let busySite = '';
    let busyBulk = false;
    let selectedSites = [];
    let sites = [];

    // Загружаем сайты повторно по запросу пользователя.
    async function loadSites() {
        refreshing = true;
        const res = await listSites($page.url.searchParams.get('project_id') || '');
        if (res.ok) {
            sites = res.data?.sites || [];
        } else {
            notify(res.error || 'Не удалось загрузить сайты', 'error');
        }
        loading = false;
        refreshing = false;
    }

    onMount(loadSites);

    // Открываем страницу конкретного сайта.
    function openSite(item) {
        goto(`/sites/${item.device_id}/${encodeURIComponent(item.site.site_id)}${$page.url.searchParams.get('project_id') ? '?project_id=' + encodeURIComponent($page.url.searchParams.get('project_id')) : ''}`);
    }
</script>
