package main

import (
	"homeinventory/db"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

var database *db.DB
var tmpl *template.Template

type PageData struct {
	LocationsWithItems []db.LocationWithItems
	SearchResults      []db.ItemWithLocation
	SearchQuery        string
	IsSearch           bool
}

func main() {
	if _, err := os.Stat("inventory.db"); os.IsNotExist(err) {
		file, err := os.Create("inventory.db")
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	}

	var err error
	database, err = db.InitDB("inventory.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		log.Fatal(err)
	}

	// Insert mock data if no locations exist
	locations, err := database.GetLocations()
	if err != nil {
		log.Fatal(err)
	}
	if len(locations) == 0 {
		// Add mock location and item
		err = database.AddLocation("Kitchen")
		if err != nil {
			log.Fatal(err)
		}
		err = database.AddItem("Apples", 5, 1) // Assuming location ID 1
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Mock data inserted: Kitchen with 5 Apples")
	}

	tmpl = template.Must(template.ParseGlob("templates/*.html"))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/add-location", addLocationHandler)
	http.HandleFunc("/edit-location/", editLocationHandler)
	http.HandleFunc("/delete-location/", deleteLocationHandler)
	http.HandleFunc("/add-item/", addItemHandler)
	http.HandleFunc("/edit-item/", editItemHandler)
	http.HandleFunc("/delete-item/", deleteItemHandler)
	http.HandleFunc("/increase-item/", increaseItemHandler)
	http.HandleFunc("/decrease-item/", decreaseItemHandler)
	http.HandleFunc("/location-history/", locationHistoryHandler)
	http.HandleFunc("/export.toml", exportHandler)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	searchQuery := r.URL.Query().Get("search")
	var data PageData
	if searchQuery != "" {
		results, err := database.SearchItems(searchQuery)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data = PageData{
			SearchResults: results,
			SearchQuery:   searchQuery,
			IsSearch:      true,
		}
	} else {
		locationsWithItems, err := database.GetAllLocationsWithItems()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data = PageData{
			LocationsWithItems: locationsWithItems,
			IsSearch:           false,
		}
	}
	tmpl.ExecuteTemplate(w, "index.html", data)
}

func addLocationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tmpl.ExecuteTemplate(w, "add_location.html", nil)
	} else if r.Method == http.MethodPost {
		name := r.FormValue("name")
		if name == "" {
			http.Error(w, "Name required", http.StatusBadRequest)
			return
		}
		err := database.AddLocation(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func editLocationHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/edit-location/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		loc, err := database.GetLocation(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		tmpl.ExecuteTemplate(w, "edit_location.html", loc)
	} else if r.Method == http.MethodPost {
		name := r.FormValue("name")
		if name == "" {
			http.Error(w, "Name required", http.StatusBadRequest)
			return
		}
		err := database.RenameLocation(id, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func deleteLocationHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/delete-location/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = database.DeleteLocation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func addItemHandler(w http.ResponseWriter, r *http.Request) {
	locIDStr := strings.TrimPrefix(r.URL.Path, "/add-item/")
	locID, err := strconv.Atoi(locIDStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		loc, err := database.GetLocation(locID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		data := struct {
			LocationID   int
			LocationName string
		}{
			LocationID:   loc.ID,
			LocationName: loc.Name,
		}
		tmpl.ExecuteTemplate(w, "add_item.html", data)
	} else if r.Method == http.MethodPost {
		name := r.FormValue("name")
		countStr := r.FormValue("count")
		count, err := strconv.Atoi(countStr)
		if err != nil || name == "" {
			http.Error(w, "Invalid input", http.StatusBadRequest)
			return
		}
		err = database.AddItem(name, count, locID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func editItemHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/edit-item/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		item, err := database.GetItem(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		tmpl.ExecuteTemplate(w, "edit_item.html", item)
	} else if r.Method == http.MethodPost {
		name := r.FormValue("name")
		countStr := r.FormValue("count")
		count, err := strconv.Atoi(countStr)
		if err != nil || name == "" {
			http.Error(w, "Invalid input", http.StatusBadRequest)
			return
		}
		err = database.UpdateItem(id, name, count)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func deleteItemHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/delete-item/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = database.DeleteItem(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func increaseItemHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/increase-item/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = database.IncreaseItemCount(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func decreaseItemHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/decrease-item/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = database.DecreaseItemCount(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func locationHistoryHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/location-history/")
	locID, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	loc, err := database.GetLocation(locID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	items, err := database.GetAllItemsForLocation(locID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		Location *db.Location
		Items    []db.Item
	}{
		Location: loc,
		Items:    items,
	}
	tmpl.ExecuteTemplate(w, "location_history.html", data)
}

func exportHandler(w http.ResponseWriter, r *http.Request) {
	data, err := database.GetAllLocationsWithItems()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/toml")
	w.Header().Set("Content-Disposition", "attachment; filename=\"inventory.toml\"")
	err = toml.NewEncoder(w).Encode(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}