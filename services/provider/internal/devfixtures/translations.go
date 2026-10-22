package devfixtures

// Demo translations are authored fixture content, not runtime machine translation.
// Enrichment never replaces a merchant's existing translations or source name.
var demoNames = map[string][2]string{
	"Döner und Yufka":           {"Döner and wraps", "دونر ولفائف"},
	"Vegetarisch":               {"Vegetarian", "أطباق نباتية"},
	"Pizza":                     {"Pizza", "بيتزا"},
	"Pide und Lahmacun":         {"Pide and lahmacun", "بيدا ولحم بعجين"},
	"Tellergerichte":            {"Platters", "أطباق رئيسية"},
	"Salate und Beilagen":       {"Salads and sides", "سلطات وأطباق جانبية"},
	"Getränke und Dessert":      {"Drinks and desserts", "مشروبات وحلويات"},
	"Döner im Fladenbrot":       {"Döner in flatbread", "دونر في خبز مسطح"},
	"Yufka":                     {"Yufka wrap", "لفافة يوفكا"},
	"Dönerbox":                  {"Döner box", "علبة دونر"},
	"Hähnchen-Döner":            {"Chicken döner", "دونر دجاج"},
	"Döner mit Weichkäse":       {"Döner with white cheese", "دونر بالجبن الأبيض"},
	"Yufka mit Weichkäse":       {"Yufka with white cheese", "يوفكا بالجبن الأبيض"},
	"Falafel im Fladenbrot":     {"Falafel in flatbread", "فلافل في خبز مسطح"},
	"Falafel-Yufka":             {"Falafel wrap", "لفافة فلافل"},
	"Halloumi im Fladenbrot":    {"Halloumi in flatbread", "حلوم في خبز مسطح"},
	"Gemüse-Yufka":              {"Vegetable wrap", "لفافة خضار"},
	"Salat im Fladenbrot":       {"Salad in flatbread", "سلطة في خبز مسطح"},
	"Falafelbox":                {"Falafel box", "علبة فلافل"},
	"Dönerteller":               {"Döner platter", "طبق دونر"},
	"Hähnchenteller":            {"Chicken platter", "طبق دجاج"},
	"Falafelteller":             {"Falafel platter", "طبق فلافل"},
	"Halloumiteller":            {"Halloumi platter", "طبق حلوم"},
	"Gemischter Teller":         {"Mixed platter", "طبق مشكل"},
	"Gemüseteller":              {"Vegetable platter", "طبق خضار"},
	"Pizza Margherita":          {"Margherita pizza", "بيتزا مارغريتا"},
	"Pizza Salami":              {"Salami pizza", "بيتزا سلامي"},
	"Pizza Funghi":              {"Mushroom pizza", "بيتزا فطر"},
	"Pizza Tonno":               {"Tuna pizza", "بيتزا تونة"},
	"Pizza Vegetarisch":         {"Vegetarian pizza", "بيتزا خضار"},
	"Pizza Döner":               {"Döner pizza", "بيتزا دونر"},
	"Pizza Spinat":              {"Spinach pizza", "بيتزا سبانخ"},
	"Pizza Hawaii":              {"Hawaiian pizza", "بيتزا هاواي"},
	"Pizza Sucuk":               {"Sucuk pizza", "بيتزا سجق"},
	"Pizza Diavolo":             {"Diavolo pizza", "بيتزا ديافولو"},
	"Pizza Käse":                {"Cheese pizza", "بيتزا جبن"},
	"Pizza Oliven":              {"Olive pizza", "بيتزا زيتون"},
	"Pide mit Käse":             {"Cheese pide", "بيدا بالجبن"},
	"Pide mit Spinat":           {"Spinach pide", "بيدا بالسبانخ"},
	"Pide mit Sucuk":            {"Sucuk pide", "بيدا بالسجق"},
	"Lahmacun":                  {"Lahmacun", "لحم بعجين"},
	"Lahmacun mit Dönerfleisch": {"Lahmacun with döner meat", "لحم بعجين مع لحم الدونر"},
	"Pide mit Dönerfleisch":     {"Döner pide", "بيدا بالدونر"},
	"Pommes":                    {"Fries", "بطاطس مقلية"},
	"Reis":                      {"Rice", "أرز"},
	"Gemischter Salat":          {"Mixed salad", "سلطة مشكلة"},
	"Bauernsalat":               {"Farmer's salad", "سلطة ريفية"},
	"Thunfischsalat":            {"Tuna salad", "سلطة تونة"},
	"Falafelsalat":              {"Falafel salad", "سلطة فلافل"},
	"Wasser 0,5 l":              {"Water 0.5 l", "ماء 0.5 لتر"},
	"Cola 0,33 l":               {"Cola 0.33 l", "كولا 0.33 لتر"},
	"Ayran 0,25 l":              {"Ayran 0.25 l", "عيران 0.25 لتر"},
	"Zitronenlimonade 0,33 l":   {"Lemonade 0.33 l", "ليمونادة 0.33 لتر"},
	"Baklava":                   {"Baklava", "بقلاوة"},
	"Milchreis":                 {"Rice pudding", "أرز بالحليب"},
	"Normal":                    {"Regular", "عادي"}, "Groß": {"Large", "كبير"},
	"Klein (26 cm)": {"Small (26 cm)", "صغير (26 سم)"}, "Groß (32 cm)": {"Large (32 cm)", "كبير (32 سم)"},
	"Soße wählen": {"Choose a sauce", "اختر الصلصة"}, "Knoblauch": {"Garlic", "ثوم"},
	"Kräuter": {"Herbs", "أعشاب"}, "Scharf": {"Hot sauce", "صلصة حارة"}, "Ohne Soße": {"No sauce", "بدون صلصة"},
	"Extras": {"Extras", "إضافات"}, "Extra Käse": {"Extra cheese", "جبن إضافي"}, "Extra Fleisch": {"Extra meat", "لحم إضافي"},
	"Extra Chili": {"Extra chilli", "فلفل حار إضافي"}, "Champignons": {"Mushrooms", "فطر"},
	"Beilage wählen": {"Choose a side", "اختر طبقًا جانبيًا"}, "Salat": {"Salad", "سلطة"},
}

func TranslateDemoNames(value any) bool {
	changed := false
	switch v := value.(type) {
	case map[string]any:
		if v["id"] == "sauce" && v["name"] == "Soße wählen" && v["default_option_ids"] == nil {
			v["default_option_ids"] = []string{"garlic"}
			changed = true
		}
		if name, ok := v["name"].(string); ok && v["translations"] == nil {
			if names, found := demoNames[name]; found {
				v["translations"] = map[string]string{"de": name, "en": names[0], "ar": names[1]}
				changed = true
			}
		}
		for key, child := range v {
			if key != "translations" && TranslateDemoNames(child) {
				changed = true
			}
		}
	case []any:
		for _, child := range v {
			if TranslateDemoNames(child) {
				changed = true
			}
		}
	}
	return changed
}
