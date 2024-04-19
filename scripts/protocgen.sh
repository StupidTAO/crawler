
protoc  -I $GOPATH/src -I .  --micro_out=. --go_out=.  --go-grpc_out=.  --grpc-gateway_out=logtostderr=true,register_func_suffix=Gw:. ./proto/greeter/hello.proto