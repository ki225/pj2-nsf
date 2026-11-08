package nsfContext

import (
	"nsf/internal/db"
	"nsf/logger"
	"nsf/model"
)

type NsfContextIE struct {
	DbType  string
	MongoDB model.MongoDBIE

	*logger.NsfLogger
}

type NsfContext struct {
	*serviceContext
	*dbContext

	*logger.NsfLogger
}

func NewNsfContext(nsfContextIE *NsfContextIE) *NsfContext {
	dbContext, err := newNsfDbContext(nsfContextIE)
	if err != nil {
		nsfContextIE.NsfLogger.CtxLog.Errorf("Failed to create dbContext: %v", err)
		return nil
	}
	serviceContext := newServiceContext(&serviceContextIE{
		DbIf: dbContext.db,

		NsfLogger: nsfContextIE.NsfLogger,
	})

	if serviceContext == nil {
		dbContext.release()
		return nil
	}

	return &NsfContext{
		serviceContext: serviceContext,
		dbContext:      dbContext,

		NsfLogger: nsfContextIE.NsfLogger,
	}
}

// Keep dbContext.go unchanged while supplying the optional custom DB settings.
func newNsfDbContext(nsfContextIE *NsfContextIE) (*dbContext, error) {
	if nsfContextIE.DbType != "custom" {
		return newDbContext(&dbContextIE{
			dbType:    nsfContextIE.DbType,
			NsfLogger: nsfContextIE.NsfLogger,
		})
	}
	store, err := db.NewDb("custom", nsfContextIE.MongoDB)
	if err != nil {
		return nil, err
	}
	return &dbContext{db: store, NsfLogger: nsfContextIE.NsfLogger}, nil
}

func (c *NsfContext) Release() {
	c.NsfLogger.CtxLog.Infoln("Release NsfContext...")

	c.serviceContext.release()
	c.dbContext.release()

	c.NsfLogger.CtxLog.Infoln("NsfContext released")
}

func (c *NsfContext) RegisterServices(nfName, address string, port int, services []string) error {
	return c.serviceContext.registerServices(nfName, address, port, services)
}

func (c *NsfContext) DeregisterServices(nfName string, services []string) error {
	return c.serviceContext.deregisterServices(nfName, services)
}

func (c *NsfContext) DiscoverService(service string) (string, string, int, error) {
	return c.serviceContext.discoverService(service)
}
