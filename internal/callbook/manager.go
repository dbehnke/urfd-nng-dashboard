package callbook

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dbehnke/urfd-nng-dashboard/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RadioIDUrl = "https://database.radioid.net/database/dump/user.csv"
)

type Manager struct {
	db      *gorm.DB
	qrzUser string
	qrzPass string
}

func NewManager(s *store.Store, qrzUser, qrzPass string) *Manager {
	return &Manager{
		db:      s.DB,
		qrzUser: qrzUser,
		qrzPass: qrzPass,
	}
}

func (m *Manager) Start() {
	go func() {
		// Initial Sync
		if err := m.SyncRadioID(); err != nil {
			log.Printf("Error syncing RadioID callbook: %v", err)
		}
		// Initial Prune
		m.PruneCache()

		// Daily Timer
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			if err := m.SyncRadioID(); err != nil {
				log.Printf("Error syncing RadioID callbook: %v", err)
			}
			m.PruneCache()
		}
	}()
}

func (m *Manager) PruneCache() {
	// Delete external records older than 30 days
	res := m.db.Where("source = ? AND updated_at < ?", "external", time.Now().AddDate(0, 0, -30)).Delete(&store.Callbook{})
	if res.Error != nil {
		log.Printf("Error pruning callbook cache: %v", res.Error)
	} else if res.RowsAffected > 0 {
		log.Printf("Pruned %d stale callbook records", res.RowsAffected)
	}
}

func (m *Manager) SyncRadioID() error {
	log.Println("Starting RadioID Callbook download...")

	resp, err := http.Get(RadioIDUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	reader := csv.NewReader(resp.Body)
	// Read Header
	_, err = reader.Read()
	if err != nil {
		return err
	}

	var batch []store.Callbook
	count := 0

	// Create batches of 1000
	// RadioID CSV Headers: RADIO_ID,CALLSIGN,FIRST_NAME,LAST_NAME,CITY,STATE,COUNTRY,REMARKS
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("CSV parse error: %v", err)
			continue
		}

		if len(record) < 7 {
			continue
		}

		id, _ := strconv.ParseUint(record[0], 10, 32)
		if id == 0 {
			continue
		}

		batch = append(batch, store.Callbook{
			ID:        uint(id),
			Callsign:  record[1],
			Name:      record[2],
			Surname:   record[3],
			City:      record[4],
			State:     record[5],
			Country:   record[6],
			Remarks:   safeGet(record, 7),
			Source:    "radioid",
			UpdatedAt: time.Now(),
		})

		if len(batch) >= 1000 {
			if err := m.saveBatch(batch); err != nil {
				log.Printf("Error saving batch: %v", err)
			}
			count += len(batch)
			batch = nil
		}
	}

	if len(batch) > 0 {
		if err := m.saveBatch(batch); err != nil {
			log.Printf("Error saving final batch: %v", err)
		}
		count += len(batch)
	}

	log.Printf("RadioID Callbook Sync Complete. Imported %d records.", count)
	return nil
}

func (m *Manager) saveBatch(batch []store.Callbook) error {
	return m.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).CreateInBatches(batch, 1000).Error
}

// SafeGet handled above

func (m *Manager) Lookup(callsign string) (*store.Callbook, error) {
	// 1. Local Cache Lookup
	var book store.Callbook
	err := m.db.Where("callsign = ?", callsign).First(&book).Error
	if err == nil {
		return &book, nil // Found in DB
	}

	// 2. Fallback: HamDB
	if cb, err := m.lookupHamDB(callsign); err == nil {
		return cb, nil
	}

	// 3. Fallback: Callook (US Only)
	if cb, err := m.lookupCallook(callsign); err == nil {
		return cb, nil
	}

	// 4. Fallback: QRZ.com (XML API)
	if cb, err := m.lookupQRZ(callsign); err == nil {
		return cb, nil
	}

	return nil, fmt.Errorf("not found in any provider")
}

func (m *Manager) lookupQRZ(callsign string) (*store.Callbook, error) {
	if m.qrzUser == "" || m.qrzPass == "" {
		return nil, fmt.Errorf("QRZ credentials not configured")
	}

	// 1. Get Session Key (TODO: Cache this key?)
	// For simplicity in this v1, we login every time. QRZ allows this but caching is better.
	// Given strict verification timeline, simple first.
	loginURL := fmt.Sprintf("https://xmldata.qrz.com/xml/current/?username=%s&password=%s", m.qrzUser, m.qrzPass)
	resp, err := http.Get(loginURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var loginData QRZDatabase
	if err := xml.NewDecoder(resp.Body).Decode(&loginData); err != nil {
		return nil, err
	}

	if loginData.Session == nil || loginData.Session.Error != "" {
		errMsg := "unknown error"
		if loginData.Session != nil {
			errMsg = loginData.Session.Error
		}
		return nil, fmt.Errorf("QRZ login failed: %s", errMsg)
	}
	key := loginData.Session.Key

	// 2. Lookup Callsign
	lookupURL := fmt.Sprintf("https://xmldata.qrz.com/xml/current/?s=%s&callsign=%s", key, callsign)
	resp2, err := http.Get(lookupURL)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()

	var data QRZDatabase
	if err := xml.NewDecoder(resp2.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Callsign == nil {
		return nil, fmt.Errorf("QRZ not found")
	}

	// Parse Name/Surname
	// QRZ 'name' is usually Last Name, 'fname' is First Name
	fname := data.Callsign.Fname
	sname := data.Callsign.Name

	// Sometimes 'name' is full name if fname is empty
	if fname == "" && sname != "" {
		// heavy heuristic, leave as is or split?
		// leave as surname or put in name?
		// Let's assume sname is fine.
	}

	cb := store.Callbook{
		Callsign:  strings.ToUpper(callsign),
		Name:      fname,
		Surname:   sname,
		City:      data.Callsign.Addr2,
		State:     data.Callsign.State,
		Country:   data.Callsign.Land,
		Source:    "qrz",
		UpdatedAt: time.Now(),
	}

	if err := m.db.Create(&cb).Error; err != nil {
		log.Printf("Failed to cache QRZ result: %v", err)
	}

	return &cb, nil
}

// Callook Response
type callookResponse struct {
	Status  string `json:"status"`
	Name    string `json:"name"`
	Address struct {
		Line1 string `json:"line1"`
		Line2 string `json:"line2"` // "City, State Zip"
	} `json:"address"`
}

func (m *Manager) lookupCallook(callsign string) (*store.Callbook, error) {
	url := fmt.Sprintf("https://callook.info/%s/json", callsign)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Callook status: %s", resp.Status)
	}

	var data callookResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Status != "VALID" {
		return nil, fmt.Errorf("Callook invalid or not found")
	}

	// Parsing Name: "First Last" -> Split
	fname, sname := "", ""
	parts := strings.Fields(data.Name)
	if len(parts) > 0 {
		fname = parts[0]
		if len(parts) > 1 {
			sname = strings.Join(parts[1:], " ")
		}
	}

	// Parsing Address: "City, State Zip"
	// Only safe for US addresses usually
	line2 := data.Address.Line2
	city, state := "", ""

	if strings.Contains(line2, ",") {
		addrParts := strings.Split(line2, ",")
		city = strings.TrimSpace(addrParts[0])

		// State Zip part
		if len(addrParts) > 1 {
			stateZip := strings.TrimSpace(addrParts[1])
			szParts := strings.Fields(stateZip)
			if len(szParts) > 0 {
				state = szParts[0]
			}
		}
	}

	cb := store.Callbook{
		Callsign:  strings.ToUpper(callsign),
		Name:      fname,
		Surname:   sname,
		City:      city,
		State:     state,
		Country:   "USA", // Callook is US only
		Source:    "external",
		UpdatedAt: time.Now(),
	}

	if err := m.db.Create(&cb).Error; err != nil {
		log.Printf("Failed to cache Callook result: %v", err)
	}

	return &cb, nil
}

// HamDB Response Structures
type hamDBResponse struct {
	Hamdb struct {
		Callsign struct {
			Call    string `json:"call"`
			Fname   string `json:"fname"`
			Name    string `json:"name"`  // Surname usually
			Addr2   string `json:"addr2"` // Usually "City, State Zip" or similar
			State   string `json:"state"`
			Country string `json:"country"`
		} `json:"callsign"`
		Messages struct {
			Status string `json:"status"`
		} `json:"messages"`
	} `json:"hamdb"`
}

// XML Structs for QRZ
type QRZDatabase struct {
	XMLName xml.Name `xml:"QRZDatabase"`
	Session *struct {
		Key   string `xml:"Key"`
		Error string `xml:"Error"`
	} `xml:"Session"`
	Callsign *struct {
		Call  string `xml:"call"`
		Fname string `xml:"fname"`
		Name  string `xml:"name"`
		Addr1 string `xml:"addr1"`
		Addr2 string `xml:"addr2"` // City, State Zip
		State string `xml:"state"`
		Land  string `xml:"land"` // Country
	} `xml:"Callsign"`
}

func (m *Manager) lookupHamDB(callsign string) (*store.Callbook, error) {
	url := fmt.Sprintf("http://api.hamdb.org/%s/json/urfd-dashboard", callsign)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HamDB status: %s", resp.Status)
	}

	var data hamDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Hamdb.Messages.Status != "OK" {
		return nil, fmt.Errorf("HamDB error or not found")
	}

	// Parse Logic
	c := data.Hamdb.Callsign

	// HamDB 'addr2' parsing is messy. "City, State Zip".
	// We try to extract City from it.
	city := c.Addr2
	if strings.Contains(city, ",") {
		parts := strings.Split(city, ",")
		city = strings.TrimSpace(parts[0])
	}
	// Also use explicit State/Country fields if available

	cb := store.Callbook{
		Callsign:  c.Call, // Use returned callsign to handle case correction
		Name:      c.Fname,
		Surname:   c.Name,
		City:      city,
		State:     c.State,
		Country:   c.Country,
		Source:    "external",
		UpdatedAt: time.Now(),
	}

	// Save to Cache (Source=external)
	// We don't have an ID for non-DMR users, so we let GORM/SQLite generate a primary key
	// OR we can make Callsign unique validation in logic?
	// ID is primary key... auto-increment.
	// Callbook struct definition: ID uint `gorm:"primaryKey"`.
	// It will auto-increment if ID is 0.

	if err := m.db.Create(&cb).Error; err != nil {
		log.Printf("Failed to cache HamDB result: %v", err)
	}

	return &cb, nil
}

func safeGet(arr []string, idx int) string {
	if idx < len(arr) {
		return arr[idx]
	}
	return ""
}
