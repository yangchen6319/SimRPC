package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yangchen6319/SimRPC/codec"
	"github.com/yangchen6319/SimRPC/service"
	"log"
	"net"
	"sync"
)

type Call struct {
	Seq           uint64
	ServiceMethod string
	Args          interface{}
	Reply         interface{}
	Err           error
	Done          chan *Call
}

func (call *Call) done() {
	call.Done <- call
}

type Client struct {
	cc       codec.Codec
	opt      *service.Option
	seq      uint64
	mu       sync.Mutex
	sending  sync.Mutex
	header   codec.Header
	pending  map[uint64]*Call
	shutdown bool
	close    bool
}

var ErrShutdown = errors.New("connection is shut down")

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.close {
		return ErrShutdown
	}
	return client.cc.Close()
}

func (client *Client) IsAvailable() bool {
	return !client.shutdown && !client.close
}

func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.shutdown || client.close {
		return 0, ErrShutdown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	var call *Call
	call = client.pending[seq]
	delete(client.pending, seq)
	return call
}

func (client *Client) terminalCalls(err error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Err = err
	}
}

func (client *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = client.cc.ReadHeader(&h); err != nil {
			break
		}
		call := client.removeCall(h.Seq)
		switch {
		case call == nil:
			err = client.cc.ReadBody(nil)
		case h.Error != "":
			call.Err = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Err = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	client.terminalCalls(err)
}

func NewClient(conn net.Conn, opt *service.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {

	}
	if err := json.NewEncoder(conn).Encode(opt); err != nil {

	}
	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(cc codec.Codec, opt *service.Option) *Client {
	client := &Client{
		seq:      1,
		cc:       cc,
		opt:      opt,
		pending:  make(map[uint64]*Call),
		close:    false,
		shutdown: false,
	}
	go client.receive()
	return client
}

func parseOptions(opts ...*service.Option) (*service.Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return service.DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more 1")
	}
	opt := opts[0]
	opt.MagicNumber = service.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = "GobCodec"
	}
	return opt, nil
}

func Dial(network, address string, opts ...*service.Option) (client *Client, err error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()
	return NewClient(conn, opt)
}

func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()
	seq, err := client.registerCall(call)
	if err != nil {
		call.Err = err
		call.done()
		return
	}
	client.header.Service = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	if err = client.cc.Writer(&client.header, call.Args); err != nil {
		call := client.removeCall(seq)
		if call != nil {
			call.Err = err
			call.done()
		}
	}
}

func (client *Client) GO(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

func (client *Client) Call(serviceMethod string, args, reply interface{}) error {
	call := <-client.GO(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Err
}
