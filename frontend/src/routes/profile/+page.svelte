<script>
 import {user,notify} from '$lib/stores.js';
 import {changePassword} from '$lib/api.js';
 import Button from '$lib/components/Button.svelte';
 let copied=false;
 let currentPassword='',newPassword='',confirmPassword='',passwordBusy=false;
 $: referralLink=typeof window!=='undefined'&&$user?window.location.origin+'/register?ref='+$user.referral_code:'';
 async function copy(value){try{await navigator.clipboard.writeText(value);copied=true;notify('Скопировано','success');setTimeout(()=>copied=false,1800)}catch{notify('Не удалось скопировать','error')}}
 function statusLabel(s){return s==='free'?'Free':s==='commercial'?'Commercial':s==='blocked'?'Blocked':s}
 async function submitPassword(){
  if(newPassword!==confirmPassword){notify('Новые пароли не совпадают','error');return}
  if(newPassword.length<12){notify('Новый пароль должен содержать минимум 12 символов','error');return}
  passwordBusy=true;
  const result=await changePassword(currentPassword,newPassword);
  passwordBusy=false;
  if(result.ok){currentPassword='';newPassword='';confirmPassword='';notify('Пароль изменён. Остальные сессии завершены.','success')}
  else notify(result.error||'Не удалось изменить пароль','error');
 }
</script>
<div class="page-header"><div><div class="eyebrow">ACCOUNT</div><h1 class="page-title">Профиль</h1><p class="page-desc">Данные аккаунта, безопасность и реферальная программа.</p></div></div>
{#if $user}
<section class="panel"><h2>Account details</h2><div class="details"><div><small>Username</small><strong>{$user.username}</strong></div><div><small>Email</small><strong>{$user.email}</strong></div><div><small>Plan status</small><strong>{statusLabel($user.status)}</strong></div><div><small>Device limit</small><strong>{$user.max_devices+$user.bonus_devices} <em>({$user.max_devices} base + {$user.bonus_devices} bonus)</em></strong></div></div></section>
<section class="panel"><h2>Сменить пароль</h2><p>Для подтверждения введите текущий пароль. Новый пароль должен содержать не менее 12 символов. После смены остальные активные сессии будут завершены.</p><form class="password-form" on:submit|preventDefault={submitPassword}><label for="current-password">Текущий пароль</label><input id="current-password" type="password" autocomplete="current-password" bind:value={currentPassword} required/><label for="new-password">Новый пароль</label><input id="new-password" type="password" autocomplete="new-password" minlength="12" maxlength="72" bind:value={newPassword} required/><label for="confirm-password">Повторите новый пароль</label><input id="confirm-password" type="password" autocomplete="new-password" minlength="12" maxlength="72" bind:value={confirmPassword} required/><div class="form-actions"><Button variant="primary" type="submit" loading={passwordBusy}>Сохранить пароль</Button></div></form></section>
<section class="panel"><h2>Referral program</h2><p>Поделитесь ссылкой-приглашением. Реферальные бонусы отображаются в вашем аккаунте.</p><label>Referral link</label><div class="copy-row"><input readonly value={referralLink}/><Button variant="primary" on:click={()=>copy(referralLink)}>{copied?'Скопировано':'Копировать'}</Button></div><label>Agent pairing code</label><div class="copy-row"><input readonly value={$user.referral_code||''}/><Button on:click={()=>copy($user.referral_code||'')}>Копировать</Button></div></section>
{/if}
<style>
.eyebrow{font-size:9px;font-weight:700;letter-spacing:.14em;color:#3b82f6;margin-bottom:7px}.panel{background:#18181b;border:1px solid #27272a;border-radius:12px;padding:18px;margin-bottom:13px}.panel h2{font-size:12px;margin-bottom:15px}.panel p{font-size:10px;color:#a1a1aa;margin-bottom:15px;line-height:1.6}.details{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18px}.details div{display:flex;flex-direction:column;gap:4px}.details small,label{font-size:9px;color:#a1a1aa;text-transform:uppercase;letter-spacing:.08em}.details strong{font-size:11px;font-weight:550}.details em{font-size:9px;color:#71717a;font-style:normal}.copy-row{display:flex;gap:8px;margin:6px 0 14px}.copy-row input,.password-form input{min-width:0;flex:1;background:#09090b;border:1px solid #27272a;border-radius:8px;color:#fafafa;padding:9px 10px;font:10px 'JetBrains Mono',monospace}.password-form{display:grid;gap:8px;max-width:440px}.password-form input{width:100%;font-family:inherit}.form-actions{margin-top:7px;display:flex;justify-content:flex-start}@media(max-width:600px){.details{grid-template-columns:1fr}.copy-row{flex-wrap:wrap}.copy-row input{flex-basis:100%}}
</style>