<script>
    import { onMount } from 'svelte';
    import { user } from '$lib/stores.js';
    import { notify } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';

    let copied = false;

    // Реферальная ссылка строится из текущего origin + код.
    $: referralLink = $user ? `${window.location.origin}/register?ref=${$user.referral_code}` : '';

    async function copyLink() {
        try {
            await navigator.clipboard.writeText(referralLink);
            copied = true;
            notify('Ссылка скопирована', 'success');
            setTimeout(() => copied = false, 2000);
        } catch {
            notify('Не удалось скопировать', 'error');
        }
    }

    function statusLabel(status) {
        if (status === 'free') return 'Бесплатный';
        if (status === 'commercial') return 'Коммерческий';
        if (status === 'blocked') return 'Заблокирован';
        return status;
    }

    function statusClass(status) {
        if (status === 'commercial') return 'status-commercial';
        if (status === 'blocked') return 'status-blocked';
        return 'status-free';
    }
</script>

<div class="page-header">
    <div>
        <h1 class="page-title">Профиль</h1>
        <p class="page-desc">Аккаунт и реферальная программа</p>
    </div>
</div>

{#if $user}
    <!-- Аккаунт -->
    <Card padding="md">
        <div class="section-title">Аккаунт</div>
        <div class="info-grid">
            <div class="info-item">
                <div class="info-label">Логин</div>
                <div class="info-value">{$user.username}</div>
            </div>
            <div class="info-item">
                <div class="info-label">Email</div>
                <div class="info-value">{$user.email}</div>
            </div>
            <div class="info-item">
                <div class="info-label">Статус</div>
                <div class="info-value">
                    <span class="user-status {statusClass($user.status)}">
                        {statusLabel($user.status)}
                    </span>
                </div>
            </div>
            <div class="info-item">
                <div class="info-label">Лимит устройств</div>
                <div class="info-value mono">
                    {$user.max_devices + $user.bonus_devices}
                    <span class="muted">
                        ({$user.max_devices} базовых + {$user.bonus_devices} бонусных)
                    </span>
                </div>
            </div>
        </div>
    </Card>

    <!-- Реферальная программа -->
    <div class="section-header">
        <h2 class="section-title">Реферальная программа</h2>
    </div>

    <Card padding="md">
        <p class="info-text">
            Пригласите друга по вашей ссылке — вы получите +1 устройство,
            а он сможет подключить свой телефон к панели.
        </p>

        <div class="form-group">
            <label class="form-label" for="referral-link">Ваша реферальная ссылка</label>
            <div class="link-row">
                <input
                    id="referral-link"
                    class="input mono"
                    type="text"
                    value={referralLink}
                    readonly
                />
                <Button variant="primary" on:click={copyLink}>
                    {copied ? 'Скопировано' : 'Копировать'}
                </Button>
            </div>
        </div>

        <div class="form-group">
            <label class="form-label" for="referral-code">Ваш код привязки (для агента)</label>
            <div class="link-row">
                <input
                    id="referral-code"
                    class="input mono"
                    type="text"
                    value={$user.referral_code}
                    readonly
                />
                <Button on:click={() => navigator.clipboard.writeText($user.referral_code)}>
                    Копировать
                </Button>
            </div>
        </div>

        <p class="info-hint">
            Этот код вы вводите при установке агента на телефон (вместо реферальной ссылки).
        </p>
    </Card>

    <!-- Как это работает -->
    <Card padding="md" class="info-card">
        <div class="section-title">Как пригласить друга</div>
        <ol class="steps">
            <li>Скопируйте реферальную ссылку выше.</li>
            <li>Отправьте её другу (в мессенджере, по почте).</li>
            <li>Друг открывает ссылку, регистрируется — код подставляется автоматически.</li>
            <li>Вы получаете +1 бонусное устройство.</li>
        </ol>
    </Card>
{/if}

<style>
    .section-title {
        font-size: 13px;
        font-weight: 600;
        color: #fafafa;
        margin-bottom: 16px;
    }

    .section-header {
        margin: 32px 0 12px;
    }

    .info-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
        gap: 16px;
    }

    .info-item {
        min-width: 0;
    }

    .info-label {
        font-size: 11px;
        color: #71717a;
        text-transform: uppercase;
        letter-spacing: 0.04em;
        margin-bottom: 4px;
    }

    .info-value {
        font-size: 13px;
        color: #fafafa;
        word-break: break-word;
    }

    .info-value.mono {
        font-family: ui-monospace, monospace;
        font-size: 12px;
    }

    .muted {
        color: #71717a;
        font-size: 11px;
    }

    .user-status {
        display: inline-block;
        font-size: 10px;
        padding: 2px 6px;
        border-radius: 4px;
        font-weight: 500;
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }

    .status-free {
        background: #18181b;
        color: #a1a1aa;
    }

    .status-commercial {
        background: rgba(34, 197, 94, 0.1);
        color: #4ade80;
    }

    .status-blocked {
        background: rgba(239, 68, 68, 0.1);
        color: #f87171;
    }

    .info-text {
        font-size: 13px;
        color: #a1a1aa;
        line-height: 1.6;
        margin-bottom: 20px;
    }

    .info-hint {
        font-size: 11px;
        color: #52525b;
        margin-top: 4px;
    }

    .link-row {
        display: flex;
        gap: 8px;
    }

    .link-row .input {
        flex: 1;
        font-family: ui-monospace, monospace;
        font-size: 12px;
    }

    .info-card {
        margin-top: 24px;
    }

    .steps {
        font-size: 13px;
        color: #a1a1aa;
        padding-left: 20px;
        line-height: 1.8;
    }

    .steps li {
        margin-bottom: 4px;
    }
</style>
