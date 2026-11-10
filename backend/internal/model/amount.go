package model

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

// Amount 金额。库里是 numeric(18,2)，Go 里是定点十进制。
//
// 为什么不用 float64：钱不能有二进制误差。0.1 + 0.2 在 float64 上是
// 0.30000000000000004，一堆流水加起来对不上账，用户第一眼就能发现。
// decimal 四舍五入到分之后，累加和展示都稳。
//
// SQLite 没有真正的 numeric，落库时按 REAL 存；但读写都经过这里，
// 每次赋值先四舍五入到两位小数，所以两种库上算出来的结果一致。
type Amount struct {
	decimal.Decimal
}

// Money 从字符串造金额，测试与常量用
func Money(s string) Amount {
	return Amount{decimal.RequireFromString(s)}
}

// AmountFromFloat 前端传来的 JSON 数字转金额，顺手四舍五入到分
func AmountFromFloat(f float64) Amount {
	return Amount{decimal.NewFromFloat(f).Round(2)}
}

// Add 加法，结果保持两位小数
func (a Amount) Add(b Amount) Amount { return Amount{a.Decimal.Add(b.Decimal).Round(2)} }

// Sub 减法
func (a Amount) Sub(b Amount) Amount { return Amount{a.Decimal.Sub(b.Decimal).Round(2)} }

// Normalize 归到分。
// 数据库的 SUM 走的是各库自己的数值类型：Postgres 的 numeric 是精确十进制，
// SQLite 里这类列是 REAL，聚合出来会带一点浮点尾巴（1000.3000000000001）。
// 落库前每个值都已经截到分，这里再归一次，两种库上的汇总结果就一致了。
func (a Amount) Normalize() Amount { return Amount{a.Decimal.Round(2)} }

// Neg 取负，用于「支出」这类方向相反的汇总
func (a Amount) Neg() Amount { return Amount{a.Decimal.Neg()} }

// Float 转 float64，只在算百分比这类展示场景用，别拿它做累加
func (a Amount) Float() float64 {
	f, _ := a.Decimal.Float64()
	return f
}

// MarshalJSON 输出成 JSON 数字：前端一路按 number 处理，改成字符串要牵动整个界面
func (a Amount) MarshalJSON() ([]byte, error) {
	if a.Decimal.IsZero() {
		return []byte("0"), nil
	}
	return []byte(a.Decimal.String()), nil
}

// UnmarshalJSON 兼容数字与字符串两种入参，顺便截到分
func (a *Amount) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		a.Decimal = decimal.Zero
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(raw, &num); err == nil {
		d, err := decimal.NewFromString(num.String())
		if err != nil {
			return err
		}
		a.Decimal = d.Round(2)
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return err
	}
	a.Decimal = d.Round(2)
	return nil
}
