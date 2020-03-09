package server

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"sync"

	"github.com/valyala/fasthttp"
	"gitlab.com/crypto_project/core/strategy_service/src/backtesting"
	backtestingMocks "gitlab.com/crypto_project/core/strategy_service/src/backtesting/mocks"
	"gitlab.com/crypto_project/core/strategy_service/src/service"
	"gitlab.com/crypto_project/core/strategy_service/src/sources/mongodb/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	addr     = flag.String("addr", ":8080", "TCP address to listen to")
	compress = flag.Bool("compress", false, "Whether to enable transparent response compression")
)

func RunServer(wg *sync.WaitGroup) {
	flag.Parse()

	h := requestHandler
	if *compress {
		h = fasthttp.CompressHandler(h)
	}

	log.Printf("API listening on %s ...", ":8080")

	if err := fasthttp.ListenAndServe(*addr, h); err != nil {
		wg.Done()
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Method()) {
	case "POST":
		postHandler(ctx)
	case "GET":
		getHandler(ctx)
	}
}

func postHandler(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Path()) {
	case "/backtest":
		backtest(ctx)
	default:
		ctx.Error("Unsupported path", fasthttp.StatusNotFound)
	}
}

func getHandler(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Path()) {
	case "/":
		index(ctx)
	case "/test":
		test(ctx)
	default:
		ctx.Error("Unsupported path", fasthttp.StatusNotFound)
	}
}

func backtest(ctx *fasthttp.RequestCtx) {
	// TODO: parse SM params
	//smartOrderModel := models.MongoStrategy{}
	smartOrderModel := getBacktestStrategy()

	// TODO: parse historical params
	historicalDataParams := backtestingMocks.HistoricalParams{
		Exchange:      "binance",
		Base:          "BTC",
		Quote:         "USDT",
		OhlcvPeriod:   60,
		MarketType:    1,
		IntervalStart: 1583317500,
		IntervalEnd:   1583317500 + 86400,
	}

	// run SM
	backtestResult := backtesting.BacktestStrategy(smartOrderModel, historicalDataParams)

	// return results
	json, err := json.Marshal(backtestResult)
	if err != nil {
		fmt.Fprintf(ctx, "{status: ERR, message: %s", err.Error())
	}

	fmt.Fprintf(ctx, string(json))
}

func test(ctx *fasthttp.RequestCtx) {
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

func index(ctx *fasthttp.RequestCtx) {
	fmt.Fprintf(ctx, "Hello, world!\n\n")
}

func getBacktestStrategy() models.MongoStrategy {

	smartOrder := models.MongoStrategy{
		ID:          &primitive.ObjectID{},
		Conditions:  &models.MongoStrategyCondition{},
		State:       &models.MongoStrategyState{Amount: 1000},
		TriggerWhen: models.TriggerOptions{},
		Expiration:  models.ExpirationSchema{},
		OwnerId:     primitive.ObjectID{},
		Social:      models.MongoSocial{},
		Enabled:     true,
	}

	smartOrder.Conditions = &models.MongoStrategyCondition{
		Pair:         "BTC_USDT",
		MarketType:   1,
		Leverage:     100,
		StopLossType: "market",
		StopLoss:     3,
		EntryOrder: &models.MongoEntryPoint{
			Side:           "buy",
			ActivatePrice:  8750,
			Amount:         0.05,
			EntryDeviation: 2,
			OrderType:      "market",
		},
		ExitLevels: []*models.MongoEntryPoint{{
			OrderType:      "market",
			Type:           1,
			ActivatePrice:  10,
			EntryDeviation: 2,
			Amount:         100,
		}},
	}

	return smartOrder
}
