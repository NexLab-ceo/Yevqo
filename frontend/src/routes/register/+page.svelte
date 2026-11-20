<script>
    import { goto } from '$app/navigation';
    import { page } from '$app/stores';
    import { register } from '$lib/api.js';
    import { user } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';

    let username = '';
    let email = '';
    let password = '';
    let referralCode = '';
    let error = '';
    let loading = false;

    // Если в URL есть ?ref=XXX — подставляем в поле.
    $: if ($page.url.searchParams.has('ref')) {
        referralCode = $page.url.searchParams.get('ref');
    }

    async function handleSubmit(e) {
        e.preventDefault();
        error = '';
        loading = true;

        const res = await register(username, email, password, referralCode);
        loading = false;

        if (!res.ok) {
            error = res.error || 'Ошибка регистрации';
            return;
        }

        // Успех — сохраняем пользователя и редирект.
        user.set(res.data.user);
        goto('/');
    }
</script>

<div class="auth-page">
    <div class="auth-card">
        <div class="auth-header">
            <h1 class="auth-title">Yevqo</h1>
            <p class="auth-subtitle">Создание аккаунта</p>
        </div>

        <form on:submit={handleSubmit}>
            {#if error}
                <div class="error">{error}</div>
            {/if}

            <div class="form-group">
                <label class="form-label" for="username">Логин</label>
                <input
                    id="username"
                    class="input"
                    type="text"
                    bind:value={username}
                    required
                    minlength="3"
                    maxlength="64"
                    autocomplete="username"
                    autofocus
                />
            </div>

            <div class="form-group">
                <label class="form-label" for="email">Email</label>
                <input
                    id="email"
                    class="input"
                    type="email"
                    bind:value={email}
                    required
                    autocomplete="email"
                />
            </div>

            <div class="form-group">
                <label class="form-label" for="password">Пароль (12–72 символа)</label>
                <input
                    id="password"
                    class="input"
                    type="password"
                    bind:value={password}
                    required
                    minlength="12"
                    maxlength="72"
                    autocomplete="new-password"
                />
            </div>

            <div class="form-group">
                <label class="form-label" for="referral">
                    Реферальный код <span class="optional">(необязательно)</span>
                </label>
                <input
                    id="referral"
                    class="input"
                    type="text"
                    bind:value={referralCode}
                    placeholder="например, a1b2c3d4e5f6"
                />
            </div>

            <Button type="submit" variant="primary" fullWidth loading={loading}>
                Зарегистрироваться
            </Button>
        </form>

        <div class="auth-footer">
            Уже есть аккаунт?
            <a href="/login" class="auth-link">Войти</a>
        </div>
    </div>
</div>

<style>
    .auth-page {
        min-height: 100vh;
        display: flex;
        align-items: center;
        justify-content: center;
        background: #080B14;
        padding: 24px;
    }

    .auth-card {
        width: 100%;
        max-width: 380px;
        background: #101727;
        border: 1px solid #253149;
        border-radius: 8px;
        padding: 32px;
    }

    .auth-header {
        margin-bottom: 24px;
        text-align: center;
    }

    .auth-title {
        font-size: 18px;
        font-weight: 600;
        color: #F1F5F9;
        margin-bottom: 4px;
    }

    .auth-subtitle {
        font-size: 13px;
        color: #64748B;
    }

    .optional {
        font-weight: 400;
        color: #64748B;
        font-size: 11px;
    }

    .auth-footer {
        margin-top: 20px;
        text-align: center;
        font-size: 12px;
        color: #64748B;
    }

    .auth-link {
        color: #F1F5F9;
        margin-left: 4px;
    }

    .auth-link:hover {
        text-decoration: underline;
    }
</style>
