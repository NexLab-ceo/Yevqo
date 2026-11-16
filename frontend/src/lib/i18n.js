/**
 * Yevqo — i18n (русский + английский).
 * Простая реализация без внешних библиотек.
 */

import { writable, derived } from 'svelte/store';

const translations = {
    ru: {
        // Nav
        'nav.home': 'Главная',
        'nav.devices': 'Устройства',
        'nav.sites': 'Сайты',
        'nav.admin': 'Админка',
        'nav.settings': 'Настройки',
        'nav.profile': 'Профиль',
        'nav.logout': 'Выйти',

        // Auth
        'auth.login': 'Войти',
        'auth.register': 'Регистрация',
        'auth.username': 'Логин',
        'auth.email': 'Email',
        'auth.password': 'Пароль',
        'auth.referral': 'Реферальный код (необязательно)',
        'auth.have_account': 'Уже есть аккаунт?',
        'auth.no_account': 'Нет аккаунта?',
        'auth.login_error': 'Неверный логин или пароль',
        'auth.register_success': 'Аккаунт создан',

        // Devices
        'devices.title': 'Устройства',
        'devices.desc': 'Телефоны, подключённые к панели',
        'devices.add': 'Добавить устройство',
        'devices.empty': 'Пока нет подключённых устройств',
        'devices.empty_hint': 'Установите агент на телефон и введите токен привязки',
        'devices.online': 'Онлайн',
        'devices.offline': 'Офлайн',
        'devices.sites': 'Сайтов',
        'devices.limit': 'Лимит устройств',
        'devices.delete': 'Отвязать',
        'devices.delete_confirm': 'Отвязать это устройство?',

        // Sites
        'sites.title': 'Сайты',
        'sites.desc': 'Все сайты на всех устройствах',
        'sites.empty': 'Пока нет сайтов',
        'sites.start': 'Запустить',
        'sites.stop': 'Остановить',
        'sites.restart': 'Перезапустить',
        'sites.logs': 'Логи',
        'sites.git_pull': 'Git Pull',
        'sites.tunnel': 'Туннель',
        'sites.running': 'Работает',
        'sites.stopped': 'Остановлен',
        'sites.error': 'Ошибка',

        // Admin
        'admin.title': 'Админка',
        'admin.desc': 'Управление пользователями',
        'admin.users': 'Пользователи',
        'admin.stats': 'Статистика',
        'admin.total_users': 'Всего пользователей',
        'admin.commercial': 'Коммерческих',
        'admin.blocked': 'Заблокированных',
        'admin.total_devices': 'Всего устройств',
        'admin.total_sites': 'Всего сайтов',
        'admin.running_sites': 'Запущенных сайтов',
        'admin.block': 'Заблокировать',
        'admin.unblock': 'Разблокировать',
        'admin.status': 'Статус',

        // Common
        'common.loading': 'Загрузка...',
        'common.save': 'Сохранить',
        'common.cancel': 'Отмена',
        'common.delete': 'Удалить',
        'common.confirm': 'Подтвердить',
        'common.close': 'Закрыть',
        'common.error': 'Ошибка',
        'common.success': 'Готово',
    },

    en: {
        // Nav
        'nav.home': 'Home',
        'nav.devices': 'Devices',
        'nav.sites': 'Sites',
        'nav.admin': 'Admin',
        'nav.settings': 'Settings',
        'nav.profile': 'Profile',
        'nav.logout': 'Log out',

        // Auth
        'auth.login': 'Log in',
        'auth.register': 'Register',
        'auth.username': 'Username',
        'auth.email': 'Email',
        'auth.password': 'Password',
        'auth.referral': 'Referral code (optional)',
        'auth.have_account': 'Already have an account?',
        'auth.no_account': 'No account?',
        'auth.login_error': 'Invalid username or password',
        'auth.register_success': 'Account created',

        // Devices
        'devices.title': 'Devices',
        'devices.desc': 'Phones connected to the panel',
        'devices.add': 'Add device',
        'devices.empty': 'No devices connected yet',
        'devices.empty_hint': 'Install the agent on your phone and enter the bind token',
        'devices.online': 'Online',
        'devices.offline': 'Offline',
        'devices.sites': 'Sites',
        'devices.limit': 'Device limit',
        'devices.delete': 'Unbind',
        'devices.delete_confirm': 'Unbind this device?',

        // Sites
        'sites.title': 'Sites',
        'sites.desc': 'All sites on all devices',
        'sites.empty': 'No sites yet',
        'sites.start': 'Start',
        'sites.stop': 'Stop',
        'sites.restart': 'Restart',
        'sites.logs': 'Logs',
        'sites.git_pull': 'Git Pull',
        'sites.tunnel': 'Tunnel',
        'sites.running': 'Running',
        'sites.stopped': 'Stopped',
        'sites.error': 'Error',

        // Admin
        'admin.title': 'Admin',
        'admin.desc': 'User management',
        'admin.users': 'Users',
        'admin.stats': 'Stats',
        'admin.total_users': 'Total users',
        'admin.commercial': 'Commercial',
        'admin.blocked': 'Blocked',
        'admin.total_devices': 'Total devices',
        'admin.total_sites': 'Total sites',
        'admin.running_sites': 'Running sites',
        'admin.block': 'Block',
        'admin.unblock': 'Unblock',
        'admin.status': 'Status',

        // Common
        'common.loading': 'Loading...',
        'common.save': 'Save',
        'common.cancel': 'Cancel',
        'common.delete': 'Delete',
        'common.confirm': 'Confirm',
        'common.close': 'Close',
        'common.error': 'Error',
        'common.success': 'Done',
    },
};

// Текущий язык (по умолчанию — русский).
export const locale = writable('ru');

// Функция перевода.
export const t = derived(locale, ($locale) => {
    return (key) => translations[$locale]?.[key] ?? key;
});
