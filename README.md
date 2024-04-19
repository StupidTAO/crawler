# crawler

存储验证引擎(tag v0.2.7)
```
docker run -d --name mysql-test -p 3326:3306 -e MYSQL_ROOT_PASSWORD=123456 mysql
docker exec -it mysql-test sh
mysql -uroot -p123456
CREATE DATABASE crawler;
use crawler;
```

创建proto文件,并编译（tag0.3.0）
```
git clone git@github.com:googleapis/googleapis.git
mv googleapis/google  $(go env GOPATH)/src/google

git clone git@github.com:protocolbuffers/protobuf.git
src/goole文件夹直接拷贝到GOPATH/src/google下

./scripts/protocgen.sh
```

bug
```
go: encryptClient/proto imports
        go-micro.dev/v4/api: go-micro.dev/v4/api@v1.18.0: parsing go.mod:
        module declares its path as: github.com/micro/go-micro
                but was required as: go-micro.dev/v4/api
```
解决办法：
```
go get go-micro.dev/v4
```