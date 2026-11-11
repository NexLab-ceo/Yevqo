-- VELTRIX Dock — схема базы данных
-- Импортируется один раз при установке (install-control.sh)
-- Для Redmi 9 настроена минимальная нагрузка на память.

SET NAMES utf8mb4;
SET time_zone = '+00:00';

-- Создаём базу (если ещё нет)
CREATE DATABASE IF NOT EXISTS veltrix
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;

USE veltrix;

-- ============================================================
-- ПОЛЬЗОВАТЕЛИ (аккаунты на панели)
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username        VARCHAR(64)  NOT NULL UNIQUE,
    email           VARCHAR(128) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,             
    status          ENUM('free','commercial','blocked') NOT NULL DEFAULT 'free',

    -- Реферальная система (плюшки за друзей)
    referral_code   VARCHAR(16)  NOT NULL UNIQUE,
    referred_by     BIGINT UNSIGNED NULL,              
    bonus_devices   INT UNSIGNED NOT NULL DEFAULT 0,   
    max_devices     INT UNSIGNED NOT NULL DEFAULT 1,  

    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen       TIMESTAMP    NULL,

    CONSTRAINT fk_users_referred_by
        FOREIGN KEY (referred_by) REFERENCES users(id)
        ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_users_status       ON users(status);
CREATE INDEX idx_users_referral     ON users(referral_code);
CREATE INDEX idx_users_referred_by  ON users(referred_by);

-- ============================================================
-- УСТРОЙСТВА (телефоны пользователей, подключённые к панели)
-- ============================================================
CREATE TABLE IF NOT EXISTS devices (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id         BIGINT UNSIGNED NOT NULL,
    device_id       CHAR(36)     NOT NULL UNIQUE,     
    hostname        VARCHAR(128) NULL,              
    tailscale_ip    VARCHAR(15)  NULL,               
    agent_version   VARCHAR(16)  NULL,                
    status          ENUM('online','offline') NOT NULL DEFAULT 'offline',
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen       TIMESTAMP    NULL,

    CONSTRAINT fk_devices_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_devices_user       ON devices(user_id);
CREATE INDEX idx_devices_status     ON devices(status);
CREATE INDEX idx_devices_last_seen  ON devices(last_seen);

-- ============================================================
-- САЙТЫ (запущенные на устройствах)
-- ============================================================
CREATE TABLE IF NOT EXISTS sites (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    device_id       BIGINT UNSIGNED NOT NULL,          
    site_id         VARCHAR(64)  NOT NULL,             
    path            VARCHAR(255) NOT NULL,             
    port            INT UNSIGNED NOT NULL,
    runtime         ENUM('php','go','static') NOT NULL DEFAULT 'php',
    domain          VARCHAR(128) NULL,                 
    status          ENUM('running','stopped','error') NOT NULL DEFAULT 'stopped',
    pid             INT UNSIGNED NULL,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uq_sites_device_site (device_id, site_id),

    CONSTRAINT fk_sites_device
        FOREIGN KEY (device_id) REFERENCES devices(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_sites_device  ON sites(device_id);
CREATE INDEX idx_sites_status  ON sites(status);

-- ============================================================
-- СЕССИИ (Cookie для входа в панель)
-- ============================================================
CREATE TABLE IF NOT EXISTS sessions (
    id              CHAR(64)     PRIMARY KEY,         
    user_id         BIGINT UNSIGNED NOT NULL,
    ip              VARCHAR(45)  NULL,               
    user_agent      VARCHAR(255) NULL,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at      TIMESTAMP    NOT NULL,

    CONSTRAINT fk_sessions_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_sessions_user     ON sessions(user_id);
CREATE INDEX idx_sessions_expires  ON sessions(expires_at);

-- ============================================================
-- ЛОГИ (аудит: кто что делал)
-- ============================================================
CREATE TABLE IF NOT EXISTS logs (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id         BIGINT UNSIGNED NULL,
    device_id       BIGINT UNSIGNED NULL,
    action          VARCHAR(64)  NOT NULL,             
    details         JSON         NULL,
    ip              VARCHAR(45)  NULL,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_logs_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE SET NULL,
    CONSTRAINT fk_logs_device
        FOREIGN KEY (device_id) REFERENCES devices(id)
        ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_logs_user     ON logs(user_id);
CREATE INDEX idx_logs_action   ON logs(action);
CREATE INDEX idx_logs_created  ON logs(created_at);

-- ============================================================
-- ОПТИМИЗАЦИЯ ДЛЯ REDMI 9 (мало RAM)
-- ============================================================
-- Эти настройки лучше прописать в my.cnf, но можно и здесь.
-- Они уменьшают потребление памяти InnoDB.
-- (Закомментировано, так как требует прав root и перезапуска mysqld)

-- SET GLOBAL innodb_buffer_pool_size = 16 * 1024 * 1024;   -- 16 MB
-- SET GLOBAL max_connections = 10;
-- SET GLOBAL query_cache_size = 0;
-- SET GLOBAL table_open_cache = 64;

-- ============================================================
-- КОНЕЦ СХЕМЫ
-- ============================================================
