package devfixtures

// AddDemoMeals enriches only unmodified fictional products, once. The marker
// prevents a later seed from undoing deliberate merchant removals or edits.
func AddDemoMeals(doc map[string]any) bool {
	if _, exists := doc["meal_deals"]; exists {
		return false
	}
	_, products, err := Restaurant()
	if err != nil {
		return false
	}
	expected := map[string]Product{}
	for _, p := range products {
		expected[p.ID] = p
	}
	items := map[string]map[string]any{}
	for _, v := range doc["items"].([]any) {
		i := v.(map[string]any)
		id, _ := i["id"].(string)
		if p, ok := expected[id]; ok && i["name"] == p.Name {
			items[id] = i
		}
	}
	choices := func(ids []string) []any {
		out := []any{}
		for _, id := range ids {
			item := items[id]
			if item == nil {
				continue
			}
			eligible := true
			for _, g := range item["option_groups"].([]any) {
				if g.(map[string]any)["min_selected"].(float64) > 0 {
					eligible = false
				}
			}
			if !eligible {
				continue
			}
			for _, v := range item["variants"].([]any) {
				if v.(map[string]any)["id"] == "regular" {
					out = append(out, map[string]any{"item_id": id, "variant_id": "regular"})
				}
			}
		}
		return out
	}
	doner, pizza := []string{}, []string{}
	for _, p := range products {
		if i := items[p.ID]; i != nil {
			if p.Section == "Döner und Yufka" {
				doner = append(doner, p.ID)
			}
			if p.Kind == "pizza" {
				pizza = append(pizza, p.ID)
			}
			for _, g := range i["option_groups"].([]any) {
				for _, o := range g.(map[string]any)["options"].([]any) {
					option := o.(map[string]any)
					if option["id"] != "cheese" || option["variant_prices"] != nil {
						continue
					}
					for _, v := range i["variants"].([]any) {
						if v.(map[string]any)["id"] == "large" {
							option["variant_prices"] = map[string]int{"large": 150}
						}
					}
				}
			}
		}
	}
	deals := []any{}
	if addons := choices([]string{"water", "cola", "ayran", "lemonade"}); len(doner) > 0 && len(addons) > 0 {
		deals = append(deals, map[string]any{"id": "doner-menu", "name": "Döner-Menü", "translations": map[string]string{"de": "Döner-Menü", "en": "Döner meal", "ar": "وجبة دونر"}, "main_item_ids": doner, "surcharge_minor": 100, "addon_choices": addons})
	}
	if addons := choices([]string{"mixed-salad", "farmers-salad"}); len(pizza) > 0 && len(addons) > 0 {
		deals = append(deals, map[string]any{"id": "healthy-pizza", "name": "Healthy Pizza Menü", "translations": map[string]string{"de": "Healthy Pizza Menü", "en": "Healthy Pizza meal", "ar": "وجبة بيتزا مع سلطة"}, "main_item_ids": pizza, "surcharge_minor": 200, "addon_choices": addons})
	}
	doc["meal_deals"] = deals
	return true
}
