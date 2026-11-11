<script>
    import { goto } from '$app/navigation';
    import { login } from '$lib/api.js';
    import { user } from '$lib/stores.js';
    import Button from '$lib/components/Button.svelte';

    let username = '';
    let password = '';
    let error = '';
    let loading = false;

    async function handleSubmit(e) {
        e.preventDefault();
        error = '';
        loading = true;

        const res = await login(username, password);
        loading = false;

        if (!res.ok) {
            error = res.error || 'Неверный логин или пароль';
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
            <h1 class="auth-title">VELTRIX Dock</h1>
            <p class="auth-subtitle">Вход в панель управления</p>
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
                    autocomplete="username"
                    autofocus
                />
            </div>

            <div class="form-group">
                <label class="form-label" for="password">Пароль</label>
                <input
                    id="password"
                    class="input"
                    type="password"
                    bind:value={password}
                    required
                    autocomplete="current-password"
                />
            </div>

            <Button type="submit" variant="primary" fullWidth loading={loading}>
                Войти
            </Button>
        </form>

        <div class="auth-footer">
            Нет аккаунта?
            <a href="/register" class="auth-link">Зарегистрироваться</a>
        </div>
    </div>
</div>

<style>
    .auth-page {
        min-height: 100vh;
        display: flex;
        align-items: center;
        justify-content: center;
        background: #000;
        padding: 24px;
    }

    .auth-card {
        width: 100%;
        max-width: 380px;
        background: #09090b;
        border: 1px solid #27272a;
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
        color: #fafafa;
        margin-bottom: 4px;
    }

    .auth-subtitle {
        font-size: 13px;
        color: #71717a;
    }

    .auth-footer {
        margin-top: 20px;
        text-align: center;
        font-size: 12px;
        color: #71717a;
    }

    .auth-link {
        color: #fafafa;
        margin-left: 4px;
    }

    .auth-link:hover {
        text-decoration: underline;
    }
</style>
