package store

// App types proposés à la création (stack légère).
var AppTypesCurrent = []string{"static", "php", "bludit", "hugo", "codeigniter"}

// AppTypesLegacy : sites déjà créés avant le changement de catalogue.
var AppTypesLegacy = []string{"wordpress", "laravel", "prestashop"}

func ValidAppType(t string) bool {
	for _, a := range AppTypesCurrent {
		if a == t {
			return true
		}
	}
	return false
}

func AllAppTypesSQL() string {
	return `'static','php','bludit','hugo','codeigniter','wordpress','laravel','prestashop'`
}
