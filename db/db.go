package db

import (
	"database/sql"

	_ "github.com/glebarez/sqlite"
)

type DB struct {
	*sql.DB
}

type Location struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Item struct {
	ID         int            `json:"id"`
	Name       string         `json:"name"`
	Count      int            `json:"count"`
	LocationID int            `json:"location_id"`
	CreatedAt  sql.NullString `json:"created_at"`
	DeletedAt  sql.NullString `json:"deleted_at"`
}

func InitDB(filepath string) (*DB, error) {
	db, err := sql.Open("sqlite", filepath)
	if err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) Migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS locations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE
	);
	CREATE TABLE IF NOT EXISTS items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		count INTEGER NOT NULL DEFAULT 0,
		location_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		deleted_at DATETIME NULL,
		FOREIGN KEY (location_id) REFERENCES locations(id) ON DELETE CASCADE
	);`
	_, err := db.Exec(query)
	return err
}

// Functions for CRUD operations

func (db *DB) GetLocations() ([]Location, error) {
	rows, err := db.Query("SELECT id, name FROM locations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locations []Location
	for rows.Next() {
		var loc Location
		if err := rows.Scan(&loc.ID, &loc.Name); err != nil {
			return nil, err
		}
		locations = append(locations, loc)
	}
	return locations, nil
}

func (db *DB) GetLocation(id int) (*Location, error) {
	var loc Location
	err := db.QueryRow("SELECT id, name FROM locations WHERE id = ?", id).Scan(&loc.ID, &loc.Name)
	if err != nil {
		return nil, err
	}
	return &loc, nil
}

func (db *DB) AddLocation(name string) error {
	_, err := db.Exec("INSERT INTO locations (name) VALUES (?)", name)
	return err
}

func (db *DB) RenameLocation(id int, newName string) error {
	_, err := db.Exec("UPDATE locations SET name = ? WHERE id = ?", newName, id)
	return err
}

func (db *DB) DeleteLocation(id int) error {
	_, err := db.Exec("DELETE FROM locations WHERE id = ?", id)
	return err
}

func (db *DB) GetItems(locationID int) ([]Item, error) {
	rows, err := db.Query("SELECT id, name, count, location_id, created_at, deleted_at FROM items WHERE location_id = ? AND deleted_at IS NULL", locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Count, &item.LocationID, &item.CreatedAt, &item.DeletedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (db *DB) GetAllItemsForLocation(locationID int) ([]Item, error) {
	rows, err := db.Query("SELECT id, name, count, location_id, created_at, deleted_at FROM items WHERE location_id = ? ORDER BY created_at DESC", locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Count, &item.LocationID, &item.CreatedAt, &item.DeletedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (db *DB) GetItem(id int) (*Item, error) {
	var item Item
	err := db.QueryRow("SELECT id, name, count, location_id, created_at, deleted_at FROM items WHERE id = ?", id).Scan(&item.ID, &item.Name, &item.Count, &item.LocationID, &item.CreatedAt, &item.DeletedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (db *DB) AddItem(name string, count int, locationID int) error {
	// Check if item with same name exists in location
	var existingID int
	var existingCount int
	err := db.QueryRow("SELECT id, count FROM items WHERE name = ? AND location_id = ?", name, locationID).Scan(&existingID, &existingCount)
	if err == nil {
		// Exists, update count
		_, err = db.Exec("UPDATE items SET count = ? WHERE id = ?", existingCount+count, existingID)
		return err
	} else if err == sql.ErrNoRows {
		// Not exists, insert new
		_, err = db.Exec("INSERT INTO items (name, count, location_id) VALUES (?, ?, ?)", name, count, locationID)
		return err
	} else {
		return err
	}
}

func (db *DB) UpdateItem(id int, name string, count int) error {
	_, err := db.Exec("UPDATE items SET name = ?, count = ? WHERE id = ?", name, count, id)
	return err
}

func (db *DB) DeleteItem(id int) error {
	_, err := db.Exec("UPDATE items SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	return err
}

func (db *DB) IncreaseItemCount(id int) error {
	_, err := db.Exec("UPDATE items SET count = count + 1 WHERE id = ?", id)
	return err
}

func (db *DB) DecreaseItemCount(id int) error {
	_, err := db.Exec("UPDATE items SET count = MAX(0, count - 1) WHERE id = ?", id)
	return err
}

// For the map view, perhaps get all locations with their items
type LocationWithItems struct {
	Location Location `json:"location"`
	Items    []Item   `json:"items"`
}

type ItemWithLocation struct {
	Item     Item     `json:"item"`
	Location Location `json:"location"`
}

func (db *DB) GetAllLocationsWithItems() ([]LocationWithItems, error) {
	locations, err := db.GetLocations()
	if err != nil {
		return nil, err
	}

	var result []LocationWithItems
	for _, loc := range locations {
		items, err := db.GetItems(loc.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, LocationWithItems{
			Location: loc,
			Items:    items,
		})
	}
	return result, nil
}

func (db *DB) SearchItems(query string) ([]ItemWithLocation, error) {
	rows, err := db.Query(`
		SELECT i.id, i.name, i.count, i.location_id, i.created_at, i.deleted_at, l.id, l.name
		FROM items i
		JOIN locations l ON i.location_id = l.id
		WHERE i.name LIKE ? AND i.deleted_at IS NULL
		ORDER BY l.name, i.name`, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ItemWithLocation
	for rows.Next() {
		var item Item
		var loc Location
		err := rows.Scan(&item.ID, &item.Name, &item.Count, &item.LocationID, &item.CreatedAt, &item.DeletedAt, &loc.ID, &loc.Name)
		if err != nil {
			return nil, err
		}
		results = append(results, ItemWithLocation{
			Item:     item,
			Location: loc,
		})
	}
	return results, nil
}