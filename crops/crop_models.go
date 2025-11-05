package crops

import "gorm.io/gorm"

type Crop struct {
	gorm.Model
	RequiresWater     bool
	Name              string
	Stage             CropLifeCycleStage
	Lifespan          CropLifespan
	Family            CropFamily
	Group             CropGroup
	GrowthEnvironment GrowthEnvironment
	MarketType        MarketType
	Type              CropType
}

type CropLifeCycleStage string

const (
	Seed        = "SEED"
	Germination = "GERMINATION"
	Seedling    = "SEEDLING"
	Veg         = "VEG"
	Flowering   = "FLOWERING"
	Polination  = "POLLINATION"
)

type CropLifespan string

const (
	Annuals    = "Annuals"
	Biennials  = "Biennials"
	Perennials = "Perennials"
)

type CropFamily string

const (
	Grasses     = "Grasses"
	Legumes     = "Legumes"
	Nightshades = "Nightshades"
	Brassicas   = "Brassicas"
	Cucurbits   = "Curcurbits"
	Umbelifers  = "Umbelifers"
)

type CropGroup string

const (
	Food       = "Food"
	Feed       = "Feed"
	Fiber      = "Fiber"
	Oil        = "Oil"
	Industrial = "Industrial"
	Medicinal  = "Medicinal"
)

type GrowthEnvironment string

const (
	Field      = "Field"
	Greenhouse = "Greenhouse"
	Orchard    = "Orchard"
	Vineyard   = "Vineyard"
	Hydroponic = "Hydroponic"
)

type MarketType string

const (
	Cash        = "Cash"
	Subsistence = "Subsistence"
	Cover       = "Cover"
	Rotation    = "Rotation"
)

type CropType string

const (
	Veggies = "Veggies"
	Fruit   = "Fruit"
	Root    = "Root"
	Grain   = "Grain"
)
