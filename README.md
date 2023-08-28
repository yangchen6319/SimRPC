### SimRPC——支持并发与异步操作的RPC框架
* 基于net、encoding、bufio等GO标准库实现了一个支持服务注册、服务发现以及负载均衡的RPC框架。
* SimRPC框架支持JSON、gob等多种消息编码格式，实现了一个RPC Client支持服务调用的并发与异步操作。
* SimRPC框架支持HTTP协议，基于HTTP的CONNECT请求，支持消息的HTTP协议与RPC协议之间的转换操作。