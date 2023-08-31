package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yangchen6319/SimRPC/codec"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
)

// server编码解码相关代码，包括如下步骤
// 1、将请求的body反序列化
// 2、完成服务调用任务
// 3、构建响应报

const MagicNumber int = 114514

type Option struct {
	MagicNumber    int
	CodecType      codec.Type
	ConnectTimeout time.Duration
	HandleTimeout  time.Duration
}

var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      "GobCodec",
	ConnectTimeout: 10 * time.Second,
}

type Server struct {
	serviceMap sync.Map
}

var DefaultServer = &Server{serviceMap: sync.Map{}}

func NewServer() *Server {
	return &Server{}
}

func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		go server.ServerConn(conn)
	}
}

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

func (server *Server) ServerConn(conn io.ReadWriteCloser) {
	var option Option
	if err := json.NewDecoder(conn).Decode(&option); err != nil {
		log.Println("rpc server: options error:", err)
		return
	}
	if option.MagicNumber != MagicNumber {
		log.Println("rpc server: magic number error!")
		return
	}
	f := codec.NewCodecFuncMap[option.CodecType]
	if f == nil {
		log.Println("rpc server: codec func error!")
		return
	}
	server.serverCodec(f(conn), option)
}

// invalidRequest 被返回，当错误发生的时候
var invalidRequest = struct{}{}

func (server *Server) serverCodec(cc codec.Codec, opt Option) {
	wg := &sync.WaitGroup{}
	sending := &sync.Mutex{}
	for {
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go server.handleRequest(cc, req, sending, wg, opt.ConnectTimeout)
	}
	wg.Wait()
	_ = cc.Close()
}

type request struct {
	// 每次请求包含的字段
	h           *codec.Header
	argv, reply reflect.Value
	mtype       *methodType
	svc         *service
}

func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	req.svc, req.mtype, err = server.findService(h.Service)
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.newArgv()
	req.reply = req.mtype.newReplyv()

	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	if err := cc.ReadBody(argvi); err != nil {
		log.Println("rpc server: read request error:", err)
		return req, err
	}
	return req, nil
}

func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (server *Server) sendResponse(cc codec.Codec, header *codec.Header, body interface{}, mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()
	if err := cc.Writer(header, body); err != nil {
		log.Println("rpc server: send response error :", err)
	}
}

// handleRequest 服务端处理来自客户端的请求并返回调用后的结果
func (server *Server) handleRequest(cc codec.Codec, req *request, mu *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	called := make(chan struct{})
	sent := make(chan struct{})
	go func() {
		err := req.svc.call(req.mtype, req.argv, req.reply)
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidRequest, mu)
			sent <- struct{}{}
			return
		}
		server.sendResponse(cc, req.h, req.reply.Interface(), mu)
		sent <- struct{}{}
	}()
	if timeout == 0 {
		<-called
		<-sent
		return
	}
	select {
	case <-time.After(timeout):
		req.h.Error = fmt.Sprintf("rpc server: request handle timeout!")
		server.sendResponse(cc, req.h, invalidRequest, mu)
	case <-called:
		<-sent

	}
}

// 关于服务注册的两个方法
func (server *Server) Register(rcvr interface{}) error {
	s := newService(rcvr)
	if _, loaded := server.serviceMap.LoadOrStore(s.name, s); loaded {
		return errors.New("rpc: service already define:" + s.name)
	}
	return nil
}

func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

func (server *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: serviceMethod format error!")
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svci, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New(fmt.Sprintf("rpc server: can't find %s service", serviceName))
	}
	svc = svci.(*service)
	mtype = svc.methods[methodName]
	if mtype == nil {
		err = errors.New(fmt.Sprintf("rpc server: can't find %s method", methodName))
	}
	return
}
