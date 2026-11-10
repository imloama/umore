package repository

import (
	"github.com/imloama/umore/internal/model"

	"gorm.io/gorm"
)

// GetFinances 按日期区间查该用户的收支流水
func (r *Repository) GetFinances(userID int, startDate, endDate string) ([]model.Finance, error) {
	finances := []model.Finance{}
	err := r.db.Where("user_id = ? AND date BETWEEN ? AND ?", userID, startDate, endDate).
		Order("date DESC, id DESC").Find(&finances).Error
	if err != nil {
		return nil, err
	}
	return finances, nil
}

// CreateFinance 记一笔收支
func (r *Repository) CreateFinance(userID int, f *model.Finance) error {
	if err := r.checkProjectOwnership(userID, f.ProjectID); err != nil {
		return err
	}
	f.UserID = userID
	f.ID = 0
	return r.db.Create(f).Error
}

// UpdateFinance 改一笔收支
func (r *Repository) UpdateFinance(userID int, f *model.Finance) error {
	if err := r.checkProjectOwnership(userID, f.ProjectID); err != nil {
		return err
	}
	res := r.db.Model(&model.Finance{}).Where("user_id = ? AND id = ?", userID, f.ID).
		Select("type", "amount", "category", "description", "date", "project_id").Updates(f)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteFinance 删一笔收支；不是自己的当作不存在
func (r *Repository) DeleteFinance(userID, id int) error {
	res := r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&model.Finance{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// FinanceSummary 区间汇总。金额是 numeric，累加走 decimal，别用浮点对账。
type FinanceSummary struct {
	TotalIncome  model.Amount `json:"total_income"`
	TotalExpense model.Amount `json:"total_expense"`
	Balance      model.Amount `json:"balance"`
}

// GetFinanceSummary 收入 / 支出 / 结余
func (r *Repository) GetFinanceSummary(userID int, startDate, endDate string) (*FinanceSummary, error) {
	var row struct {
		Income  model.Amount
		Expense model.Amount
	}
	err := r.db.Model(&model.Finance{}).
		Where("user_id = ? AND date BETWEEN ? AND ?", userID, startDate, endDate).
		Select(`COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0) AS income,
			COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0) AS expense`).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	income := row.Income.Normalize()
	expense := row.Expense.Normalize()
	return &FinanceSummary{
		TotalIncome:  income,
		TotalExpense: expense,
		Balance:      income.Sub(expense),
	}, nil
}
