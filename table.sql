#D:\mysql\bin\mysql –uroot –p123456 -Dtest<d:\test\ss.sql
#mysql -uroot -p -Dmyblog<C:\Users\exc\Working\blog\blogserver\table.sql

#use mysql
#ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '123456';
#FLUSH PRIVILEGES;

#mysqld --initialize-insecure --user=mysqld
#mysqld -install
#net start mysql
#net stop mysql

#create database myblog;
#desc myblog;

DROP TABLE IF EXISTS `s_blog`;

CREATE TABLE IF NOT EXISTS `s_blog`(
   `id` INT UNSIGNED AUTO_INCREMENT,
   `blog_id` VARCHAR(64),
   `blog_title` VARCHAR(255) NOT NULL,
   `blog_content` TEXT NOT NULL,
   `blog_content_type` VARCHAR(64),
   `blog_channel` VARCHAR(64),
   `blog_edit_time` BIGINT,
   `sql_update_time` BIGINT,
   PRIMARY KEY ( `id` )
)ENGINE=InnoDB DEFAULT CHARSET=utf8;