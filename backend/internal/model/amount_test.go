package model

import (
	"encoding/json"
	"testing"
)

// 金额按分累计，不能有二进制误差
func TestAmountArithmetic(t *testing.T) {
	sum := Amount{}
	for i := 0; i < 10; i++ {
		sum = sum.Add(AmountFromFloat(0.1))
	}
	if got := sum.String(); got != "1" {
		t.Fatalf("0.1 累加 10 次应为 1，实际 %s", got)
	}
	if got := Money("100").Sub(Money("33.33")).String(); got != "66.67" {
		t.Fatalf("减法不对: %s", got)
	}
	if got := Money("10.005").Round(2).String(); got != "10.01" {
		t.Fatalf("四舍五入应向上: %s", got)
	}
}

// 序列化：数字进出，空值给 0
func TestAmountJSON(t *testing.T) {
	type payload struct {
		A Amount `json:"a"`
		B Amount `json:"b"`
	}
	var p payload
	if err := json.Unmarshal([]byte(`{"a":1234.567,"b":"88.8"}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.A.String() != "1234.57" || p.B.String() != "88.8" {
		t.Fatalf("入参没截到分: a=%s b=%s", p.A, p.B)
	}

	raw, err := json.Marshal(payload{})
	if err != nil {
		t.Fatalf("marshal zero: %v", err)
	}
	if string(raw) != `{"a":0,"b":0}` {
		t.Fatalf("空金额应输出 0: %s", raw)
	}
}
