package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

const TenkanPeriod = 9
const KijunPeriod = 26

type Ichimoku struct {
	*types.Float64Series
	HighValues   *types.Queue
	LowValues    *types.Queue
	TenkanValues *types.Queue
	KijunValues  *types.Queue
}

func (ich *Ichimoku) Update(kLine types.KLine) {
	if ich.HighValues == nil {
		ich.HighValues = types.NewQueue(KijunPeriod)
		ich.LowValues = types.NewQueue(KijunPeriod)
		ich.TenkanValues = types.NewQueue(TenkanPeriod)
		ich.KijunValues = types.NewQueue(KijunPeriod)
	}
	ich.HighValues.Update(kLine.High.Float64())
	ich.LowValues.Update(kLine.Low.Float64())

	if ich.HighValues.Length() >= 9 {
		ich.TenkanValues.Update((ich.HighValues.Highest(9) + ich.LowValues.Lowest(9)) / 2)
		if ich.HighValues.Length() >= 26 {
			ich.KijunValues.Update((ich.HighValues.Highest(26) + ich.LowValues.Lowest(26)) / 2)
		}
	}
}

func (ich *Ichimoku) Length() int {
	if ich.KijunValues == nil {
		return 0
	}
	return ich.KijunValues.Length()
}
