<script>
 const providers=[
 {name:'GitHub',kind:'Git provider',desc:'Repositories, source access and deployment triggers',icon:'GH'},
 {name:'GitLab',kind:'Git provider',desc:'Projects, repositories and CI/CD events',icon:'GL'},
 {name:'Cloudflare',kind:'Networking',desc:'Tunnel lifecycle, DNS and hostname routing',icon:'CF'},
 {name:'Discord',kind:'Notifications',desc:'Operational alerts in a Discord channel',icon:'DC'},
 {name:'Telegram',kind:'Notifications',desc:'Device and deployment alerts through a bot',icon:'TG'},
 {name:'Webhooks',kind:'Automation',desc:'Signed outgoing events for external systems',icon:'WH'}
 ];
</script>
<div class="page-header"><div><div class="eyebrow">CONNECTIVITY</div><h1 class="page-title">Integrations</h1><p class="page-desc">External services connected to your VELTRIX workspace.</p></div></div>
<div class="provider-list">{#each providers as p}<article class="provider"><span class="provider-icon">{p.icon}</span><span class="provider-copy"><strong>{p.name}</strong><small>{p.kind} · {p.desc}</small></span><span class="status">Not connected</span></article>{/each}</div>
<div class="notice">Подключение OAuth и API-токенов будет включено после настройки серверного шифрования секретов, минимальных scopes и журнала отзыва доступа. Пока credentials не принимаются и не сохраняются.</div>
<style>
.eyebrow{font-size:9px;letter-spacing:.14em;color:#818cf8;font-weight:700;margin-bottom:7px}.provider-list{border:1px solid #253149;border-radius:11px;background:#101727;overflow:hidden}.provider{display:flex;align-items:center;gap:11px;padding:12px;border-bottom:1px solid #1e293b}.provider:last-child{border:0}.provider-icon{width:32px;height:32px;display:grid;place-items:center;background:#172136;border-radius:8px;color:#a5b4fc;font-size:10px;font-weight:700}.provider-copy{display:grid;gap:3px;flex:1;min-width:0}.provider-copy strong{font-size:10px}.provider-copy small,.status{font-size:9px;color:#64748b}.status{white-space:nowrap}.notice{margin-top:13px;padding:12px;border:1px solid #334155;border-radius:9px;background:#101727;color:#94a3b8;font-size:10px;line-height:1.6}
</style>