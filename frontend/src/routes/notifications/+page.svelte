<script>
 import {formatDate} from '$lib/date.js';
 import {locale} from '$lib/i18n.js';
 import {onMount} from 'svelte';import {listNotifications,markNotificationRead} from '$lib/api.js';import {notify} from '$lib/stores.js';import Button from '$lib/components/Button.svelte';
 let items=[],loading=true;
 $: ru=$locale!=='en';
 async function load(){const r=await listNotifications();if(r.ok)items=r.data.notifications||[];else notify(r.error||(ru?'Не удалось загрузить уведомления':'Could not load notifications'),'error');loading=false}
 onMount(load);
 async function mark(item){const r=await markNotificationRead(item.id);if(r.ok){item.read_at=new Date().toISOString();items=[...items]}else notify(r.error||'{ru?'Ошибка':'Error'},'error')}
</script>
<div class="page-header"><div><div class="eyebrow">OPERATIONS</div><h1 class="page-title">{ru?'Уведомления':'Notifications'}</h1><p class="page-desc">{ru?'Системные события и уведомления аккаунта.':'System events and account notifications.'}</p></div><Button on:click={load}>{ru?'↻ Обновить':'↻ Refresh'}</Button></div>
{#if loading}<div class="empty">{ru?'Загрузка…':'Loading…'}</div>{:else if !items.length}<div class="empty">{ru?'Новых уведомлений нет.':'No notifications yet.'}</div>{:else}<div class="notification-list">{#each items as item}<article class:unread={!item.read_at} class="notification"><span class="notification-dot"></span><div class="notification-copy"><strong>{item.title}</strong><span class="notification-type">{item.type}</span><p>{item.body}</p><small>{formatDate(item.created_at)}</small></div>{#if !item.read_at}<Button size="sm" on:click={()=>mark(item)}>{ru?'Прочитано':'Mark read'}</Button>{:else}<span class="read-label">{ru?'Прочитано':'Read'}</span>{/if}</article>{/each}</div>{/if}
<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:7px}.notification-list{display:grid;gap:7px}.notification{display:flex;align-items:center;gap:11px;padding:13px;background:#101727;border:1px solid #253149;border-radius:10px}.notification.unread{border-color:#3b4b68;background:#121c30}.notification-dot{width:7px;height:7px;border-radius:50%;background:#64748b;flex:none}.unread .notification-dot{background:#818cf8;box-shadow:0 0 8px #818cf866}.notification-copy{flex:1;min-width:0}.notification-copy strong{font-size:11px}.notification-type{display:inline-block;margin-left:7px;padding:2px 5px;border-radius:4px;background:#202b40;color:#a5b4fc;font-size:8px;text-transform:uppercase;letter-spacing:.06em}.notification-copy p{font-size:10px;color:#94a3b8;margin-top:3px}.notification-copy small,.read-label{font-size:9px;color:#64748b}.read-label{white-space:nowrap}
</style>