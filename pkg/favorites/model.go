package favorites

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// AlternativesColumn a Gorm custom-column to store primary and alternative translations as JSON while in memory as an []string.
type AlternativesColumn []string

func (a AlternativesColumn) Value() (driver.Value, error) {
	str, err := json.Marshal(a)
	return driver.Value(str), err
}

func (a *AlternativesColumn) Scan(src interface{}) error {
	if src == nil {
		return fmt.Errorf("cannot scan nil value")
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("cannot scan value of type %T", v)
	}
	var err error
	if len(b) > 0 {
		arr := []string{}
		err = json.Unmarshal(b, &arr)
		*a = arr
	}
	return err
}

func (c *AlternativesColumn) GormDataType() string {
	return "json"
}

func (c *AlternativesColumn) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	return "JSON"
}

// Favorite a table to store favorite translations
type Favorite struct {
	gorm.Model
	// SourceLang the language of the Source
	SourceLang string `json:"sourceLang"`
	// TargetLang the language of the target
	TargetLang string `json:"targetLang"`
	// Source the source language text prior to translation
	Source string `json:"source" gorm:"unique;not null"`
	// Target JSON string array of translated text and all alternates
	Target AlternativesColumn `json:"target" gorm:"type:text"`
}
