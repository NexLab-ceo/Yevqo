<script>
 import {user,notify} from '$lib/stores.js';import Button from '$lib/components/Button.svelte';
 let copied=false;
 $: referralLink=typeof window!=='undefined'&&$user?window.location.origin+'/register?ref='+$user.referral_code:'';
 async function copy(value){try{await navigator.clipboard.writeText(value);copied=true;notify('Скопировано','success');setTimeout(()=>copied=false,1800)}catch{notify('Не удалось скопировать','error')}}
 function statusLabel(s){return s==='free'?'Free':s==='commercial'?'Commercial':s==='blocked'?'Blocked':s}
</script>
<div class="page-header"><div><div class="eyebrow">ACCOUNT</div><h1 class="page-title">Профиль</h1><p class="page-desc">Данные аккаунта и реферальная программа.</p></div></div>
{#if $user}
<section class="panel"><h2>Account details</h2><div class="details"><div><small>Username</small><strong>{$user.username}</strong></div><div><small>Email</small><strong>{$user.email}</strong></div><div><small>Plan status</small><strong>{statusLabel($user.status)}</strong></div><div><small>Device limit</small><strong>{$user.max_devices+$user.bonus_devices} <em>({$user.max_devices} base + {$user.bonus_devices} bonus)</em></strong></div></div></section>
<section class="panel"><h2>Referral program</h2><p>Поделитесь ссылкой-приглашением. Реферальные бонусы отображаются в вашем аккаунте.</p><label>Referral link</label><div class="copy-row"><input readonly value={referralLink}/><Button variant="primary" on:click={()=>copy(referralLink)}>{copied?'Скопировано':'Копировать'}</Button></div><label>Agent pairing code</label><div class="copy-row"><input readonly value={$user.referral_code||''}/><Button on:click={()=>copy($user.referral_code||'')}>Копировать</Button></div></section>
{/if}
<style>
.eyebrow{font-size:9px;font-weight:700;letter-spacing:.14em;color:#818cf8;margin-bottom:7px}.panel{background:#101727;border:1px solid #253149;border-radius:12px;padding:18px;margin-bottom:13px}.panel h2{font-size:12px;margin-bottom:15px}.panel p{font-size:10px;color:#94a3b8;margin-bottom:15px}.details{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18px}.details div{display:flex;flex-direction:column;gap:4px}.details small,label{font-size:9px;color:#64748b;text-transform:uppercase;letter-spacing:.08em}.details strong{font-size:11px;font-weight:550}.details em{font-size:9px;color:#64748b;font-style:normal}.copy-row{display:flex;gap:8px;margin:6px 0 14px}.copy-row input{min-width:0;flex:1;background:#080b14;border:1px solid #253149;border-radius:8px;color:#cbd5e1;padding:8px 10px;font:10px 'JetBrains Mono',monospace}@media(max-width:600px){.details{grid-template-columns:1fr}.copy-row{flex-wrap:wrap}.copy-row input{flex-basis:100%}}
</style>