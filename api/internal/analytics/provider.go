// Package analytics lit un service de mesure d'audience (Umami pour
// commencer) : les sites, leurs chiffres sur une periode, leurs courbes,
// leurs classements, leurs visiteurs du moment.
//
// Le service garde l'historique : Piilot n'en recopie que les vues qu'il
// affiche, dans un cache local que remplit une tache de fond. Ce paquet ne
// fait que parler au service, dans un vocabulaire qui ne depend pas de lui —
// les dimensions, les periodes et les chiffres sont neutres, chaque
// fournisseur les traduit dans ses propres noms.
package analytics

import (
	"context"
	"errors"
	"time"
)

// Range est une periode, bornes comprises.
type Range struct {
	Start time.Time
	End   time.Time
}

// Unit est le pas d'une courbe.
type Unit string

const (
	Hour  Unit = "hour"
	Day   Unit = "day"
	Month Unit = "month"
)

// Dimension est un axe de classement.
type Dimension string

const (
	Page     Dimension = "page"
	Referrer Dimension = "referrer"
	Browser  Dimension = "browser"
	OS       Dimension = "os"
	Device   Dimension = "device"
	Country  Dimension = "country"
	Region   Dimension = "region"
	City     Dimension = "city"
	Language Dimension = "language"
	Screen   Dimension = "screen"
	Event    Dimension = "event"
)

// Dimensions liste les axes, dans l'ordre ou les ecrans les montrent.
var Dimensions = []Dimension{Page, Referrer, Country, Region, City, Device, Browser, OS, Language, Screen, Event}

// Website est un site mesure.
type Website struct {
	ID     string
	Name   string
	Domain string
}

// Totals sont les chiffres d'une periode. TotalTime est en secondes.
type Totals struct {
	Visitors  int64
	Visits    int64
	Pageviews int64
	Bounces   int64
	TotalTime int64
}

// Comparison porte la periode et celle qui la precede, de meme duree.
type Comparison struct {
	Current  Totals
	Previous Totals
}

// Point est un pas de courbe. At est l'heure locale du pas, dans le fuseau
// demande, sans decalage : « 2026-10-12T00:00:00 ».
type Point struct {
	At        string
	Pageviews int64
	Visitors  int64
}

// MetricRow est une ligne de classement.
type MetricRow struct {
	Value string
	Count int64
}

// Provider est un service de mesure d'audience. Umami en est la premiere
// implementation.
type Provider interface {
	ListWebsites(ctx context.Context) ([]Website, error)
	Stats(ctx context.Context, websiteID string, r Range) (Comparison, error)
	Timeseries(ctx context.Context, websiteID string, r Range, unit Unit, timezone string) ([]Point, error)
	Metrics(ctx context.Context, websiteID string, r Range, dim Dimension, limit int) ([]MetricRow, error)
	Active(ctx context.Context, websiteID string) (int64, error)
}

var (
	// ErrUnauthorized : le service refuse les identifiants.
	ErrUnauthorized = errors.New("identifiants refusés par le service")
	// ErrNotFound : le site n'existe plus chez le service.
	ErrNotFound = errors.New("site introuvable chez le service")
)
