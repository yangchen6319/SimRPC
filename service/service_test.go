package service

import (
	"fmt"
	"reflect"
	"testing"
)

type Foo int

type Args struct {
	num1 int
	num2 int
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.num1 + args.num2
	return nil
}

func (f Foo) sum(args Args, reply *int) error {
	*reply = args.num1 + args.num2
	return nil
}

func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("insert failed "+msg, v...))
	}

}

func TestNewService(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	_assert(len(s.methods) == 1, "wrong method, expect 1, but got %d", len(s.methods))
	mType := s.methods["Sum"]
	_assert(mType != nil, "wrong method, Sum shouldn't nil")
}

func TestMethodType_NumCalls(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	mType := s.methods["Sum"]

	argv := mType.newArgv()
	replyv := mType.newReplyv()
	argv.Set(reflect.ValueOf(Args{
		num1: 1,
		num2: 2,
	}))
	err := s.call(mType, argv, replyv)
	_assert(err == nil && *replyv.Interface().(*int) == 3 && mType.NumCalls() == 1, "failed to call Foo.Sum")
}
