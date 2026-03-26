package domain

import (
	"log"

	cronadoCtx "github.com/teqneers/cronado/internal/context"
)

func SanityChecks() {
	ctx := cronadoCtx.AppCtx
	cronLabelPrefix := ctx.Config.CronLabelPrefix
	if cronLabelPrefix == "" {
		log.Panicln("cronLabelPrefix is empty, exiting ...")
	}
}
