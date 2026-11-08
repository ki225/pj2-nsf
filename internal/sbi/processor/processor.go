package processor

import (
	"nsf/internal/nsfContext"
	"nsf/internal/sbi/consumer"
	"nsf/logger"
	"nsf/model"
)

type ProcessorIE struct {
	*consumer.Consumer

	DbType  string
	MongoDB model.MongoDBIE

	*logger.NsfLogger
}

type Processor struct {
	*nsfContext.NsfContext
	*consumer.Consumer

	*logger.NsfLogger
}

func NewProcessor(processorIE *ProcessorIE) *Processor {
	nsfCtx := nsfContext.NewNsfContext(&nsfContext.NsfContextIE{
		DbType:  processorIE.DbType,
		MongoDB: processorIE.MongoDB,

		NsfLogger: processorIE.NsfLogger,
	})

	return &Processor{
		NsfContext: nsfCtx,
		Consumer:   processorIE.Consumer,

		NsfLogger: processorIE.NsfLogger,
	}
}

func (p *Processor) Release() {
	p.NsfLogger.ProcLog.Infoln("Release processor...")

	p.NsfContext.Release()

	p.NsfLogger.ProcLog.Infoln("Processor released")
}
