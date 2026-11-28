<script>
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { me, adminGetUser, adminBlockUser, adminUnblockUser, adminSetUserPlan } from '$lib/api.js';
    import { notify } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';
    import Card from '$lib/components/Card.svelte';
    import StatusDot from '$lib/components/StatusDot.svelte';
    import Modal from '$lib/components/Modal.svelte';

    let user = null;
    let devices = [];
    let loading = true;
    let showConfirm = false;
    let confirmAction = 'block'; // 'block' | 'unblock'
    let busy = false;
    let adminError = null;

    function reportAdminError(title, res) {
        adminError = { title, message: res?.error || 'Причина не классифицирована сервером.', operation: res?.operation || '', status: res?.status ?? null, code: res?.code || '', requestId: res?.requestId || '' };
        notify(adminError.message, 'error');
    }

    onMount(async () => {
        const session = await me();
        if (!session.ok || !session.data?.user?.is_admin) {
            goto('/overview');
            return;
        }
        await loadUser();
        loading = false;
    });

    async function loadUser() {
        const id = $page.params.id;
        const res = await adminGetUser(id);
        if (!res.ok) {
            reportAdminError('Не удалось загрузить пользователя', res);
            goto('/admin');
            return;
        }
        user = res.data.user;
        devices = res.data.devices || [];
    }

    function askBlock(action) {
        confirmAction = action;
        showConfirm = true;
    }

    async function confirm() {
        busy = true;
        const fn = confirmAction === 'block' ? adminBlockUser : adminUnblockUser;
        const res = await fn(user.id);
        busy = false;

        if (res.ok) {
            notify(confirmAction === 'block' ? 'Пользователь заблокирован' : 'Пользователь разблокирован', 'success');
            showConfirm = false;
            await loadUser();
        } else {
            reportAdminError('Не удалось изменить статус пользователя', res);
        }
    }

    async function setPlan(plan) {
        busy = true;
        const res = await adminSetUserPlan(user.id, plan);
        busy = false;
        if (res.ok) { notify('Тариф ' + plan.toUpperCase() + ' назначен', 'success'); await loadUser(); }
        else reportAdminError('Не удалось изменить тариф пользователя', res);
    }

    function statusLabel(status) {
        if (status === 'free') return 'Free';
        if (status === 'pro') return 'Pro';
        if (status === 'advanced') return 'Advanced';
        if (status === 'commercial') return 'Коммерческий';
        if (status === 'blocked') return 'Заблокирован';
        return status;
    }

    function statusClass(status) {
        if (status === 'pro') return 'status-pro';
        if (status === 'advanced') return 'status-advanced';
        if (status === 'commercial') return 'status-commercial';
        if (status === 'blocked') return 'status-blocked';
        return 'status-free';
    }
</script>

{#if adminError}
    <section class="admin-error" role="alert">
        <strong>{adminError.title}</strong><p>{adminError.message}</p>
        <dl>{#if adminError.operation}<div><dt>Операция</dt><dd><code>{adminError.operation}</code></dd></div>{/if}{#if adminError.status !== null}<div><dt>HTTP</dt><dd>{adminError.status}</dd></div>{/if}{#if adminError.code}<div><dt>Код</dt><dd><code>{adminError.code}</code></dd></div>{/if}{#if adminError.requestId}<div><dt>ID диагностики</dt><dd><code>{adminError.requestId}</code></dd></div>{/if}</dl>
        <small>Секреты, токены и приватные данные не отображаются.</small>
    </section>
{/if}
{#if loading}
    <div class="empty">Загрузка...</div>
{:else if user}
    <!-- Header -->
    <div class="page-header">
        <div>
            <h1 class="page-title">
                {user.username}
                <span class="user-status {statusClass(user.status)}">
                    {statusLabel(user.status)}
                </span>
            </h1>
            <p class="page-desc">{user.email} · ID {user.id}</p>
        </div>
        <div class="header-actions">
            {#if !user.is_admin && user.status !== 'blocked'}
                {#each ['free','pro','advanced'] as plan}
                    {#if user.status !== plan}
                        <Button variant="default" size="sm" on:click={() => setPlan(plan)} disabled={busy}>Назначить {plan === 'free' ? 'Free' : plan === 'pro' ? 'Pro' : 'Advanced'}</Button>
                    {/if}
                {/each}
            {/if}
            {#if user.status === 'blocked'}
                <Button variant="primary" size="sm" on:click={() => askBlock('unblock')} disabled={busy}>
                    Разблокировать
                </Button>
            {:else}
                <Button variant="danger" size="sm" on:click={() => askBlock('block')} disabled={busy}>
                    Заблокировать
                </Button>
            {/if}
        </div>
    </div>

    <!-- Инфо -->
    <Card padding="md">
        <div class="info-grid">
            <div class="info-item">
                <div class="info-label">Реферальный код</div>
                <div class="info-value mono">{user.referral_code}</div>
            </div>
            <div class="info-item">
                <div class="info-label">Приглашён</div>
                <div class="info-value mono">{user.referred_by || '—'}</div>
            </div>
            <div class="info-item">
                <div class="info-label">Лимит устройств</div>
                <div class="info-value mono">{user.is_admin || user.status === 'pro' || user.status === 'advanced' ? 'Без ограничений' : `${user.max_devices + user.bonus_devices} (${user.max_devices} базовых + ${user.bonus_devices} бонусных)`}</div>
            </div>
            <div class="info-item">
                <div class="info-label">Создан</div>
                <div class="info-value mono">{new Date(user.created_at).toLocaleString()}</div>
            </div>
            <div class="info-item">
                <div class="info-label">Последний вход</div>
                <div class="info-value mono">{user.last_seen ? new Date(user.last_seen).toLocaleString() : '—'}</div>
            </div>
        </div>
    </Card>

    <!-- Устройства -->
    <div class="section-header">
        <h2 class="section-title">Устройства</h2>
        <span class="section-count">{devices.length}</span>
    </div>

    {#if devices.length === 0}
        <div class="empty">
            <p>Пользователь ещё не подключил устройств</p>
        </div>
    {:else}
        <div class="devices-list">
            {#each devices as device (device.id)}
                <Card padding="sm">
                    <div class="device-row">
                        <div class="device-info">
                            <div class="device-name">{device.hostname || device.device_id.slice(0, 8)}</div>
                            <div class="device-meta">
                                {device.device_id.slice(0, 8)} · {device.tailscale_ip || 'без IP'}
                            </div>
                        </div>
                        <div class="device-status">
                            <StatusDot status={device.status} />
                            <span class="status-text">{device.status === 'online' ? 'Онлайн' : 'Офлайн'}</span>
                        </div>
                    </div>
                </Card>
            {/each}
        </div>
    {/if}

    <!-- Модалка подтверждения -->
    <Modal
        open={showConfirm}
        title={confirmAction === 'block' ? 'Заблокировать пользователя?' : 'Разблокировать пользователя?'}
        onClose={() => showConfirm = false}
    >
        {#if confirmAction === 'block'}
            <p class="modal-text">
                Пользователь будет заблокирован: все сессии закроются, а агенты на его устройствах получат отказ при подключении. Сайты на его телефонах продолжат работать, но управлять ими через панель будет нельзя.
            </p>
        {:else}
            <p class="modal-text">
                Пользователь снова сможет войти в панель и управлять своими устройствами.
            </p>
        {/if}
        <svelte:fragment slot="footer">
            <Button on:click={() => showConfirm = false}>Отмена</Button>
            <Button
                variant={confirmAction === 'block' ? 'danger' : 'primary'}
                on:click={confirm}
                loading={busy}
            >
                {confirmAction === 'block' ? 'Заблокировать' : 'Разблокировать'}
            </Button>
        </svelte:fragment>
    </Modal>
{/if}

<style>
    .admin-error { margin:16px 0; padding:14px 16px; border:1px solid var(--danger); border-radius:10px; background:color-mix(in srgb, var(--danger) 7%, var(--bg-surface)); }
    .admin-error p { white-space:pre-wrap; overflow-wrap:anywhere; }
    .admin-error dl { display:grid; gap:6px; }
    .admin-error dl div { display:flex; flex-wrap:wrap; gap:8px; }
    .admin-error dt { color:var(--text-muted); min-width:105px; }
    .admin-error dd { margin:0; overflow-wrap:anywhere; }
    .admin-error small { color:var(--text-muted); }
    .header-actions {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .user-status {
        font-size: 11px;
        padding: 2px 8px;
        border-radius: 4px;
        font-weight: 500;
        text-transform: uppercase;
        letter-spacing: 0.04em;
        margin-left: 8px;
        vertical-align: middle;
    }

    .status-free {
        background: var(--bg-elevated);
        color: var(--text-muted);
    }

    .status-commercial {
        background: rgba(34, 197, 94, 0.1);
        color: var(--success);
    }

    .status-pro, .status-advanced { background: color-mix(in srgb, var(--accent) 12%, transparent); color: var(--accent); }

    .status-blocked {
        background: rgba(239, 68, 68, 0.1);
        color: var(--danger);
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
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.04em;
        margin-bottom: 4px;
    }

    .info-value {
        font-size: 13px;
        color: var(--text-main);
        word-break: break-all;
    }

    .info-value.mono {
        font-family: ui-monospace, monospace;
        font-size: 12px;
    }

    .section-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin: 32px 0 16px;
    }

    .section-title {
        font-size: 14px;
        font-weight: 600;
        color: var(--text-main);
    }

    .section-count {
        font-size: 12px;
        color: var(--text-muted);
    }

    .devices-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .device-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 16px;
    }

    .device-name {
        font-size: 13px;
        font-weight: 500;
        color: var(--text-main);
    }

    .device-meta {
        font-family: ui-monospace, monospace;
        font-size: 11px;
        color: var(--text-muted);
        margin-top: 2px;
    }

    .device-status {
        display: flex;
        align-items: center;
        gap: 6px;
        flex-shrink: 0;
    }

    .status-text {
        font-size: 12px;
        color: var(--text-muted);
    }

    .modal-text {
        font-size: 13px;
        color: var(--text-muted);
        line-height: 1.6;
    }
</style>
