package entity

import (
	"fmt"
	"time"

	"github.com/getnimbus/ultrago/u_validator"
)

const (
	BlockTimeType_REALTIME = 0
	BlockTimeType_BACKFILL = 1
)

const (
	BlockTime_NOT_READY = iota
	BlockTime_PROCESSING
	BlockTime_DONE
	BlockTime_FAIL
)

type BlockTime struct {
	Id        string    `json:"id"`
	FromTs    int64     `json:"fromTs" validate:"required,min=0"`
	ToTs      int64     `json:"toTs" validate:"required,min=0"`
	Status    int       `json:"status" validate:"required"`
	Type      int       `json:"type" validate:"-"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func (b *BlockTime) Validate() error {
	if err := u_validator.Struct(b); err != nil {
		return err
	}

	if b.FromTs >= b.ToTs {
		return fmt.Errorf("invalid from ts")
	} else if b.ToTs-b.FromTs > 1800000 {
		return fmt.Errorf("time span cannot be greater than Duration(1800000ms)")
	}

	switch b.Status {
	case BlockTime_NOT_READY,
		BlockTime_PROCESSING,
		BlockTime_DONE,
		BlockTime_FAIL:
	default:
		return fmt.Errorf("invalid status %v", b.Status)
	}

	switch b.Type {
	case BlockTimeType_REALTIME,
		BlockTimeType_BACKFILL:
	default:
		return fmt.Errorf("invalid type %v", b.Type)
	}

	return nil
}
