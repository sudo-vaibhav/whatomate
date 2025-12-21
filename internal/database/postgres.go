package database

import (
	"fmt"
	"time"

	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewPostgres creates a new PostgreSQL connection
func NewPostgres(cfg *config.DatabaseConfig, debug bool) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
	)

	logLevel := logger.Silent
	if debug {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	return db, nil
}

// AutoMigrate runs auto migration for all models
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		// Core models
		&models.Organization{},
		&models.User{},
		&models.APIKey{},
		&models.WhatsAppAccount{},
		&models.Contact{},
		&models.Message{},
		&models.Template{},
		&models.WhatsAppFlow{},

		// Bulk & Notifications
		&models.BulkMessageCampaign{},
		&models.BulkMessageRecipient{},
		&models.NotificationRule{},

		// Chatbot models
		&models.ChatbotSettings{},
		&models.KeywordRule{},
		&models.ChatbotFlow{},
		&models.ChatbotFlowStep{},
		&models.ChatbotSession{},
		&models.ChatbotSessionMessage{},
		&models.AIContext{},
		&models.AgentTransfer{},
	)
}

// CreateIndexes creates additional indexes not handled by GORM tags
func CreateIndexes(db *gorm.DB) error {
	indexes := []string{
		// Messages indexes
		`CREATE INDEX IF NOT EXISTS idx_messages_contact_created ON messages(contact_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id)`,

		// Contacts indexes
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_org_phone ON contacts(organization_id, phone_number)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_assigned_read ON contacts(assigned_user_id, is_read)`,

		// Sessions indexes
		`CREATE INDEX IF NOT EXISTS idx_sessions_phone_status ON chatbot_sessions(organization_id, phone_number, status)`,

		// Keyword rules indexes
		`CREATE INDEX IF NOT EXISTS idx_keyword_rules_priority ON keyword_rules(organization_id, is_enabled, priority DESC)`,

		// Agent transfers indexes
		`CREATE INDEX IF NOT EXISTS idx_agent_transfers_active ON agent_transfers(organization_id, phone_number, status)`,

		// WhatsApp accounts indexes
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_whatsapp_accounts_org_phone ON whatsapp_accounts(organization_id, phone_id)`,

		// Templates indexes
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_templates_account_name_lang ON templates(whats_app_account, name, language)`,

		// Chatbot indexes with account
		`CREATE INDEX IF NOT EXISTS idx_keyword_rules_account ON keyword_rules(whats_app_account, is_enabled, priority DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chatbot_flows_account ON chatbot_flows(whats_app_account, is_enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_contexts_account ON ai_contexts(whats_app_account, is_enabled, priority DESC)`,

		// Bulk messaging indexes
		`CREATE INDEX IF NOT EXISTS idx_bulk_campaigns_account ON bulk_message_campaigns(whats_app_account, status)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_rules_account ON notification_rules(whats_app_account, is_enabled)`,

		// Messages and contacts by account
		`CREATE INDEX IF NOT EXISTS idx_messages_account ON messages(whats_app_account, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_account ON contacts(whats_app_account)`,
	}

	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}
