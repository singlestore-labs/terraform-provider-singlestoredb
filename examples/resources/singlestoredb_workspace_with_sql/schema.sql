-- Create the database
CREATE DATABASE IF NOT EXISTS my_app_db;

-- Create users
CREATE USER IF NOT EXISTS 'app_user'@'%' IDENTIFIED BY 'password123';
CREATE USER IF NOT EXISTS 'app_readonly'@'%' IDENTIFIED BY 'readonly123';

-- Grant privileges
GRANT
    SELECT,
    INSERT,
    UPDATE,
    DELETE,
    CREATE,
    PROCESS,
    INDEX,
    ALTER,
    DROP,
    SHOW METADATA,
    CREATE DATABASE,
    DROP DATABASE,
    CREATE USER
ON my_app_db.*
TO 'app_user'@'%';
GRANT SELECT ON my_app_db.* TO 'app_readonly'@'%';

USE my_app_db;

CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(100) NOT NULL,
    password VARCHAR(100) NOT NULL
);

CREATE TABLE IF NOT EXISTS posts (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    title VARCHAR(200),
    body TEXT
);
