# redigosrv

## what

redis server side protocol for golang.

redis server侧的golang版本实现。

## why

Many services will use some existing application layer protocols. http have many libs to use and it's strong, but service maybe simple, we need a lightweight protocol.

很多服务都会使用一些已有的应用层协议，`http`有很多库使用，并且也很强大，但是服务可能很简单，我们需要的是一个轻量级的协议。

When many developers implement redis server by themselves, they parse the protocol themselves and find the corresponding handler according to the parameters. However, developers usually think that it is small, so the final implementation is coupled together, and once the scale is large, it is difficult to re- maintain.

很多开发者自己实现redis server的时候，都是自己解析协议，根据参数找到对应的handler，但是开发者通常认为他很小，因此最终的实现都耦合在一起，并且一旦规模大起来后很难再维护。

## how

On the basis of golang net/http. Add option and middleware. 

在`go`语言的`net/http`的基础上修改开发，新增了`option`和中间件。

