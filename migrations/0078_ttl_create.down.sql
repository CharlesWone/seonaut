ALTER TABLE `crawls` DROP COLUMN `create_time`;
ALTER TABLE `links` DROP COLUMN `create_time`;
ALTER TABLE `external_links` DROP COLUMN `create_time`;
ALTER TABLE `hreflangs` DROP COLUMN `create_time`;
ALTER TABLE `issues` DROP COLUMN `create_time`;
ALTER TABLE `images` DROP COLUMN `create_time`;
ALTER TABLE `scripts` DROP COLUMN `create_time`;
ALTER TABLE `styles` DROP COLUMN `create_time`;
ALTER TABLE `iframes` DROP COLUMN `create_time`;
ALTER TABLE `audios` DROP COLUMN `create_time`;
ALTER TABLE `videos` DROP COLUMN `create_time`;
ALTER TABLE `pagereports` DROP COLUMN `create_time`;


ALTER TABLE `crawls` REMOVE TTL;
ALTER TABLE `links` REMOVE TTL;
ALTER TABLE `external_links` REMOVE TTL;
ALTER TABLE `hreflangs` REMOVE TTL;
ALTER TABLE `issues` REMOVE TTL;
ALTER TABLE `images` REMOVE TTL;
ALTER TABLE `scripts` REMOVE TTL;
ALTER TABLE `styles` REMOVE TTL;
ALTER TABLE `iframes` REMOVE TTL;
ALTER TABLE `audios` REMOVE TTL;
ALTER TABLE `videos` REMOVE TTL;
ALTER TABLE `pagereports` REMOVE TTL;
