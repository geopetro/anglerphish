package models

import "github.com/jinzhu/gorm"

// GlobalVariables holds server-wide template variable overrides for a user.
// When a field is non-empty it replaces the per-target value from groups
// during template rendering for all campaigns.
type GlobalVariables struct {
	UserId    int64  `json:"-"    gorm:"column:user_id;primary_key"`
	FirstName string `json:"first_name" gorm:"column:first_name"`
	LastName  string `json:"last_name"  gorm:"column:last_name"`
	Email     string `json:"email"      gorm:"column:email"`
	Phone     string `json:"phone"      gorm:"column:phone"`
	Position  string `json:"position"   gorm:"column:position"`
	Custom    string `json:"custom"     gorm:"column:custom"`
}

// GetGlobalVariables returns the global variable overrides for the given user.
// If no row exists yet an empty (all-fields-blank) struct is returned without error.
func GetGlobalVariables(uid int64) (GlobalVariables, error) {
	gv := GlobalVariables{UserId: uid}
	err := db.Where("user_id = ?", uid).First(&gv).Error
	if err == gorm.ErrRecordNotFound {
		return GlobalVariables{UserId: uid}, nil
	}
	return gv, err
}

// TableName tells GORM which table to use for this model.
func (gv GlobalVariables) TableName() string {
	return "global_variables"
}

// PutGlobalVariables upserts the global variable overrides for a user.
// FirstOrCreate ensures the row exists before Save updates it.
func PutGlobalVariables(gv *GlobalVariables, uid int64) error {
	gv.UserId = uid
	existing := &GlobalVariables{UserId: uid}
	if err := db.FirstOrCreate(existing, GlobalVariables{UserId: uid}).Error; err != nil {
		return err
	}
	existing.FirstName = gv.FirstName
	existing.LastName = gv.LastName
	existing.Email = gv.Email
	existing.Phone = gv.Phone
	existing.Position = gv.Position
	existing.Custom = gv.Custom
	return db.Save(existing).Error
}

// ApplyTo overrides non-empty fields in r with the global variable values.
func (gv GlobalVariables) ApplyTo(r *BaseRecipient) {
	if gv.FirstName != "" {
		r.FirstName = gv.FirstName
	}
	if gv.LastName != "" {
		r.LastName = gv.LastName
	}
	if gv.Email != "" {
		r.Email = gv.Email
	}
	if gv.Phone != "" {
		r.Phone = gv.Phone
	}
	if gv.Position != "" {
		r.Position = gv.Position
	}
	if gv.Custom != "" {
		r.Custom = gv.Custom
	}
}
