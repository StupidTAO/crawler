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

访问grpc服务
```
curl -H "content-type: application/json" -d '{"name": "john"}' http://127.0.0.1:8081/greeter/hello
```

启动etcd服务
```
mkdir -p /tmp/etcd-data.tmp && \
 docker run \
  -p 2379:2379 \
  -p 2380:2380 \
  --mount type=bind,source=/tmp/etcd-data.tmp,destination=/etcd-data \
  --name etcd-gcr-v3.5.13 \
  gcr.io/etcd-development/etcd:v3.5.13 \
  /usr/local/bin/etcd \
  --name s1 \
  --data-dir /etcd-data \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://0.0.0.0:2379 \
  --listen-peer-urls http://0.0.0.0:2380 \
  --initial-advertise-peer-urls http://0.0.0.0:2380 \
  --initial-cluster s1=http://0.0.0.0:2380 \
  --initial-cluster-token tkn \
  --initial-cluster-state new \
  --log-level info \
  --logger zap \
  --log-outputs stderr
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