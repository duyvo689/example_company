package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"example-company/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, dsn string, verbose bool) (*gorm.DB, error) {
	logMode := logger.Silent
	if verbose {
		logMode = logger.Warn
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logMode),
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&domain.Company{}, &domain.Location{}, &domain.User{}, &domain.Role{},
		&domain.Membership{}, &domain.Patient{}, &domain.PatientLocation{},
		&domain.LocationAssignment{}, &domain.Order{}, &domain.AuditLog{},
		&domain.IntegrationEvent{}, &domain.RefreshToken{},
	)
}

func Seed(db *gorm.DB) error {
	roles := []domain.Role{
		{Code: "ADMIN", Scope: domain.ScopeCompany, Description: "Company administrator"},
		{Code: "STAFF", Scope: domain.ScopeCompany, Description: "Company staff"},
		{Code: "CLINICIAN", Scope: domain.ScopeLocation, Description: "Location-scoped clinician"},
	}
	for i := range roles {
		if err := db.Where("code = ?", roles[i].Code).FirstOrCreate(&roles[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func SQLDB(db *gorm.DB) (*sql.DB, error) { return db.DB() }
