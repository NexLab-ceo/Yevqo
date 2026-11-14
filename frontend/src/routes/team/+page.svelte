<script>
 import {user} from '$lib/stores.js';
 const roles=[
 {name:'Owner',desc:'Полный контроль workspace, billing и удаления ресурсов'},
 {name:'Administrator',desc:'Управление проектами, устройствами, сайтами и настройками'},
 {name:'Developer',desc:'Развёртывание и управление назначенными сайтами'},
 {name:'Moderator',desc:'Просмотр активности, логов и очереди модерации'},
 {name:'Viewer',desc:'Только просмотр разрешённых ресурсов'}
 ];
</script>
<div class="page-header"><div><div class="eyebrow">WORKSPACE MANAGEMENT</div><h1 class="page-title">Team & Permissions</h1><p class="page-desc">Роли и границы доступа в VELTRIX workspace.</p></div></div>
<section class="panel"><h2>Current workspace owner</h2><div class="member"><span class="avatar">{($user?.username||'U').slice(0,1).toUpperCase()}</span><span><strong>{$user?.username}</strong><small>{$user?.email}</small></span><span class="role owner">Owner</span></div></section>
<section class="panel"><h2>Role model</h2>{#each roles as role}<div class="role-row"><strong>{role.name}</strong><span>{role.desc}</span></div>{/each}</section>
<div class="notice">Приглашения и совместный доступ пока отключены: сначала нужно завершить workspace membership и проверку ролей на каждом API-ресурсе. Нельзя выдавать доступ только на уровне интерфейса.</div>
<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:7px}.panel{padding:16px;background:#101727;border:1px solid #253149;border-radius:11px;margin-bottom:12px}.panel h2{font-size:12px;margin-bottom:12px}.member{display:flex;align-items:center;gap:10px}.avatar{width:32px;height:32px;display:grid;place-items:center;border-radius:9px;background:#242b50;color:#c7d2fe;font-size:12px;font-weight:700}.member>span:nth-child(2){display:grid;gap:3px;flex:1}.member strong,.role-row strong{font-size:10px}.member small,.role-row span{font-size:9px;color:#64748b}.role{font-size:9px;padding:4px 7px;border-radius:6px}.owner{background:#25264a;color:#a5b4fc}.role-row{display:grid;grid-template-columns:130px 1fr;gap:12px;padding:10px 0;border-top:1px solid #253149}.notice{padding:12px;border:1px solid #334155;border-radius:9px;background:#101727;color:#94a3b8;font-size:10px;line-height:1.6}
</style>