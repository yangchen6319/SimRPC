package main

import (
	"github.com/yangchen6319/SimRPC/client"
	"github.com/yangchen6319/SimRPC/service"
	"log"
	"net"
	"sync"
	"time"
)

type Foo int

type Args struct {
	Num1 int
	Num2 int
}

func (f Foo) Sum(args2 Args, reply *int) error {
	*reply = args2.Num1 + args2.Num2
	return nil
}

func startServer(addr chan string) {
	var foo Foo
	if err := service.Register(&foo); err != nil {
		log.Fatal("rpc main: register error", err)
	}
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Println("start server fail!", err)
	}
	addr <- l.Addr().String()
	service.Accept(l)
}

func main() {
	log.SetFlags(0)
	addr := make(chan string)
	go startServer(addr)
	// 启动一个client
	cli, _ := client.Dial("tcp", <-addr)
	defer func() { _ = cli.Close() }()
	// 这里sleep一下，然后向server发送请求
	time.Sleep(3 * time.Second)
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{
				Num1: i,
				Num2: i * i,
			}
			var reply int
			if err := cli.Call("Foo.Sum", args, &reply); err != nil {
				log.Println(err)
			}
			log.Printf("%d + %d = %d:", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}
