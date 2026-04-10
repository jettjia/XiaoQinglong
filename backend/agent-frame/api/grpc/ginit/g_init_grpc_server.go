package ginit

import (
	"google.golang.org/grpc"

	grpcGoodsProto "github.com/jettjia/xiaoqinglong/agent-frame/idl/proto/goods"

	"github.com/jettjia/xiaoqinglong/agent-frame/api/grpc/ghandler"
)

// RegisterGrpcSrv 初始化grpc的服务
func RegisterGrpcSrv(server *grpc.Server) {
	grpcGoodsProto.RegisterGoodsServer(server, &ghandler.GrpcGoodsServer{})
}
