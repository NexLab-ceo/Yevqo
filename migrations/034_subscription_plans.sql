-- Add the Advanced platform subscription tier without changing existing account assignments.
ALTER TABLE users MODIFY status ENUM('free','commercial','pro','advanced','blocked') NOT NULL DEFAULT 'free';
