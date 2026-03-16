package repository

import "github.com/RAF-SI-2025/EXBanka-3-Backend/client-service/internal/models"

// ClientRepositoryInterface defines the methods used by ClientService.
type ClientRepositoryInterface interface {
	Create(client *models.Client) error
	FindByID(id uint) (*models.Client, error)
	List(filter ClientFilter) ([]models.Client, int64, error)
	Update(client *models.Client) error
	EmailExists(email string, excludeID uint) (bool, error)
	SetPermissions(client *models.Client, permissions []models.Permission) error
}

// PermissionRepositoryInterface defines the methods used by ClientService.
type PermissionRepositoryInterface interface {
	FindByNamesForSubject(names []string, subjectType string) ([]models.Permission, error)
}

var _ ClientRepositoryInterface = (*ClientRepository)(nil)
var _ PermissionRepositoryInterface = (*PermissionRepository)(nil)
