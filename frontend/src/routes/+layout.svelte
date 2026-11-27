<script>
 import '../app.css';
 import {onMount} from 'svelte';
 import {page} from '$app/stores';
 import {goto} from '$app/navigation';
 import {user,isAuthenticated,uiMode,theme,notify,notifications,workspaceName,compactMode} from '$lib/stores.js';
 import {logout,me,getSettings,saveSettings,listProjects,listDevices,listDeployments,listLogs,listNotifications,markNotificationRead,listIntegrations,listIntegrationRepositories} from '$lib/api.js';
 import {locale} from '$lib/i18n.js';
 import {setDatePreferences,formatDate} from '$lib/date.js';
 let loading=true,mobileNavOpen=false,modeSaving=false,searchOpen=false,searchQuery='',searchLoading=false,searchResults=[],notificationsOpen=false,notifications=[],notificationsLoading=false,profileOpen=false;
 let searchTimer;
 $: ru=$locale!=='en';
 $: unreadCount=notifications.filter(n=>!n.read_at&&!n.read).length;
 const globalGroups=[
  {label:'',items:[['Обзор','/overview','◉'],['Проекты','/projects','▦']]},
  {label:'ОПЕРАЦИИ',items:[['Журналы','/logs','⌘'],['Отчёты','/reports','▤']]},
  {label:'ИНФРАСТРУКТУРА',items:[['Устройства','/devices','▣'],['Домены','/domains','◎'],['Резервные копии','/backups','⤓']]},
  {label:'УПРАВЛЕНИЕ',items:[['Команда','/team','♧'],['Уведомления','/notifications','◌'],['Обслуживание','/maintenance','⚙'],['Настройки','/settings','⚙']]}
 ];
 $: pathParts=$page.url.pathname.split('/');
 $: projectId=(pathParts[1]==='projects'&&Number.isInteger(Number(pathParts[2]))&&Number(pathParts[2])>0?pathParts[2]:'')||$page.url.searchParams.get('project_id')||'';
 $: inProject=!!projectId;
 $: inSite=pathParts[1]==='sites'&&!!pathParts[2]&&!!pathParts[3];
 $: siteContextBase=inSite?'/sites/'+encodeURIComponent(pathParts[2])+'/'+encodeURIComponent(pathParts[3]):'';
 $: groups=globalGroups;
 function siteTabHref(tab){const q=new URLSearchParams();for(const key of ['project_id','environment_id']){const value=$page.url.searchParams.get(key);if(value)q.set(key,value)}q.set('tab',tab);return siteContextBase+'?'+q.toString()}
 onMount(async()=>{const [a,b]=await Promise.all([me(),getSettings()]);if(a.ok&&a.data?.user)user.set(a.data.user);if(b.ok){uiMode.set(b.data.settings?.mode||'simple');theme.set(b.data.settings?.theme||'dark');locale.set(b.data.settings?.locale||'ru');workspaceName.set(b.data.settings?.workspace_name||'Personal workspace');compactMode.set(!!b.data.settings?.compact_mode);setDatePreferences(b.data.settings)}loading=false;loadNotifications()});
 $: if(!loading&&!$isAuthenticated&&!$page.url.pathname.startsWith('/login')&&!$page.url.pathname.startsWith('/register'))goto('/login');
 $: currentTitle=groups.flatMap(g=>g.items).find(i=>{const href=i[1].split('?')[0];return $page.url.pathname===href||($page.url.pathname.startsWith(href+'/')&&href!=='/')})?.[0]||'Overview';
 async function runSearch(){const q=searchQuery.trim().toLowerCase();if(q.length<2){searchResults=[];return}searchLoading=true;const [p,d,dep,logs,ints]=await Promise.all([listProjects(),listDevices(),listDeployments(),listLogs(100),listIntegrations()]);const results=[];if(p.ok)for(const x of p.data.projects||[])if((x.name+' '+(x.description||'')).toLowerCase().includes(q))results.push({type:ru?'Проект':'Project',name:x.name,description:x.description||x.slug,url:'/projects/'+x.id});if(d.ok)for(const x of d.data.devices||[])if((x.hostname+' '+x.device_id+' '+x.platform).toLowerCase().includes(q))results.push({type:ru?'Устройство':'Device',name:x.hostname||x.device_id,description:(x.platform||'Android')+' · '+x.status,url:'/devices/'+x.id});if(dep.ok)for(const x of dep.data.deployments||[])if(JSON.stringify(x).toLowerCase().includes(q))results.push({type:ru?'Развёртывание':'Deployment',name:x.action||'Deployment',description:x.details?.site_id||x.device||'',url:'/deployments'});if(logs.ok)for(const x of logs.data.logs||[])if(JSON.stringify(x).toLowerCase().includes(q))results.push({type:ru?'Журнал':'Log',name:x.action||'Event',description:x.device||'',url:'/logs'});if(ints.ok){const repoLists=await Promise.all((ints.data.integrations||[]).map(integration=>listIntegrationRepositories(integration.id)));for(const repos of repoLists)if(repos.ok)for(const repo of repos.data.repositories||[])if((repo.full_name||repo.name||'').toLowerCase().includes(q))results.push({type:ru?'GitHub репозиторий':'GitHub repository',name:repo.full_name||repo.name,description:repo.default_branch||'',url:'/settings/integrations/github'})}searchResults=results.slice(0,20);searchLoading=false}
 function onSearchInput(){if(searchTimer)clearTimeout(searchTimer);searchTimer=setTimeout(runSearch,220)}
 async function loadNotifications(){notificationsLoading=true;const r=await listNotifications();if(r.ok)notifications=r.data.notifications||[];else notify(r.error||'Could not load notifications','error');notificationsLoading=false}
 async function readNotification(item){if(item.read_at||item.read)return;const r=await markNotificationRead(item.id);if(r.ok){notifications=notifications.map(n=>n.id===item.id?{...n,read:true}:n)}}
 function active(href,pathname){const clean=href.split('?')[0];return clean==='/'?pathname==='/':pathname===clean||pathname.startsWith(clean+'/')}
 function onGlobalKey(event){if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='k'){event.preventDefault();searchOpen=true;setTimeout(()=>document.getElementById('global-search-input')?.focus(),0)}if(event.key==='Escape'){searchOpen=false;notificationsOpen=false;profileOpen=false}}
 async function toggleMode(){if(modeSaving)return;modeSaving=true;const next=$uiMode==='simple'?'advanced':'simple';const r=await saveSettings({mode:next});if(r.ok)uiMode.set(next);modeSaving=false}
 async function toggleTheme(){if(modeSaving)return;modeSaving=true;const next=$theme==='dark'?'light':'dark';const r=await saveSettings({theme:next});if(r.ok)theme.set(next);else notify(r.error||'Не удалось изменить тему','error');modeSaving=false}
 async function handleLogout(){await logout();user.set(null);goto('/login')}
</script>
<svelte:head><meta name="theme-color" content={$theme === "dark" ? "#090D16" : "#F4F6FA"}/><title>Yevqo</title></svelte:head>
<svelte:window on:keydown={onGlobalKey}/>
{#if loading}<div class="loading-screen"><span class="loading-mark">Y</span><span>Yevqo</span><i></i></div>
{:else if !$isAuthenticated}<slot/>
{:else}
<div class="app-shell" data-mode={$uiMode} data-theme={$theme} data-compact={$compactMode}>
 {#if mobileNavOpen}<button class="mobile-scrim" aria-label="Закрыть меню" on:click={()=>mobileNavOpen=false}></button>{/if}
 <aside class:mobile-open={mobileNavOpen} class="sidebar">
  <a class="brand-lockup" href="/overview"><svg class="brand-mark" viewBox="0 0 30 24" aria-label="Yevqo" role="img"><path d="M2 17.5 7.1 6.2h5.1L7.1 17.5H2Z" fill="currentColor"/><path d="M10.8 17.5 16 3.5h5.1L16 17.5h-5.2Z" fill="currentColor"/><path d="M19.4 17.5 24.5 0h5.1l-5.1 17.5h-5.1Z" fill="currentColor"/></svg><span class="brand-copy"><strong>YEVQO</strong></span></a>
  <button class="workspace-switcher" on:click={()=>goto(inProject?'/projects':'/settings')}><span class="workspace-avatar">Y</span><span class="workspace-copy"><strong>{inProject?(ru?'Рабочая область проекта':'Project workspace'):$workspaceName}</strong><small>{inProject?'Project #'+projectId:(ru?'Личный тариф · Локальный хост':'Personal workspace · Local host')}</small></span><span class="switch-chevron">⌄</span></button>
  <nav class="sidebar-nav">
   {#each groups as group}
    <section class="nav-group"><div class="nav-label">{ru?group.label:({'ОПЕРАЦИИ':'OPERATIONS','ИНФРАСТРУКТУРА':'INFRASTRUCTURE','УПРАВЛЕНИЕ':'MANAGEMENT'}[group.label]||'')}</div>
     {#each group.items as item}
      <a href={item[1]} class:active={active(item[1],$page.url.pathname)} class="nav-item" aria-current={active(item[1],$page.url.pathname) ? 'page' : undefined} on:click={()=>mobileNavOpen=false}><span class="nav-icon">{item[2]}</span><span>{ru?item[0]:({'Обзор':'Overview','Проекты':'Projects','Развёртывания':'Deployments','Сборки':'Builds','Расписания':'Schedules','Журналы':'Logs','Отчёты':'Reports','Устройства':'Devices','Приложения':'Applications','Домены':'Domains','Мониторинг':'Monitoring','Базы данных':'Databases','Резервные копии':'Backups','Команда':'Team','Уведомления':'Notifications','Обслуживание':'Maintenance','Настройки':'Settings'}[item[0]]||item[0])}</span>{#if item[0]==='Databases'}<span class="nav-tag">DB</span>{/if}</a>
     {/each}
    </section>
   {/each}
   {#if inSite}
    <section class="nav-group app-context-nav">
     <div class="nav-label">{ru?'ТЕКУЩЕЕ ПРИЛОЖЕНИЕ':'CURRENT APPLICATION'}</div>
     {#each [['Обзор','general','◉'],['Развёртывания','deployments','↗'],['Сборки','build','⌁'],['Расписания','tasks','◷'],['Мониторинг','monitoring','⌁'],['Базы данных','databases','◈'],['Журналы','logs','⌘']] as item}
      <a href={siteTabHref(item[1])} class:active={$page.url.searchParams.get('tab')===(item[1]==='general'?'':item[1])||(item[1]==='general'&&!$page.url.searchParams.get('tab'))} class="nav-item" on:click={()=>mobileNavOpen=false}><span class="nav-icon">{item[2]}</span><span>{ru?item[0]:({'Обзор':'Overview','Развёртывания':'Deployments','Сборки':'Builds','Расписания':'Schedules','Мониторинг':'Monitoring','Базы данных':'Databases','Журналы':'Logs'}[item[0]]||item[0])}</span></a>
     {/each}
    </section>
   {/if}
  </nav>
  <div class="sidebar-bottom"><div class="sidebar-meta"><span>{ru?"Среда выполнения":"Runtime"}</span><small>Android + Termux · Agent</small></div>
   
   <div class="account-row"><span class="account-avatar">{($user?.username||'U').slice(0,1).toUpperCase()}</span><span class="account-copy"><strong>{$user?.username||'User'}</strong><small>{$user?.email||'Personal account'}</small></span><button class="account-menu" title="Выйти" aria-label="Выйти" on:click={handleLogout}>↗</button></div>
  </div>
 </aside>
 <main class="main-area">
  <header class="topbar"><div class="topbar-left"><button class="mobile-menu" aria-label="Открыть меню" on:click={()=>mobileNavOpen=true}>☰</button><div class="breadcrumbs"><span>{inProject?'Project '+projectId:(ru?'Рабочая область':'Workspace')}</span><b>/</b><strong>{ru?currentTitle:({'Обзор':'Overview','Проекты':'Projects','Развёртывания':'Deployments','Сборки':'Builds','Расписания':'Schedules','Журналы':'Logs','Отчёты':'Reports','Устройства':'Devices','Домены':'Domains','Мониторинг':'Monitoring','Базы данных':'Databases','Резервные копии':'Backups','Команда':'Team','Уведомления':'Notifications','Обслуживание':'Maintenance','Настройки':'Settings'}[currentTitle]||currentTitle)}</strong></div></div>
   <div class="topbar-actions"><button class="global-search-trigger" on:click={()=>{searchOpen=true;setTimeout(()=>document.getElementById('global-search-input')?.focus(),0)}}><span>⌕</span><span>{ru?'Поиск проектов, устройств…':'Search projects, devices…'}</span><kbd>⌘ K</kbd></button><button class="theme-switch" on:click={toggleTheme} disabled={modeSaving} aria-label="Переключить тему" title="Переключить тему"><span>{$theme === "dark" ? "☀" : $theme==="system" ? "◐" : "☾"}</span>{$theme === "system" ? (ru?"Системная":"System") : $theme === "dark" ? (ru?"Тёмная":"Dark") : (ru?"Светлая":"Light")}</button><button class="mode-switch" on:click={toggleMode} disabled={modeSaving}><span class:mode-dot-advanced={$uiMode==='advanced'} class="mode-dot"></span>{$uiMode==='advanced'?'Advanced':'Simple'}<span class="switch-arrows">⇄</span></button><div class="topbar-popover"><button class="icon-action notification-trigger" aria-label="Notifications" on:click={()=>{notificationsOpen=!notificationsOpen;profileOpen=false;if(notificationsOpen)loadNotifications()}}>♧{#if unreadCount}<i>{unreadCount>9?'9+':unreadCount}</i>{/if}</button>{#if notificationsOpen}<div class="popover-panel notification-panel"><div class="popover-heading"><strong>{ru?'Уведомления':'Notifications'}</strong><a href="/notifications" on:click={()=>notificationsOpen=false}>{ru?'Все':'View all'}</a></div>{#if notificationsLoading}<div class="popover-empty">{ru?'Загрузка…':'Loading…'}</div>{:else if !notifications.length}<div class="popover-empty">{ru?'Новых уведомлений нет':'No notifications yet'}</div>{:else}{#each notifications.slice(0,8) as item}<button class:unread={!item.read&&!item.read_at} class="notification-item" on:click={()=>readNotification(item)}><strong>{item.title||item.event||item.type||'Yevqo'}</strong><span>{item.body||item.description||item.message||''}</span><small>{item.created_at?formatDate(item.created_at):''}</small><em class={'notification-priority '+(item.type||'info')}>{item.type||'info'}</em></button>{/each}{/if}</div>{/if}</div><div class="topbar-popover"><button class="profile-action" on:click={()=>{profileOpen=!profileOpen;notificationsOpen=false}}>{($user?.username||'U').slice(0,1).toUpperCase()}</button>{#if profileOpen}<div class="popover-panel profile-panel"><div class="profile-summary"><strong>{$user?.username||'User'}</strong><small>{$user?.email||''}</small></div><a href="/profile" on:click={()=>profileOpen=false}>{ru?'Аккаунт':'Account'}</a><a href="/settings" on:click={()=>profileOpen=false}>{ru?'Настройки':'Settings'}</a><a href="https://github.com/NexLab-ceo/Yevqo#readme" target="_blank" rel="noreferrer" on:click={()=>profileOpen=false}>{ru?'Документация':'Documentation'}</a><button on:click={handleLogout}>{ru?'Выйти':'Sign out'}</button></div>{/if}</div></div>
  </header>
  <div class="page-scroll"><div class="page-container"><slot/></div></div>
 </main>
 {#if $notifications.length}
  <div class="toast-stack" aria-live="polite" aria-relevant="additions">
   {#each $notifications as item (item.id)}
    <div class={"toast-item toast-"+(item.type||"info")} role={item.type==="error"?"alert":"status"}>{item.message}</div>
   {/each}
  </div>
 {/if}
</div>
{/if}
<style>
.sidebar-meta{display:grid;gap:5px;padding:10px 10px 12px;border-bottom:1px solid var(--border);margin-bottom:8px}.sidebar-meta span{display:flex;align-items:center;gap:7px;color:var(--text-muted);font-size:9px}.sidebar-meta i{width:6px;height:6px;border-radius:50%;background:var(--accent);box-shadow:0 0 7px #8AB4FF66}.sidebar-meta small{font-size:8px;color:var(--text-dim)}.global-search-trigger{display:flex;align-items:center;gap:9px;min-width:220px;padding:6px 8px;border:1px solid var(--border);border-radius:7px;background:var(--bg-card);color:var(--text-muted);font-size:10px;text-align:left}.global-search-trigger>span:first-child{font-size:16px}.global-search-trigger kbd,.search-dialog kbd{margin-left:auto;padding:2px 5px;border:1px solid var(--border);border-radius:4px;color:var(--text-dim);font:9px var(--mono)}.topbar-popover{position:relative}.popover-panel{position:absolute;z-index:80;right:0;top:39px;width:310px;max-width:calc(100vw - 28px);background:var(--bg-card);border:1px solid var(--border);border-radius:10px;box-shadow:0 18px 50px #0009;padding:8px}.popover-heading{display:flex;justify-content:space-between;align-items:center;padding:8px 9px 11px;border-bottom:1px solid var(--border);font-size:11px}.popover-heading a{color:var(--accent);font-size:10px}.popover-empty{padding:22px 10px;text-align:center;color:var(--text-muted);font-size:10px}.notification-item{display:grid;width:100%;gap:4px;padding:10px;border:0;border-bottom:1px solid var(--border);background:transparent;color:var(--text-main);text-align:left}.notification-item.unread{background:#8AB4FF0A}.notification-item strong{font-size:10px}.notification-item span,.notification-item small{font-size:9px;color:var(--text-muted)}.notification-item em{font-size:8px;font-style:normal;text-transform:uppercase;color:var(--text-dim)}.notification-priority.error{color:var(--danger)}.notification-priority.warning{color:var(--warning)}.notification-priority.success{color:var(--accent)}.profile-panel{width:190px;display:grid;gap:2px}.profile-summary{display:grid;padding:8px 9px 11px;border-bottom:1px solid var(--border)}.profile-summary strong{font-size:11px}.profile-summary small{font-size:9px;color:var(--text-muted)}.profile-panel>a,.profile-panel>button{padding:8px 9px;border:0;border-radius:6px;background:transparent;color:var(--text-main);font-size:10px;text-align:left}.profile-panel>a:hover,.profile-panel>button:hover{background:var(--bg-hover)}.search-backdrop{position:fixed;inset:0;z-index:100;display:flex;align-items:flex-start;justify-content:center;padding:13vh 16px 20px;background:#0009;backdrop-filter:blur(3px)}.search-dialog{width:min(620px,100%);max-height:70vh;overflow:hidden;border:1px solid var(--border);border-radius:12px;background:var(--bg-card);box-shadow:0 25px 80px #000b}.search-input-wrap{display:flex;align-items:center;gap:10px;padding:13px 15px;border-bottom:1px solid var(--border);color:var(--text-muted)}.search-input-wrap input{flex:1;min-width:0;border:0;outline:0;background:transparent;color:var(--text-main);font-size:12px}.search-input-wrap kbd{margin-left:0}.search-hint{padding:22px;text-align:center;color:var(--text-muted);font-size:10px}.search-results{max-height:55vh;overflow:auto;padding:6px}.search-group-title{padding:8px 10px 4px;color:var(--text-dim);font-size:9px;text-transform:uppercase;letter-spacing:.08em}.search-results>a{display:flex;align-items:center;gap:12px;padding:10px;border-radius:7px;color:var(--text-main)}.search-results>a:hover{background:var(--bg-hover)}.search-result-type{min-width:90px;color:var(--accent);font-size:9px}.search-result-copy{display:grid;gap:3px;min-width:0;flex:1}.search-result-copy strong{font-size:10px}.search-result-copy small{font-size:9px;color:var(--text-muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
@media(max-width:700px){.global-search-trigger{min-width:0;width:32px;height:30px;justify-content:center;padding:4px}.global-search-trigger>span:nth-child(2),.global-search-trigger kbd{display:none}.topbar-actions{gap:6px}.notification-panel{right:-45px}.search-backdrop{padding-top:8vh}}
.loading-screen{min-height:100vh;display:flex;align-items:center;justify-content:center;gap:12px;background:#090D16;color:#9AA8BD;font-size:12px}.loading-mark{width:30px;height:30px;display:grid;place-items:center;border-radius:9px;background:linear-gradient(145deg,#8AB4FF,#8AB4FF);color:#090D16;font-weight:800}.loading-screen i{width:13px;height:13px;border:2px solid #2B3548;border-top-color:#8AB4FF;border-radius:50%;animation:spin .7s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}
.app-shell{display:flex;min-height:100vh;background:#090D16;align-items:flex-start}.sidebar{width:258px;flex:0 0 258px;height:100vh;height:100dvh;position:sticky;top:0;align-self:flex-start;background:#090D16;border-right:1px solid #2B3548;display:flex;flex-direction:column;padding:20px 13px 13px;min-height:0;overflow:hidden}.brand-lockup{height:40px;display:flex;align-items:center;gap:10px;padding:0 8px;margin-bottom:22px}.brand-mark{width:27px;height:24px;display:block;flex:0 0 27px;color:#8AB4FF;background:transparent;filter:drop-shadow(0 0 7px #8AB4FF33)}.brand-copy{display:flex;align-items:baseline;gap:5px;letter-spacing:.1em}.brand-copy strong{font-size:13px;color:#E9EEF8}.brand-copy small{font-size:9px;color:#6D9EFF;font-weight:800;letter-spacing:.18em}.brand-version{margin-left:auto;border:1px solid #2B3548;color:#9AA8BD;border-radius:5px;padding:2px 5px;font-size:8px;letter-spacing:.08em}
.workspace-switcher{width:100%;display:flex;align-items:center;gap:10px;padding:11px;background:#111827;border:1px solid #2B3548;border-radius:10px;margin:0 0 23px;text-align:left}.workspace-avatar,.account-avatar{width:29px;height:29px;border-radius:8px;display:grid;place-items:center;background:linear-gradient(145deg,#2B3548,#111827);color:#E9EEF8;font-weight:700;font-size:11px;flex-shrink:0}.workspace-copy,.account-copy{display:flex;flex-direction:column;min-width:0;flex:1}.workspace-copy strong,.account-copy strong{font-size:11px;color:#E9EEF8;font-weight:600}.workspace-copy small,.account-copy small{font-size:9px;color:#9AA8BD;margin-top:3px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.switch-chevron{color:#9AA8BD}
.sidebar-nav{overflow-x:hidden;overflow-y:auto;flex:1;min-height:0;scrollbar-width:thin;overscroll-behavior:contain}.nav-group{margin-bottom:21px}.nav-label{padding:0 11px;margin:0 0 8px;color:#65748B;font-size:9px;font-weight:750;letter-spacing:.15em}.nav-item{min-height:36px;padding:8px 11px;border-radius:8px;display:flex;align-items:center;gap:11px;color:#9AA8BD;font-size:11px;font-weight:500;margin:2px 0;transition:background .16s,color .16s,border-color .16s;border:1px solid transparent}.nav-item:hover{background:#2B3548;color:#E9EEF8}.nav-item.active{background:linear-gradient(90deg,#1A2638,#111827);color:#E9EEF8;border-color:#8AB4FF;box-shadow:inset 2px 0 #8AB4FF}.nav-icon{width:16px;text-align:center;font-size:14px;color:#9AA8BD}.nav-item.active .nav-icon{color:#6D9EFF}.nav-tag{margin-left:auto;font-size:8px;color:#8AB4FF;background:#1A2638;border:1px solid #2B3548;border-radius:4px;padding:2px 5px}
.sidebar-bottom{border-top:1px solid #2B3548;padding-top:10px}.account-row{display:flex;align-items:center;gap:9px;padding:12px 5px 2px;margin-top:8px;border-top:1px solid #2B3548}.account-menu{background:transparent;border:0;color:#9AA8BD;font-size:15px;padding:5px}.main-area{min-width:0;flex:1;display:flex;flex-direction:column;min-height:100vh;min-height:100dvh}.topbar{height:58px;flex:0 0 58px;border-bottom:1px solid #2B3548;background:#090D16e8;display:flex;align-items:center;justify-content:space-between;padding:0 30px;backdrop-filter:blur(16px)}.topbar-left,.topbar-actions,.breadcrumbs{display:flex;align-items:center;gap:13px}.breadcrumbs{font-size:11px}.breadcrumbs span{color:#9AA8BD}.breadcrumbs b{color:#2B3548;font-weight:400}.breadcrumbs strong{color:#E9EEF8;font-weight:600}.connection-state{font-size:10px;color:#9AA8BD;display:flex;align-items:center;gap:7px}.connection-state i{width:6px;height:6px;border-radius:50%;background:var(--accent);box-shadow:0 0 9px color-mix(in srgb,var(--accent) 40%,transparent)}.theme-switch{display:flex;align-items:center;gap:6px;border:1px solid #2B3548;background:#111827;color:#E9EEF8;border-radius:7px;padding:6px 9px;font-size:9px;font-weight:650}.theme-switch span{font-size:13px}.mode-switch{display:flex;align-items:center;gap:7px;border:1px solid #2B3548;background:#111827;color:#E9EEF8;border-radius:7px;padding:6px 9px;font-size:9px;font-weight:650}.mode-dot{width:6px;height:6px;border-radius:50%;background:#8AB4FF}.mode-dot-advanced{background:#8AB4FF;box-shadow:0 0 8px #8AB4FF66}.switch-arrows{color:#9AA8BD;font-size:12px}.icon-action,.profile-action{width:29px;height:29px;border:1px solid #2B3548;background:#111827;color:#9AA8BD;border-radius:8px;display:grid;place-items:center;font-size:12px}.profile-action{background:#2B3548;color:#E9EEF8;border-color:#8AB4FF;font-size:10px;font-weight:700}.page-scroll{flex:1;overflow:auto;padding:30px}.page-container{max-width:1460px;margin:0 auto;width:100%}.mobile-menu,.mobile-scrim{display:none}
@media(max-width:900px){.sidebar{width:226px;flex-basis:226px}.page-scroll{padding:22px}}@media(max-width:700px){.sidebar{position:fixed;z-index:50;inset:0 auto 0 0;width:276px;height:100vh;height:100dvh;transform:translateX(-100%);transition:transform .22s;box-shadow:20px 0 60px #00000088}.sidebar.mobile-open{transform:translateX(0)}.mobile-scrim{display:block;position:fixed;inset:0;background:#000000bb;border:0;z-index:40}.mobile-menu{display:grid;place-items:center;width:30px;height:30px;border:1px solid #2B3548;background:#111827;color:#E9EEF8;border-radius:7px}.topbar{padding:0 14px}.page-scroll{padding:19px 14px}.connection-state{display:none}.topbar-actions{gap:7px}}

/* Dokploy-style shell: compact neutral sidebar, restrained active states */
.sidebar{width:256px;flex-basis:256px;padding:12px 10px 10px;background:#090D16;border-right:1px solid #2B3548}
.brand-lockup{height:36px;margin-bottom:16px;padding:0 8px;gap:9px}
.brand-mark{width:28px;height:28px;border-radius:7px;background:#E9EEF8;color:#090D16;box-shadow:none;font-size:14px}
.brand-copy{letter-spacing:.015em;gap:4px}
.brand-copy strong{font-size:14px;letter-spacing:-.025em}
.brand-copy small{font-size:9px;color:#9AA8BD;letter-spacing:.12em}
.brand-version{font-size:9px;border-color:#2B3548;color:#9AA8BD;border-radius:4px}
.workspace-switcher{padding:9px 8px;margin-bottom:14px;background:transparent;border:1px solid #2B3548;border-radius:7px}
.workspace-avatar{border-radius:6px;background:#2B3548}
.workspace-copy strong{font-size:12px;font-weight:500}
.workspace-copy small{font-size:10px}
.nav-group{margin-bottom:17px}
.nav-label{font-size:10px;letter-spacing:.06em;font-weight:500;color:#65748B;padding:0 9px;margin-bottom:5px}
.nav-item{min-height:34px;padding:7px 9px;gap:9px;border-radius:6px;font-size:12px;color:#9AA8BD;margin:1px 0}
.nav-item:hover{background:#111827;color:#E9EEF8}
.nav-item.active{background:#2B3548;color:#E9EEF8;border-color:transparent;box-shadow:none}
.nav-item.active .nav-icon{color:#E9EEF8}
.nav-icon{font-size:14px;color:#9AA8BD}
.nav-tag{background:#2B3548;border-color:#2B3548;color:#d4d4d8}
.sidebar-bottom{border-color:#2B3548;padding-top:8px}
.account-row{border-color:#2B3548;padding-top:10px}
.account-avatar{border-radius:6px;background:#2B3548}
.account-copy strong{font-size:11px;font-weight:500}
.account-copy small{font-size:10px}
.account-menu{border-radius:5px}
.topbar{height:56px;flex-basis:56px;padding:0 24px;background:#090D16;border-color:#2B3548}
.breadcrumbs{font-size:12px;gap:10px}
.breadcrumbs span{color:#9AA8BD}
.breadcrumbs strong{font-weight:500}
.connection-state{font-size:11px}
.theme-switch,.mode-switch,.icon-action,.profile-action{border-radius:6px;background:transparent;border-color:#2B3548}
.mode-switch,.theme-switch{font-size:11px;padding:6px 8px}
.icon-action,.profile-action{width:30px;height:30px}
.page-scroll{padding:24px}
.page-container{max-width:1400px}
.loading-mark{border-radius:7px;background:#E9EEF8;color:#090D16;box-shadow:none}
@media(max-width:900px){.sidebar{width:240px;flex-basis:240px}.page-scroll{padding:20px}}
@media(max-width:700px){.sidebar{width:276px}.topbar{padding:0 14px}.page-scroll{padding:18px 14px}}

.toast-stack{position:fixed;z-index:200;right:22px;bottom:22px;display:grid;gap:8px;width:min(380px,calc(100vw - 28px));pointer-events:none}.toast-item{padding:12px 14px;border:1px solid var(--border);border-left:3px solid var(--accent);border-radius:8px;background:var(--bg-card);color:var(--text-main);box-shadow:0 12px 34px #0006;font-size:11px;line-height:1.5;overflow-wrap:anywhere;animation:toast-in .16s ease-out}.toast-item.toast-error{border-left-color:var(--danger)}.toast-item.toast-warning{border-left-color:var(--warning)}@keyframes toast-in{from{opacity:0;transform:translateY(6px)}to{opacity:1;transform:translateY(0)}}@media(max-width:700px){.toast-stack{right:14px;bottom:14px}}\n</style>