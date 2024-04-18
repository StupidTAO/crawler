# crawler

存储验证引擎(tag v0.2.7)
```
docker run -d --name mysql-test -p 3326:3306 -e MYSQL_ROOT_PASSWORD=123456 mysql
docker exec -it mysql-test sh
mysql -uroot -p123456
CREATE DATABASE crawler;
use crawler;
```
