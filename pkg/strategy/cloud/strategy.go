package cloud

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "cloud"

var log = logrus.WithField("strategy", ID)

func init() {
	// Register our struct type to BBGO
	// Note that you don't need to field the fields.
	// BBGO uses reflect to parse your type information.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	Counter int `json:"counter,omitempty"`
}

type Strategy struct {
	*common.Strategy
	// bbgo.OrderExecutor
	// *bbgo.MarketDataStore
	Market      types.Market
	Environment *bbgo.Environment
	// Market   types.Market
	Symbol   string         `json:"symbol"`
	State    *State         `persistence:"state"`
	Interval types.Interval `json:"interval"`
	bbgo.OpenPositionOptions
	// activeOrders *bbgo.ActiveOrderBook
	// profitOrders *bbgo.ActiveOrderBook
	// orders       *core.OrderStore
	profit *types.Queue
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s:%s", ID, s.Symbol, s.Interval)
}

func (s *Strategy) Defaults() error {
	if s.Interval == "" {
		s.Interval = types.Interval1m
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

// This strategy simply spent all available quote currency to buy the symbol whenever kline gets closed
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// Initialize the default value for state
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())
	if s.State == nil {
		s.State = &State{Counter: 1}
	}
	s.profit = types.NewQueue(2)
	// s.orders = core.NewOrderStore(s.Symbol)
	// s.orders.BindStream(session.UserDataStream)

	// // we don't persist orders so that we can not clear the previous orders for now. just need time to support this.
	// s.activeOrders = bbgo.NewActiveOrderBook(s.Symbol)
	// s.activeOrders.OnFilled(func(o types.Order) {
	// 	s.submitReverseOrder(o, session)
	// })
	// s.activeOrders.BindStream(session.UserDataStream)

	// s.profitOrders = bbgo.NewActiveOrderBook(s.Symbol)
	// s.profitOrders.OnFilled(func(o types.Order) {
	// 	// we made profit here!
	// })
	// s.profitOrders.BindStream(session.UserDataStream)

	ichi := session.Indicators(s.Symbol).Ichimoku(s.Interval)

	err := s.CompleteIchi(ichi, session)
	if err != nil {
		log.Warnf("complete ichi failed \r\n")
		return err
	}
	// To get the market information from the current session
	// The market object provides the precision, MoQ (minimal of quantity) information

	log.Infof("high: %.3f Low: %.3f  Tenkan: %.3f Kijun: %.3f, length: %d",
		ichi.HighValues.Last(0),
		ichi.LowValues.Last(0),
		ichi.TenkanValues.Last(0),
		ichi.KijunValues.Last(0), ichi.Length())

	balance, err := session.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		log.Warnf("get balance failed %v \r\n", err)
		return err
	}
	log.Infof("Your balance: %+v", balance)
	// here we define a kline callback
	// when a kline is closed, we will do something
	callback := func(kline types.KLine) {
		// // skip kline events that does not belong to this symbol
		// if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
		// 	return
		// }
		kumo, kumoPercent := ichi.KumoTrend(0)
		trend129 := (kline.Close.Float64() - ichi.Sup129.Last(0)) * 100 / ichi.Sup129.Last(0)
		trend65 := (kline.Close.Float64() - ichi.Sup65.Last(0)) * 100 / ichi.Sup65.Last(0)
		// log.Infof("base numOfOrders: %+v", s.OrderExecutor.CurrentPosition())
		// log.Infof("C:%.3f 65:%.3f%% 129:%.3f%% V:%.3f Tenkan: %.3f Kijun: %.3f Kumo: %s(%.3f%%) Chikou:%s",
		// 	// kline.Symbol,
		// 	kline.Close.Float64(),
		// 	trend65,
		// 	trend129,
		// 	kline.Volume.Float64(),
		// 	ichi.TenkanValues.Last(0),
		// 	ichi.KijunValues.Last(0),
		// 	kumo.String(),
		// 	kumoPercent,
		// 	ichi.ChikouTrend().String(),
		// )

		if kumo == indicatorv2.UpTrend && kumoPercent > 0.01 {
			if ichi.TenkanValues.Last(0) > ichi.KijunValues.Last(0) &&
				kline.Close.Float64() >= ichi.CloseValues.Highest(26) &&
				kline.Close.Float64() > trend65 &&
				kline.Close.Float64() > trend129 {
				opts := s.OpenPositionOptions
				opts.Long = true

				if price, ok := session.LastPrice(s.Symbol); ok {
					opts.Price = price
				}

				// opts.Price = closePrice
				if s.OrderExecutor.CurrentPosition().Base == 0 {
					if _, err := s.OrderExecutor.OpenPosition(ctx, opts); err != nil {
						logErr(err, "unable to open position")
					}
				}
			}
		}

		if kline.Close.Float64() < (ichi.KijunValues.Last(0) - ichi.KijunValues.Last(0)*0.01) {
			if err := s.OrderExecutor.ClosePosition(ctx, fixedpoint.One, "close"); err != nil {
				logErr(err, "failed to close position")
			}
		}
		// Update our counter and sync the changes to the persistence layer on time
		// If you don't do this, BBGO will sync it automatically when BBGO shuts down.
		s.State.Counter++
		bbgo.Sync(ctx, s)

		// Order handler
	}

	// register our kline event handler
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, callback))

	// session.MarketDataStream.OnKLine(func(kline types.KLine) {
	// 	kumo, kumoPercent := ichi.KumoTrend(0)
	// 	log.Infof("C:%.3f H:%.3f L:%.3f V:%.3f Tenkan: %.3f Kijun: %.3f Kumo: %s(%.3f%%) Chikou:%s",
	// 		// kline.Symbol,
	// 		kline.Close.Float64(),
	// 		kline.High.Float64(),
	// 		kline.Low.Float64(),
	// 		kline.Volume.Float64(),
	// 		ichi.TenkanValues.Last(0),
	// 		ichi.KijunValues.Last(0),
	// 		kumo.String(),
	// 		kumoPercent,
	// 		ichi.ChikouTrend().String(),
	// 	)
	// })
	session.UserDataStream.OnBalanceUpdate(func(b types.BalanceMap) {
		log.Infof("balance: %+v", b)
	})
	// if you need to do something when the user data stream is ready
	// note that you only receive order update, trade update, balance update when the user data stream is connect.
	session.UserDataStream.OnStart(func() {
		log.Infof("connected")
	})

	session.UserDataStream.OnOrderUpdate(func(order types.Order) {
		if order.Status == types.OrderStatusFilled {
			// log.Infof("your order is filled: %+v", order)
			s.profit.Update(order.Price.Float64() * order.Quantity.Float64())
			if order.Tag == "close" {
				log.Infof("Your profit: %.03f", s.profit.Last(0)-s.profit.Last(1))
			}
		}
	})

	session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		// log.Infof("trade price %f, fee %f %s", trade.Price.Float64(), trade.Fee.Float64(), trade.FeeCurrency)
	})

	return nil
}

func (s *Strategy) CompleteIchi(i *indicatorv2.IchimokuStream, session *bbgo.ExchangeSession) error {
	if i != nil {
		if i.Length() == 0 {
			limit := indicatorv2.IchimokuMaxRequired
			now := time.Now()
			klines, err := session.Exchange.QueryKLines(context.Background(), s.Symbol, s.Interval, types.KLineQueryOptions{
				Limit:   limit,
				EndTime: &now,
			})
			if err != nil {
				return err
			}
			for _, k := range klines {
				i.Update(k)
			}
		}
	}
	return nil
}

func logErr(err error, msgAndArgs ...interface{}) bool {
	if err == nil {
		return false
	}

	if len(msgAndArgs) == 0 {
		log.WithError(err).Error(err.Error())
	} else if len(msgAndArgs) == 1 {
		msg := msgAndArgs[0].(string)
		log.WithError(err).Error(msg)
	} else if len(msgAndArgs) > 1 {
		msg := msgAndArgs[0].(string)
		log.WithError(err).Errorf(msg, msgAndArgs[1:]...)
	}

	return true
}
