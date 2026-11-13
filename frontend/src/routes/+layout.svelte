<script>
 import '../app.css';
 import {onMount} from 'svelte';
 import {page} from '$app/stores';
 import {goto} from '$app/navigation';
 import {user,isAuthenticated,uiMode} from '$lib/stores.js';
 import {logout,me,getSettings,saveSettings} from '$lib/api.js';
 import {locale} from '$lib/i18n.js';
 let loading=true,mobileNavOpen=false,modeSaving=false;
 const groups=[
  {label:'WORKSPACE',items:[['Overview','/','◫'],['Projects','/projects','▦']]},
  {label:'INFRASTRUCTURE',items:[['Devices','/devices','▣'],['Websites','/sites','◉'],['Databases','/databases','◈'],['Deployments','/deployments','⇧'],['Domains','/domains','◎']]},
  {label:'OPERATIONS',items:[['Monitoring','/monitoring','⌁'],['Logs','/logs','≡'],['Notifications','/notifications','♧'],['Reports & Export','/reports','▤']]},
  {label:'MANAGEMENT',items:[['Team & Permissions','/team','♙'],['Integrations','/integrations','⌘'],['Backup & Restore','/backups','⤓'],['Maintenance','/maintenance','⚙']]}
 ];
 onMount(async()=>{const [a,b]=await Promise.all([me(),getSettings()]);if(a.ok&&a.data?.user)user.set(a.data.user);if(b.ok){uiMode.set(b.data.settings?.mode||'simple');locale.set(b.data.settings?.locale||'ru')}loading=false});
 $: if(!loading&&!$isAuthenticated&&!$page.url.pathname.startsWith('/login')&&!$page.url.pathname.startsWith('/register'))goto('/login');
 $: currentTitle=groups.flatMap(g=>g.items).find(i=>$page.url.pathname===i[1]||($page.url.pathname.startsWith(i[1]+'/')&&i[1]!=='/'))?.[0]||'Overview';
 function active(href){return href==='/'?$page.url.pathname==='/':$page.url.pathname===href||$page.url.pathname.startsWith(href+'/')}
 async function toggleMode(){if(modeSaving)return;modeSaving=true;const next=$uiMode==='simple'?'advanced':'simple';const r=await saveSettings({mode:next});if(r.ok)uiMode.set(next);modeSaving=false}
 async function handleLogout(){await logout();user.set(null);goto('/login')}
</script>
<svelte:head><meta name="theme-color" content="#09090B"/><title>VELTRIX Dock</title></svelte:head>
{#if loading}<div class="loading-screen"><span class="loading-mark">V</span><span>VELTRIX Dock</span><i></i></div>
{:else if !$isAuthenticated}<slot/>
{:else}
<div class="app-shell" data-mode={$uiMode}>
 {#if mobileNavOpen}<button class="mobile-scrim" aria-label="Закрыть меню" on:click={()=>mobileNavOpen=false}></button>{/if}
 <aside class:mobile-open={mobileNavOpen} class="sidebar">
  <a class="brand-lockup" href="/"><span class="brand-mark">V</span><span class="brand-copy"><strong>VELTRIX</strong><small>DOCK</small></span><span class="brand-version">BETA</span></a>
  <button class="workspace-switcher" on:click={()=>goto('/settings')}><span class="workspace-avatar">N</span><span class="workspace-copy"><strong>Personal workspace</strong><small>Free plan · Local host</small></span><span class="switch-chevron">⌄</span></button>
  <nav class="sidebar-nav">
   {#each groups as group}
    <section class="nav-group"><div class="nav-label">{group.label}</div>
     {#each group.items as item}
      <a href={item[1]} class:active={active(item[1])} class="nav-item" on:click={()=>mobileNavOpen=false}><span class="nav-icon">{item[2]}</span><span>{item[0]}</span>{#if item[0]==='Databases'}<span class="nav-tag">DB</span>{/if}</a>
     {/each}
    </section>
   {/each}
   {#if $user?.is_admin===true}<section class="nav-group"><div class="nav-label">ADMINISTRATION</div><a href="/admin" class:active={active('/admin')} class="nav-item"><span class="nav-icon">♜</span><span>Admin panel</span></a></section>{/if}
  </nav>
  <div class="sidebar-bottom">
   <a href="/settings" class:active={active('/settings')} class="nav-item"><span class="nav-icon">⚙</span><span>Settings</span></a>
   <div class="account-row"><span class="account-avatar">{($user?.username||'U').slice(0,1).toUpperCase()}</span><span class="account-copy"><strong>{$user?.username||'User'}</strong><small>{$user?.email||'Personal account'}</small></span><button class="account-menu" title="Выйти" aria-label="Выйти" on:click={handleLogout}>↗</button></div>
  </div>
 </aside>
 <main class="main-area">
  <header class="topbar"><div class="topbar-left"><button class="mobile-menu" aria-label="Открыть меню" on:click={()=>mobileNavOpen=true}>☰</button><div class="breadcrumbs"><span>Workspace</span><b>/</b><strong>{currentTitle}</strong></div></div>
   <div class="topbar-actions"><span class="connection-state"><i></i> Control plane</span><button class="mode-switch" on:click={toggleMode} disabled={modeSaving}><span class:mode-dot-advanced={$uiMode==='advanced'} class="mode-dot"></span>{$uiMode==='advanced'?'Advanced':'Simple'}<span class="switch-arrows">⇄</span></button><button class="icon-action" aria-label="Уведомления" on:click={()=>goto('/notifications')}>♧</button><a class="profile-action" href="/profile">{($user?.username||'U').slice(0,1).toUpperCase()}</a></div>
  </header>
  <div class="page-scroll"><div class="page-container"><slot/></div></div>
 </main>
</div>
{/if}
<style>
.loading-screen{min-height:100vh;display:flex;align-items:center;justify-content:center;gap:12px;background:#09090B;color:#A1A1AA;font-size:12px}.loading-mark{width:30px;height:30px;display:grid;place-items:center;border-radius:9px;background:linear-gradient(145deg,#3B82F6,#3B82F6);color:#09090B;font-weight:800}.loading-screen i{width:13px;height:13px;border:2px solid #27272A;border-top-color:#3B82F6;border-radius:50%;animation:spin .7s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}
.app-shell{display:flex;min-height:100vh;background:#09090B}.sidebar{width:258px;flex:0 0 258px;background:#09090B;border-right:1px solid #27272A;display:flex;flex-direction:column;padding:20px 13px 13px;min-height:100vh}.brand-lockup{height:40px;display:flex;align-items:center;gap:10px;padding:0 8px;margin-bottom:22px}.brand-mark{width:31px;height:31px;display:grid;place-items:center;border-radius:10px;background:linear-gradient(145deg,#3B82F6,#3B82F6);color:#09090B;font-weight:800;font-size:16px;box-shadow:0 5px 22px #3B82F644}.brand-copy{display:flex;align-items:baseline;gap:5px;letter-spacing:.1em}.brand-copy strong{font-size:13px;color:#FAFAFA}.brand-copy small{font-size:9px;color:#60A5FA;font-weight:800;letter-spacing:.18em}.brand-version{margin-left:auto;border:1px solid #334155;color:#A1A1AA;border-radius:5px;padding:2px 5px;font-size:8px;letter-spacing:.08em}
.workspace-switcher{width:100%;display:flex;align-items:center;gap:10px;padding:11px;background:#18181B;border:1px solid #27272A;border-radius:10px;margin:0 0 23px;text-align:left}.workspace-avatar,.account-avatar{width:29px;height:29px;border-radius:8px;display:grid;place-items:center;background:linear-gradient(145deg,#27272A,#18181B);color:#FAFAFA;font-weight:700;font-size:11px;flex-shrink:0}.workspace-copy,.account-copy{display:flex;flex-direction:column;min-width:0;flex:1}.workspace-copy strong,.account-copy strong{font-size:11px;color:#FAFAFA;font-weight:600}.workspace-copy small,.account-copy small{font-size:9px;color:#A1A1AA;margin-top:3px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.switch-chevron{color:#A1A1AA}
.sidebar-nav{overflow:auto;flex:1;scrollbar-width:thin}.nav-group{margin-bottom:21px}.nav-label{padding:0 11px;margin:0 0 8px;color:#71717A;font-size:9px;font-weight:750;letter-spacing:.15em}.nav-item{min-height:36px;padding:8px 11px;border-radius:8px;display:flex;align-items:center;gap:11px;color:#A1A1AA;font-size:11px;font-weight:500;margin:2px 0;transition:background .16s,color .16s,border-color .16s;border:1px solid transparent}.nav-item:hover{background:#27272A;color:#FAFAFA}.nav-item.active{background:linear-gradient(90deg,#1e3a5f,#18181B);color:#FAFAFA;border-color:#3B82F6;box-shadow:inset 2px 0 #3B82F6}.nav-icon{width:16px;text-align:center;font-size:14px;color:#A1A1AA}.nav-item.active .nav-icon{color:#60A5FA}.nav-tag{margin-left:auto;font-size:8px;color:#22C55E;background:#14532D;border:1px solid #166534;border-radius:4px;padding:2px 5px}
.sidebar-bottom{border-top:1px solid #27272A;padding-top:10px}.account-row{display:flex;align-items:center;gap:9px;padding:12px 5px 2px;margin-top:8px;border-top:1px solid #27272A}.account-menu{background:transparent;border:0;color:#A1A1AA;font-size:15px;padding:5px}.main-area{min-width:0;flex:1;display:flex;flex-direction:column;min-height:100vh}.topbar{height:58px;flex:0 0 58px;border-bottom:1px solid #27272A;background:#09090Be8;display:flex;align-items:center;justify-content:space-between;padding:0 30px;backdrop-filter:blur(16px)}.topbar-left,.topbar-actions,.breadcrumbs{display:flex;align-items:center;gap:13px}.breadcrumbs{font-size:11px}.breadcrumbs span{color:#A1A1AA}.breadcrumbs b{color:#27272A;font-weight:400}.breadcrumbs strong{color:#FAFAFA;font-weight:600}.connection-state{font-size:10px;color:#A1A1AA;display:flex;align-items:center;gap:7px}.connection-state i{width:6px;height:6px;border-radius:50%;background:#22C55E;box-shadow:0 0 9px #22C55E99}.mode-switch{display:flex;align-items:center;gap:7px;border:1px solid #27272A;background:#18181B;color:#FAFAFA;border-radius:7px;padding:6px 9px;font-size:9px;font-weight:650}.mode-dot{width:6px;height:6px;border-radius:50%;background:#3B82F6}.mode-dot-advanced{background:#22C55E;box-shadow:0 0 8px #22C55E99}.switch-arrows{color:#A1A1AA;font-size:12px}.icon-action,.profile-action{width:29px;height:29px;border:1px solid #27272A;background:#18181B;color:#A1A1AA;border-radius:8px;display:grid;place-items:center;font-size:12px}.profile-action{background:#27272A;color:#FAFAFA;border-color:#3B82F6;font-size:10px;font-weight:700}.page-scroll{flex:1;overflow:auto;padding:30px}.page-container{max-width:1460px;margin:0 auto;width:100%}.mobile-menu,.mobile-scrim{display:none}
@media(max-width:900px){.sidebar{width:226px;flex-basis:226px}.page-scroll{padding:22px}}@media(max-width:700px){.sidebar{position:fixed;z-index:50;inset:0 auto 0 0;width:276px;transform:translateX(-100%);transition:transform .22s;box-shadow:20px 0 60px #00000088}.sidebar.mobile-open{transform:translateX(0)}.mobile-scrim{display:block;position:fixed;inset:0;background:#000000bb;border:0;z-index:40}.mobile-menu{display:grid;place-items:center;width:30px;height:30px;border:1px solid #27272A;background:#18181B;color:#FAFAFA;border-radius:7px}.topbar{padding:0 14px}.page-scroll{padding:19px 14px}.connection-state{display:none}.topbar-actions{gap:7px}}
</style>