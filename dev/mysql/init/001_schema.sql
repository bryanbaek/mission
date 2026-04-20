CREATE TABLE IF NOT EXISTS sample_measurements (
    id INT AUTO_INCREMENT PRIMARY KEY,
    site_name VARCHAR(128) NOT NULL,
    reading DECIMAL(10,2) NOT NULL,
    captured_on DATE NOT NULL,
    captured_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    notes TEXT,
    payload BLOB
);

INSERT INTO sample_measurements (
    site_name,
    reading,
    captured_on,
    captured_at,
    notes,
    payload
) VALUES (
    'alpha-site',
    12.34,
    '2026-04-19',
    '2026-04-19 12:00:00',
    'baseline sample',
    'blob-data'
);

CREATE USER IF NOT EXISTS 'mission_ro'@'%' IDENTIFIED BY 'mission_ro';
CREATE USER IF NOT EXISTS 'mission_rw'@'%' IDENTIFIED BY 'mission_rw';

GRANT SELECT, SHOW VIEW ON mission_app.* TO 'mission_ro'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON mission_app.* TO 'mission_rw'@'%';

FLUSH PRIVILEGES;
