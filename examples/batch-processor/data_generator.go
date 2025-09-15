package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"
)

// DataGenerator creates sample data files
type DataGenerator struct {
	filename string
	file     *os.File
}

// NewDataGenerator creates a new data generator
func NewDataGenerator(filename string) *DataGenerator {
	return &DataGenerator{
		filename: filename,
	}
}

// GenerateRecords creates sample records
func (g *DataGenerator) GenerateRecords(count int) error {
	file, err := os.Create(g.filename)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	g.file = file

	recordTypes := []string{"user", "transaction", "analytics", "inventory", "audit"}

	for i := 0; i < count; i++ {
		record := g.generateRecord(i, recordTypes[rand.Intn(len(recordTypes))])

		data, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}

		if _, err := g.file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}

	return nil
}

// generateRecord creates a single record
func (g *DataGenerator) generateRecord(id int, recordType string) Record {
	record := Record{
		ID:        fmt.Sprintf("%s-%06d", recordType, id),
		Type:      recordType,
		Timestamp: time.Now().Add(-time.Duration(rand.Intn(3600)) * time.Second),
		Data:      make(map[string]interface{}),
	}

	// Add type-specific data
	switch recordType {
	case "user":
		record.Data["username"] = fmt.Sprintf("user_%d", rand.Intn(10000))
		record.Data["email"] = fmt.Sprintf("user%d@example.com", rand.Intn(10000))
		record.Data["age"] = 18 + rand.Intn(60)
		record.Data["country"] = g.randomCountry()
		record.Data["premium"] = rand.Float32() < 0.2

	case "transaction":
		record.Data["amount"] = rand.Float64() * 1000
		record.Data["currency"] = g.randomCurrency()
		record.Data["merchant"] = fmt.Sprintf("merchant_%d", rand.Intn(100))
		record.Data["status"] = g.randomTransactionStatus()
		record.Data["payment_method"] = g.randomPaymentMethod()

	case "analytics":
		record.Data["event"] = g.randomEvent()
		record.Data["page"] = g.randomPage()
		record.Data["duration"] = rand.Intn(300)
		record.Data["browser"] = g.randomBrowser()
		record.Data["referrer"] = g.randomReferrer()

	case "inventory":
		record.Data["sku"] = fmt.Sprintf("SKU-%05d", rand.Intn(10000))
		record.Data["quantity"] = rand.Intn(1000)
		record.Data["warehouse"] = fmt.Sprintf("WH-%02d", rand.Intn(10))
		record.Data["reorder_point"] = rand.Intn(100)
		record.Data["unit_cost"] = rand.Float64() * 100

	case "audit":
		record.Data["action"] = g.randomAction()
		record.Data["user_id"] = fmt.Sprintf("user_%d", rand.Intn(1000))
		record.Data["ip_address"] = g.randomIP()
		record.Data["success"] = rand.Float32() < 0.95
		record.Data["details"] = fmt.Sprintf("Audit details for action %d", id)
	}

	// Add common fields
	record.Data["version"] = "1.0"
	record.Data["source"] = "batch_processor"
	record.Data["environment"] = g.randomEnvironment()

	// Occasionally add extra data to simulate variable record sizes
	if rand.Float32() < 0.1 {
		record.Data["extra_data"] = g.generateExtraData()
	}

	return record
}

// Helper functions for generating random data
func (g *DataGenerator) randomCountry() string {
	countries := []string{"USA", "UK", "Canada", "Germany", "France", "Japan", "Australia", "Brazil", "India", "China"}
	return countries[rand.Intn(len(countries))]
}

func (g *DataGenerator) randomCurrency() string {
	currencies := []string{"USD", "EUR", "GBP", "JPY", "CAD", "AUD"}
	return currencies[rand.Intn(len(currencies))]
}

func (g *DataGenerator) randomTransactionStatus() string {
	statuses := []string{"completed", "pending", "failed", "refunded", "cancelled"}
	return statuses[rand.Intn(len(statuses))]
}

func (g *DataGenerator) randomPaymentMethod() string {
	methods := []string{"credit_card", "debit_card", "paypal", "bank_transfer", "crypto"}
	return methods[rand.Intn(len(methods))]
}

func (g *DataGenerator) randomEvent() string {
	events := []string{"page_view", "click", "purchase", "signup", "login", "logout", "search", "share"}
	return events[rand.Intn(len(events))]
}

func (g *DataGenerator) randomPage() string {
	pages := []string{"/home", "/products", "/checkout", "/account", "/search", "/about", "/contact"}
	return pages[rand.Intn(len(pages))]
}

func (g *DataGenerator) randomBrowser() string {
	browsers := []string{"Chrome", "Firefox", "Safari", "Edge", "Opera"}
	return browsers[rand.Intn(len(browsers))]
}

func (g *DataGenerator) randomReferrer() string {
	referrers := []string{"google.com", "facebook.com", "twitter.com", "direct", "reddit.com", "linkedin.com"}
	return referrers[rand.Intn(len(referrers))]
}

func (g *DataGenerator) randomAction() string {
	actions := []string{"login", "logout", "create", "update", "delete", "view", "export", "import"}
	return actions[rand.Intn(len(actions))]
}

func (g *DataGenerator) randomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

func (g *DataGenerator) randomEnvironment() string {
	environments := []string{"production", "staging", "development"}
	// Weight towards production
	if rand.Float32() < 0.7 {
		return "production"
	}
	return environments[rand.Intn(len(environments))]
}

func (g *DataGenerator) generateExtraData() map[string]interface{} {
	extra := make(map[string]interface{})

	// Random number of extra fields
	numFields := 1 + rand.Intn(5)

	for i := 0; i < numFields; i++ {
		key := fmt.Sprintf("field_%d", i)

		// Random value types
		switch rand.Intn(4) {
		case 0:
			extra[key] = rand.Intn(1000)
		case 1:
			extra[key] = rand.Float64() * 100
		case 2:
			extra[key] = fmt.Sprintf("value_%d", rand.Intn(100))
		case 3:
			extra[key] = rand.Float32() < 0.5
		}
	}

	return extra
}
