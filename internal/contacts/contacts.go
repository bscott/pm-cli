package contacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Contact represents a single address book entry.
type Contact struct {
	Email   string    `json:"email"`
	Name    string    `json:"name,omitempty"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// Store manages the contacts address book.
type Store struct {
	Contacts []Contact `json:"contacts"`
	path     string
}

// contactsPath returns the path to the contacts JSON file.
func contactsPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}
	return filepath.Join(configDir, "pm-cli", "contacts.json"), nil
}

// Load reads the contacts store from disk.
func Load() (*Store, error) {
	path, err := contactsPath()
	if err != nil {
		return nil, err
	}

	store := &Store{
		Contacts: []Contact{},
		path:     path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, fmt.Errorf("failed to read contacts: %w", err)
	}

	if err := json.Unmarshal(data, store); err != nil {
		return nil, fmt.Errorf("failed to parse contacts: %w", err)
	}

	store.path = path
	return store, nil
}

// Save writes the contacts store to disk.
func (s *Store) Save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal contacts: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write contacts: %w", err)
	}

	return nil
}

// List returns all contacts, optionally sorted by name or email.
func (s *Store) List() []Contact {
	contacts := make([]Contact, len(s.Contacts))
	copy(contacts, s.Contacts)

	// Sort by name (or email if no name)
	sort.Slice(contacts, func(i, j int) bool {
		nameI := contacts[i].Name
		if nameI == "" {
			nameI = contacts[i].Email
		}
		nameJ := contacts[j].Name
		if nameJ == "" {
			nameJ = contacts[j].Email
		}
		return strings.ToLower(nameI) < strings.ToLower(nameJ)
	})

	return contacts
}

// Search finds contacts matching the query string.
// Searches both name and email fields (case-insensitive).
func (s *Store) Search(query string) []Contact {
	if query == "" {
		return s.List()
	}

	query = strings.ToLower(query)
	var results []Contact

	for _, c := range s.Contacts {
		if strings.Contains(strings.ToLower(c.Email), query) ||
			strings.Contains(strings.ToLower(c.Name), query) {
			results = append(results, c)
		}
	}

	// Sort results by relevance (exact matches first)
	sort.Slice(results, func(i, j int) bool {
		// Exact email match has highest priority
		if strings.EqualFold(results[i].Email, query) {
			return true
		}
		if strings.EqualFold(results[j].Email, query) {
			return false
		}
		// Then by name
		nameI := results[i].Name
		if nameI == "" {
			nameI = results[i].Email
		}
		nameJ := results[j].Name
		if nameJ == "" {
			nameJ = results[j].Email
		}
		return strings.ToLower(nameI) < strings.ToLower(nameJ)
	})

	return results
}

// Add creates a new contact. Returns error if email already exists.
func (s *Store) Add(email, name string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)

	if email == "" {
		return fmt.Errorf("email is required")
	}

	// Check for duplicates
	for _, c := range s.Contacts {
		if strings.EqualFold(c.Email, email) {
			return fmt.Errorf("contact with email %s already exists", email)
		}
	}

	now := time.Now()
	s.Contacts = append(s.Contacts, Contact{
		Email:   email,
		Name:    name,
		Created: now,
		Updated: now,
	})

	return s.Save()
}

// Remove deletes a contact by email. Returns error if not found.
func (s *Store) Remove(email string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	for i, c := range s.Contacts {
		if strings.EqualFold(c.Email, email) {
			s.Contacts = append(s.Contacts[:i], s.Contacts[i+1:]...)
			return s.Save()
		}
	}

	return fmt.Errorf("contact with email %s not found", email)
}

// Get retrieves a contact by email. Returns nil if not found.
func (s *Store) Get(email string) *Contact {
	email = strings.TrimSpace(strings.ToLower(email))

	for _, c := range s.Contacts {
		if strings.EqualFold(c.Email, email) {
			return &c
		}
	}

	return nil
}

// Update modifies an existing contact's name.
func (s *Store) Update(email, name string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)

	for i, c := range s.Contacts {
		if strings.EqualFold(c.Email, email) {
			s.Contacts[i].Name = name
			s.Contacts[i].Updated = time.Now()
			return s.Save()
		}
	}

	return fmt.Errorf("contact with email %s not found", email)
}

// Count returns the number of contacts.
func (s *Store) Count() int {
	return len(s.Contacts)
}
