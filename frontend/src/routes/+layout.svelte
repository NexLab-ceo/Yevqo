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
<svelte:head><meta name="theme-color" content="#080b14"/><title>VELTRIX Dock</title></svelte:head>
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
.loading-screen{min-height:100vh;display:flex;align-items:center;justify-content:center;gap:12px;background:#080b14;color:#94a3b8;font-size:12px}.loading-mark{width:30px;height:30px;display:grid;place-items:center;border-radius:9px;background:linear-gradient(145deg,#818cf8,#4f46e5);color:#fff;font-weight:800}.loading-screen i{width:13px;height:13px;border:2px solid #28334b;border-top-color:#818cf8;border-radius:50%;animation:spin .7s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}
.app-shell{display:flex;min-height:100vh;background:#080b14}.sidebar{width:258px;flex:0 0 258px;background:#0b1020;border-right:1px solid #202b40;display:flex;flex-direction:column;padding:20px 13px 13px;min-height:100vh}.brand-lockup{height:40px;display:flex;align-items:center;gap:10px;padding:0 8px;margin-bottom:22px}.brand-mark{width:31px;height:31px;display:grid;place-items:center;border-radius:10px;background:linear-gradient(145deg,#818cf8,#4f46e5);color:#fff;font-weight:800;font-size:16px;box-shadow:0 5px 22px #4f46e544}.brand-copy{display:flex;align-items:baseline;gap:5px;letter-spacing:.1em}.brand-copy strong{font-size:13px;color:#f1f5f9}.brand-copy small{font-size:9px;color:#a5b4fc;font-weight:800;letter-spacing:.18em}.brand-version{margin-left:auto;border:1px solid #334155;color:#94a3b8;border-radius:5px;padding:2px 5px;font-size:8px;letter-spacing:.08em}
.workspace-switcher{width:100%;display:flex;align-items:center;gap:10px;padding:11px;background:#111a2c;border:1px solid #26344c;border-radius:10px;margin:0 0 23px;text-align:left}.workspace-avatar,.account-avatar{width:29px;height:29px;border-radius:8px;display:grid;place-items:center;background:linear-gradient(145deg,#243450,#17243b);color:#c7d2fe;font-weight:700;font-size:11px;flex-shrink:0}.workspace-copy,.account-copy{display:flex;flex-direction:column;min-width:0;flex:1}.workspace-copy strong,.account-copy strong{font-size:11px;color:#e2e8f0;font-weight:600}.workspace-copy small,.account-copy small{font-size:9px;color:#71809b;margin-top:3px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.switch-chevron{color:#71809b}
.sidebar-nav{overflow:auto;flex:1;scrollbar-width:thin}.nav-group{margin-bottom:21px}.nav-label{padding:0 11px;margin:0 0 8px;color:#63718a;font-size:9px;font-weight:750;letter-spacing:.15em}.nav-item{min-height:36px;padding:8px 11px;border-radius:8px;display:flex;align-items:center;gap:11px;color:#96a3b8;font-size:11px;font-weight:500;margin:2px 0;transition:background .16s,color .16s,border-color .16s;border:1px solid transparent}.nav-item:hover{background:#131f34;color:#e2e8f0}.nav-item.active{background:linear-gradient(90deg,#252b50,#1a2440);color:#e0e7ff;border-color:#343c69;box-shadow:inset 2px 0 #818cf8}.nav-icon{width:16px;text-align:center;font-size:14px;color:#71809b}.nav-item.active .nav-icon{color:#a5b4fc}.nav-tag{margin-left:auto;font-size:8px;color:#8be9d1;background:#12352f;border:1px solid #1c5147;border-radius:4px;padding:2px 5px}
.sidebar-bottom{border-top:1px solid #202b40;padding-top:10px}.account-row{display:flex;align-items:center;gap:9px;padding:12px 5px 2px;margin-top:8px;border-top:1px solid #1c2639}.account-menu{background:transparent;border:0;color:#71809b;font-size:15px;padding:5px}.main-area{min-width:0;flex:1;display:flex;flex-direction:column;min-height:100vh}.topbar{height:58px;flex:0 0 58px;border-bottom:1px solid #202b40;background:#0b1020e8;display:flex;align-items:center;justify-content:space-between;padding:0 30px;backdrop-filter:blur(16px)}.topbar-left,.topbar-actions,.breadcrumbs{display:flex;align-items:center;gap:13px}.breadcrumbs{font-size:11px}.breadcrumbs span{color:#71809b}.breadcrumbs b{color:#34415a;font-weight:400}.breadcrumbs strong{color:#e2e8f0;font-weight:600}.connection-state{font-size:10px;color:#8998b0;display:flex;align-items:center;gap:7px}.connection-state i{width:6px;height:6px;border-radius:50%;background:#34d399;box-shadow:0 0 9px #34d39999}.mode-switch{display:flex;align-items:center;gap:7px;border:1px solid #33415c;background:#111a2c;color:#c7d2fe;border-radius:7px;padding:6px 9px;font-size:9px;font-weight:650}.mode-dot{width:6px;height:6px;border-radius:50%;background:#818cf8}.mode-dot-advanced{background:#34d399;box-shadow:0 0 8px #34d39999}.switch-arrows{color:#71809b;font-size:12px}.icon-action,.profile-action{width:29px;height:29px;border:1px solid #26344c;background:#111a2c;color:#a7b3c8;border-radius:8px;display:grid;place-items:center;font-size:12px}.profile-action{background:#302f70;color:#dbe2ff;border-color:#4c4b91;font-size:10px;font-weight:700}.page-scroll{flex:1;overflow:auto;padding:30px}.page-container{max-width:1460px;margin:0 auto;width:100%}.mobile-menu,.mobile-scrim{display:none}
@media(max-width:900px){.sidebar{width:226px;flex-basis:226px}.page-scroll{padding:22px}}@media(max-width:700px){.sidebar{position:fixed;z-index:50;inset:0 auto 0 0;width:276px;transform:translateX(-100%);transition:transform .22s;box-shadow:20px 0 60px #0008}.sidebar.mobile-open{transform:translateX(0)}.mobile-scrim{display:block;position:fixed;inset:0;background:#020617bb;border:0;z-index:40}.mobile-menu{display:grid;place-items:center;width:30px;height:30px;border:1px solid #26344c;background:#111a2c;color:#cbd5e1;border-radius:7px}.topbar{padding:0 14px}.page-scroll{padding:19px 14px}.connection-state{display:none}.topbar-actions{gap:7px}}
</style>