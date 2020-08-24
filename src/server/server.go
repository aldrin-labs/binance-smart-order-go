package server

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/valyala/fasthttp"
	"github.com/buaazp/fasthttprouter"
	"gitlab.com/crypto_project/core/strategy_service/src/service"
	"gitlab.com/crypto_project/core/strategy_service/src/trading"
	"log"
	"sync"
)

var (
	addr     = flag.String("addr", ":8080", "TCP address to listen to")
	compress = flag.Bool("compress", false, "Whether to enable transparent response compression")
)

func RunServer(wg *sync.WaitGroup) {
	router := fasthttprouter.New()
	router.GET("/", Index)
	router.POST("/createOrder", CreateOrder)
	println("Listening on port :5901")
	if err := fasthttp.ListenAndServe(*addr, router.Handler); err != nil {
		wg.Done()
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

func CreateOrder(ctx *fasthttp.RequestCtx) {
	var createOrder trading.CreateOrderRequest
	_ := json.Unmarshal(ctx.PostBody(), &createOrder)
	service.GetStrategyService().CreateOrder(createOrder)
}
func Index(ctx *fasthttp.RequestCtx) {
	fmt.Fprintf(ctx, "Hello, world!\n\n")

	fmt.Fprintf(ctx, "Request method is %q\n", ctx.Method())
	fmt.Fprintf(ctx, "RequestURI is %q\n", ctx.RequestURI())
	fmt.Fprintf(ctx, "Requested path is %q\n", ctx.Path())
	fmt.Fprintf(ctx, "Host is %q\n", ctx.Host())
	fmt.Fprintf(ctx, "Query string is %q\n", ctx.QueryArgs())
	fmt.Fprintf(ctx, "User-Agent is %q\n", ctx.UserAgent())
	fmt.Fprintf(ctx, "Connection has been established at %s\n", ctx.ConnTime())
	fmt.Fprintf(ctx, "Request has been started at %s\n", ctx.Time())
	fmt.Fprintf(ctx, "Serial request number for the current connection is %d\n", ctx.ConnRequestNum())
	fmt.Fprintf(ctx, "Your ip is %q\n\n", ctx.RemoteIP())

	fmt.Fprintf(ctx, "Raw request is:\n---CUT---\n%s\n---CUT---", &ctx.Request)

	fmt.Fprintf(ctx, "\n\n\n\n signals :\n---CUT---\n%s\n---CUT---", service.GetStrategyService())

	ctx.SetContentType("text/plain; charset=utf8")

	// Set arbitrary headers
	ctx.Response.Header.Set("X-My-Header", "my-header-value")

	// Set cookies
	var c fasthttp.Cookie
	c.SetKey("cookie-name")
	c.SetValue("cookie-value")
	ctx.Response.Header.SetCookie(&c)
}
