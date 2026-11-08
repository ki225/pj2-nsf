package nsfContext

import (
	"encoding/json"
	"fmt"
	"nsf/internal/db"
	"nsf/logger"
	"sync"
)

// TODO: You may need to add some fields
type serviceContextIE struct {
	db.DbIf

	*logger.NsfLogger
}

// TODO: You may need to add some fields
type serviceContext struct {
	db.DbIf

	*logger.NsfLogger

	mu       sync.RWMutex
	services map[string]struct {
		NfName     string `json:"nf_name"`
		SbiAddress string `json:"sbi_address"`
		SbiPort    int    `json:"sbi_port"`
	}
}

// TODO: Finish the initialization of the serviceContext instance
func newServiceContext(serviceContextIE *serviceContextIE) *serviceContext {

	s := &serviceContext{
		DbIf: serviceContextIE.DbIf,

		NsfLogger: serviceContextIE.NsfLogger,
	}
	s.services = make(map[string]struct {
		NfName     string `json:"nf_name"`
		SbiAddress string `json:"sbi_address"`
		SbiPort    int    `json:"sbi_port"`
	})
	data, err := s.DbIf.LoadAll()
	if err != nil {
		s.NsfLogger.SvcLog.Errorf("Failed to load services: %v", err)
		return nil
	}
	for service, value := range data {
		record := s.services[service]
		if err := json.Unmarshal([]byte(value), &record); err != nil {
			s.NsfLogger.SvcLog.Errorf("Failed to decode service %s: %v", service, err)
			return nil
		}
		s.services[service] = record
	}
	return s
}

// TODO: Finish the release of the serviceContext instance
// The owning dbContext closes the database; service records remain persisted.
func (s *serviceContext) release() {
	s.NsfLogger.SvcLog.Infoln("Release serviceContext...")

	s.mu.Lock()
	s.services = nil
	s.mu.Unlock()

	s.NsfLogger.SvcLog.Infoln("serviceContext released")
}

// TODO: Finish the implementation of the registerServices method with an error return type
func (s *serviceContext) registerServices(nfName, sbiAddr string, sbiPort int, services []string) error {
	s.NsfLogger.SvcLog.Tracef("Registering %s services", nfName)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.services == nil {
		return fmt.Errorf("serviceContext is released")
	}

	record := struct {
		NfName     string `json:"nf_name"`
		SbiAddress string `json:"sbi_address"`
		SbiPort    int    `json:"sbi_port"`
	}{}
	record.NfName = nfName
	record.SbiAddress = sbiAddr
	record.SbiPort = sbiPort
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode service record: %w", err)
	}
	data := make(map[string]string, len(services))
	for _, service := range services {
		data[service] = string(encoded)
	}
	if err := s.DbIf.SaveAll(data); err != nil {
		return fmt.Errorf("save services for %s: %w", nfName, err)
	}
	for _, service := range services {
		s.services[service] = record
	}
	return nil
}

// TODO: Finish the implementation of the deregisterServices method with an error return type
func (s *serviceContext) deregisterServices(nfName string, services []string) error {
	s.NsfLogger.SvcLog.Tracef("De-registering %s services", nfName)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.services == nil {
		return fmt.Errorf("serviceContext is released")
	}

	// Check all owners before deleting any service.
	for _, service := range services {
		record, exists := s.services[service]
		if !exists {
			return fmt.Errorf("service %s not found", service)
		}
		if record.NfName != nfName {
			return fmt.Errorf("service %s belongs to %s, not %s", service, record.NfName, nfName)
		}
	}
	for _, service := range services {
		if err := s.DbIf.Remove(service); err != nil {
			return fmt.Errorf("remove service %s: %w", service, err)
		}
		delete(s.services, service)
	}
	return nil
}

// TODO: Finish the implementation of the discoverService method with these return values:
// (string, string, int, error) representing the NF name, SBI address, SBI port, and an error if any
func (s *serviceContext) discoverService(service string) (string, string, int, error) {
	s.NsfLogger.SvcLog.Tracef("Discovering service %s", service)

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.services == nil {
		return "", "", 0, fmt.Errorf("serviceContext is released")
	}
	record, exists := s.services[service]
	if !exists {
		return "", "", 0, fmt.Errorf("service %s not found", service)
	}
	return record.NfName, record.SbiAddress, record.SbiPort, nil
}
