package proxy

import "testing"

// TestGetGRPCConnReuse 验证 H1 修复：同一地址复用同一连接，不再每请求新建。
func TestGetGRPCConnReuse(t *testing.T) {
	addr := "127.0.0.1:59999" // 懒连接，不会真正拨号

	c1, err := getGRPCConn(addr)
	if err != nil {
		t.Fatalf("first getGRPCConn error: %v", err)
	}
	c2, err := getGRPCConn(addr)
	if err != nil {
		t.Fatalf("second getGRPCConn error: %v", err)
	}
	if c1 != c2 {
		t.Fatalf("expected the same cached *grpc.ClientConn for addr %s, got different instances", addr)
	}

	// 不同地址应得到不同连接
	c3, err := getGRPCConn("127.0.0.1:59998")
	if err != nil {
		t.Fatalf("third getGRPCConn error: %v", err)
	}
	if c1 == c3 {
		t.Fatalf("expected different conns for different addrs")
	}

	_ = c1.Close()
	_ = c3.Close()
	grpcConnPool.Delete(addr)
	grpcConnPool.Delete("127.0.0.1:59998")
}
