package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

const TenkanPeriod = 9
const KijunPeriod = 26
const IchimokuMaxRequired = 130

type IchimokuStream struct {
	*types.Float64Series
	Symbol       string
	HighValues   *types.Queue
	LowValues    *types.Queue
	CloseValues  *types.Queue
	TenkanValues *types.Queue
	KijunValues  *types.Queue
	SpanA        *types.Queue
	SpanB        *types.Queue
	Chikou       *types.Queue
	Sup129       *types.Queue
	Sup65        *types.Queue
}

type Trend int

const (
	UpTrend   Trend = 0
	DownTrend Trend = 1
	NoTrend   Trend = 2
)

func (t Trend) String() string {
	switch t {
	case UpTrend:
		return "Up"
	case DownTrend:
		return "Down"
	case NoTrend:
		return "None"
	}
	return ""
}

func Ichimoku(source KLineSubscription) *IchimokuStream {
	s := &IchimokuStream{
		Float64Series: types.NewFloat64Series(),
		HighValues:    types.NewQueue(IchimokuMaxRequired),
		LowValues:     types.NewQueue(IchimokuMaxRequired),
		TenkanValues:  types.NewQueue(TenkanPeriod),
		KijunValues:   types.NewQueue(KijunPeriod),
		SpanA:         types.NewQueue(52),
		SpanB:         types.NewQueue(52),
		Chikou:        types.NewQueue(52),
		CloseValues:   types.NewQueue(52),
		Sup129:        types.NewQueue(129),
		Sup65:         types.NewQueue(129),
	}

	source.AddSubscriber(s.Update)
	return s
}

func (s *IchimokuStream) Update(kLine types.KLine) {
	s.Symbol = kLine.Symbol
	s.HighValues.Update(kLine.High.Float64())
	s.LowValues.Update(kLine.Low.Float64())

	if s.HighValues.Length() >= 9 {
		s.TenkanValues.Update((s.HighValues.Highest(9) + s.LowValues.Lowest(9)) / 2)
	}

	if s.HighValues.Length() >= 26 {
		s.KijunValues.Update((s.HighValues.Highest(26) + s.LowValues.Lowest(26)) / 2)
	}

	if s.KijunValues.Length() > 0 {
		s.SpanA.Update((s.TenkanValues.Last(0) + s.KijunValues.Last(0)) / 2)
	}

	if s.HighValues.Length() >= 52 {
		s.SpanB.Update((s.HighValues.Highest(52) + s.LowValues.Lowest(52)) / 2)
	}

	if s.HighValues.Length() >= 65 {
		s.Sup65.Update((s.HighValues.Highest(65) + s.LowValues.Lowest(65)) / 2)
	}

	if s.HighValues.Length() >= 129 {
		s.Sup129.Update((s.HighValues.Highest(129) + s.LowValues.Lowest(129)) / 2)
	}

	s.CloseValues.Update(kLine.Close.Float64())
	s.Chikou.Update(kLine.Close.Float64())
}

func (s *IchimokuStream) Length() int {
	if s.CloseValues == nil {
		return 0
	}
	return s.CloseValues.Length()
}

func (s *IchimokuStream) Valid() bool {
	return (s.HighValues.Length() == 52)
}

func (s *IchimokuStream) ChikouTrend() Trend {
	if s.CloseValues.Length() < 26 {
		return NoTrend
	} else {
		if s.Chikou.Last(0) > s.CloseValues.Last(25) {
			return UpTrend
		} else {
			return DownTrend
		}
	}
}

func (s *IchimokuStream) KumoTrend(last int) (Trend, float64) {
	A := s.SpanA.Last(last)
	B := s.SpanB.Last(last)
	if A > B {
		C := A - B
		return UpTrend, (C / A) * 100
	} else {
		C := B - A
		return DownTrend, (C / B) * 100
	}
}
