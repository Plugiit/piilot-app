package usecase

// Colonnes de l'import et de l'export CSV, une definition par entite.
//
// C'est la source unique : l'export ecrit ces en-tetes, l'import les
// reconnait (ou l'un de leurs alias). Ce qu'on exporte se reimporte donc sans
// correspondance a refaire.

// Entites que l'on peut importer et exporter. Ce sont les valeurs de
// imports.entity.
const (
	DataEntityClients     = "clients"
	DataEntityContacts    = "contacts"
	DataEntityProjects    = "projects"
	DataEntityTimeEntries = "time_entries"
)

// DataEntities liste les entites, dans l'ordre d'import conseille : un
// contact, un projet ou une saisie designent un client.
var DataEntities = []string{
	DataEntityClients,
	DataEntityContacts,
	DataEntityProjects,
	DataEntityTimeEntries,
}

// Field est une colonne du fichier.
type Field struct {
	// Key est l'identifiant stable du champ : celui du mapping stocke, du
	// code et de l'API. Il ne change jamais, contrairement a l'en-tete.
	Key string
	// Header est l'en-tete ecrit a l'export.
	Header string
	// Aliases sont d'autres en-tetes reconnus a l'import (noms usuels, anglais,
	// exports d'autres outils). La comparaison ignore casse, accents et
	// ponctuation : inutile de lister les variantes typographiques.
	Aliases []string
	// Required : sans cette colonne, la ligne ne peut pas etre importee.
	Required bool
}

// Cles communes a toutes les entites.
const (
	FieldKeyID       = "id"
	FieldKeySourceID = "source_id"
)

var (
	idField = Field{
		Key: FieldKeyID, Header: "Identifiant Piilot",
		Aliases: []string{"id", "uuid", "piilot id"},
	}
	sourceIDField = Field{
		Key: FieldKeySourceID, Header: "Identifiant d'origine",
		Aliases: []string{"id origine", "source id", "external id", "id externe", "reference externe"},
	}
)

var dataSpecs = map[string][]Field{
	DataEntityClients: {
		idField,
		sourceIDField,
		{Key: "name", Header: "Nom", Aliases: []string{"client", "nom du client", "name", "societe", "entreprise"}, Required: true},
		{Key: "kind", Header: "Type", Aliases: []string{"kind", "type de client"}},
		{Key: "legal_name", Header: "Raison sociale", Aliases: []string{"denomination", "legal name"}},
		{Key: "legal_form", Header: "Forme juridique", Aliases: []string{"legal form", "forme"}},
		{Key: "siret", Header: "SIRET", Aliases: []string{"n siret", "numero siret"}},
		{Key: "vat_number", Header: "N° de TVA", Aliases: []string{"tva", "numero de tva", "tva intracommunautaire", "vat", "vat number"}},
		{Key: "status", Header: "Statut", Aliases: []string{"status", "etat"}},
		{Key: "website", Header: "Site web", Aliases: []string{"site", "url", "website"}},
		{Key: "phone", Header: "Téléphone", Aliases: []string{"tel", "telephone", "phone"}},
		{Key: "address", Header: "Adresse", Aliases: []string{"rue", "address"}},
		{Key: "postal_code", Header: "Code postal", Aliases: []string{"cp", "postal code", "zip"}},
		{Key: "city", Header: "Ville", Aliases: []string{"city", "commune"}},
		{Key: "country", Header: "Pays", Aliases: []string{"country"}},
	},
	DataEntityContacts: {
		idField,
		sourceIDField,
		{Key: "firstname", Header: "Prénom", Aliases: []string{"prenom", "first name", "firstname"}},
		{Key: "lastname", Header: "Nom", Aliases: []string{"nom de famille", "last name", "lastname"}},
		{Key: "role", Header: "Fonction", Aliases: []string{"poste", "role", "titre", "job title"}},
		{Key: "email", Header: "E-mail", Aliases: []string{"email", "mail", "courriel", "adresse e mail"}},
		{Key: "phone", Header: "Téléphone", Aliases: []string{"tel", "telephone", "mobile", "phone"}},
		{Key: "client", Header: "Client", Aliases: []string{"societe", "entreprise", "nom du client", "company"}},
		{Key: "client_siret", Header: "SIRET client", Aliases: []string{"siret", "siret du client"}},
	},
	DataEntityProjects: {
		idField,
		sourceIDField,
		{Key: "name", Header: "Nom", Aliases: []string{"projet", "nom du projet", "name", "titre"}, Required: true},
		{Key: "client", Header: "Client", Aliases: []string{"nom du client", "societe", "entreprise", "company"}, Required: true},
		{Key: "client_siret", Header: "SIRET client", Aliases: []string{"siret", "siret du client"}},
		{Key: "status", Header: "Statut", Aliases: []string{"status", "etat"}},
		{Key: "priority", Header: "Priorité", Aliases: []string{"priorite", "priority"}},
		{Key: "description", Header: "Description", Aliases: []string{"details", "notes"}},
		{Key: "starts_on", Header: "Début", Aliases: []string{"debut", "date de debut", "starts on", "start"}},
		{Key: "due_on", Header: "Échéance", Aliases: []string{"echeance", "date de fin", "fin", "livraison", "due on", "due"}},
		{Key: "hours_sold", Header: "Heures vendues", Aliases: []string{"heures", "heures vendues h", "hours sold", "hours"}},
	},
	DataEntityTimeEntries: {
		idField,
		sourceIDField,
		{Key: "email", Header: "E-mail", Aliases: []string{"email", "mail", "courriel", "personne", "utilisateur"}, Required: true},
		{Key: "client", Header: "Client", Aliases: []string{"nom du client", "societe", "customer"}},
		{Key: "project", Header: "Projet", Aliases: []string{"nom du projet", "project"}, Required: true},
		{Key: "service", Header: "Service", Aliases: []string{"prestation", "activite", "activity"}},
		{Key: "date", Header: "Date", Aliases: []string{"jour", "date de saisie", "spent on"}, Required: true},
		{Key: "minutes", Header: "Durée (min)", Aliases: []string{"duree", "minutes", "temps", "duration"}, Required: true},
		{Key: "note", Header: "Note", Aliases: []string{"notes", "commentaire", "description"}},
	},
}

// DataSpec rend les colonnes d'une entite, dans l'ordre d'ecriture de
// l'export. La liste rendue est une copie : l'appelant peut la modifier.
// Le second retour est faux pour une entite inconnue.
func DataSpec(entity string) ([]Field, bool) {
	fields, ok := dataSpecs[entity]
	if !ok {
		return nil, false
	}

	out := make([]Field, len(fields))
	copy(out, fields)

	return out, true
}

// DataHeaders rend les en-tetes de l'export d'une entite.
func DataHeaders(entity string) []string {
	fields, _ := DataSpec(entity)

	headers := make([]string, len(fields))
	for i, f := range fields {
		headers[i] = f.Header
	}

	return headers
}
