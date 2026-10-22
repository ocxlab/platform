// Package devfixtures contains fictional, reusable development acceptance data.
package devfixtures

import (
	_ "embed"
	"encoding/json"
)

//go:embed restaurant.json
var restaurant []byte

type Product struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Section string `json:"section"`
	Price   int64  `json:"price_minor"`
	Kind    string `json:"kind"`
}

type Variant struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int64  `json:"price_minor"`
}
type OptionGroup struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Min     int       `json:"min_selected"`
	Max     int       `json:"max_selected"`
	Options []Variant `json:"options"`
}
type Item struct {
	ImageID   string        `json:"image_id,omitempty"`
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Available bool          `json:"available"`
	Variants  []Variant     `json:"variants"`
	Groups    []OptionGroup `json:"option_groups"`
}
type Menu struct {
	Currency     string `json:"currency"`
	Availability string `json:"availability_mode"`
	Items        []Item `json:"items"`
}

// Restaurant keeps intended categories in the fixture source, not fake prefixes
// in product names. The current v1 menu has no category field; that is a real gap.
func Restaurant() (Menu, []Product, error) {
	var products []Product
	if err := json.Unmarshal(restaurant, &products); err != nil {
		return Menu{}, nil, err
	}
	menu := Menu{Currency: "EUR", Availability: "manual", Items: []Item{}}
	for _, product := range products {
		item := Item{ID: product.ID, Name: product.Name, Available: product.ID != "baklava", Variants: []Variant{{"regular", "Normal", product.Price}}, Groups: []OptionGroup{}}
		switch product.Kind {
		case "kebab":
			item.Variants = append(item.Variants, Variant{"large", "Groß", product.Price + 200})
			item.Groups = []OptionGroup{
				{"sauce", "Soße wählen", 1, 1, []Variant{{"garlic", "Knoblauch", 0}, {"herbs", "Kräuter", 0}, {"hot", "Scharf", 0}, {"none", "Ohne Soße", 0}}},
				{"extras", "Extras", 0, 3, []Variant{{"cheese", "Extra Käse", 100}, {"meat", "Extra Fleisch", 250}, {"chilli", "Extra Chili", 0}}},
			}
			if product.Section == "Vegetarisch" {
				item.Groups[1].Options = []Variant{{"cheese", "Extra Käse", 100}, {"chilli", "Extra Chili", 0}}
				item.Groups[1].Max = 2
			}
		case "pizza":
			item.Variants = []Variant{{"small", "Klein (26 cm)", product.Price}, {"large", "Groß (32 cm)", product.Price + 250}}
			item.Groups = []OptionGroup{{"extras", "Extras", 0, 3, []Variant{{"cheese", "Extra Käse", 100}, {"mushrooms", "Champignons", 150}, {"garlic", "Knoblauch", 0}}}}
		case "plate":
			item.Groups = []OptionGroup{{"side", "Beilage wählen", 1, 1, []Variant{{"fries", "Pommes", 0}, {"rice", "Reis", 0}, {"salad", "Salat", 0}}}}
		}
		menu.Items = append(menu.Items, item)
	}
	return menu, products, nil
}
