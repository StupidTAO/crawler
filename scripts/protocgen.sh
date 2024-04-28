
protoc  -I $GOPATH/src -I .  --micro_out=. --go_out=.  --go-grpc_out=.  --grpc-gateway_out=logtostderr=true,register_func_suffix=Gw:. ./proto/greeter/hello.proto
#protoc  -I $GOPATH/src -I .  --micro_out=. --go_out=.  --go-grpc_out=.  --grpc-gateway_out=logtostderr=true,allow_delete_body=false,register_func_suffix=Gw:. ./proto/crawler/crawler.proto
protoc  -I $GOPATH/src -I .  --micro_out=. --go_out=.  --go-grpc_out=.  --grpc-gateway_out=logtostderr=true,allow_delete_body=true,register_func_suffix=Gw:. ./proto/crawler/crawler.proto