package client

import (
	"context"
	"fmt"
	"github.com/yangchen6319/SimRPC/service"
	"net"
	"testing"
	"time"
)

type Bar int

func (b Bar) Timeout(argv int, replyv *int) error {
	time.Sleep(3 * time.Second)
	return nil
}
func startServer(addr chan string) {
	var b Bar
	_ = service.Register(&b)
	// pick a free port
	l, _ := net.Listen("tcp", ":8080")
	addr <- l.Addr().String()
	service.Accept(l)
}

func TestClient_Call(t *testing.T) {
	t.Parallel()
	addrCh := make(chan string)
	go startServer(addrCh)
	addr := <-addrCh
	time.Sleep(time.Second)
	t.Run("client timeout", func(t *testing.T) {
		client, _ := Dial("tcp", addr)
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		var reply int
		err := client.Call(ctx, "Bar.Timeout", 1, &reply)
		if err != nil {
			fmt.Println(err)
		}
	})
	t.Run("server handle timeout", func(t *testing.T) {
		client, _ := Dial("tcp", addr, &service.Option{
			HandleTimeout: time.Second,
		})
		var reply int
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		err := client.Call(ctx, "Bar.Timeout", 1, &reply)
		if err != nil {
			fmt.Println(err)
		}
	})
}

func TestClient_dialTimeout(t *testing.T) {
	t.Parallel()
	l, _ := net.Listen("tcp", ":8080")
	f := func(conn net.Conn, opt *service.Option) (client *Client, err error) {
		_ = conn.Close()
		time.Sleep(5 * time.Second)
		return nil, nil
	}

	t.Run("timeout", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &service.Option{ConnectTimeout: time.Second})
		if err != nil {
			fmt.Println(err)
		}
	})
	t.Run("0", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &service.Option{ConnectTimeout: 0})
		if err != nil {
			fmt.Println(err)
		}
	})
}
