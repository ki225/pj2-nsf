package sbi

import (
	"context"
	"net/http"
	"nsf/internal/sbi/consumer"
	"nsf/internal/sbi/processor"
	"nsf/logger"
	"nsf/model"
	"time"

	"github.com/gin-gonic/gin"
)

type ServerIE struct {
	SbiAddress string

	DbType  string
	MongoDB model.MongoDBIE

	*logger.NsfLogger
}

type Server struct {
	*consumer.Consumer
	*processor.Processor

	router *gin.Engine
	server *http.Server

	nsfAddress string

	*logger.NsfLogger
}

func NewServer(serverIE *ServerIE) *Server {
	cons := consumer.NewConsumer(&consumer.ConsumerIE{
		NsfLogger: serverIE.NsfLogger,
	})
	proc := processor.NewProcessor(&processor.ProcessorIE{
		Consumer: cons,

		DbType:  serverIE.DbType,
		MongoDB: serverIE.MongoDB,

		NsfLogger: serverIE.NsfLogger,
	})

	return &Server{
		Consumer:  cons,
		Processor: proc,

		router: nil,
		server: nil,

		nsfAddress: serverIE.SbiAddress,

		NsfLogger: serverIE.NsfLogger,
	}
}

func (s *Server) Run(ctx context.Context) error {
	s.NsfLogger.SbiLog.Infoln("Starting SBI server...")

	gin.SetMode(gin.ReleaseMode)
	s.router = gin.Default()

	addService(s)

	s.server = &http.Server{
		Addr:    s.nsfAddress,
		Handler: s.router,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.NsfLogger.SbiLog.Errorf("Failed to start SBI Server: %v", err)
		}
	}()
	time.Sleep(500 * time.Millisecond)

	s.NsfLogger.SbiLog.Infoln("SBI Server started")
	return nil
}

func (s *Server) ShutDown() {
	s.NsfLogger.SbiLog.Infoln("Shutting down SBI Server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		s.NsfLogger.SbiLog.Errorf("Failed to shut down SBI Server: %v", err)
		return
	}

	s.Consumer.Release()
	s.Processor.Release()

	s.NsfLogger.SbiLog.Infoln("SBI Server gracefully shut down")

}
