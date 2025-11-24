ALTER TABLE `crawls` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `links` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `external_links` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `hreflangs` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `issues` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `images` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `scripts` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `styles` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `iframes` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `audios` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `videos` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE `pagereports` ADD COLUMN `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;


ALTER TABLE `crawls` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `links` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `external_links` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `hreflangs` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `issues` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `images` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `scripts` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `styles` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `iframes` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `audios` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `videos` TTL = `create_time` + INTERVAL 7 DAY;
ALTER TABLE `pagereports` TTL = `create_time` + INTERVAL 7 DAY;
