package ghandler

import (
	grpcGoodsProto "github.com/jettjia/xiaoqinglong/agent-frame/idl/proto/goods"
)

type GrpcGoodsServer struct {
	grpcGoodsProto.UnimplementedGoodsServer
}
