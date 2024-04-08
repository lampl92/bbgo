package cloud

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
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
	// Market   types.Market
	Symbol   string         `json:"symbol"`
	State    *State         `persistence:"state"`
	Interval types.Interval `json:"interval"`
}

func (s *Strategy) Initialize() error {
	// s.Strategy = &common.Strategy{}
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
	// s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())
	if s.State == nil {
		s.State = &State{Counter: 1}
	}

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
		// if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
		// 	return
		// }

		kumo, kumoPercent := ichi.KumoTrend(0)

		log.Infof("%s C:%.3f H:%.3f L:%.3f V:%.3f Tenkan: %.3f Kijun: %.3f Kumo: %s(%.3f%%) Chikou:%s",
			kline.Symbol,
			kline.Close.Float64(),
			kline.High.Float64(),
			kline.Low.Float64(),
			kline.Volume.Float64(),
			ichi.TenkanValues.Last(0),
			ichi.KijunValues.Last(0),
			kumo.String(),
			kumoPercent,
			ichi.ChikouTrend().String(),
		)

		// Update our counter and sync the changes to the persistence layer on time
		// If you don't do this, BBGO will sync it automatically when BBGO shuts down.
		s.State.Counter++
		bbgo.Sync(ctx, s)
	}

	// register our kline event handler
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, callback))

	// session.MarketDataStream.OnKLine(func(kline types.KLine) {
	// 	log.Infof("---%s high: %.3f Low: %.3f Volume: %.3f Tenkan: %.3f Kijun: %.3f length: %d",
	// 		kline.Symbol,
	// 		s.ichi.HighValues.Last(0),
	// 		s.ichi.LowValues.Last(0),
	// 		kline.Volume.Float64(),
	// 		s.ichi.TenkanValues.Last(0), s.ichi.KijunValues.Last(0), s.ichi.Length())
	// })

	session.UserDataStream.OnBalanceUpdate(func(b types.BalanceMap) {
		log.Infof("balance: %+v", b)
	})
	// if you need to do something when the user data stream is ready
	// note that you only receive order update, trade update, balance update when the user data stream is connect.
	session.UserDataStream.OnStart(func() {
		log.Infof("connected")
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
