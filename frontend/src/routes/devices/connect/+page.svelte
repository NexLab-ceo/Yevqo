<script>
 import {onMount} from 'svelte';
 import {locale} from '$lib/i18n.js';
 import {listDevices,createDevicePairing} from '$lib/api.js';
 import {notify} from '$lib/stores.js';
 let devices=[],limit=1,loading=true,creating=false,pairing=null;
 $: ru=$locale!=='en';
 async function generateCommand(){
  creating=true;pairing=null;
  const r=await createDevicePairing();
  if(r.ok){pairing=r.data;notify(ru?'Команда установки готова':'Install command is ready','success')}
  else notify(r.error||'Could not create pairing code','error');
  creating=false;
 }
 async function copyCommand(){
  if(!pairing?.command)return;
  try{await navigator.clipboard.writeText(pairing.command);notify(ru?'Команда скопирована':'Command copied','success')}
  catch{notify(ru?'Не удалось скопировать':'Could not copy command','error')}
 }
 onMount(async()=>{const r=await listDevices();if(r.ok){devices=r.data.devices||[];limit=r.data.limit||1}else notify(r.error||'Could not load devices','error');loading=false});
</script>
<svelte:head><title>{ru?'Подключить устройство':'Connect device'} · Yevqo</title></svelte:head>
<div class="page-header"><div><div class="eyebrow">INFRASTRUCTURE / DEVICE PAIRING</div><h1 class="page-title">{ru?'Подключить устройство':'Connect a device'}</h1><p class="page-desc">{ru?'Одна команда установит Yevqo Agent и безопасно подключит Android-устройство.':'One command installs Yevqo Agent and securely connects your Android device.'}</p></div><a class="back" href="/devices">← {ru?'Устройства':'Devices'}</a></div>
{#if loading}<div class="empty">{ru?'Проверяем лимит…':'Checking device allowance…'}</div>{:else}
<section class="connect-card"><div class="step-head"><span class="step-number">01</span><div><h2>{ru?'Лимит устройств':'Device allowance'}</h2><p>{ru?'Текущие подключения в твоём тарифе':'Current connected devices on your plan'}</p></div></div><div class="limit-line"><strong>{devices.length} / {limit}</strong><span>{ru?'устройств':'devices'}</span></div>{#if devices.length>=limit}<div class="warning">{ru?'Лимит достигнут. Отключи устройство или измени тариф.':'Device limit reached. Disconnect a device or change your plan.'}</div>{/if}</section>
<section class="connect-card"><div class="step-head"><span class="step-number">02</span><div><h2>{ru?'Создай команду подключения':'Generate install command'}</h2><p>{ru?'Код будет действовать 10 минут и сработает только один раз.':'The embedded code expires in 10 minutes and can only be used once.'}</p></div></div>
<button class="primary" on:click={generateCommand} disabled={creating||devices.length>=limit}>{creating?(ru?'Создаём…':'Generating…'):(ru?'Сгенерировать команду':'Generate command')}</button>
{#if pairing}<div class="command"><code>{pairing.command}</code><button on:click={copyCommand}>{ru?'Копировать':'Copy'}</button></div><p class="hint">{ru?'Вставь команду в Termux на Android. Не публикуй её и не пересылай другим: внутри находится одноразовый код.':'Paste this command into Termux on Android. Do not share it: it contains your one-time pairing code.'}</p><p class="expiry">{ru?'Код истекает: ':'Code expires: '}{new Date(pairing.expires_at).toLocaleTimeString()}</p>{/if}
</section>
<section class="connect-card"><div class="step-head"><span class="step-number">03</span><div><h2>{ru?'Автоматическая установка':'Automatic installation'}</h2><p>{ru?'Установщик проверит пакет и настроит Agent без ручного ввода URL или постоянного токена.':'The installer verifies the package and configures the Agent without asking for a server URL or reusable token.'}</p></div></div><ol><li>{ru?'Открой Termux на Android и вставь команду.':'Open Termux on Android and paste the command.'}</li><li>{ru?'Yevqo установит зависимости и подходящий ARM64 .deb.':'Yevqo installs dependencies and the ARM64 .deb package.'}</li><li>{ru?'Код обменяется на отдельный credential устройства.':'The code is exchanged for a device-specific credential.'}</li><li>{ru?'Agent запустится и появится в списке устройств после успешной авторизации.':'The Agent starts and appears in Devices after successful authentication.'}</li></ol></section>
<section class="connect-card"><div class="step-head"><span class="step-number">04</span><div><h2>{ru?'Проверка':'Verification'}</h2><p>{ru?'Подключение подтверждается сервером, а не самим установщиком.':'The server, not the installer, confirms the connection.'}</p></div></div><a class="profile-link" href="/devices">{ru?'Открыть список устройств':'Open devices'} →</a></section>
{/if}
<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:var(--accent);font-weight:700;margin-bottom:7px}.back{font-size:10px;color:var(--text-muted)}.connect-card{padding:17px;margin-bottom:12px;border:1px solid var(--border);border-radius:10px;background:var(--bg-card)}.step-head{display:flex;align-items:flex-start;gap:11px;margin-bottom:13px}.step-number{width:28px;height:28px;display:grid;place-items:center;border:1px solid var(--border);border-radius:7px;color:var(--accent);font:11px var(--mono)}.step-head h2{font-size:12px;margin-bottom:4px}.step-head p,.hint{font-size:10px;color:var(--text-muted);line-height:1.65}.limit-line{display:flex;align-items:baseline;gap:8px}.limit-line strong{font-size:22px}.limit-line span{font-size:10px;color:var(--text-muted)}.warning{margin-top:10px;padding:10px;border:1px solid #E9B85C55;border-radius:7px;color:var(--warning);font-size:10px}.connect-card ol{padding-left:39px}.connect-card li{padding:4px 0;color:var(--text-muted);font-size:10px;line-height:1.6}.command{display:flex;align-items:center;gap:10px;margin-top:14px;padding:10px;border:1px solid var(--border);border-radius:7px;background:var(--bg-input);overflow-wrap:anywhere}.command code{font:10px var(--mono);color:var(--text-main);overflow-wrap:anywhere;word-break:break-all}.command button{flex-shrink:0;margin-left:auto;padding:6px 9px;border:1px solid var(--border);border-radius:6px;background:var(--bg-elevated);color:var(--accent);font-size:9px}.hint{margin-top:10px}.expiry{margin-top:8px;color:var(--text-muted);font-size:10px}.profile-link{display:inline-block;margin-top:10px;color:var(--accent);font-size:10px}
</style>
