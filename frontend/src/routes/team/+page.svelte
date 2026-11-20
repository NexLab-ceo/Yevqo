<script>
 import { onMount } from 'svelte';
 import { user } from '$lib/stores.js';
 import { listWorkspaces, listWorkspaceMembers, inviteWorkspaceMember, removeWorkspaceMember, listWorkspaceInvitations, acceptWorkspaceInvitation } from '$lib/api.js';
 import { notify } from '$lib/stores.js';
 import Button from '$lib/components/Button.svelte';

 const roles=[
  {name:'Owner',desc:'Полный контроль workspace и всех ресурсов'},
  {name:'Admin',desc:'Управление устройствами, сайтами, конфигурацией, участниками и аудитом'},
  {name:'Developer',desc:'Создание сайтов, deployments, управление процессами и просмотр логов'}
 ];
 let workspaces=[], selectedOwner='', members=[], invitations=[], loading=true, busy=false;
 $: currentWorkspace=workspaces.find(w=>String(w.owner_user_id)===String(selectedOwner));
 let email='', role='developer';

 onMount(load);
 async function load(){
  loading=true;
  const w=await listWorkspaces();
  if(w.ok){workspaces=w.data.workspaces||[];if(!selectedOwner&&workspaces.length)selectedOwner=String(workspaces[0].owner_user_id);if(!workspaces.some(x=>String(x.owner_user_id)===String(selectedOwner)))selectedOwner=String(workspaces[0]?.owner_user_id||'');}
  else notify(w.error||'Не удалось загрузить рабочие области','error');
  const [m,i]=await Promise.all([selectedOwner?listWorkspaceMembers(selectedOwner):Promise.resolve({ok:true,data:{members:[]}}),listWorkspaceInvitations()]);
  if(m.ok)members=m.data.members||[];else {members=[];notify(m.error||'Управление участниками доступно только Owner/Admin','error');}
  if(i.ok)invitations=i.data.invitations||[];else notify(i.error||'Не удалось загрузить приглашения','error');
  loading=false;
 }
 async function invite(){
  if(!email.trim())return;
  busy=true;const r=await inviteWorkspaceMember(email,role,selectedOwner);busy=false;
  if(!r.ok){notify(r.error||'Не удалось отправить приглашение','error');return}
  email='';notify('Приглашение отправлено','success');await load();
 }
 async function remove(member){
  if(!confirm('Удалить участника из workspace?'))return;
  const r=await removeWorkspaceMember(member.id,selectedOwner);
  if(!r.ok){notify(r.error||'Не удалось удалить участника','error');return}
  notify('Участник удалён','success');await load();
 }
 async function accept(invitation){
  const r=await acceptWorkspaceInvitation(invitation.id);
  if(!r.ok){notify(r.error||'Не удалось принять приглашение','error');return}
  notify('Вы присоединились к workspace','success');await load();
 }
</script>

<div class="page-header">
 <div><div class="eyebrow">WORKSPACE MANAGEMENT</div><h1 class="page-title">Team & Permissions</h1><p class="page-desc">Участники, приглашения и реальные серверные роли доступа.</p></div>
 <Button on:click={load}>↻ Обновить</Button>
</div>

<section class="panel">
 <div class="section-head"><h2>Рабочая область</h2><select class="input workspace-select" bind:value={selectedOwner} on:change={load}>{#each workspaces as w}<option value={String(w.owner_user_id)}>{w.owner_username} · {w.role}</option>{/each}</select></div>
 {#if currentWorkspace}<div class="member owner-row"><span class="avatar">{(currentWorkspace.owner_username||'W').slice(0,1).toUpperCase()}</span><span class="member-main"><strong>{currentWorkspace.owner_username}</strong><small>{currentWorkspace.owner_email}</small></span><span class:owner={currentWorkspace.role==='Owner'} class:admin={currentWorkspace.role==='Admin'} class:developer={currentWorkspace.role==='Developer'} class="role">{currentWorkspace.role}</span></div>{/if}
</section>

{#if currentWorkspace?.can_manage_members}
<section class="panel">
 <h2>Пригласить участника</h2>
 <div class="invite-form">
  <input class="input" type="email" bind:value={email} placeholder="user@example.com" />
  <select class="input" bind:value={role}><option value="admin">Admin</option><option value="developer">Developer</option></select>
  <Button variant="primary" on:click={invite} loading={busy}>Пригласить</Button>
 </div>
 <p class="hint">Пользователь должен сначала зарегистрировать аккаунт NexLab. Приглашение появится у него в Team & Permissions.</p>
</section>
{:else}<section class="panel"><p class="hint">Просмотр и управление участниками доступны Owner и Admin выбранной рабочей области.</p></section>{/if}

<section class="panel">
 <h2>Участники</h2>
 {#if loading}<div class="empty">Загрузка…</div>{:else if !members.length}<div class="empty">Пока нет участников.</div>{:else}
  {#each members as member (member.id)}
   <div class="member-row">
    <div class="member-main"><strong>{member.username}</strong><small>{member.email} · {member.status}</small></div>
    <span class:admin={member.role==='admin'} class:developer={member.role==='developer'} class="role">{member.role}</span>
    {#if currentWorkspace?.can_manage_members}<Button variant="ghost" size="sm" on:click={()=>remove(member)}>Удалить</Button>{/if}
   </div>
  {/each}
 {/if}
</section>

{#if invitations.length}
<section class="panel">
 <h2>Входящие приглашения</h2>
 {#each invitations as invitation (invitation.id)}
  <div class="member-row">
   <div class="member-main"><strong>{invitation.owner_username}</strong><small>{invitation.owner_email} · роль {invitation.role}</small></div>
   <Button variant="primary" size="sm" on:click={()=>accept(invitation)}>Принять</Button>
  </div>
 {/each}
</section>
{/if}

<section class="panel">
 <h2>Ролевая модель</h2>
 {#each roles as item}
  <div class="role-row"><strong>{item.name}</strong><span>{item.desc}</span></div>
 {/each}
</section>

<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:7px}
.panel{padding:16px;background:#101727;border:1px solid #253149;border-radius:11px;margin-bottom:12px}
.panel h2{font-size:12px;margin-bottom:12px}.section-head{display:flex;align-items:center;justify-content:space-between;gap:12px}.workspace-select{max-width:240px}
.member,.member-row{display:flex;align-items:center;gap:10px;padding:9px 0;border-bottom:1px solid #1e293b}
.member-row:last-child{border-bottom:0}
.avatar{width:32px;height:32px;display:grid;place-items:center;border-radius:9px;background:#242b50;color:#c7d2fe;font-size:12px;font-weight:700}
.member-main{display:grid;gap:3px;flex:1;min-width:0}
.member strong,.member-row strong,.role-row strong{font-size:10px}
.member small,.member-row small,.role-row span,.hint{font-size:9px;color:#64748b}
.role{font-size:9px;padding:4px 7px;border-radius:6px;background:#1e293b;color:#cbd5e1}
.owner{background:#25264a;color:#a5b4fc}.admin{background:#183b36;color:#6ee7b7}.developer{background:#26334b;color:#93c5fd}
.invite-form{display:grid;grid-template-columns:minmax(160px,1fr) 130px auto;gap:8px}
.hint{line-height:1.6;margin-top:10px}
.role-row{display:grid;grid-template-columns:100px 1fr;gap:12px;padding:10px 0;border-top:1px solid #253149}
@media(max-width:600px){.invite-form{grid-template-columns:1fr}.member-row{flex-wrap:wrap}}
</style>
