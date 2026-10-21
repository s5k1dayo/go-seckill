-- 秒杀系统数据库初始化（Docker 首次启动时自动执行）
-- 说明：这是一份「自包含快照」，用于 docker compose 一键部署。

CREATE DATABASE IF NOT EXISTS `seckill`
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE `seckill`;

-- ---------- 用户 ----------
CREATE TABLE IF NOT EXISTS `user` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '用户ID',
  `username` varchar(50) NOT NULL COMMENT '用户名',
  `password` varchar(100) NOT NULL COMMENT '密码(bcrypt)',
  `create_time` datetime NOT NULL DEFAULT current_timestamp() COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- ---------- 商品 ----------
CREATE TABLE IF NOT EXISTS `product` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '商品ID',
  `name` varchar(100) NOT NULL COMMENT '商品名称',
  `price` decimal(10,2) NOT NULL COMMENT '价格',
  `stock` int(11) NOT NULL DEFAULT 0 COMMENT '库存',
  `create_time` datetime NOT NULL DEFAULT current_timestamp() COMMENT '创建时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商品表';

-- ---------- 普通订单 ----------
CREATE TABLE IF NOT EXISTS `orders` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned NOT NULL,
  `product_id` bigint(20) unsigned NOT NULL,
  `create_time` datetime NOT NULL DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_product` (`user_id`, `product_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='普通订单表';

-- ---------- 秒杀活动 ----------
CREATE TABLE IF NOT EXISTS `seckill_activity` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT '活动ID',
  `product_id` int(10) unsigned NOT NULL COMMENT '商品ID',
  `stock` int(11) NOT NULL DEFAULT 0 COMMENT '活动库存',
  `start_time` datetime NOT NULL COMMENT '开始时间',
  `end_time` datetime NOT NULL COMMENT '结束时间',
  `seckill_price` decimal(10,2) NOT NULL DEFAULT 0.00 COMMENT '秒杀价',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='秒杀活动表';

-- ---------- 秒杀订单 ----------
CREATE TABLE IF NOT EXISTS `seckill_orders` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned NOT NULL,
  `product_id` bigint(20) unsigned NOT NULL,
  `activity_id` int(10) unsigned NOT NULL,
  `create_time` datetime NOT NULL DEFAULT current_timestamp(),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_activity` (`user_id`, `activity_id`),
  KEY `idx_activity` (`activity_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='秒杀订单表';

-- ---------- 演示数据 ----------
-- 演示账号：test / 123456
INSERT INTO `user` (`id`, `username`, `password`) VALUES
  (1, 'test', '$2a$10$ryOyXNfNRrHSZ5BRpvHiU.Tc0ovhe22MQk/u0t9K2p3VHR.Ma.wQK')
ON DUPLICATE KEY UPDATE `password` = VALUES(`password`);

-- 演示商品
INSERT INTO `product` (`id`, `name`, `price`, `stock`) VALUES
  (1, '测试商品', 99.00, 100000)
ON DUPLICATE KEY UPDATE `stock` = VALUES(`stock`);

-- 演示活动：id=1，长期有效，库存 1000
INSERT INTO `seckill_activity`
  (`id`, `product_id`, `stock`, `start_time`, `end_time`, `seckill_price`)
VALUES
  (1, 1, 1000, '2020-01-01 00:00:00', '2099-12-31 23:59:59', 9.90)
ON DUPLICATE KEY UPDATE
  `stock` = VALUES(`stock`),
  `start_time` = VALUES(`start_time`),
  `end_time` = VALUES(`end_time`);
